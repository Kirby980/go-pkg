package saramax

import (
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/Kirby980/study/webook/pkg/logger"
)

type Handler[T any] struct {
	l  logger.Logger
	fn func(msg *sarama.ConsumerMessage, t T) error
}

// NewHandler 一个基本的sarama consumer的handler
func NewHandler[T any](l logger.Logger, fn func(msg *sarama.ConsumerMessage, t T) error) Handler[T] {
	return Handler[T]{
		l:  l,
		fn: fn,
	}
}

func (h Handler[T]) Setup(session sarama.ConsumerGroupSession) error {
	h.l.Info("setup")
	return nil
}

func (h Handler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	h.l.Info("cleanup")
	return nil
}
func (h Handler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgs := claim.Messages()
	for msg := range msgs {
		var t T
		err := json.Unmarshal(msg.Value, &t)
		if err != nil {
			h.l.Error("反序列化失败", logger.Error(err), logger.String("topic", msg.Topic),
				logger.Int64("offset", msg.Offset), logger.Int32("partition", msg.Partition))
			continue
		}
		for i := 0; i < 3; i++ {
			err = h.fn(msg, t)
			if err == nil {
				break
			}
			h.l.Error("处理失败", logger.Error(err), logger.String("topic", msg.Topic),
				logger.Int64("offset", msg.Offset), logger.Int32("partition", msg.Partition))
		}
		if err != nil {
			h.l.Error("处理失败--重试超上限", logger.Error(err), logger.String("topic", msg.Topic),
				logger.Int64("offset", msg.Offset), logger.Int32("partition", msg.Partition))
		}
		session.MarkMessage(msg, "")

	}
	return nil
}
