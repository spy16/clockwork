package inmem_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/inmem"
	"github.com/spy16/clockwork/schedule"
	"github.com/stretchr/testify/assert"
)

func TestScheduler_Get(t *testing.T) {
	t.Parallel()

	t.Run("ScheduleNotFound", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		sc, err := sched.Get(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, sc)
	})

	t.Run("ScheduleFound", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.NoError(t, err)

		sc, err := sched.Get(context.Background(), "foo")
		assert.NoError(t, err)
		assert.NotNil(t, sc)
		assert.Equal(t, "foo", sc.ID)
	})

}

func TestScheduler_Put(t *testing.T) {
	t.Parallel()

	t.Run("SuccessfulInsert", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.NoError(t, err)

		sc, err := sched.Get(context.Background(), "foo")
		assert.NoError(t, err)
		assert.NotNil(t, sc)
		assert.Equal(t, "foo", sc.ID)
	})

	t.Run("UpsertNonExistentSchedule", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, true)
		assert.True(t, errors.Is(err, clockwork.ErrNotFound), "expected ErrNotFound, got %v", err)
	})

	t.Run("ConflictingInsert", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.NoError(t, err)

		err = sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.True(t, errors.Is(err, clockwork.ErrConflict), "expected ErrConflict, got %v", err)
	})
}

func TestScheduler_Del(t *testing.T) {
	t.Parallel()

	t.Run("ScheduleNotFound", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Del(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
	})

	t.Run("ScheduleFound", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.NoError(t, err)

		err = sched.Del(context.Background(), "foo")
		assert.NoError(t, err)

		sc, err := sched.Get(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, sc)
	})
}

func TestScheduler_List(t *testing.T) {
	t.Parallel()

	t.Run("NoSchedules", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		schedules, err := sched.List(context.Background(), 0, -1)
		assert.NoError(t, err)
		assert.Empty(t, schedules)
	})

	t.Run("SchedulesExist", func(t *testing.T) {
		frozenTime := time.Unix(1697439139, 0)

		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{
			ID:        "foo",
			CreatedAt: frozenTime,
		}, false)
		assert.NoError(t, err)

		err = sched.Put(context.Background(), schedule.Schedule{
			ID:        "bar",
			CreatedAt: frozenTime.Add(10 * time.Second),
		}, false)
		assert.NoError(t, err)

		schedules, err := sched.List(context.Background(), 0, -1)
		assert.NoError(t, err)
		assert.Equal(t, []string{"foo", "bar"}, []string{schedules[0].ID, schedules[1].ID})
	})

	t.Run("OffsetAndCount", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{ID: "foo"}, false)
		assert.NoError(t, err)

		err = sched.Put(context.Background(), schedule.Schedule{ID: "bar"}, false)
		assert.NoError(t, err)

		schedules, err := sched.List(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, schedules, 1)
	})
}

func TestScheduler_Run(t *testing.T) {
	t.Parallel()

	t.Run("NoSchedules", func(t *testing.T) {
		sched := &inmem.Scheduler{}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// cancel the context after 1 second.

			<-time.After(1 * time.Second)
			cancel()
		}()

		// run the scheduler. it should exit after 1 second.
		err := sched.Run(ctx, func(ctx context.Context, sc schedule.Schedule, cur schedule.Execution) (next *schedule.Execution, err error) {
			return nil, nil
		})
		assert.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("SingleScheduleWithMultipleExecutions", func(t *testing.T) {
		frozenTime := time.Unix(1697439139, 0)

		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{
			ID:      "foo",
			Status:  "ACTIVE",
			Version: 1,
		}, false, []schedule.Execution{
			{
				Version:    1, // this point should be ignored since it's older than the schedule's version.
				ScheduleID: "foo",
				EnqueueAt:  frozenTime.Add(1 * time.Second),
				Manual:     false,
			},
		}...)
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var ids []string
		execSchedule := func(ctx context.Context, sc schedule.Schedule, cur schedule.Execution) (next *schedule.Execution, err error) {
			ids = append(ids, fmt.Sprintf("%s-%d-%d", cur.ScheduleID, sc.Version, cur.EnqueueAt.Unix()))

			if frozenTime.Add(1 * time.Second).Equal(cur.EnqueueAt) {
				// enqueue another execution after 1 second from this point.
				return &schedule.Execution{
					Version:    1,
					ScheduleID: "foo",
					EnqueueAt:  cur.EnqueueAt.Add(1 * time.Second),
					Manual:     false,
				}, nil
			}

			return nil, nil
		}

		err = sched.Run(ctx, execSchedule)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
		assert.Equal(t, []string{"foo-1-1697439140", "foo-1-1697439141"}, ids)
	})

	t.Run("MultipleSchedules", func(t *testing.T) {
		frozenTime := time.Unix(1697439139, 0)

		sched := &inmem.Scheduler{}
		err := sched.Put(context.Background(), schedule.Schedule{
			ID:      "foo",
			Status:  "ACTIVE",
			Version: 2,
		}, false, []schedule.Execution{
			{
				Version:    2,
				ScheduleID: "foo",
				EnqueueAt:  frozenTime.Add(2 * time.Second),
				Manual:     false,
			},
			{
				Version:    1, // this point should be ignored since it's older than the schedule's version.
				ScheduleID: "foo",
				EnqueueAt:  frozenTime.Add(2 * time.Second),
				Manual:     false,
			},
		}...)
		assert.NoError(t, err)

		err = sched.Put(context.Background(), schedule.Schedule{
			ID:      "bar",
			Status:  "ACTIVE",
			Version: 1,
		}, false, schedule.Execution{
			Version:    1,
			ScheduleID: "bar",
			EnqueueAt:  frozenTime.Add(1 * time.Second),
			Manual:     false,
		})
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var ids []string
		execSchedule := func(ctx context.Context, sc schedule.Schedule, cur schedule.Execution) (next *schedule.Execution, err error) {
			if cur.Version < sc.Version {
				return nil, nil
			}

			ids = append(ids, fmt.Sprintf("%s-%d-%d", cur.ScheduleID, sc.Version, cur.EnqueueAt.Unix()))
			return nil, nil
		}

		err = sched.Run(ctx, execSchedule)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
		assert.Equal(t, []string{"bar-1-1697439140", "foo-2-1697439141"}, ids)
	})
}
