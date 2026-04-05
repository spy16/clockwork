package kafka

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
)

func TestNewChannel(t *testing.T) {
	ch, err := NewChannel(Config{
		"bootstrap.servers": "localhost:9092",
	})
	require.NoError(t, err)
	require.NotNil(t, ch)
	_ = ch.Close()
}

func TestChannel_Publish(t *testing.T) {
	var (
		sampleClient = client.Client{
			ChannelName: "sample-events",
		}

		sampleSchedule = schedule.Schedule{
			ID: "sample-schedule",
		}
	)

	produced := int64(0)
	ch := &Channel{
		producer: fakeProducer(func(msg *kafka.Message, deliveryChan chan kafka.Event) error {
			defer close(deliveryChan)

			assert.Equal(t, *msg.TopicPartition.Topic, sampleClient.ChannelName)
			atomic.AddInt64(&produced, 1)
			return nil
		}),
	}

	err := ch.Publish(context.Background(), sampleClient, sampleSchedule, schedule.Execution{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), produced)
}

type fakeProducer func(msg *kafka.Message, deliveryChan chan kafka.Event) error

func (fp fakeProducer) Produce(msg *kafka.Message, deliveryChan chan kafka.Event) error {
	return fp(msg, deliveryChan)
}
