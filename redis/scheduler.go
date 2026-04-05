package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/pkg/telemetry"
	"github.com/spy16/clockwork/schedule"
)

var _ schedule.Scheduler = (*Scheduler)(nil)

func NewScheduler(client redis.UniversalClient, doneScheduleTTL time.Duration, opts ...QOptions) *Scheduler {
	return &Scheduler{
		client:  client,
		delayq:  NewDelayQ("clockwork", client, opts...),
		doneTTL: doneScheduleTTL,
	}
}

// Scheduler implements clockwork scheduler backend using Redis as the
// persistence layer.
type Scheduler struct {
	client  redis.UniversalClient
	delayq  *DelayQ
	doneTTL time.Duration
}

func (sched *Scheduler) List(ctx context.Context, offset, count int) ([]schedule.Schedule, error) {
	return nil, clockwork.ErrUnsupported
}

func (sched *Scheduler) Get(ctx context.Context, scheduleID string) (*schedule.Schedule, error) {
	val, err := sched.client.Get(ctx, scheduleKey(scheduleID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, clockwork.ErrNotFound
		}
		return nil, err
	}
	var sc schedule.Schedule
	if err := json.Unmarshal([]byte(val), &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

func (sched *Scheduler) Put(ctx context.Context, sc schedule.Schedule, isUpdate bool, requests ...schedule.Execution) error {
	var isOk bool
	var err error

	if isUpdate {
		isOk, err = sched.client.SetXX(ctx, scheduleKey(sc.ID), jsonBytes(sc), 0).Result()
	} else {
		isOk, err = sched.client.SetNX(ctx, scheduleKey(sc.ID), jsonBytes(sc), 0).Result()
	}

	if err != nil {
		return err
	} else if !isOk {
		if isUpdate {
			return clockwork.ErrNotFound.WithMsgf("schedule with given id not found")
		} else {
			return clockwork.ErrConflict.WithMsgf("schedule with given id already exists")
		}
	}

	return sched.enqueueAll(ctx, !isUpdate, sc.ID, requests)
}

func (sched *Scheduler) Del(ctx context.Context, scheduleID string) error {
	val, err := sched.client.Del(ctx, scheduleKey(scheduleID)).Result()
	if err != nil {
		return err
	} else if val == 0 {
		return clockwork.ErrNotFound
	}
	return nil
}

func (sched *Scheduler) Run(ctx context.Context, onReady schedule.OnReadyFunc) error {
	return sched.delayq.Run(ctx, func(ctx context.Context, value []byte) error {
		var item schedule.Execution
		if err := json.Unmarshal(value, &item); err != nil {
			log.Ctx(ctx).Error().Err(err).
				Msgf("failed to unmarshal data into executionRequest: '%s'", string(value))
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			return sched.handle(ctx, item, onReady)
		}
	})
}

func (sched *Scheduler) enqueueAll(ctx context.Context, rollbackOnFail bool,
	scheduleID string, requests []schedule.Execution) (err error) {
	entry := log.Ctx(ctx).With().Str("schedule_id", scheduleID).Logger()

	defer func() {
		if err != nil && rollbackOnFail {
			if delErr := sched.Del(ctx, scheduleID); delErr != nil {
				entry.Error().Err(delErr).Msg("failed to rollback")
			}
		}
	}()

	for _, req := range requests {
		if _, err = sched.delayq.Delay(ctx, req.EnqueueAt, jsonBytes(req)); err != nil {
			entry.Error().Err(err).Msg("failed to schedule next execution")
			return err
		}
	}
	return nil
}

func (sched *Scheduler) handle(ctx context.Context, curReq schedule.Execution, onReady schedule.OnReadyFunc) error {
	requestPicked := telemetry.Incr("redis_request_picked", 1)
	defer requestPicked.Publish()
	requestPicked.Status("success")

	sc, err := sched.Get(ctx, curReq.ScheduleID)
	if err != nil {
		if err == clockwork.ErrNotFound {
			requestPicked.Status("skip").Tag("cause", "schedule_deleted")
			return nil
		}
		return err
	}

	if sc.Version > curReq.Version {
		requestPicked.Status("skip").Tag("cause", "schedule_deleted")
		return nil
	}

	nextRequest, err := onReady(ctx, *sc, curReq)
	if err != nil {
		requestPicked.Status("failed").Tag("cause", "onready_failed")
		return err
	}

	if nextRequest == nil {
		if sc.Status != schedule.StatusDisabled {
			sc.Status = schedule.StatusDone
		}
		return sched.updateExisting(ctx, *sc)
	}

	sc.Status = schedule.StatusActive
	sc.EnqueueCount++
	sc.NextExecutionAt = nextRequest.EnqueueAt
	return sched.updateExisting(ctx, *sc, *nextRequest)
}

func (sched *Scheduler) updateExisting(ctx context.Context, sc schedule.Schedule, requests ...schedule.Execution) error {
	isOk, err := sched.client.SetXX(ctx, scheduleKey(sc.ID), jsonBytes(sc), sched.doneTTL).Result()
	if err != nil {
		return err
	} else if !isOk {
		return clockwork.ErrNotFound
	}
	for _, e := range requests {
		if _, err := sched.delayq.Delay(ctx, e.EnqueueAt, jsonBytes(e)); err != nil {
			log.Ctx(ctx).Error().Err(err).
				Str("schedule_id", sc.ID).
				Msg("failed to schedule next execution")
			return err
		}
	}
	return nil
}

func scheduleKey(id string) string {
	// WARNING: changing this key will cause lookups for ALL existing schedules in
	// clockwork deployments to fail.
	return fmt.Sprintf("clockwork:schedule:{%s}", id)
}

func jsonBytes(v any) []byte {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
