package saramax

import (
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/Kirby980/study/webook/pkg/logger"
)

type PickBatchProConsumer[T any] struct {
	l  logger.LoggerV1
	fn func(msg *sarama.ConsumerMessage, t T) error
}

func NewPickBatchProConsumer[T any](l logger.LoggerV1, fn func(msg *sarama.ConsumerMessage, t T) error) PickBatchProConsumer[T] {
	return PickBatchProConsumer[T]{
		l:  l,
		fn: fn,
	}
}

func (h PickBatchProConsumer[T]) Setup(session sarama.ConsumerGroupSession) error {
	h.l.Info("setup")
	return nil
}

func (h PickBatchProConsumer[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	h.l.Info("cleanup")
	return nil
}
func (h PickBatchProConsumer[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
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
		} else {
			session.MarkMessage(msg, "")
		}
	}
	return nil
}
