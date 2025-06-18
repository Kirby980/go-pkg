package events

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
)

// Producer 生产者
type Producer interface {
	ProduceInconsistentEvent(ctx context.Context, event InconsistentEvent) error
}

// SaramaProducer sarama 生产者
type SaramaProducer struct {
	p     sarama.SyncProducer
	topic string
}

// NewSaramaProducer 创建一个 sarama 生产者
func NewSaramaProducer(
	p sarama.SyncProducer,
	topic string) *SaramaProducer {
	return &SaramaProducer{p: p, topic: topic}
}

// ProduceInconsistentEvent 生产不一致事件
func (s *SaramaProducer) ProduceInconsistentEvent(ctx context.Context,
	event InconsistentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, _, err = s.p.SendMessage(&sarama.ProducerMessage{
		Topic: s.topic,
		Value: sarama.ByteEncoder(data),
	})
	return err
}
