package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/spy16/clockwork/pkg/config"
	"github.com/spy16/clockwork/pkg/telemetry"
)

const serviceName = "clockwork"

var (
	Version = "N/A"
	Commit  = "N/A"
	BuiltOn = "N/A"

	rootCmd = &cobra.Command{
		Use:   serviceName,
		Short: "⏱ Just that, a clockwork service.",
		Long: `
⏱  Clockwork is a fault-tolerant, distributed generic clockwork system. Users 
schedule notification events to be sent at some specific points in time
using crontab, clockwork makes it happen.`,
		Version: fmt.Sprintf("%s\ncommit: %s\nbuild date: %s", Version, Commit, BuiltOn),
	}
)

func main() {
	rand.Seed(time.Now().UnixNano())

	chdirIfNeeded()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var closeLogger func()

	rootCmd.PersistentFlags().StringP("config", "c", "", "override configuration file")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		err := config.CobraPreRunHook("", serviceName)(cmd, args)
		if err != nil {
			return err
		}
		closeLogger = setupLogger(ctx)
		return nil
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		closeLogger()
	}

	rootCmd.AddCommand(
		cmdAgent(ctx),
		cmdTimeUtil(),
	)

	_ = rootCmd.Execute()
}

func setupLogger(ctx context.Context) (closer func()) {
	var (
		level       = config.String("log.level", "info")
		format      = config.String("log.format", "json")
		bufferSize  = config.Int("log.buffer_size", 1000)
		pollingTime = config.Duration("log.polling_time", 10, time.Millisecond)
	)

	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	var wr io.Writer
	if format == "text" {
		wr = &zerolog.ConsoleWriter{
			Out: os.Stderr,
		}
	} else {
		wr = diode.NewWriter(os.Stderr, bufferSize, pollingTime, func(missed int) {
			fmt.Printf("logger dropped %d messages", missed)
		})
	}

	log.Logger = zerolog.New(wr).With().Caller().Timestamp().Logger().Level(logLevel)

	return func() {
		// shutdown logger on context cancellation.
		if closer, ok := wr.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}

func setupTelemetry(ctx context.Context) {
	telemetry.Init(ctx, telemetry.Config{
		DebugAddr: config.String("telemetry.debug_addr", ":8888"),

		// OpenCensus configurations.
		ServiceName:      serviceName,
		EnableExporters:  config.Bool("telemetry.export", true),
		OpenTelAgentAddr: config.String("telemetry.ocagent_addr", "localhost:55678"),
		SamplingFraction: config.Float64("telemetry.sampling_fraction", 1),

		// StatsD configurations.
		StatsdAddr:      config.String("telemetry.statsd_addr", "localhost:8125"),
		StatsdTags:      strings.Split(config.String("telemetry.statsd_tags", ""), ","),
		StatsdNamespace: serviceName,
		PublishRuntime:  config.Bool("telemetry.publish_runtime", true),
	})
}

func chdirIfNeeded() {
	if os.Getenv("CHDIR_EXEC_PATH") == "true" {
		execPath, err := os.Executable()
		if err == nil {
			_ = os.Chdir(filepath.Dir(execPath))
		}
	}
}
