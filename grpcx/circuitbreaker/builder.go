package circuitbreaker

import (
	"context"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InterceptorBuilder 熔断器拦截器构建器 基于kratos的构造器
type InterceptorBuilder struct {
	breaker circuitbreaker.CircuitBreaker
}

// NewInterceptorBuilder 创建熔断器拦截器构建器
func NewInterceptorBuilder(opts ...sre.Option) *InterceptorBuilder {
	return &InterceptorBuilder{
		breaker: sre.NewBreaker(opts...),
	}
}

// BuildServerInterceptor 构建服务端拦截器
func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if b.breaker.Allow() == nil {
			resp, err := handler(ctx, req)
			if err != nil {
				b.breaker.MarkFailed()
			} else {
				b.breaker.MarkSuccess()
			}
			return resp, err
		}
		b.breaker.MarkFailed()
		// 触发了熔断器
		return nil, status.Errorf(codes.Aborted, "服务不可用")
	}
}
