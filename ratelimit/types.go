package ratelimit

import (
	"context"
)

//go:generate mockgen -source=./types.go -package=limitermocks -destination=./mocks/limiter.mock.go Limiter
type Limiter interface {
	// Limit 限流
	Limit(ctx context.Context, key string) (bool, error)
}
