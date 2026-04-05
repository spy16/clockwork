package telemetry

import (
	"context"
	"net/http"
	_ "net/http/pprof"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/rs/zerolog/log"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"
)

func setupOpenCensus(ctx context.Context, cfg Config) error {
	if !cfg.EnableExporters {
		return nil
	}

	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.ProbabilitySampler(cfg.SamplingFraction),
	})

	if err := setupViews(); err != nil {
		log.Fatal().Err(err).Msg("failed to setup views")
	}

	ocExporter, err := ocagent.NewExporter(
		ocagent.WithServiceName(cfg.ServiceName),
		ocagent.WithInsecure(),
		ocagent.WithAddress(cfg.OpenTelAgentAddr),
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to setup open-census exporter")
	}
	go func() {
		<-ctx.Done()
		_ = ocExporter.Stop()
	}()

	trace.RegisterExporter(ocExporter)
	view.RegisterExporter(ocExporter)

	zpages.Handle(http.DefaultServeMux, "/debug")
	return nil
}

func setupViews() error {
	if err := setupClientViews(); err != nil {
		return err
	}

	if err := setupServerViews(); err != nil {
		return err
	}

	return nil
}

func setupServerViews() error {
	var serverViewTags = []tag.Key{
		ochttp.KeyServerRoute,
		ochttp.Method,
	}

	return view.Register(
		&view.View{
			Name:        "opencensus.io/http/server/request_bytes",
			Description: "Size distribution of HTTP request body",
			Measure:     ochttp.ServerRequestBytes,
			Aggregation: ochttp.DefaultSizeDistribution,
			TagKeys:     serverViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/server/response_bytes",
			Description: "Size distribution of HTTP response body",
			Measure:     ochttp.ServerResponseBytes,
			Aggregation: ochttp.DefaultSizeDistribution,
			TagKeys:     serverViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/server/latency",
			Description: "Latency distribution of HTTP requests",
			Measure:     ochttp.ServerLatency,
			Aggregation: ochttp.DefaultLatencyDistribution,
			TagKeys:     serverViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/server/request_count_by_method",
			Description: "Server request count by HTTP method",
			Measure:     ochttp.ServerRequestCount,
			Aggregation: view.Count(),
			TagKeys:     serverViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/server/response_count_by_status_code",
			Description: "Server response count by status code",
			TagKeys:     append(serverViewTags, ochttp.StatusCode),
			Measure:     ochttp.ServerLatency,
			Aggregation: view.Count(),
		},
	)
}

func setupClientViews() error {
	if err := view.Register(ocgrpc.DefaultClientViews...); err != nil {
		return err
	}

	var clientViewTags = []tag.Key{
		ochttp.KeyClientMethod,
		ochttp.KeyClientStatus,
		ochttp.KeyClientHost,
	}

	return view.Register(
		&view.View{
			Name:        "opencensus.io/http/client/roundtrip_latency",
			Measure:     ochttp.ClientRoundtripLatency,
			Aggregation: ochttp.DefaultLatencyDistribution,
			Description: "End-to-end latency, by HTTP method and response status",
			TagKeys:     clientViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/client/sent_bytes",
			Measure:     ochttp.ClientSentBytes,
			Aggregation: ochttp.DefaultSizeDistribution,
			Description: "Total bytes sent in request body (not including headers), by HTTP method and response status",
			TagKeys:     clientViewTags,
		},
		&view.View{
			Name:        "opencensus.io/http/client/received_bytes",
			Measure:     ochttp.ClientReceivedBytes,
			Aggregation: ochttp.DefaultSizeDistribution,
			Description: "Total bytes received in response bodies (not including headers but including error responses with bodies), by HTTP method and response status",
			TagKeys:     clientViewTags,
		},
	)
}
