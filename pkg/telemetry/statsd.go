package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/rs/zerolog/log"
)

var client *statsd.Client

const tagStatus = "status"

func setupStatsD(ctx context.Context, cfg Config) (err error) {
	client, err = statsd.New(strings.TrimSpace(cfg.StatsdAddr))
	if err != nil {
		return err
	}
	client.Namespace = strings.TrimSuffix(strings.TrimSpace(cfg.StatsdNamespace), ".") + "."
	client.Tags = cleanupTags(cfg.StatsdTags)

	if cfg.PublishRuntime {
		go func() {
			_ = newCollector(func(key string, val uint64) {
				if err := client.Gauge(key, float64(val), nil, 1); err != nil {
					log.Error().Err(err).Msg("failed to publish runtime metrics")
				}
			}).Run(ctx)
		}()
	}

	return nil
}

// Incr returns a increment counter metric.
func Incr(name string, rate float64) *Metric {
	return &Metric{
		name: name,
		publishFunc: func(name string, tags []string) error {
			if client == nil {
				return nil
			}
			return client.Incr(name, tags, rate)
		},
	}
}

// Timing returns a timer metric.
func Timing(name string, value time.Duration, rate float64) *Metric {
	return &Metric{
		name: name,
		publishFunc: func(name string, tags []string) error {
			if client == nil {
				return nil
			}
			return client.Timing(name, value, tags, rate)
		},
	}
}

// Gauge creates and returns a new gauge metric.
func Gauge(name string, value float64, rate float64) *Metric {
	return &Metric{
		name: name,
		publishFunc: func(name string, tags []string) error {
			if client == nil {
				return nil
			}
			return client.Gauge(name, value, tags, rate)
		},
	}
}

// Metric represents a statsd metric. Not safe for concurrent use.
type Metric struct {
	name        string
	tags        map[string]string
	publishFunc func(name string, tags []string) error
}

// Status sets the status tag for the metric. If not explicitly set, status
// success is automatically added when published.
func (m *Metric) Status(status string) *Metric {
	return m.Tag(tagStatus, strings.TrimSpace(strings.ToLower(status)))
}

// Tag adds a tag to the metric.
func (m *Metric) Tag(key string, val string) *Metric {
	if m == nil {
		return nil
	}

	if m.tags == nil {
		m.tags = map[string]string{}
	}

	m.tags[key] = val
	return m
}

// Publish publishes the metric with collected tags. Intended to
// be used with defer.
func (m *Metric) Publish() {
	if m == nil {
		return
	}

	if m.tags == nil {
		m.tags = map[string]string{}
	}

	if _, found := m.tags[tagStatus]; !found {
		m.tags[tagStatus] = "success"
	}

	tags := m.tagSlice()
	if err := m.publishFunc(m.name, tags); err != nil {
		log.Error().Err(err).Msg("failed to publish metric")
		return
	}
}

func (m *Metric) tagSlice() []string {
	var tags []string
	for k, v := range m.tags {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}
	return tags
}

func cleanupTags(tags []string) []string {
	var result []string

	for _, tag := range tags {
		tag = strings.TrimSpace(strings.Trim(tag, ","))

		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}
