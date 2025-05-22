package redisx

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

type PrometheusHook struct {
	vector *prometheus.SummaryVec
}

func NewPrometheusHook(opt prometheus.SummaryOpts, lables ...string) *PrometheusHook {
	vector := prometheus.NewSummaryVec(opt, lables)
	prometheus.MustRegister(vector)
	return &PrometheusHook{
		vector: vector,
	}
}

func (p *PrometheusHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		var err error
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime).Milliseconds()
			keyExist := err == redis.Nil
			p.vector.WithLabelValues(cmd.Name(), strconv.FormatBool(keyExist)).Observe(float64(duration))
		}()
		err = next(ctx, cmd)
		return err
	}
}
func (p *PrometheusHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 相当于，你这里啥也不干
		return next(ctx, network, addr)
	}
}
func (p *PrometheusHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		var err error
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime).Milliseconds()
			p.vector.WithLabelValues("pipeline", "false").Observe(float64(duration))
		}()
		err = next(ctx, cmds)
		return err
	}
}
