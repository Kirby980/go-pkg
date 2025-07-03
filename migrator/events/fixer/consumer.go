package fixer

import (
	"context"
	"errors"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kirby980/go-pkg/logger"
	"github.com/Kirby980/go-pkg/migrator"
	"github.com/Kirby980/go-pkg/migrator/events"
	"github.com/Kirby980/go-pkg/migrator/fixer"
	"github.com/Kirby980/go-pkg/saramax"
	"gorm.io/gorm"
)

// Consumer 消费者
type Consumer[T migrator.Entity] struct {
	client   sarama.Client
	l        logger.Logger
	srcFirst *fixer.OverrideFixer[T]
	dstFirst *fixer.OverrideFixer[T]
	topic    string
}

// NewConsumer 创建一个消费者
func NewConsumer[T migrator.Entity](
	client sarama.Client,
	l logger.Logger,
	topic string,
	src *gorm.DB,
	dst *gorm.DB) (*Consumer[T], error) {
	srcFirst, err := fixer.NewOverrideFixer[T](src, dst)
	if err != nil {
		return nil, err
	}
	dstFirst, err := fixer.NewOverrideFixer[T](dst, src)
	if err != nil {
		return nil, err
	}
	return &Consumer[T]{
		client:   client,
		l:        l,
		srcFirst: srcFirst,
		dstFirst: dstFirst,
		topic:    topic,
	}, nil
}

// Start 这边就是自己启动 goroutine 了
func (r *Consumer[T]) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("migrator-fix",
		r.client)
	if err != nil {
		return err
	}
	go func() {
		err := cg.Consume(context.Background(),
			[]string{r.topic},
			saramax.NewHandler(r.l, r.Consume))
		if err != nil {
			r.l.Error("退出了消费循环异常", logger.Error(err))
		}
	}()
	return err
}

// Consume 消费消息
func (r *Consumer[T]) Consume(msg *sarama.ConsumerMessage, t events.InconsistentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r.l.Info("收到数据不一致事件",
		logger.Int64("id", t.ID),
		logger.String("direction", t.Direction),
		logger.String("type", t.Type))

	var err error
	switch t.Direction {
	case "SRC":
		err = r.srcFirst.Fix(ctx, t.ID)
	case "DST":
		err = r.dstFirst.Fix(ctx, t.ID)
	default:
		err = errors.New("未知的校验方向")
	}

	if err != nil {
		r.l.Error("修复数据失败",
			logger.Int64("id", t.ID),
			logger.String("direction", t.Direction),
			logger.Error(err))
	} else {
		r.l.Info("数据修复成功",
			logger.Int64("id", t.ID),
			logger.String("direction", t.Direction))
	}

	return err
}
