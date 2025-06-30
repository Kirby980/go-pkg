package ratelimit

import (
	"context"
	"strings"

	"github.com/Kirby980/study/webook/pkg/logger"
	"github.com/Kirby980/study/webook/pkg/ratelimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ctxLimitedKeyType string

var ctxLimitedKey = ctxLimitedKeyType("limited")

type InterceptorBuilder struct {
	limiter *ratelimit.RedisSlidingWindowLimiter
	key     string
	l       logger.Logger
}

// NewInterceptorBuilder 创建限流拦截器构建器
func NewInterceptorBuilder(key string, l logger.Logger) *InterceptorBuilder {
	return &InterceptorBuilder{
		key: key,
		l:   l,
	}
}

// WithLimiter 设置限流器
func (b *InterceptorBuilder) WithLimiter(limiter *ratelimit.RedisSlidingWindowLimiter) *InterceptorBuilder {
	b.limiter = limiter
	return b
}

// BuildServerInterceptor 构建服务端拦截器
func (b *InterceptorBuilder) BuildServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			b.l.Error("限流器错误", logger.Error(err))
			return nil, status.Errorf(codes.Internal, "限流器错误")
		}
		if limited {
			return nil, status.Errorf(codes.ResourceExhausted, "请求过多")
		}
		return handler(ctx, req)
	}
}

// BuildClientInterceptor 构建客户端拦截器
func (b *InterceptorBuilder) BuildClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		limited, err := b.limiter.Limit(ctx, b.key)
		if err != nil {
			b.l.Error("限流器错误", logger.Error(err))
			return err
		}
		if limited {
			return status.Errorf(codes.ResourceExhausted, "请求过多")
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// BuildServerInterceptorDowngrade 构建服务端拦截器服务通知业务方进行限流，或者降级
func (b *InterceptorBuilder) BuildServerInterceptorDowngrade(limiter *ratelimit.Limiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		limiter, err := b.limiter.Limit(ctx, b.key)
		if err != nil || limiter {
			ctx = context.WithValue(ctx, ctxLimitedKey, true)
		}
		return handler(ctx, req)
	}
}

// BuildServerInterceptorService 服务级别限流
func (b *InterceptorBuilder) BuildServerInterceptorService() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if strings.HasPrefix(info.FullMethod, "/UserService") {
			limited, err := b.limiter.Limit(ctx, "limiter:service:user:UserService")
			if err != nil {
				b.l.Error("判定限流出现问题", logger.Error(err))
				return nil, status.Errorf(codes.ResourceExhausted, "触发限流")
			}
			if limited {
				return nil, status.Errorf(codes.ResourceExhausted, "触发限流")
			}
		}
		return handler(ctx, req)
	}
}
