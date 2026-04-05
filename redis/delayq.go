package redis

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"go.opencensus.io/trace"

	"github.com/spy16/clockwork/pkg/telemetry"
)

const reclaimTTLBuffer = 5 * time.Second

// NewDelayQ returns a new delay-queue instance with given queue name.
func NewDelayQ(queueName string, client redis.UniversalClient, opts ...QOptions) *DelayQ {
	var opt QOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if _, isClusterMode := client.(*redis.ClusterClient); !isClusterMode {
		// Disable sharding when using non-cluster Redis setup.
		opt.ReadShards = 1
		opt.WriteShards = 1
	}
	opt.setDefaults()

	return &DelayQ{
		client:      client,
		workers:     opt.Workers,
		pollInt:     opt.PollInterval,
		prefetch:    opt.PreFetchCount,
		queueName:   queueName,
		readShards:  opt.ReadShards,
		writeShards: opt.WriteShards,
		reclaimTTL:  opt.ReclaimInterval,
	}
}

// Process function is invoked for every item that becomes ready. An item
// remains on the queue until this function returns no error.
type Process func(ctx context.Context, value []byte) error

// QOptions represents queue-level configurations for the delay queue.
type QOptions struct {
	// ReadShards represents the number of splits to read from. This will
	// always be >= WriteShards.
	// Note: Only applicable when using Redis cluster.
	ReadShards int

	// WriteShards represents the number of splits made to the delay & un-ack
	// sets. This value must always be <= ReadShards. Defaults to 1.
	// Note: Only applicable when using Redis cluster.
	WriteShards int

	// Workers represents the number of threads to launch for fetching batch
	// and processing. A single worker fetches a batch and processes items in
	// the batch sequentially.
	Workers int

	// PollInterval decides the interval between each fetch from Redis.
	PollInterval time.Duration

	// PreFetchCount decides the number of items to fetch in a single fetch
	// call. Once fetched, entire batch is processed in-memory. Entire batch
	// must complete within ReclaimInterval to ensure exactly-once semantics. If
	// this guarantee is breached, at-least-once semantics apply.
	PreFetchCount int

	// ReclaimInterval decides the time required to process an entire batch.
	// If this is lesser than the actual time required, there may be some more-
	// than-once processing.
	ReclaimInterval time.Duration
}

func (opt *QOptions) setDefaults() {
	if opt.Workers == 0 {
		opt.Workers = 10
	}
	if opt.PollInterval == 0 {
		opt.PollInterval = 500 * time.Millisecond
	}
	if opt.PreFetchCount == 0 {
		opt.PreFetchCount = 20
	}
	if opt.ReclaimInterval == 0 {
		opt.ReclaimInterval = 3 * time.Minute
	}
	if opt.WriteShards == 0 {
		opt.WriteShards = 1
	}
	if opt.ReadShards < opt.WriteShards {
		opt.ReadShards = opt.WriteShards
	}
}

// DelayQ represents a distributed, reliable delay-queue backed by Redis.
type DelayQ struct {
	client      redis.UniversalClient
	pollInt     time.Duration
	workers     int
	prefetch    int
	queueName   string
	reclaimTTL  time.Duration
	readShards  int
	writeShards int
}

// Delay pushes an item onto the delay-queue with the given delay. Value must
// be unique. Duplicate values will be ignored.
func (dq *DelayQ) Delay(ctx context.Context, at time.Time, value []byte) (string, error) {
	delaySet := dq.getSetName(true)

	metric := telemetry.Incr("delayq_enqueue", 1).Tag("delay_set", delaySet)
	defer metric.Publish()

	_, err := dq.client.ZAdd(ctx, delaySet, &redis.Z{
		Score:  float64(at.Unix()),
		Member: value,
	}).Result()
	return delaySet, err
}

// Run runs the worker loop that invoke fn whenever a delayed value is ready.
// It blocks until all worker goroutines exit due to some critical error or
// until context is cancelled, whichever happens first.
func (dq *DelayQ) Run(ctx context.Context, fn Process) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = log.Logger.WithContext(ctx)

	wg := &sync.WaitGroup{}
	for i := 0; i < dq.workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			if err := dq.reaper(ctx, fn); err != nil && !errors.Is(err, context.Canceled) {
				log.Ctx(ctx).Error().Err(err).Msgf("reaper-%d died", id)
			} else {
				log.Ctx(ctx).Info().Msgf("reaper-%d exited gracefully", id)
			}
		}(i)
	}
	wg.Wait() // wait for workers to exit.

	return nil
}

func (dq *DelayQ) reaper(ctx context.Context, fn Process) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("panic: %v", v)
		}
	}()

	pollTimer := time.NewTimer(0)
	defer pollTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-pollTimer.C:
			pollTimer.Reset(dq.pollInt)

			if err := dq.reap(ctx, fn); err != nil {
				log.Printf("reap error: %v", err)
				continue
			}
		}
	}
}

// reap fetches ready items from the set, applies 'fn' to each item, acknowledges
// each item and returns nil if everything went fine.
// Ready-items are those that have their score in range [-inf, now()].
func (dq *DelayQ) reap(ctx context.Context, fn Process) error {
	delaySet := dq.getSetName(false)

	metric := telemetry.Incr("delayq_reap", 1).Tag("delay_set", delaySet)
	defer metric.Publish()

	ctx, span := trace.StartSpan(ctx, "DelayQ_Reap")
	defer span.End()

	// add some buffer as a safety measure to prevent reclaiming too soon.
	reclaimTTL := (dq.reclaimTTL + reclaimTTLBuffer).Seconds()

	list, err := dq.zRangeIncrBy(ctx, delaySet, reclaimTTL, dq.prefetch)
	if err != nil {
		metric.Status("failure").Tag("cause", "zrangeincrby_failed")
		return err
	}

	// entire batch of items is subjected to the same TTL. So we create
	// a single context with timeout. Note that the context timeout does
	// not include the reclaim-ttl buffer.
	ctx, cancel := context.WithTimeout(ctx, dq.reclaimTTL)
	defer cancel()

	if err := dq.processAll(ctx, delaySet, fn, list); err != nil {
		metric.Status("failure").Tag("cause", "process_failed")
		return err
	}

	metric.Status("success")
	return nil
}

func (dq *DelayQ) processAll(ctx context.Context, delaySet string, fn Process, items []any) error {
	metric := telemetry.Incr("delayq_process", float64(len(items)))
	defer metric.Publish()

	for _, v := range items {
		select {
		case <-ctx.Done():
			metric.Status("discarded").Tag("cause", "context_cancelled")
			return ctx.Err()

		default:
			item, isStr := v.(string)
			if !isStr {
				panic(fmt.Errorf("expecting string, got %s", reflect.TypeOf(v)))
			}

			failure := false

			fnStarted := time.Now()
			fnErr := fn(ctx, []byte(item))
			telemetry.Timing("delayq_invoke_fn", time.Since(fnStarted), 1).Publish()

			if fnErr != nil {
				failure = true
				log.Ctx(ctx).Warn().Err(fnErr).Msg("fn failed to process item, will requeue")
			}

			// ack/nAck the item. ignoring the error is okay since if the ack failed,
			// the item remains in the un-ack set and will be claimed by the reclaimer.
			if err := dq.ack(ctx, delaySet, item, failure); err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("failed to ack item")
			}
		}
	}

	return nil
}

// ack acknowledges the item in the delay-set. If it is a positive acknowledgement,
// the item is removed from the set. If it is negative, the score of the item is
// reset to 0 so that some other worker picks up the job right away.
func (dq *DelayQ) ack(ctx context.Context, delaySet, item string, negative bool) error {
	metric := telemetry.Incr("delayq_ack", 1).Tag("delay_set", delaySet)
	defer metric.Publish()

	var err error
	var count int64

	if negative {
		metric.Tag("negative", "true")
		count, err = dq.client.ZAddXXCh(ctx, delaySet, &redis.Z{
			Score:  0,
			Member: item,
		}).Result()
	} else {
		metric.Tag("negative", "false")
		count, err = dq.client.ZRem(ctx, delaySet, item).Result()
	}

	if err != nil || count != 1 {
		metric.Status("failed")
		log.Ctx(ctx).Warn().
			Err(err).
			Int64("result_count", count).
			Msg("failed to ack item")
		return err
	}

	metric.Status("success")
	return nil
}

// getSetName returns the sharded set-name for reading-from or writing-to.
func (dq *DelayQ) getSetName(forEnqueue bool) string {
	var shardID int

	if forEnqueue {
		shardID = rand.Intn(dq.writeShards)
	} else {
		shardID = rand.Intn(dq.readShards)
	}

	// Refer https://redis.io/topics/cluster-spec for understanding reason for using '{}'
	// in the following keys.
	if shardID == 0 {
		// This ensures the polling is backward compatible.
		return fmt.Sprintf("delay_set:{%s}", dq.queueName)
	} else {
		return fmt.Sprintf("delay_set:{%s-%d}", dq.queueName, shardID)
	}
}

func (dq *DelayQ) zRangeIncrBy(ctx context.Context, fromSet string, scoreDelta float64, batchSz int) ([]any, error) {
	ctx, span := trace.StartSpan(ctx, "DelayQ_zRangeIncrBy")
	defer span.End()
	span.AddAttributes(
		trace.StringAttribute("set", fromSet),
		trace.Float64Attribute("score_delta", scoreDelta),
		trace.Int64Attribute("batch_size", int64(batchSz)),
	)

	now := time.Now().Unix()
	newScore := float64(now) + scoreDelta

	l := log.Ctx(ctx).With().
		Str("set", fromSet).
		Float64("new_score", newScore).
		Float64("score_delta", scoreDelta).
		Int("limit", batchSz).
		Int64("max_priority", now).
		Logger()

	// atomically fetch & increment score for ready items from the queue.
	// Any item that has score in range [-inf, now] is considered ready.
	// The score of ready-items is set to the newScore to move them out
	// of the above range so that other workers do not pick the same item
	// for some time.
	keys := []string{fromSet}
	args := []any{now, newScore, batchSz}
	items, err := zRangeSetScore.Run(ctx, dq.client, keys, args...).Result()
	if err != nil {
		l.Error().Err(err).Msg("lua script exec failed")
		return nil, err
	}

	list, ok := items.([]any)
	if !ok {
		panic(fmt.Errorf("expecting list, got %s", reflect.TypeOf(list)))
	}

	l.Debug().Msgf("fetched %d items", len(list))
	return list, nil
}

// zRangeSetScore performs ZRANGE on the set for a batch of ready-items,
// sets the priority of the ready items to given value and then returns
// those items.
var zRangeSetScore = redis.NewScript(`
local from_set = KEYS[1]
local max_priority, set_priority, limit = ARGV[1], ARGV[2] or 0.0, ARGV[3]

local items
if limit then
	items = redis.call('ZRANGE', from_set, '-inf', max_priority, 'BYSCORE', 'LIMIT', 0, limit)
else
	items = redis.call('ZRANGE', from_set, '-inf', max_priority, 'BYSCORE')
end

for i, value in ipairs(items) do
	redis.call('ZADD', from_set, 'XX', set_priority, value)
end

return items
`)
