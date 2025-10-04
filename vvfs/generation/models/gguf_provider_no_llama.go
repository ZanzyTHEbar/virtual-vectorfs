//go:build !llama || no_llama

package models

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Placeholder for non-CGO builds
var llamaPackageNotAvailable = fmt.Errorf("llama.cpp not available in this build")

// GGUFProvider wraps llama.cpp models with Go-friendly interface (no-op for non-CGO)
type GGUFProvider struct {
	config       *GGUFModelConfig
	llamaModel   interface{} // Placeholder
	tempFilePath string
	health       *ModelHealth
	mu           sync.RWMutex

	// Pooling
	pool   chan interface{}
	poolMu sync.Mutex

	// Circuit breaker
	failureCount    int64
	lastFailureTime time.Time
	breakerMu       sync.Mutex

	// Logger
	logger *slog.Logger
}

// NewGGUFProvider creates a new GGUF model provider (no-op for non-CGO)
func NewGGUFProvider(config *GGUFModelConfig) (*GGUFProvider, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	logger := slog.Default().With("component", "GGUFProvider", "model_path", config.ModelPath)

	provider := &GGUFProvider{
		config: config,
		health: &ModelHealth{
			IsHealthy:     true,
			SuccessRate:   1.0,
			ErrorMessages: make([]string, 0),
		},
		pool:   make(chan interface{}, config.PoolSize),
		logger: logger,
	}

	provider.logger.Info("GGUFProvider initialized (no-op)", "pool_size", config.PoolSize, "model_type", config.ModelType)
	return provider, nil
}

// loadModel handles the GGUF model loading with temp file extraction (no-op)
func (p *GGUFProvider) loadModel() (interface{}, error) {
	return nil, llamaPackageNotAvailable
}

// initializePool loads multiple model instances into the pool (no-op)
func (p *GGUFProvider) initializePool() error {
	for i := 0; i < p.config.PoolSize; i++ {
		model, err := p.loadModel()
		if err != nil {
			p.logger.Error("Failed to load model instance", "instance", i, "error", err)
			return fmt.Errorf("failed to load model instance %d: %w", i, err)
		}
		p.pool <- model
		p.logger.Debug("Loaded model instance", "instance", i, "pool_size", len(p.pool))
	}
	return nil
}

// Borrow retrieves a model instance from the pool with timeout (no-op)
func (p *GGUFProvider) Borrow(ctx context.Context) (interface{}, error) {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()

	if p.isBreakerOpen() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	borrowCtx, cancel := context.WithTimeout(ctx, p.config.BorrowTimeout)
	defer cancel()

	select {
	case model := <-p.pool:
		p.logger.Debug("Borrowed model from pool", "pool_remaining", len(p.pool))
		return model, nil
	case <-borrowCtx.Done():
		return nil, fmt.Errorf("borrow timeout after %v", p.config.BorrowTimeout)
	}
}

// Return returns a model instance to the pool (no-op)
func (p *GGUFProvider) Return(model interface{}) {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()

	if len(p.pool) >= p.config.PoolSize {
		p.logger.Warn("Attempted to return model to full pool", "pool_size", len(p.pool))
		return
	}

	select {
	case p.pool <- model:
		p.logger.Debug("Returned model to pool", "pool_size", len(p.pool))
	default:
		p.logger.Warn("Pool channel full")
	}
}

// isBreakerOpen checks if the circuit breaker is tripped (no-op)
func (p *GGUFProvider) isBreakerOpen() bool {
	p.breakerMu.Lock()
	defer p.breakerMu.Unlock()

	failures := atomic.LoadInt64(&p.failureCount)
	if failures >= int64(p.config.BreakerThreshold) {
		cooldownElapsed := time.Since(p.lastFailureTime) > p.config.BreakerCooldown
		if !cooldownElapsed {
			return true
		}
		atomic.StoreInt64(&p.failureCount, 0)
		p.logger.Info("Circuit breaker reset after cooldown")
	}
	return false
}

// GenerateText generates text using the GGUF model (no-op)
func (p *GGUFProvider) GenerateText(ctx context.Context, prompt string, options ...interface{}) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.config.RequestTimeout)
	defer cancel()

	model, err := p.Borrow(reqCtx)
	if err != nil {
		p.recordFailure(fmt.Sprintf("borrow failed: %v", err))
		return "", fmt.Errorf("failed to borrow model: %w", err)
	}
	defer p.Return(model)

	p.logger.Debug("Text generation completed (no-op)", "output_length", 0)

	return "No-op response", nil
}

// EmbedText generates embeddings using the GGUF model (no-op)
func (p *GGUFProvider) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.config.RequestTimeout)
	defer cancel()

	model, err := p.Borrow(reqCtx)
	if err != nil {
		p.recordFailure(fmt.Sprintf("borrow failed: %v", err))
		return nil, fmt.Errorf("failed to borrow model: %w", err)
	}
	defer p.Return(model)

	embedding := make([]float32, 768) // Default size
	for i := range embedding {
		embedding[i] = float32(i) / 1000.0
	}

	p.logger.Debug("Embedding generation completed (no-op)")

	return embedding, nil
}

// GetHealth returns current model health status
func (p *GGUFProvider) GetHealth() *ModelHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := *p.health
	return &health
}

// Close gracefully shuts down the provider
func (p *GGUFProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.poolMu.Lock()
	for len(p.pool) > 0 {
		<-p.pool
	}
	close(p.pool)
	p.poolMu.Unlock()

	p.health.IsHealthy = false
	p.health.ErrorMessages = append(p.health.ErrorMessages, "Provider closed")

	p.logger.Info("GGUFProvider closed (no-op)")

	return nil
}

// recordSuccess updates health metrics on successful operation (no-op)
func (p *GGUFProvider) recordSuccess(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.health.TotalCalls++
	p.health.SuccessCalls++
	p.health.LastUsed = time.Now()

	if p.health.AverageLatency == 0 {
		p.health.AverageLatency = duration
	} else {
		alpha := 0.1
		p.health.AverageLatency = time.Duration(float64(p.health.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}

	p.health.IsHealthy = true
}

// recordFailure updates health metrics on failed operation (no-op)
func (p *GGUFProvider) recordFailure(errorMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.health.TotalCalls++
	p.health.FailureCalls++
	p.health.LastUsed = time.Now()
	p.health.IsHealthy = false

	if len(p.health.ErrorMessages) >= 10 {
		p.health.ErrorMessages = p.health.ErrorMessages[1:]
	}
	p.health.ErrorMessages = append(p.health.ErrorMessages, errorMsg)

	if p.health.TotalCalls > 0 {
		p.health.SuccessRate = float64(p.health.SuccessCalls) / float64(p.health.TotalCalls)
	}

	p.breakerMu.Lock()
	atomic.AddInt64(&p.failureCount, 1)
	p.lastFailureTime = time.Now()
	p.breakerMu.Unlock()

	p.logger.Warn("Operation failed", "error", errorMsg, "failure_count", atomic.LoadInt64(&p.failureCount))
}

// IsHealthy returns whether the model is considered healthy (no-op)
func (p *GGUFProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.health.IsHealthy
}

// GetModelType returns the type of model this provider handles
func (p *GGUFProvider) GetModelType() ModelType {
	return p.config.ModelType
}

// GetConfig returns the current configuration
func (p *GGUFProvider) GetConfig() *GGUFModelConfig {
	return p.config
}

// extractEmbeddedModel extracts embedded GGUF model to temporary file (no-op)
func (p *GGUFProvider) extractEmbeddedModel() (string, error) {
	return "", llamaPackageNotAvailable
}
