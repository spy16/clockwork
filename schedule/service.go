package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"go.opencensus.io/trace"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/pkg/telemetry"
)

//go:generate mockery --with-expecter --keeptree --case snake --name ClientRegistry

type Service struct {
	Clock     ClockFunc
	Clients   ClientRegistry
	Scheduler Scheduler
	Channels  map[string]Channel
	Changes   ChangeLogger
}

type ClockFunc func() time.Time

type ClientRegistry interface {
	IsAdmin(ctx context.Context, id string) bool
	GetClient(ctx context.Context, id string) (*client.Client, error)
}

// List returns the paginated list of schedules using the offset and
// count. If listing is not supported by the Scheduler backend, this
// may return ErrUnsupported.
func (tim *Service) List(ctx context.Context, offset, count int) ([]Schedule, error) {
	return tim.Scheduler.List(ctx, offset, count)
}

func (tim *Service) Fetch(ctx context.Context, scheduleID string) (*Schedule, error) {
	sc, err := tim.Scheduler.Get(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	if err := tim.authorise(ctx, *sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (tim *Service) Create(ctx context.Context, sc Schedule) (*Schedule, error) {
	if err := sc.Validate(); err != nil {
		return nil, err
	}
	sc.CreatedAt = tim.Clock().UTC()
	sc.UpdatedAt = sc.CreatedAt
	sc.Triggers = nil

	_, err := tim.Clients.GetClient(ctx, sc.ClientID)
	if err != nil {
		return nil, err
	}

	nextAt, err := sc.ComputeNext(sc.CreatedAt)
	if err != nil {
		return nil, err
	}
	if nextAt.Before(sc.CreatedAt) {
		nextAt = sc.CreatedAt
	}

	sc.EnqueueCount = 1
	sc.NextExecutionAt = nextAt
	exec := Execution{
		Manual:     false,
		Version:    0,
		EnqueueAt:  nextAt,
		ScheduleID: sc.ID,
	}

	if err := tim.Scheduler.Put(ctx, sc, false, exec); err != nil {
		return nil, err
	}
	tim.publishChange(ctx, changeTypeCreated, sc)
	return &sc, nil
}

func (tim *Service) Update(ctx context.Context, id string, update Updates) (*Schedule, error) {
	sc, err := tim.Scheduler.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if _, err := tim.Clients.GetClient(ctx, sc.ClientID); err != nil {
		return nil, err
	}

	if err := tim.authorise(ctx, *sc); err != nil {
		return nil, err
	}

	rescheduleNeeded := sc.Apply(update, tim.Clock)
	if err := sc.Validate(); err != nil {
		return nil, err
	}

	var executions []Execution
	if rescheduleNeeded {
		if update.Trigger > 0 {
			sc.Triggers = append(sc.Triggers, update.Trigger)
			executions = append(executions, Execution{
				Manual:     true,
				Version:    sc.Version,
				EnqueueAt:  time.Unix(update.Trigger, 0),
				ScheduleID: sc.ID,
			})
		}

		nextAt, err := sc.ComputeNext(sc.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if !nextAt.IsZero() {
			executions = append(executions, Execution{
				Manual:     false,
				Version:    sc.Version,
				EnqueueAt:  nextAt,
				ScheduleID: sc.ID,
			})
		}
	}

	if err := tim.Scheduler.Put(ctx, *sc, true, executions...); err != nil {
		return nil, err
	}
	tim.publishChange(ctx, changeTypeUpdated, *sc)
	return sc, nil
}

func (tim *Service) Delete(ctx context.Context, scheduleID string) error {
	sc, err := tim.Fetch(ctx, scheduleID)
	if err != nil {
		return err
	}
	if err := tim.authorise(ctx, *sc); err != nil {
		return err
	}
	if err := tim.Scheduler.Del(ctx, scheduleID); err != nil {
		return err
	}
	tim.publishChange(ctx, changeTypeDeleted, *sc)
	return nil
}

func (tim *Service) publish(ctx context.Context, sc Schedule, req Execution) error {
	ctx, span := trace.StartSpan(ctx, "Clockwork_Publish")
	defer span.End()
	span.AddAttributes(
		trace.StringAttribute("client_id", sc.ClientID),
		trace.StringAttribute("schedule_id", sc.ID),
	)

	scClient, err := tim.Clients.GetClient(ctx, sc.ClientID)
	if err != nil {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeUnknown,
			Message: fmt.Sprintf("failed to get client: %v", err),
		})
		return err
	}
	span.AddAttributes(
		trace.StringAttribute("channel_type", scClient.ChannelType),
		trace.StringAttribute("channel_name", scClient.ChannelName),
	)

	channel, found := tim.Channels[scClient.ChannelType]
	if !found {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeNotFound,
			Message: "CHANNEL_TYPE_INVALID",
		})
		return clockwork.ErrInvalid.WithCausef("channel type '%s' not supported", scClient.ChannelType)
	}

	if err := channel.Publish(ctx, *scClient, sc, req); err != nil {
		return err
	}

	telemetry.Timing("execution_delay", time.Since(req.EnqueueAt), 1).
		Tag("category", sc.Category).
		Tag("client_id", sc.ClientID).
		Publish()
	return nil
}

func (tim *Service) authorise(ctx context.Context, sc Schedule) error {
	var isAdmin, isOwner bool
	if cl := client.From(ctx); cl != nil {
		isAdmin = tim.Clients.IsAdmin(ctx, cl.ID)
		isOwner = cl.ID == sc.ClientID
	}

	if isAdmin || isOwner {
		return nil
	}
	return clockwork.ErrUnauthorized.WithMsgf("not authorised to access this schedule")
}

func (tim *Service) publishChange(ctx context.Context, changeType int, sc Schedule) {
	entry := log.Ctx(ctx).With().
		Str("schedule_id", sc.ID).
		Int("change_type", changeType).
		Str("schedule_data", jsonStr(sc)).Logger()

	if tim.Changes == nil {
		entry.Info().Msg("no change-logger set, not logging change")
		return
	}

	switch changeType {
	case 1, 2, 3:
		if err := tim.Changes.Publish(ctx, changeType, sc); err != nil {
			entry.Err(err).Msg("failed to publish schedule-change info to change-log")
			return
		}

	default:
		entry.Warn().Msg("invalid change type")
	}
}

func jsonStr(sc Schedule) string {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(sc); err != nil {
		// JSON encode error for Schedule is not acceptable.
		// Hence fail-fast (panic).
		panic(err)
	}
	return buf.String()
}

const (
	changeTypeCreated = 1
	changeTypeUpdated = 2
	changeTypeDeleted = 3
)
