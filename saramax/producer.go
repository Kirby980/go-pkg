package saramax

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/Kirby980/go-pkg/logger"
)

//go:generate mockgen -source=./producer.go -package=eventmock -destination=./mocks/article_producer.mock.go Producer
type Producer interface {
	Producer(ctx context.Context, evt any, topic string) error
	BatchProducer(ctx context.Context, evt any, topic string) error
}

type KafkaProducer struct {
	l         logger.Logger
	producer  sarama.SyncProducer
	aproducer sarama.AsyncProducer
}

func (k *KafkaProducer) BatchProducer(ctx context.Context, evt any, topic string) error {
	k.l.Debug("批量发送消息", logger.Field{Key: "evt", Value: evt})
	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: NewJsonEncoder(k.l, evt),
	})
	return err
}

func (k *KafkaProducer) Producer(ctx context.Context, evt any, topic string) error {
	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: NewJsonEncoder(k.l, evt),
	})
	return err
}

func NewKafkaProducer(l logger.Logger, producer sarama.SyncProducer, aproducer sarama.AsyncProducer) Producer {
	return &KafkaProducer{
		l:         l,
		producer:  producer,
		aproducer: aproducer,
	}
}

type ReadEvent struct {
	Uid int64
	Aid int64
}
type ReadEvents struct {
	Uids []int64
	Aids []int64
}
