package saramax

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kirby980/go-pkg/logger"
)

type BatchConsumer[T any] struct {
	l             logger.Logger
	fn            func(msg []*sarama.ConsumerMessage, t []T) error
	batchSize     int
	batchDuration time.Duration
}

// NewBathConsumer 创建一个批量消费者
func NewBathConsumer[T any](l logger.Logger, fn func(msg []*sarama.ConsumerMessage, t []T) error) *BatchConsumer[T] {
	return &BatchConsumer[T]{
		l:             l,
		fn:            fn,
		batchSize:     10,
		batchDuration: time.Second,
	}
}

func (h BatchConsumer[T]) Setup(session sarama.ConsumerGroupSession) error {
	h.l.Info("setup")
	return nil
}

func (h BatchConsumer[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	h.l.Info("cleanup")
	return nil
}
func (h BatchConsumer[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	//批量消费
	msgsCh := claim.Messages()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), h.batchDuration)
		done := false
		msgs := make([]*sarama.ConsumerMessage, 0, h.batchSize)
		ts := make([]T, 0, h.batchSize)
		for i := 0; i < h.batchSize && !done; i++ {
			select {
			case msg, ok := <-msgsCh:
				if !ok {
					cancel()
					// 代表消费者被关闭了
					return nil
				}
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					h.l.Error("反序列化失败", logger.Error(err), logger.String("topic", msg.Topic),
						logger.Int64("offset", msg.Offset), logger.Int32("partition", msg.Partition))
					continue
				}
				msgs = append(msgs, msg)
				ts = append(ts, t)
			case <-ctx.Done():
				done = true
			}
		}
		cancel()
		if len(msgs) == 0 {
			continue
		}
		err := h.fn(msgs, ts)
		if err != nil {
			h.l.Error("调用批量接口失败")
		}
		for _, v := range msgs {
			session.MarkMessage(v, "")
		}

	}
}
