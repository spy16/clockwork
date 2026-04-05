package kafka

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spy16/clockwork/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChangeLogger(t *testing.T) {
	logger, err := NewChangeLogger("foo", Config{
		"bootstrap.servers": "localhost:9092",
	})
	require.NoError(t, err)
	require.NotNil(t, logger)
	_ = logger.Close()
}

func TestChangeLogger_Publish(t *testing.T) {
	sampleSchedule := schedule.Schedule{
		ID: "sample-schedule",
	}

	produced := int64(0)
	ch := &ChangeLogger{topic: "foo"}
	ch.producer = fakeProducer(func(msg *kafka.Message, deliveryChan chan kafka.Event) error {
		defer close(deliveryChan)

		assert.Equal(t, *msg.TopicPartition.Topic, ch.topic)
		atomic.AddInt64(&produced, 1)
		return nil
	})

	err := ch.Publish(context.Background(), 1, sampleSchedule)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), produced)
}
