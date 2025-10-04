package adapters

import (
	"context"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
	"github.com/rs/zerolog"
)

// ZerologTracer implements the Tracer interface using zerolog.
type ZerologTracer struct {
	logger zerolog.Logger
}

// NewZerologTracer creates a new zerolog tracer.
func NewZerologTracer(logger zerolog.Logger) *ZerologTracer {
	return &ZerologTracer{
		logger: logger,
	}
}

// StartSpan starts a new tracing span and returns the context and finish function.
func (t *ZerologTracer) StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, func(err error)) {
	// Create a child logger with the span name
	spanLogger := t.logger.With().Str("span", name).Logger()

	// Add attributes to logger
	for k, v := range attrs {
		spanLogger = spanLogger.With().Interface(k, v).Logger()
	}

	// Store logger in context for use in events
	ctx = context.WithValue(ctx, "zerolog_span_logger", spanLogger)

	startTime := time.Now()

	// Log span start
	spanLogger.Info().Str("event", "span_start").Time("start_time", startTime).Msg("Starting span")

	finish := func(err error) {
		duration := time.Since(startTime)

		event := spanLogger.Info()
		if err != nil {
			event = spanLogger.Error().Err(err)
		}

		event.
			Str("event", "span_end").
			Dur("duration", duration).
			Time("end_time", time.Now()).
			Msg("Ending span")
	}

	return ctx, finish
}

// Event logs a tracing event with the current span context.
func (t *ZerologTracer) Event(ctx context.Context, name string, attrs map[string]any) {
	// Try to get span logger from context
	if logger, ok := ctx.Value("zerolog_span_logger").(zerolog.Logger); ok {
		event := logger.Info()

		// Add attributes
		for k, v := range attrs {
			event = event.Interface(k, v)
		}

		event.
			Str("event", name).
			Time("timestamp", time.Now()).
			Msg("Tracing event")
	} else {
		// Fallback to main logger if no span context
		event := t.logger.Info()

		for k, v := range attrs {
			event = event.Interface(k, v)
		}

		event.
			Str("event", name).
			Time("timestamp", time.Now()).
			Msg("Tracing event (no span context)")
	}
}

// Ensure ZerologTracer implements the Tracer interface.
var _ ports.Tracer = (*ZerologTracer)(nil)
