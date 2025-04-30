package ratelimit

import (
	"context"
)

type Limiter interface {
	// Limit 限流
	Limit(ctx context.Context, key string) (bool, error)
}
