package cronx

import (
	"context"
	"strconv"
	"time"

	"github.com/Kirby980/go-pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/trace"
)

type CronJobBuilder struct {
	l      logger.Logger
	p      *prometheus.SummaryVec
	tracer trace.Tracer
}

func NewCronJobBuilder(l logger.Logger, p prometheus.SummaryOpts, labelNames []string, tracer trace.Tracer) *CronJobBuilder {
	ps := prometheus.NewSummaryVec(p, labelNames)
	prometheus.MustRegister(ps)
	return &CronJobBuilder{l: l, p: ps, tracer: tracer}
}
func (b *CronJobBuilder) Build(job Job) cron.Job {
	name := job.Name()
	return cronJobFuncAdapter(func() error {
		ctx, span := b.tracer.Start(context.Background(), name)
		defer span.End()
		start := time.Now()
		b.l.Info("任务开始",
			logger.String("job", name))
		var success bool
		defer func() {
			b.l.Info("任务结束",
				logger.String("job", name))
			duration := time.Since(start).Milliseconds()
			b.p.WithLabelValues(name,
				strconv.FormatBool(success)).Observe(float64(duration))
		}()
		err := job.Run(ctx)
		success = err == nil
		if err != nil {
			span.RecordError(err)
			b.l.Error("运行任务失败", logger.Error(err),
				logger.String("job", name))
		}
		return nil
	})
}

type cronJobFuncAdapter func() error

func (c cronJobFuncAdapter) Run() {
	_ = c()
}
