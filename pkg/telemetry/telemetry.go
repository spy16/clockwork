package telemetry

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/spy16/clockwork/pkg/httputil"
)

type Config struct {
	DebugAddr string

	// OpenCensus related configurations.
	ServiceName      string
	EnableExporters  bool
	OpenTelAgentAddr string
	SamplingFraction float64

	// StatsD telemetry configurations.
	StatsdAddr      string
	StatsdTags      []string
	StatsdNamespace string
	PublishRuntime  bool
}

// Init initialises StatsD and OpenCensus based async-telemetry processes and
// returns (i.e., it does not block).
func Init(ctx context.Context, cfg Config) {
	if err := setupStatsD(ctx, cfg); err != nil {
		log.Error().Err(err).Msg("failed to setup statsd")
	}

	if err := setupOpenCensus(ctx, cfg); err != nil {
		log.Error().Err(err).Msg("failed to setup open-census")
	}

	go func() {
		if cfg.DebugAddr == "" {
			return
		}

		if err := httputil.Serve(ctx, cfg.DebugAddr, http.DefaultServeMux); err != nil {
			log.Err(err).Msg("debug server exited due to error")
		}
	}()
}

// ReportUptime reports uptime statsd metric with given tags at given interval.
// On every publish, a gauge is published with seconds-since start of this call.
func ReportUptime(interval time.Duration, tags map[string]any) {
	start := time.Now()
	if interval <= 0 {
		interval = 1 * time.Second
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()
	for range tick.C {
		uptimeMetric := Gauge("uptime", time.Since(start).Seconds(), 1).Tag("status", "up")
		for k, v := range tags {
			uptimeMetric.Tag(k, fmt.Sprintf("%s", v))
		}
		uptimeMetric.Publish()
	}
}
