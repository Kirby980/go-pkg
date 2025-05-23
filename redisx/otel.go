package redisx

import (
	"context"
	"net"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type OTELHook struct {
	tracer trace.Tracer
}

func NewOTELHook(name string) *OTELHook {
	tp := otel.GetTracerProvider()
	tracer := tp.Tracer(name)
	return &OTELHook{
		tracer: tracer,
	}
}

func (p *OTELHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		ctx, span := p.tracer.Start(ctx, cmd.Name())
		defer span.End(trace.WithStackTrace(true))
		return next(ctx, cmd)
	}
}
func (p *OTELHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 相当于，你这里啥也不干
		return next(ctx, network, addr)
	}
}
func (p *OTELHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx, span := p.tracer.Start(ctx, "pipeline")
		defer span.End(trace.WithStackTrace(true))
		return next(ctx, cmds)
	}
}
