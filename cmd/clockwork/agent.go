package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"
	"time"

	confluentkafka "github.com/confluentinc/confluent-kafka-go/kafka"
	goredis "github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/inmem"
	"github.com/spy16/clockwork/kafka"
	"github.com/spy16/clockwork/pkg/config"
	"github.com/spy16/clockwork/pkg/httputil"
	"github.com/spy16/clockwork/pkg/telemetry"
	"github.com/spy16/clockwork/redis"
	"github.com/spy16/clockwork/schedule"
	"github.com/spy16/clockwork/server"
)

func cmdAgent(ctx context.Context) *cobra.Command {
	var storageBackend, serveAddr string

	cmd := &cobra.Command{
		Use:     "agent",
		Short:   "🕵️  Start clockwork agent with http server and enqueuer",
		Aliases: []string{"agent", "start", "server"},
		Run: func(cmd *cobra.Command, args []string) {
			versionStr := fmt.Sprintf("%s (commit %s built at %s)", Version, Commit, BuiltOn)

			setupTelemetry(ctx)

			if config.Bool("telemetry.report_uptime", true) {
				go telemetry.ReportUptime(
					config.Duration("telemetry.report_uptime_interval", 1, time.Second),
					map[string]any{
						"version": Version,
						"backend": storageBackend,
					},
				)
			}

			scheduler, clientStore := setupStorageBackend(storageBackend)
			log.Debug().Msgf("using scheduler '%s'", reflect.TypeOf(scheduler))

			channelNames, channels := setupChannels()

			clientSvc := client.NewRegistry(
				clientStore,
				strings.Split(config.String("admin_clients"), ","),
				channelNames,
			)
			log.Info().Msgf("admin clients: %s", clientSvc.Admins)

			scheduleSvc := &schedule.Service{
				Clock:     time.Now,
				Clients:   clientSvc,
				Channels:  channels,
				Scheduler: scheduler,
				Changes:   setupChangeLogger(),
			}

			enableClientAuth := config.Bool("verify_client", true)

			go func() {
				router := server.Router(versionStr, scheduler, scheduleSvc, clientSvc, enableClientAuth)

				log.Info().Msgf("rest API server listening on %s...", serveAddr)
				if err := httputil.Serve(ctx, serveAddr, router); err != nil {
					log.Fatal().Err(err).Msg("server exited with error")
				}
				log.Info().Msg("api server exited gracefully")
			}()

			if err := scheduleSvc.Loop(ctx); err != nil {
				log.Fatal().Err(err).Msg("scheduler loop exited with error")
			}
			log.Info().Msg("clockwork service exited gracefully")
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&storageBackend, "backend", "b", "redis", "Scheduler & storage backend to use")
	flags.StringVarP(&serveAddr, "addr", "a", ":8081", "Bind address for the REST server")

	return cmd
}

func setupChangeLogger() schedule.ChangeLogger {
	enableLog := config.Bool("changelog.enabled", true)
	if !enableLog {
		return nil
	}

	topic := config.String("changelog.topic", "clockwork_schedule_changes")

	sslEnabled := config.Bool("changelog.kafka.ssl.enabled", false)
	securityProtocol := "plaintext"
	clientCert := config.String("changelog.kafka.ssl.cert", "")
	clientKey := config.String("changelog.kafka.ssl.key", "")
	caCert := config.String("changelog.kafka.ssl.ca_cert", "")

	if sslEnabled {
		if clientCert == "" || clientKey == "" || caCert == "" {
			log.Fatal().Msg("SSL is enabled for Changelog Kafka Client but SSL keys are not provided correctly")
		}

		securityProtocol = "ssl"
	}

	kafkaConf := map[string]confluentkafka.ConfigValue{
		"acks":                                  "all",
		"bootstrap.servers":                     config.String("changelog.kafka.brokers", "localhost:9092"),
		"message.send.max.retries":              config.Int("changelog.kafka.send_max_retries", 5),
		"max.in.flight.requests.per.connection": 1,
		"security.protocol":                     securityProtocol,
		"ssl.certificate.pem":                   clientCert,
		"ssl.key.pem":                           clientKey,
		"ssl.ca.pem":                            caCert,
	}
	cl, err := kafka.NewChangeLogger(topic, kafkaConf)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise kafka change logger")
	}
	return cl
}

func setupChannels() ([]string, map[string]schedule.Channel) {
	channels := map[string]schedule.Channel{
		"kafka": setupKafkaChannel(),
		"log":   schedule.LogChannel{},
	}

	var supported []string
	for name := range channels {
		supported = append(supported, name)
	}

	return supported, channels
}

func setupStorageBackend(spec string) (schedule.Scheduler, client.Store) {
	const (
		specInMem = "in_memory"
		specRedis = "redis"
	)

	switch {
	case spec == specInMem:
		return &inmem.Scheduler{}, &inmem.ClientStore{}

	case spec == specRedis:
		return setupRedisBackend()

	default:
		log.Fatal().Msgf("invalid store spec: %s", spec)
		return nil, nil
	}
}

func setupKafkaChannel() *kafka.Channel {
	sslEnabled := config.Bool("kafka.ssl.enabled", false)
	securityProtocol := "plaintext"
	clientCert := config.String("kafka.ssl.cert", "")
	clientKey := config.String("kafka.ssl.key", "")
	caCert := config.String("kafka.ssl.ca_cert", "")

	if sslEnabled {
		if clientCert == "" || clientKey == "" || caCert == "" {
			log.Fatal().Msg("SSL is enabled for Kafka Client but SSL keys are not provided correctly")
		}

		securityProtocol = "ssl"
	}

	kafkaChannel, err := kafka.NewChannel(map[string]confluentkafka.ConfigValue{
		"acks":                                  "all",
		"bootstrap.servers":                     config.String("kafka.brokers", "localhost:9092"),
		"message.send.max.retries":              config.Int("kafka.send_max_retries", 5),
		"max.in.flight.requests.per.connection": 1,
		"security.protocol":                     securityProtocol,
		"ssl.certificate.pem":                   clientCert,
		"ssl.key.pem":                           clientKey,
		"ssl.ca.pem":                            caCert,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise kafka queue")
	}

	return kafkaChannel
}

func setupRedisBackend() (*redis.Scheduler, *redis.ClientStore) {
	var redisClient goredis.UniversalClient
	var tlsConfig *tls.Config
	addr := config.String("redis_scheduler.client.addr", "localhost:6379")
	sslEnabled := config.Bool("redis_scheduler.client.ssl.enabled", false)

	if sslEnabled {
		clientCert := config.String("redis_scheduler.client.ssl.cert", "")
		clientKey := config.String("redis_scheduler.client.ssl.key", "")
		var certificates = []tls.Certificate{}

		if clientCert != "" || clientKey != "" {
			cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
			if err != nil {
				log.Fatal().Err(err).Msg("failed to import ssl certs for redis")
			}
			certificates = append(certificates, cert)
		}

		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			log.Warn().Msg("failed to load system cert pool, using an empty pool")
			rootCAs = x509.NewCertPool()
		}

		// add CA cert in cert pool
		caCert := config.String("redis_scheduler.client.ssl.ca_cert", "")
		if caCert != "" {
			rootCAs.AppendCertsFromPEM([]byte(caCert))
		}

		tlsConfig = &tls.Config{
			RootCAs:      rootCAs,
			Certificates: certificates,
		}
	}

	if strings.Contains(addr, ",") || config.Bool("redis_scheduler_cluster_mode", false) {
		redisConfig := goredis.ClusterOptions{
			Addrs:        strings.Split(addr, ","),
			PoolSize:     config.Int("redis_scheduler.client.pool_size", 10),
			MinIdleConns: config.Int("redis_scheduler.client.min_idle_conns", 5),
			IdleTimeout:  config.Duration("redis_scheduler.client.idle_timeout", 1, time.Minute),
			MaxConnAge:   config.Duration("redis_scheduler.client.max_conn_age", 10, time.Minute),
			PoolTimeout:  config.Duration("redis_scheduler.client.pool_timeout", 2, time.Second),
			ReadTimeout:  config.Duration("redis_scheduler.client.read_timeout", 1, time.Second),
			WriteTimeout: config.Duration("redis_scheduler.client.write_timeout", 1, time.Second),
			TLSConfig:    tlsConfig,
			Username:     config.String("redis_scheduler.client.username"),
			Password:     config.String("redis_scheduler.client.password"),
		}

		redisClient = goredis.NewClusterClient(&redisConfig)
	} else {
		redisConfig := goredis.Options{
			Addr:         addr,
			PoolSize:     config.Int("redis_scheduler.client.pool_size", 10),
			MinIdleConns: config.Int("redis_scheduler.client.min_idle_conns", 5),
			IdleTimeout:  config.Duration("redis_scheduler.client.idle_timeout", 1, time.Minute),
			MaxConnAge:   config.Duration("redis_scheduler.client.max_conn_age", 10, time.Minute),
			PoolTimeout:  config.Duration("redis_scheduler.client.pool_timeout", 2, time.Second),
			ReadTimeout:  config.Duration("redis_scheduler.client.read_timeout", 1, time.Second),
			WriteTimeout: config.Duration("redis_scheduler.client.write_timeout", 1, time.Second),
			TLSConfig:    tlsConfig,
			Username:     config.String("redis_scheduler.client.username"),
			Password:     config.String("redis_scheduler.client.password"),
		}

		redisClient = goredis.NewClient(&redisConfig)
	}

	redisClient = telemetry.WrapRedis(redisClient)

	clientStore := &redis.ClientStore{Client: redisClient}
	scheduler := redis.NewScheduler(
		redisClient,
		config.Duration("redis_scheduler.done_schedule_ttl", 0, timeDay),
		redis.QOptions{
			Workers:         config.Int("redis_scheduler.workers", 1),
			ReadShards:      config.Int("redis_scheduler.read_shards", 1),
			WriteShards:     config.Int("redis_scheduler.write_shards", 1),
			PollInterval:    config.Duration("redis_scheduler.poll_interval", 100, time.Millisecond),
			PreFetchCount:   config.Int("redis_scheduler.prefetch_count", 100),
			ReclaimInterval: config.Duration("redis_scheduler.reclaim_interval", 5, time.Minute),
		},
	)

	return scheduler, clientStore
}

const timeDay = 24 * time.Hour
