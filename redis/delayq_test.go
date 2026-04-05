//go:build integration
// +build integration

package redis

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

// Note: These tests require actual Redis instance. Custom Redis address can be
// provided using the following environment variable.
const redisHostConfigVar = "DELAYQ_REDIS_HOST"

var redisClient redis.UniversalClient

func flushRedis() {
	_, _ = redisClient.FlushDB(context.Background()).Result()
}

func TestMain(m *testing.M) {
	log.Logger = zerolog.New(&zerolog.ConsoleWriter{
		Out: os.Stderr,
	}).With().Caller().Timestamp().Logger().Level(zerolog.WarnLevel)

	redisHost := strings.TrimSpace(os.Getenv(redisHostConfigVar))
	if redisHost == "" {
		redisHost = "localhost:6379"
	}

	if strings.Contains(redisHost, ",") {
		redisClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: strings.Split(redisHost, ","),
		})
	} else {
		redisClient = redis.NewClient(&redis.Options{
			Addr: redisHost,
		})
	}

	os.Exit(m.Run())
}

func TestDelayQ_Delay(t *testing.T) {
	t.Cleanup(flushRedis)

	frozenTime := time.Now().Add(1 * time.Hour)
	value := []byte("foo")

	dq := NewDelayQ(t.Name(), redisClient)

	setName, err := dq.Delay(context.Background(), frozenTime, value)
	assert.NotEmpty(t, setName)
	assert.NoError(t, err)

	items, err := redisClient.ZRange(context.Background(), setName, 0, 1000).Result()
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, string(value), items[0])
}

func TestDelayQ_Run(t *testing.T) {
	t.Cleanup(flushRedis)

	dq := NewDelayQ(t.Name(), redisClient)

	var err error
	deliverAt := time.Now().Add(-1 * time.Hour)
	_, err = dq.Delay(context.Background(), deliverAt, []byte("foo"))
	assert.NoError(t, err)
	_, err = dq.Delay(context.Background(), deliverAt, []byte("bar"))
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	counter := int64(0)
	go func() {
		err := dq.Run(ctx, func(ctx context.Context, value []byte) error {
			atomic.AddInt64(&counter, 1)
			return nil
		})
		assert.NoError(t, err)
	}()

	<-ctx.Done()
	assert.Equal(t, int64(2), atomic.LoadInt64(&counter))
}
