package schedule

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/spy16/clockwork/client"
)

var _ Channel = (*LogChannel)(nil)

//go:generate mockery --with-expecter --keeptree --case snake --name Channel

// Channel implementation is responsible for publishing the execution events
// of a schedule.
type Channel interface {
	Publish(ctx context.Context, cl client.Client, sc Schedule, req Execution) error
}

// LogChannel implements Channel using a logger to publish the schedule events.
type LogChannel struct{}

func (LogChannel) Publish(_ context.Context, cl client.Client, sc Schedule, req Execution) error {
	log.Info().
		Str("client_id", cl.ID).
		Str("schedule_id", sc.ID).
		Time("execute_at", req.EnqueueAt).
		Str("channel_name", cl.ChannelName).
		Int("execution_number", sc.EnqueueCount).
		Msg("execution request handled")
	return nil
}
