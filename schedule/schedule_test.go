package schedule_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/schedule"
)

func TestSchedule_NextAt(t *testing.T) {
	t.Parallel()

	table := []struct {
		title    string
		sched    schedule.Schedule
		relative time.Time
		want     time.Time
		wantErr  bool
	}{
		{
			title: "InvalidCrontab",
			sched: schedule.Schedule{
				Crontab: "@asdfsft 1",
			},
			relative: time.Time{},
			wantErr:  true,
		},
		{
			title: "OneActivation",
			sched: schedule.Schedule{
				Crontab: "@at 1",
			},
			relative: time.Time{},
			want:     time.Unix(1, 0),
		},
		{
			title: "OneActivation_LargeValue",
			sched: schedule.Schedule{
				Crontab: "@at 253402300800",
			},
			relative: time.Time{},
			wantErr:  true,
		},
		{
			title: "ActivationInPast",
			sched: schedule.Schedule{
				Crontab: "@at 1",
			},
			relative: time.Unix(1000, 0),
			want:     time.Time{},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			next, err := tt.sched.ComputeNext(tt.relative)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, next.IsZero())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Unix(), next.Unix())
			}
		})
	}
}

func TestSchedule_Apply(t *testing.T) {
	t.Parallel()

	frozenTime := time.Unix(1697439140, 0).UTC()

	boolPtr := func(b bool) *bool { return &b }

	table := []struct {
		title          string
		sched          schedule.Schedule
		update         schedule.Updates
		want           schedule.Schedule
		wantReschedule bool
	}{
		{
			title: "Disable",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
			},
			update: schedule.Updates{
				Disable: boolPtr(true),
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@at crontab",
				Category:  "default",
				ClientID:  "foo",
				Status:    schedule.StatusDisabled,
				Version:   1,
				UpdatedAt: frozenTime,
			},
		},
		{
			title: "Enable",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusDisabled,
			},
			update: schedule.Updates{
				Disable: boolPtr(false),
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@at crontab",
				Category:  "default",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   1,
				UpdatedAt: frozenTime,
			},
			wantReschedule: true,
		},
		{
			title: "Category",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
			},
			update: schedule.Updates{
				Category: "new_category",
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@at crontab",
				Category:  "new_category",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   0,
				UpdatedAt: frozenTime,
			},
		},
		{
			title: "Crontab",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
			},
			update: schedule.Updates{
				Crontab: "@every 1h",
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@every 1h",
				Category:  "default",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   1,
				UpdatedAt: frozenTime,
			},
			wantReschedule: true,
		},
		{
			title: "Crontab_WithNoChange",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@every 1h",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
			},
			update: schedule.Updates{
				Crontab: "@every 1h",
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@every 1h",
				Category:  "default",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   0,
				UpdatedAt: frozenTime,
			},
			wantReschedule: false,
		},
		{
			title: "Tags",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
				Tags:     []string{"a", "b"},
			},
			update: schedule.Updates{
				Tags:     []string{"c"},
				Category: "new_category",
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Tags:      []string{"c"},
				Crontab:   "@at crontab",
				Category:  "new_category",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   0,
				UpdatedAt: frozenTime,
			},
		},
		{
			title: "Payload",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusActive,
				Payload:  "old-payload",
			},
			update: schedule.Updates{
				Payload: stringPtr("new-payload"),
			},
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@at crontab",
				Category:  "default",
				Payload:   "new-payload",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   0,
				UpdatedAt: frozenTime,
			},
		},
		{
			title: "Trigger",
			sched: schedule.Schedule{
				ID:       "sched1",
				Crontab:  "@at crontab",
				Category: "default",
				ClientID: "foo",
				Status:   schedule.StatusDone,
				Payload:  "old-payload",
			},
			update: schedule.Updates{
				Trigger: 123,
				Payload: stringPtr("new-payload"),
			},
			wantReschedule: true,
			want: schedule.Schedule{
				ID:        "sched1",
				Crontab:   "@at crontab",
				Category:  "default",
				Payload:   "new-payload",
				ClientID:  "foo",
				Status:    schedule.StatusActive,
				Version:   0,
				UpdatedAt: frozenTime,
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			got := tt.sched.Apply(tt.update, func() time.Time {
				return frozenTime
			})

			assert.Equal(t, tt.wantReschedule, got)
			assert.Equal(t, tt.want, tt.sched)
		})
	}
}

func TestSchedule_Validate(t *testing.T) {
	t.Parallel()

	table := []struct {
		title   string
		sched   schedule.Schedule
		wantErr error
	}{
		{
			title: "EmptyID",
			sched: schedule.Schedule{
				ID:       "",
				Crontab:  "@at 1",
				Category: "default",
				ClientID: "clockwork",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "InvalidCrontab",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "@ aasjf",
				Category: "default",
				ClientID: "clockwork",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "EmptyCrontab",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "",
				Category: "default",
				ClientID: "clockwork",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "NoCategory",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "@at 1",
				Category: "",
				ClientID: "clockwork",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "NoClientID",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "@at 1",
				Category: "default",
				ClientID: "",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "OutOfRange",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "@at 253402300800",
				Category: "default",
				ClientID: "foo",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "Successful",
			sched: schedule.Schedule{
				ID:       "schedule_1",
				Crontab:  "@at 1697439140",
				Category: "default",
				ClientID: "foo",
			},
			wantErr: nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			err := tt.sched.Validate()
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchedule_JSON(t *testing.T) {
	frozenTime := time.Unix(1697439140, 0).UTC()

	sc := schedule.Schedule{
		ID:           "schedule_1",
		Tags:         []string{"tag1", "tag2"},
		Status:       schedule.StatusActive,
		Payload:      `{}`,
		Version:      1,
		Crontab:      "@at 1697439140",
		Category:     "default",
		ClientID:     "foo",
		CreatedAt:    frozenTime,
		UpdatedAt:    frozenTime,
		EnqueueCount: 0,
	}

	assert.JSONEq(t, `{
		"id": "schedule_1",
		"tags": ["tag1", "tag2"],
		"status": "ACTIVE",
		"payload": "{}",
		"version": 1,
		"crontab": "@at 1697439140",
		"category": "default",
		"client_id": "foo",
		"created_at": "2023-10-16T06:52:20Z",
		"updated_at": "2023-10-16T06:52:20Z",
		"enqueue_count": 0,
		"last_enqueued_at": "0001-01-01T00:00:00Z"
	}`, string(sc.JSON()))
}

func stringPtr(s string) *string { return &s }
