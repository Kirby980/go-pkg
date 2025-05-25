package saramax

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kirby980/study/webook/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	once   sync.Once
	vector *prometheus.SummaryVec
)

type PickBatchProConsumer[T any] struct {
	l      logger.LoggerV1
	fn     func(msg *sarama.ConsumerMessage, t T) error
	vector *prometheus.SummaryVec
}

// 创建一个新的PickBatchProConsumer实例，T为任意类型
// 该函数接收一个日志记录器、一个处理函数和Prometheus的SummaryOpts选项以及可选的标签
func NewPickBatchProConsumer[T any](l logger.LoggerV1, fn func(msg *sarama.ConsumerMessage, t T) error,
	opt prometheus.SummaryOpts, labels ...string) PickBatchProConsumer[T] {
	once.Do(func() {
		vector = prometheus.NewSummaryVec(opt, labels)
		prometheus.MustRegister(vector)
	})

	return PickBatchProConsumer[T]{
		l:      l,
		fn:     fn,
		vector: vector,
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
		h.consumeClaim(msg)
		session.MarkMessage(msg, "")

	}
	return nil
}

func (h *PickBatchProConsumer[T]) consumeClaim(msg *sarama.ConsumerMessage) {
	startTime := time.Now()
	var err error
	defer func() {
		errInfo := strconv.FormatBool(err != nil)
		duration := time.Since(startTime).Milliseconds()
		h.vector.WithLabelValues(msg.Topic, errInfo).Observe(float64(duration))
	}()
	var t T
	err = json.Unmarshal(msg.Value, &t)
	if err != nil {
		h.l.Error("反序列化失败", logger.Error(err), logger.String("topic", msg.Topic),
			logger.Int64("offset", msg.Offset), logger.Int32("partition", msg.Partition))
		return
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
}
