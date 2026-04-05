package schedule

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opencensus.io/trace"

	"github.com/spy16/clockwork/pkg/telemetry"
)

// Loop starts the Scheduler worker loop and blocks until a critical error
// or until the Scheduler returns due to context cancellation.
func (tim *Service) Loop(ctx context.Context) error {
	err := tim.Scheduler.Run(ctx, onReadyHandler(tim))
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func onReadyHandler(svc *Service) OnReadyFunc {
	const (
		skipInactive  = "SKIP_INACTIVE"
		publishFailed = "PUBLISH_FAILED"
		computeFailed = "COMPUTE_NEXT_FAILED"
		scheduleNext  = "SCHEDULE_NEXT"
		scheduleDone  = "SCHEDULE_ENDED"
	)

	return func(ctx context.Context, sc Schedule, req Execution) (*Execution, error) {
		ctx, entry, metric, span, endAll := instrumentOnReady(ctx, sc, req)
		defer endAll()

		if sc.Status == StatusDisabled || sc.Status == StatusDone {
			entry.Debug().Msg("schedule is not in active status, skipping")
			span.SetStatus(trace.Status{Message: skipInactive})
			metric.Status(skipInactive)
			return nil, nil
		}

		if err := svc.publish(ctx, sc, req); err != nil {
			entry.Error().Err(err).Msg("failed to publish")
			span.SetStatus(trace.Status{
				Message: publishFailed,
				Code:    trace.StatusCodeInternal,
			})
			metric.Status(publishFailed)
			return nil, err
		}

		nextAt, err := sc.ComputeNext(req.EnqueueAt)
		if err != nil {
			entry.Error().Err(err).Msg("failed to compute next execution")
			span.SetStatus(trace.Status{
				Message: computeFailed,
				Code:    trace.StatusCodeInternal,
			})
			metric.Status(computeFailed)
			return nil, err
		}

		if !req.Manual && !nextAt.IsZero() {
			entry.Info().Msg("next execution point is available")
			span.SetStatus(trace.Status{Message: scheduleNext})
			metric.Status(scheduleNext)
			return &Execution{
				Version:    sc.Version,
				EnqueueAt:  nextAt,
				ScheduleID: sc.ID,
			}, nil
		}

		entry.Info().Msg("no more executions required, schedule is done")
		metric.Status(scheduleDone)
		span.SetStatus(trace.Status{Message: scheduleDone})
		return nil, nil
	}
}

func instrumentOnReady(ctx context.Context, sc Schedule, req Execution) (context.Context, zerolog.Logger, *telemetry.Metric, *trace.Span, func()) {
	metric := telemetry.
		Incr("on_ready", 1).
		Tag("category", sc.Category)

	ctx, span := trace.StartSpan(ctx, "Clockwork_OnReady")
	span.AddAttributes(
		trace.StringAttribute("client_id", sc.ClientID),
		trace.StringAttribute("schedule_id", sc.ID),
		trace.StringAttribute("schedule_status", sc.Status),
	)

	entry := log.Ctx(ctx).With().
		Str("schedule_id", sc.ID).
		Str("client_id", sc.ClientID).
		Str("schedule_status", sc.Status).
		Time("execution_at", req.EnqueueAt).
		Logger()

	return ctx, entry, metric, span, func() {
		metric.Publish()
		span.End()
	}
}
