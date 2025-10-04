package harnessports

import "context"

// Cache provides idempotent memoization for prompt → completion.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool)
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
	Delete(ctx context.Context, key string) error
}
