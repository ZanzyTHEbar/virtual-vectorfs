package harnessports

import "context"

// Tracer emits spans/metrics for observability.
type Tracer interface {
	StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, func(err error))
	Event(ctx context.Context, name string, attrs map[string]any)
}
