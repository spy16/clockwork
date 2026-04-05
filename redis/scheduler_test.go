//go:build integration
// +build integration

package redis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/schedule"
)

func TestScheduler_Get(t *testing.T) {
	sched := NewScheduler(redisClient, 0, QOptions{PollInterval: 1 * time.Second})
	require.NotNil(t, sched)
	require.NotNil(t, sched.client)

	t.Run("NonExistentSchedule", func(t *testing.T) {
		sc, err := sched.Get(context.Background(), "foo")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, sc)
	})

	t.Run("InvalidScheduleEntry", func(t *testing.T) {
		t.Cleanup(flushRedis)

		_, err := sched.client.Set(context.Background(), "clockwork:schedule:{foo}", "{", -1).Result()
		require.NoError(t, err)

		sc, err := sched.Get(context.Background(), "foo")
		assert.Error(t, err)
		assert.False(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, sc)
	})

	t.Run("ValidEntry", func(t *testing.T) {
		t.Cleanup(flushRedis)

		_, err := sched.client.Set(context.Background(), "clockwork:schedule:{foo}", `{"id":"foo"}`, -1).Result()
		require.NoError(t, err)

		sc, err := sched.Get(context.Background(), "foo")
		assert.NoError(t, err)
		assert.Equal(t, schedule.Schedule{ID: "foo"}, *sc)
	})
}

func TestScheduler_Put(t *testing.T) {
	sched := NewScheduler(redisClient, 0, QOptions{PollInterval: 1 * time.Second})
	require.NotNil(t, sched)
	require.NotNil(t, sched.client)

	sampleSchedule := schedule.Schedule{
		ID:           "foo",
		Tags:         []string{"tag1", "tag2"},
		Crontab:      "@at 1,2,3",
		Version:      1,
		Status:       schedule.StatusActive,
		EnqueueCount: 1,
	}

	t.Run("SimplePut", func(t *testing.T) {
		t.Cleanup(flushRedis)

		err := sched.Put(context.Background(), sampleSchedule, false)
		assert.NoError(t, err)

		val, err := sched.client.Get(context.Background(), "clockwork:schedule:{foo}").Result()
		assert.NoError(t, err)
		assert.NotEmpty(t, val)

		var actualSchedule schedule.Schedule
		assert.NoError(t, json.Unmarshal([]byte(val), &actualSchedule))

		assert.Equal(t, sampleSchedule, actualSchedule)
	})

	t.Run("ConflictHandling", func(t *testing.T) {
		t.Cleanup(flushRedis)

		err1 := sched.Put(context.Background(), sampleSchedule, false)
		assert.NoError(t, err1)

		err2 := sched.Put(context.Background(), sampleSchedule, false)
		assert.Error(t, err2)
		assert.True(t, errors.Is(err2, clockwork.ErrConflict))
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		t.Cleanup(flushRedis)

		updated := schedule.Schedule{ID: "non-existent-schedule", Crontab: "@at 1,2,3,4,5"}

		err := sched.Put(context.Background(), updated, true)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
	})

	t.Run("UpdateSuccess", func(t *testing.T) {
		t.Cleanup(flushRedis)

		// create one schedule
		err1 := sched.Put(context.Background(), sampleSchedule, false)
		assert.NoError(t, err1)

		// update it
		updated := sampleSchedule
		updated.Crontab = "@at 10,20,30"
		err2 := sched.Put(context.Background(), updated, true)
		assert.NoError(t, err2)

		val, err := sched.client.Get(context.Background(), "clockwork:schedule:{foo}").Result()
		assert.NoError(t, err)
		assert.NotEmpty(t, val)

		var actualSchedule schedule.Schedule
		assert.NoError(t, json.Unmarshal([]byte(val), &actualSchedule))

		assert.Equal(t, updated, actualSchedule)
	})
}
