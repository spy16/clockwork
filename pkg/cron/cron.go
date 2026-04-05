package cron

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Schedule represents a cron schedule.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Parse parses crontab expressions and returns a schedule object that can
// generate activation points as per the schedule. Supported formats can be
// found here: https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format
// In addition, this also supports "@at t1,t2,t3" extension format which
// represents pre-defined timestamps. (t1, t2, etc. are unix timestamps).
func Parse(crontab string) (Schedule, error) {
	crontab = strings.TrimSpace(crontab)
	if strings.HasPrefix(crontab, "@at") {
		times, err := parseAtCron(crontab)
		if err != nil {
			return nil, err
		}
		return ScheduleFunc(func(t time.Time) time.Time {
			for _, t2 := range times {
				if t2.After(t) {
					return t2
				}
			}
			return time.Time{}
		}), nil
	}

	return cron.ParseStandard(crontab)
}

// ScheduleFunc implements Schedule using a simple Go function.
type ScheduleFunc func(t time.Time) time.Time

func (fn ScheduleFunc) Next(t time.Time) time.Time { return fn(t) }

func parseAtCron(spec string) ([]time.Time, error) {
	spec = strings.TrimPrefix(spec, "@at")

	var times []time.Time
	parts := strings.SplitSeq(strings.TrimSpace(spec), ",")
	for timestampStr := range parts {
		ts, err := strconv.Atoi(strings.TrimSpace(timestampStr))
		if err != nil {
			return nil, err
		}
		t := time.Unix(int64(ts), 0)
		if t.Year() > 9999 {
			// References:
			// 1. https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
			// 2. https://datatracker.ietf.org/doc/html/rfc3339#section-1
			return nil, errors.New("timestamps are allowed only upto the year 9999")
		}
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	return times, nil
}
