package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/spy16/clockwork/pkg/cron"
)

func TestParse(t *testing.T) {
	t.Parallel()

	table := []struct {
		title   string
		crontab string
		wantErr bool
		want    []time.Time
	}{
		{
			title:   "InvalidAtCron",
			crontab: "@at a,b,c",
			wantErr: true,
		},
		{
			title:   "LargeValue",
			crontab: "@at 253402300800",
			wantErr: true,
		},
		{
			title:   "LargeValue",
			crontab: "@at 1,2,253402300800,4",
			wantErr: true,
		},
		{
			title:   "ValidAtCron",
			crontab: "@at 1,2,3",
			want: []time.Time{
				time.Unix(1, 0),
				time.Unix(2, 0),
				time.Unix(3, 0),
				{}, // after specified schedule, returns zero value.
				{},
			},
		},
		{
			title:   "Every1h",
			crontab: "@every 1h",
			want: []time.Time{
				(time.Time{}).Add(1 * time.Hour),
				(time.Time{}).Add(2 * time.Hour),
				(time.Time{}).Add(3 * time.Hour),
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			schedule, err := cron.Parse(tt.crontab)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, schedule)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, schedule)
				assertCronResult(t, tt.want, schedule)
			}
		})
	}
}

func assertCronResult(t *testing.T, want []time.Time, schedule cron.Schedule) {
	if len(want) == 0 {
		assert.True(t, schedule == nil ||
			schedule.Next(time.Now()) == time.Time{},
			"schedule must be nil or have no activation point",
		)
		return
	}

	var lastAt time.Time
	for _, t2 := range want {
		next := schedule.Next(lastAt)
		assert.Equal(t, t2.Unix(), next.Unix(),
			"expected schedule point (%d) is not same as actual (%d)", t2.Unix(), next.Unix())
		if t2 != (time.Time{}) {
			lastAt = t2
		}
	}
}
