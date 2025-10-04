package adapters

import (
	"context"
	"sync"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	capacity   int           // max tokens per bucket
	refillRate time.Duration // time between token refills
}

// bucket represents a single token bucket for a key.
type bucket struct {
	tokens     int
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket rate limiter.
func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
	return &TokenBucket{
		buckets:    make(map[string]*bucket),
		capacity:   capacity,
		refillRate: refillRate,
	}
}

// Acquire attempts to acquire a token for the given key.
func (tb *TokenBucket) Acquire(ctx context.Context, key string) (release func(), err error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, exists := tb.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     tb.capacity,
			lastRefill: time.Now(),
		}
		tb.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	elapsed := time.Since(b.lastRefill)
	tokensToAdd := int(elapsed / tb.refillRate)
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, tb.capacity)
		b.lastRefill = b.lastRefill.Add(time.Duration(tokensToAdd) * tb.refillRate)
	}

	// Check if we have a token available
	if b.tokens <= 0 {
		return nil, ErrRateLimitExceeded
	}

	// Consume a token
	b.tokens--

	// Return release function
	release = func() {
		tb.mu.Lock()
		defer tb.mu.Unlock()
		if b, exists := tb.buckets[key]; exists {
			b.tokens = min(b.tokens+1, tb.capacity)
		}
	}

	return release, nil
}

// ErrRateLimitExceeded is returned when the rate limit is exceeded.
var ErrRateLimitExceeded = &RateLimitError{Message: "rate limit exceeded"}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure TokenBucket implements the RateLimiter interface.
var _ ports.RateLimiter = (*TokenBucket)(nil)
