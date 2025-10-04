package models

import (
	"fmt"
	"time"
)

// GGUFModelConfig holds configuration for GGUF model loading
type GGUFModelConfig struct {
	ModelPath       string
	ModelType       ModelType
	ContextSize     int
	GPULayers       int
	Threads         int
	F16Memory       bool
	MMAP            bool
	TempFileCleanup bool
	BatchSize       int
	MaxTokens       int
	Temperature     float32
	TopP            float32
	// Pooling and resilience settings
	PoolSize         int
	BorrowTimeout    time.Duration
	RequestTimeout   time.Duration
	BreakerThreshold int
	BreakerCooldown  time.Duration
}

// DefaultGGUFConfig returns default configuration for a GGUF model
func DefaultGGUFConfig(modelPath string, modelType ModelType) *GGUFModelConfig {
	return &GGUFModelConfig{
		ModelPath:        modelPath,
		ModelType:        modelType,
		ContextSize:      2048,
		GPULayers:        0, // CPU-only by default
		Threads:          4,
		F16Memory:        true,
		MMAP:             true,
		TempFileCleanup:  true,
		BatchSize:        512,
		MaxTokens:        256,
		Temperature:      0.7,
		TopP:             0.9,
		PoolSize:         2,
		BorrowTimeout:    5 * time.Second,
		RequestTimeout:   30 * time.Second,
		BreakerThreshold: 5,
		BreakerCooldown:  60 * time.Second,
	}
}

// ValidateConfig validates the GGUF model configuration
func ValidateConfig(config *GGUFModelConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.ModelPath == "" {
		return fmt.Errorf("model path cannot be empty")
	}

	if config.ContextSize <= 0 {
		return fmt.Errorf("context size must be positive, got %d", config.ContextSize)
	}

	if config.GPULayers < 0 {
		return fmt.Errorf("GPU layers cannot be negative, got %d", config.GPULayers)
	}

	if config.Threads <= 0 {
		return fmt.Errorf("threads must be positive, got %d", config.Threads)
	}

	if config.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive, got %d", config.BatchSize)
	}

	if config.MaxTokens <= 0 {
		return fmt.Errorf("max tokens must be positive, got %d", config.MaxTokens)
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got %f", config.Temperature)
	}

	if config.TopP < 0 || config.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1, got %f", config.TopP)
	}

	if config.PoolSize <= 0 {
		return fmt.Errorf("pool size must be positive, got %d", config.PoolSize)
	}

	if config.BorrowTimeout <= 0 {
		return fmt.Errorf("borrow timeout must be positive, got %v", config.BorrowTimeout)
	}

	if config.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive, got %v", config.RequestTimeout)
	}

	if config.BreakerThreshold <= 0 {
		return fmt.Errorf("breaker threshold must be positive, got %d", config.BreakerThreshold)
	}

	if config.BreakerCooldown <= 0 {
		return fmt.Errorf("breaker cooldown must be positive, got %v", config.BreakerCooldown)
	}

	return nil
}

// ModelHealth tracks the health status of a model
type ModelHealth struct {
	IsHealthy       bool
	SuccessRate     float64
	AverageLatency  time.Duration
	TotalCalls      int64
	SuccessCalls    int64
	FailureCalls    int64
	LastUsed        time.Time
	LastError       error
	ErrorMessages   []string
	LastHealthCheck time.Time
}
