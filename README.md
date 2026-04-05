# Clockwork

[![CI](https://github.com/spy16/clockwork/actions/workflows/ci.yml/badge.svg)](https://github.com/spy16/clockwork/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/spy16/clockwork)](https://goreportcard.com/report/github.com/spy16/clockwork)
[![GoDoc](https://pkg.go.dev/badge/github.com/spy16/clockwork.svg)](https://pkg.go.dev/github.com/spy16/clockwork)
[![License](https://img.shields.io/github/license/spy16/clockwork)](./LICENSE)

A distributed, fault-tolerant cron scheduler. Define schedules using standard crontab expressions and Clockwork delivers notification events to your services at the right time via Kafka (or other pluggable channels).

## Features

- Single binary, easy to deploy
- Pluggable scheduler backends — in-memory (for dev/testing) and Redis (for production)
- REST API for managing clients and schedules
- Standard crontab expressions with extensions (e.g., `@at` for one-shot timestamps)
- Schedule change events published to Kafka for downstream consumers

## Quick Start

```bash
# in-memory backend (no dependencies, great for local dev)
clockwork agent --addr :8081 --backend in_memory

# redis backend
clockwork agent --addr :8081 --backend redis
```

See [`clockwork.yml`](./clockwork.yml) for all configuration options. All config keys can also be set via environment variables.

## Building

Requires Go 1.25+.

```bash
make          # test + build (output: ./out/clockwork)
make install  # install to GOBIN
```

## Running with TLS

Generate certs, start Redis and Kafka via Docker, then point Clockwork at the certs:

```bash
./setup_certs.sh
docker compose up --remove-orphans

# Redis TLS
export REDIS_SCHEDULER_CLIENT_SSL_ENABLED=true
export REDIS_SCHEDULER_CLIENT_SSL_CERT="$(cat certs/redis/client.crt)"
export REDIS_SCHEDULER_CLIENT_SSL_KEY="$(cat certs/redis/client-unencrypted.key)"
export REDIS_SCHEDULER_CLIENT_SSL_CA_CERT="$(cat certs/redis/root.crt)"
export REDIS_SCHEDULER_CLIENT_USERNAME=default
export REDIS_SCHEDULER_CLIENT_PASSWORD=redis123

# Kafka TLS (same certs for both event channel and changelog)
export KAFKA_SSL_ENABLED=true
export KAFKA_BROKERS=localhost:9093
export KAFKA_SSL_CERT="$(cat certs/kafka/client.crt)"
export KAFKA_SSL_KEY="$(cat certs/kafka/client-unencrypted.key)"
export KAFKA_SSL_CA_CERT="$(cat certs/kafka/root.crt)"
export CHANGELOG_KAFKA_SSL_ENABLED=true
export CHANGELOG_KAFKA_BROKERS=localhost:9093
export CHANGELOG_KAFKA_SSL_CERT="$(cat certs/kafka/client.crt)"
export CHANGELOG_KAFKA_SSL_KEY="$(cat certs/kafka/client-unencrypted.key)"
export CHANGELOG_KAFKA_SSL_CA_CERT="$(cat certs/kafka/root.crt)"
```

## License

See [LICENSE](./LICENSE).
