package harnessports

import "context"

// RateLimiter coordinates throughput across providers/models.
type RateLimiter interface {
	Acquire(ctx context.Context, key string) (release func(), err error)
}
