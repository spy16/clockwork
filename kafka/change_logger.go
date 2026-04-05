package kafka

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/spy16/clockwork/api/proto"
	"github.com/spy16/clockwork/schedule"
)

func NewChangeLogger(topic string, cfg Config) (*ChangeLogger, error) {
	p, err := kafka.NewProducer(&cfg)
	if err != nil {
		return nil, err
	}
	return &ChangeLogger{producer: p, topic: topic}, nil
}

type ChangeLogger struct {
	topic    string
	producer kafkaProducer
}

// Publish generates and pushes a schedule-change event to kafka. changeType is
// one of 1:created, 2:updated, 3:deleted.
func (cl *ChangeLogger) Publish(ctx context.Context, changeType int, sc schedule.Schedule) error {
	msg, err := cl.changeMessage(changeType, sc)
	if err != nil {
		return err
	}
	return publishKafka(ctx, cl.producer, msg)
}

func (cl *ChangeLogger) Close() error {
	if closer, ok := cl.producer.(interface{ Close() }); ok {
		closer.Close()
	}
	return nil
}

func (cl *ChangeLogger) changeMessage(changeType int, sc schedule.Schedule) (*kafka.Message, error) {
	changeTypeEnum := map[int]proto.ScheduleChangeType{
		1: proto.ScheduleChangeType_CREATED,
		2: proto.ScheduleChangeType_UPDATED,
		3: proto.ScheduleChangeType_DELETED,
	}

	key := &proto.ScheduleChangeEventKey{
		ClientId:   sc.ClientID,
		ScheduleId: sc.ID,
	}
	val := &proto.ScheduleChangeEvent{
		DoneAt: timestamppb.Now(),
		Schedule: &proto.Schedule{
			ScheduleId: sc.ID,
			Crontab:    sc.Crontab,
			Category:   sc.Category,
			ClientId:   sc.ClientID,
			Tags:       sc.Tags,
			Payload:    sc.Payload,
			Status:     sc.Status,
			JsonDump:   jsonStr(sc),
		},
		ChangeType: changeTypeEnum[changeType],
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
			Topic:     &cl.topic,
			Partition: kafka.PartitionAny,
		},
	}, nil
}

func jsonStr(sc schedule.Schedule) string {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(sc); err != nil {
		panic(err)
	}
	return buf.String()
}
