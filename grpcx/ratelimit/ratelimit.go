package ratelimit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kirby980/study/webook/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CounterLimiter 计数器算法
type CounterLimiter struct {
	cnt       *atomic.Int32
	threshold int32
}

func (l *CounterLimiter) NewServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		cnt := l.cnt.Add(1)
		defer func() {
			l.cnt.Add(-1)
		}()
		if cnt > l.threshold {
			// 这里就是拒绝
			return nil, status.Errorf(codes.ResourceExhausted, "限流")
		}
		return handler(ctx, req)
	}
}

// FixedWindowLimiter 固定窗口算法
type FixedWindowLimiter struct {
	// 窗口大小
	window time.Duration
	// 上一个窗口的起始时间
	lastStart time.Time

	// 当前窗口的请求数量
	cnt int
	// 窗口允许的最大的请求数量
	threshold int

	lock sync.Mutex
}

func (l *FixedWindowLimiter) NewServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		l.lock.Lock()
		now := time.Now()
		// 要换窗口了
		if now.After(l.lastStart.Add(l.window)) {
			l.lastStart = now
			l.cnt = 0
		}
		l.cnt++
		if l.cnt <= l.threshold {
			l.lock.Unlock()
			res, err := handler(ctx, req)
			return res, err
		}
		l.lock.Unlock()
		return nil, status.Errorf(codes.ResourceExhausted, "限流了")
	}
}

// SlideWindowLimiter 滑动窗口算法
type SlideWindowLimiter struct {
	window time.Duration
	// 请求的时间戳
	queue     pkg.Queue[time.Time]
	lock      sync.Mutex
	threshold int
}

func (l *SlideWindowLimiter) NewServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		l.lock.Lock()
		now := time.Now()
		if l.queue.Len() < l.threshold {
			l.queue.Enqueue(time.Now())
			l.lock.Unlock()
			return handler(ctx, req)
		}
		windowStart := now.Add(-l.window)
		for {
			// 最早的请求
			first := l.queue.Peek()
			if first.Before(windowStart) {
				// 就是删了 first
				_ = l.queue.Dequeue()
			} else {
				// 退出循环
				break
			}
		}
		if l.queue.Len() < l.threshold {
			l.queue.Enqueue(time.Now())
			l.lock.Unlock()
			return handler(ctx, req)
		}
		l.lock.Unlock()
		return nil, status.Errorf(codes.ResourceExhausted, "限流了")
	}
}

// TokenBucketLimiter 令牌桶算法
type TokenBucketLimiter struct {
	//ch      *time.Ticker
	buckets chan struct{}
	// 每隔多久一个令牌
	interval  time.Duration
	closeOnce sync.Once
	closeCh   chan struct{}
}

// 把 capacity 设置成0，就是漏桶算法
func NewTokenBucketLimiter(interval time.Duration, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		interval: interval,
		buckets:  make(chan struct{}, capacity),
	}
}

func (l *TokenBucketLimiter) NewServerInterceptor() grpc.UnaryServerInterceptor {
	ticker := time.NewTicker(l.interval)
	go func() {
		for {
			select {
			case <-l.closeCh:
				return
			case <-ticker.C:
				select {
				case l.buckets <- struct{}{}:
				default:

				}
			}
		}
	}()
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		select {
		case <-l.buckets:
			// 拿到了令牌
			return handler(ctx, req)
			//default:
		// 就意味着你认为，没有令牌不应阻塞，直接返回
		//return nil, status.Errorf(codes.ResourceExhausted, "限流了")
		case <-ctx.Done():
			// 没有令牌就等令牌，直到超时
			return nil, ctx.Err()
		}
	}
}

func (l *TokenBucketLimiter) Close() error {
	l.closeOnce.Do(func() {
		close(l.closeCh)
	})
	return nil
}

// LeakBucketLimiter 漏桶算法
type LeakBucketLimiter struct {
	interval  time.Duration
	closeCh   chan struct{}
	closeOnce sync.Once
}

func NewLeakBucketLimiter(interval time.Duration) *LeakBucketLimiter {
	return &LeakBucketLimiter{
		interval: interval,
		closeCh:  make(chan struct{}),
	}
}

func (l *LeakBucketLimiter) NewServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(l.interval):
				// 漏桶算法
				return handler(ctx, req)
			case <-l.closeCh:
				return nil, errors.New("限流器被关了")

			}
		}
	}
}
func (l *LeakBucketLimiter) Close() error {
	l.closeOnce.Do(func() {
		close(l.closeCh)
	})
	return nil
}
