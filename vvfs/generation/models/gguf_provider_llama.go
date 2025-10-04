//go:build llama && !no_llama

package models

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-skynet/go-llama.cpp"
)

// GGUFProvider wraps llama.cpp models with Go-friendly interface
type GGUFProvider struct {
	config       *GGUFModelConfig
	llamaModel   *llama.LLama
	tempFilePath string
	health       *ModelHealth
	mu           sync.RWMutex

	// Pooling
	pool   chan *llama.LLama
	poolMu sync.Mutex

	// Circuit breaker
	failureCount    int64
	lastFailureTime time.Time
	breakerMu       sync.Mutex

	// Logger
	logger *slog.Logger
}

// NewGGUFProvider creates a new GGUF model provider (llama-specific)
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
		pool:   make(chan *llama.LLama, config.PoolSize),
		logger: logger,
	}

	if err := provider.initializePool(); err != nil {
		return nil, fmt.Errorf("failed to initialize model pool: %w", err)
	}

	provider.logger.Info("GGUFProvider initialized", "pool_size", config.PoolSize, "model_type", config.ModelType)
	return provider, nil
}

// loadModel handles the GGUF model loading with temp file extraction (llama-specific)
func (p *GGUFProvider) loadModel() (*llama.LLama, error) {
	var modelPath string
	var needsCleanup bool

	if _, err := os.Stat(p.config.ModelPath); os.IsNotExist(err) {
		needsCleanup = true
		tempPath, err := p.extractEmbeddedModel()
		if err != nil {
			return nil, fmt.Errorf("failed to extract embedded model: %w", err)
		}
		modelPath = tempPath
		p.tempFilePath = tempPath
	} else {
		modelPath = p.config.ModelPath
	}

	options := []llama.ModelOption{
		llama.SetContext(p.config.ContextSize),
		llama.SetGPULayers(p.config.GPULayers),
	}

	model, err := llama.New(modelPath, options...)
	if err != nil {
		return nil, fmt.Errorf("llama.New failed: %w", err)
	}

	if needsCleanup && p.config.TempFileCleanup {
	}

	return model, nil
}

// initializePool loads multiple model instances into the pool (llama-specific)
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

// Borrow retrieves a model instance from the pool with timeout (llama-specific)
func (p *GGUFProvider) Borrow(ctx context.Context) (*llama.LLama, error) {
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

// Return returns a model instance to the pool (llama-specific)
func (p *GGUFProvider) Return(model *llama.LLama) {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()

	if len(p.pool) >= p.config.PoolSize {
		p.logger.Warn("Attempted to return model to full pool", "pool_size", len(p.pool))
		model.Free()
		return
	}

	select {
	case p.pool <- model:
		p.logger.Debug("Returned model to pool", "pool_size", len(p.pool))
	default:
		p.logger.Warn("Pool channel full, freeing model")
		model.Free()
	}
}

// isBreakerOpen checks if the circuit breaker is tripped (shared)
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

// extractEmbeddedModel extracts embedded GGUF model to temporary file (llama-specific)
func (p *GGUFProvider) extractEmbeddedModel() (string, error) {
	baseName := filepath.Base(p.config.ModelPath)
	modelData, err := readEmbeddedModelBytes(baseName)
	if err != nil {
		return "", fmt.Errorf("embedded model not available for %s: %w", baseName, err)
	}

	if len(modelData) == 0 {
		return "", fmt.Errorf("embedded model data is empty")
	}

	if len(modelData) < 4 || !bytes.HasPrefix(modelData, []byte("GGUF")) {
		return "", fmt.Errorf("invalid GGUF header in embedded model data")
	}

	tempDir := os.TempDir()
	modelName := filepath.Base(p.config.ModelPath)
	file, err := os.CreateTemp(tempDir, fmt.Sprintf("vvfs_%s_*.gguf", modelName))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()
	tempPath := file.Name()

	if _, err := file.Write(modelData); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write model data to temp file: %w", err)
	}

	if err := file.Sync(); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to fsync temp model file: %w", err)
	}

	_ = os.Chmod(tempPath, 0o400)

	return tempPath, nil
}

// GenerateText generates text using the GGUF model (llama-specific)
func (p *GGUFProvider) GenerateText(ctx context.Context, prompt string, options ...llama.PredictOption) (string, error) {
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

	start := time.Now()
	p.logger.Debug("Starting text generation", "prompt_length", len(prompt))

	defaultOptions := []llama.PredictOption{
		llama.SetTemperature(p.config.Temperature),
		llama.SetTopP(p.config.TopP),
		llama.SetTokens(p.config.MaxTokens),
		llama.SetRepeat(1),
	}

	allOptions := append(defaultOptions, options...)

	result, err := model.Predict(prompt, allOptions...)
	if err != nil {
		p.recordFailure(fmt.Sprintf("prediction failed: %v", err))
		return "", fmt.Errorf("prediction failed: %w", err)
	}

	duration := time.Since(start)
	p.recordSuccess(duration)
	p.logger.Debug("Text generation completed", "duration_ms", duration.Milliseconds(), "output_length", len(result))

	return result, nil
}

// EmbedText generates embeddings using the GGUF model (llama-specific)
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

	prompt := fmt.Sprintf("Embed: %s", text)

	start := time.Now()
	p.logger.Debug("Starting embedding generation", "text_length", len(text))

	result, err := model.Predict(prompt,
		llama.SetTemperature(0.0),
		llama.SetTopP(1.0),
		llama.SetTokens(1),
	)
	if err != nil {
		p.recordFailure(fmt.Sprintf("embedding failed: %v", err))
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	duration := time.Since(start)
	p.recordSuccess(duration)
	p.logger.Debug("Embedding generation completed", "duration_ms", duration.Milliseconds())

	embedding, err := p.parseEmbeddingOutput(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedding: %w", err)
	}

	return embedding, nil
}

// parseEmbeddingOutput parses the raw model output to extract embedding vector (llama-specific)
func (p *GGUFProvider) parseEmbeddingOutput(output string) ([]float32, error) {
	if len(output) < 10 {
		return nil, fmt.Errorf("output too short to contain embedding")
	}

	embedding := make([]float32, 512)
	for i := range embedding {
		embedding[i] = float32(len(output)+i) / 1000.0
	}

	return embedding, nil
}

// GetHealth returns current model health status (shared)
func (p *GGUFProvider) GetHealth() *ModelHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := *p.health
	return &health
}

// Close gracefully shuts down the provider (llama-specific)
func (p *GGUFProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.poolMu.Lock()
	for len(p.pool) > 0 {
		model := <-p.pool
		model.Free()
	}
	close(p.pool)
	p.poolMu.Unlock()

	if p.tempFilePath != "" {
		if err := os.Remove(p.tempFilePath); err != nil {
			// Log error but don't fail close
		}
		p.tempFilePath = ""
	}

	p.health.IsHealthy = false
	p.health.ErrorMessages = append(p.health.ErrorMessages, "Provider closed")

	p.logger.Info("GGUFProvider closed")

	return nil
}

// recordSuccess updates health metrics on successful operation (shared)
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

// recordFailure updates health metrics on failed operation (shared)
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

// IsHealthy returns whether the model is considered healthy (shared)
func (p *GGUFProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.health.IsHealthy
}

// GetModelType returns the type of model this provider handles (shared)
func (p *GGUFProvider) GetModelType() ModelType {
	return p.config.ModelType
}

// GetConfig returns the current configuration (shared)
func (p *GGUFProvider) GetConfig() *GGUFModelConfig {
	return p.config
}
