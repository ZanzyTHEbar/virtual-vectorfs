package harness

import (
	"context"
	"database/sql"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/adapters"
	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
	"github.com/rs/zerolog"
)

// Factory creates and wires harness components from configuration.
type Factory struct {
	harnessConfig *config.HarnessConfig
	db            *sql.DB // Optional, for conversation store
	logger        zerolog.Logger
}

// NewFactory creates a new harness factory.
func NewFactory(harnessConfig *config.HarnessConfig, db *sql.DB, logger zerolog.Logger) *Factory {
	return &Factory{
		harnessConfig: harnessConfig,
		db:            db,
		logger:        logger,
	}
}

// CreateOrchestrator creates a fully wired HarnessOrchestrator from config.
func (f *Factory) CreateOrchestrator() (*HarnessOrchestrator, error) {
	// Create adapters from config
	cache := f.createCache()
	limiter := f.createRateLimiter()
	tracer := f.createTracer()
	store := f.createStore()

	// Create core components
	builder := NewPromptBuilder()
	assembler := NewContextAssembler(
		Budget{
			MaxContextTokens: 4000,
			MaxSnippets:      10,
		},
		nil, // Use default token estimator
	)

	// Create orchestrator
	orchestrator := NewHarnessOrchestrator(
		nil, // Provider must be injected separately (inference-specific)
		builder,
		assembler,
		store,
		cache,
		limiter,
		tracer,
	)

	return orchestrator, nil
}

// CreateCache creates a cache adapter from config.
func (f *Factory) createCache() ports.Cache {
	if !f.harnessConfig.CacheEnabled {
		return &noOpCache{}
	}

	return adapters.NewLRUCache(f.harnessConfig.CacheCapacity)
}

// CreateRateLimiter creates a rate limiter adapter from config.
func (f *Factory) createRateLimiter() ports.RateLimiter {
	if !f.harnessConfig.RateLimitEnabled {
		return &noOpRateLimiter{}
	}

	// Use the configured refill rate (already a time.Duration)
	refillRate := f.harnessConfig.RateLimitRefillRate

	return adapters.NewTokenBucket(f.harnessConfig.RateLimitCapacity, refillRate)
}

// CreateTracer creates a tracer adapter from config.
func (f *Factory) createTracer() ports.Tracer {
	if !f.harnessConfig.EnableTracing {
		return &noOpTracer{}
	}

	return adapters.NewZerologTracer(f.logger)
}

// CreateStore creates a conversation store adapter from config.
func (f *Factory) createStore() ports.ConversationStore {
	if f.db == nil {
		return &noOpStore{}
	}

	return adapters.NewLibSQLConversationStore(f.db)
}

// CreateGuardrails creates guardrails from config.
func (f *Factory) CreateGuardrails() *Guardrails {
	guardrails := NewGuardrails()

	if f.harnessConfig.EnableGuardrails {
		// Add allowed tools from config
		for _, toolName := range f.harnessConfig.AllowedTools {
			guardrails.AddAllowedTool(toolName)
		}

		// Set blocked words from config
		if len(f.harnessConfig.BlockedWords) > 0 {
			// Note: Guardrails struct doesn't currently support dynamic blocked words
			// This would need to be extended in the Guardrails struct
		}
	}

	return guardrails
}

// CreatePolicy creates a policy from config with validation.
func (f *Factory) CreatePolicy() *Policy {
	policy := &Policy{
		MaxToolDepth:      f.harnessConfig.MaxToolDepth,
		MaxIterations:     f.harnessConfig.MaxIterations,
		ToolTimeout:       30 * time.Second,
		RequireJSONOutput: false,
		Deterministic:     false,
		RetryCount:        2,
		RetryBackoff:      100 * time.Millisecond,
	}

	// Validate and clamp policy values
	if policy.MaxToolDepth < 1 {
		policy.MaxToolDepth = 1
		f.logger.Warn().Int("max_tool_depth", f.harnessConfig.MaxToolDepth).Msg("MaxToolDepth clamped to minimum of 1")
	}
	if policy.MaxToolDepth > 10 {
		policy.MaxToolDepth = 10
		f.logger.Warn().Int("max_tool_depth", f.harnessConfig.MaxToolDepth).Msg("MaxToolDepth clamped to maximum of 10")
	}

	if policy.MaxIterations < 1 {
		policy.MaxIterations = 1
		f.logger.Warn().Int("max_iterations", f.harnessConfig.MaxIterations).Msg("MaxIterations clamped to minimum of 1")
	}
	if policy.MaxIterations > 50 {
		policy.MaxIterations = 50
		f.logger.Warn().Int("max_iterations", f.harnessConfig.MaxIterations).Msg("MaxIterations clamped to maximum of 50")
	}

	return policy
}

// noOpCache implements Cache interface with no-op behavior for testing/disabled cache.
type noOpCache struct{}

func (c *noOpCache) Get(ctx context.Context, key string) ([]byte, bool) { return nil, false }
func (c *noOpCache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (c *noOpCache) Delete(ctx context.Context, key string) error { return nil }

// noOpRateLimiter implements RateLimiter interface with no-op behavior.
type noOpRateLimiter struct{}

func (r *noOpRateLimiter) Acquire(ctx context.Context, key string) (release func(), err error) {
	return func() {}, nil
}

// noOpTracer implements Tracer interface with no-op behavior.
type noOpTracer struct{}

func (t *noOpTracer) StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, func(err error)) {
	return ctx, func(err error) {}
}

func (t *noOpTracer) Event(ctx context.Context, name string, attrs map[string]any) {}

// noOpStore implements ConversationStore interface with no-op behavior.
type noOpStore struct{}

func (s *noOpStore) SaveTurn(ctx context.Context, conversationID string, turn ports.Turn) error {
	return nil
}

func (s *noOpStore) LoadContext(ctx context.Context, conversationID string, k int) ([]ports.Turn, error) {
	return nil, nil
}

func (s *noOpStore) AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error {
	return nil
}

// Ensure all no-op types implement their interfaces.
var (
	_ ports.Cache             = (*noOpCache)(nil)
	_ ports.RateLimiter       = (*noOpRateLimiter)(nil)
	_ ports.Tracer            = (*noOpTracer)(nil)
	_ ports.ConversationStore = (*noOpStore)(nil)
)
