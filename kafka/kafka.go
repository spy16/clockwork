package kafka

import (
	"context"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spy16/clockwork/pkg/telemetry"
	"go.opencensus.io/trace"
)

type kafkaProducer interface {
	Produce(msg *kafka.Message, deliveryChan chan kafka.Event) error
}

func publishKafka(ctx context.Context, p kafkaProducer, msg *kafka.Message) error {
	ctx, span := trace.StartSpan(ctx, "Kafka_Publish")
	defer span.End()
	span.AddAttributes(
		trace.StringAttribute("topic", *msg.TopicPartition.Topic),
		trace.Int64Attribute("value_size", int64(len(msg.Value))),
	)

	metric := telemetry.
		Incr("kafka_publish", 1).
		Tag("topic", *msg.TopicPartition.Topic)
	defer metric.Publish()

	deliveryCh := make(chan kafka.Event, 1)
	if err := p.Produce(msg, deliveryCh); err != nil {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeInternal,
			Message: fmt.Sprintf("produce failed: %v", err),
		})
		metric.Status("failure").Tag("cause", "produce_failed")
		return err
	}

	select {
	case <-ctx.Done():
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeCancelled,
			Message: fmt.Sprintf("%v", ctx.Err()),
		})
		metric.Status("unknown").Tag("cause", "context_cancelled")
		return ctx.Err()

	case ev := <-deliveryCh:
		if err, ok := ev.(*kafka.Error); ok {
			span.SetStatus(trace.Status{
				Code:    trace.StatusCodeInternal,
				Message: fmt.Sprintf("delivery failed: %v", err),
			})
			metric.Status("failure").Tag("cause", "delivery_error")
			return err
		}

		metric.Status("success")
		return nil
	}
}
