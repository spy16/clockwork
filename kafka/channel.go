package kafka

import (
	"context"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/spy16/clockwork/api/proto"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
)

type Config = kafka.ConfigMap

func NewChannel(conf Config) (*Channel, error) {
	p, err := kafka.NewProducer(&conf)
	if err != nil {
		return nil, err
	}
	return &Channel{producer: p}, nil
}

// Channel implements a clockwork job queue using Kafka.
type Channel struct{ producer kafkaProducer }

// Publish generates and pushes a clockwork event to kafka. The kafka message
// will have the schedule id as the key and schedule execution details as
// value. Since schedule id acts as partition key, it ensures events are
// published in clockwork order.
func (q *Channel) Publish(ctx context.Context, cl client.Client, sc schedule.Schedule, req schedule.Execution) error {
	msg, err := q.newMessage(cl.ChannelName, req, sc)
	if err != nil {
		return err
	}

	return publishKafka(ctx, q.producer, msg)
}

func (q *Channel) Close() error {
	if closer, ok := q.producer.(interface{ Close() }); ok {
		closer.Close()
	}
	return nil
}

func (q *Channel) newMessage(chName string, req schedule.Execution, sc schedule.Schedule) (*kafka.Message, error) {
	key := &proto.ExecutionEventKey{ScheduleId: sc.ID}
	val := &proto.ExecutionEvent{
		ScheduleId:       sc.ID,
		Category:         sc.Category,
		ExecuteAt:        timestamppb.New(req.EnqueueAt),
		ManuallyEnqueued: req.Manual,
		ExecutionNumber:  int64(sc.EnqueueCount),
		Payload:          sc.Payload,
	}

	keyBytes, err := protobuf.Marshal(key)
	if err != nil {
		return nil, err
	}

	valBytes, err := protobuf.Marshal(val)
	if err != nil {
		return nil, err
	}

	return &kafka.Message{
		Key:   keyBytes,
		Value: valBytes,
		TopicPartition: kafka.TopicPartition{
			Topic:     &chName,
			Partition: kafka.PartitionAny,
		},
	}, nil
}
