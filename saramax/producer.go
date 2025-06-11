package saramax

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/Kirby980/study/webook/pkg/logger"
)

//go:generate mockgen -source=./producer.go -package=eventmock -destination=./mocks/article_producer.mock.go Producer
type Producer interface {
	ProducerReadEvent(ctx context.Context, evt ReadEvent, topic string) error
	BatchProducerReadEvent(ctx context.Context, evt ReadEvents, topic string) error
}

type KafkaProducer struct {
	l         logger.Logger
	producer  sarama.SyncProducer
	aproducer sarama.AsyncProducer
}

// BatchProducerReadEvent implements Producer.
func (k *KafkaProducer) BatchProducerReadEvent(ctx context.Context, evt ReadEvents, topic string) error {
	k.l.Debug("批量发送消息", logger.Field{Key: "evt", Value: evt})
	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: NewJsonEncoder(k.l, evt),
	})
	return err
}

// ProducerReadEvent implements Producer.
func (k *KafkaProducer) ProducerReadEvent(ctx context.Context, evt ReadEvent, topic string) error {
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
