//go:build !llama || no_llama

package models

import (
	"context"
	"fmt"
	"runtime"
)

// OpenEmbedProvider wraps GGUFProvider for embedding tasks with Qwen3-Embedding-0.6B defaults (no-op)
type OpenEmbedProvider struct {
	*GGUFProvider
	matryoshkaDims int
}

// NewOpenEmbedProvider creates a new OpenEmbedProvider with Qwen3-Embedding-0.6B defaults (no-op)
func NewOpenEmbedProvider(modelPath string) (*OpenEmbedProvider, error) {
	config := DefaultGGUFConfig(modelPath, ModelTypeEmbedding)
	config.ContextSize = 2048
	config.MaxTokens = 1
	config.Temperature = 0.0
	config.TopP = 1.0
	config.BatchSize = 512
	config.Threads = runtime.NumCPU()
	config.PoolSize = 2

	ggufProvider, err := NewGGUFProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create GGUFProvider: %w", err)
	}

	provider := &OpenEmbedProvider{
		GGUFProvider:   ggufProvider,
		matryoshkaDims: 768,
	}

	return provider, nil
}

// EmbedText generates embeddings with Matryoshka support (no-op)
func (p *OpenEmbedProvider) EmbedText(ctx context.Context, text string) ([]float32, error) {
	embedding, err := p.GGUFProvider.EmbedText(ctx, text)
	if err != nil {
		return nil, err
	}

	if len(embedding) != p.matryoshkaDims {
		embedding = p.adjustToDims(embedding, p.matryoshkaDims)
	}

	return embedding, nil
}

// SetMatryoshkaDims sets the target dimension for Matryoshka embeddings (no-op)
func (p *OpenEmbedProvider) SetMatryoshkaDims(dims int) {
	p.matryoshkaDims = dims
}

// GetMatryoshkaDims returns the current target dimension for Matryoshka embeddings (no-op)
func (p *OpenEmbedProvider) GetMatryoshkaDims() int {
	return p.matryoshkaDims
}

// adjustToDims truncates or pads the embedding vector to the target dimensions (no-op)
func (p *OpenEmbedProvider) adjustToDims(vec []float32, target int) []float32 {
	if len(vec) == target {
		return vec
	}

	if len(vec) > target {
		return vec[:target]
	}

	padded := make([]float32, target)
	copy(padded, vec)
	return padded
}

// OpenChatProvider wraps GGUFProvider for chat tasks with Qwen3-1.7B defaults (no-op)
type OpenChatProvider struct {
	*GGUFProvider
	systemPrompt string
}

// NewOpenChatProvider creates a new OpenChatProvider with Qwen3-1.7B defaults (no-op)
func NewOpenChatProvider(modelPath string) (*OpenChatProvider, error) {
	config := DefaultGGUFConfig(modelPath, ModelTypeChat)
	config.ContextSize = 4096
	config.MaxTokens = 512
	config.Temperature = 0.7
	config.TopP = 0.9
	config.BatchSize = 256
	config.Threads = runtime.GOMAXPROCS(0)
	config.PoolSize = 2

	ggufProvider, err := NewGGUFProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create GGUFProvider: %w", err)
	}

	provider := &OpenChatProvider{
		GGUFProvider: ggufProvider,
		systemPrompt: "You are a helpful AI assistant.",
	}

	return provider, nil
}

// GenerateText generates chat responses with system prompt (no-op)
func (p *OpenChatProvider) GenerateText(ctx context.Context, userInput string, options ...interface{}) (string, error) {
	prompt := fmt.Sprintf("System: %s\n\nUser: %s\n\nAssistant:", p.systemPrompt, userInput)
	return p.GGUFProvider.GenerateText(ctx, prompt, options...)
}

// SetSystemPrompt sets the system prompt for chat interactions (no-op)
func (p *OpenChatProvider) SetSystemPrompt(prompt string) {
	p.systemPrompt = prompt
}

// OpenVisionProvider wraps GGUFProvider for vision tasks with LLaMA 3.2 Vision 3B defaults (no-op)
type OpenVisionProvider struct {
	*GGUFProvider
}

// NewOpenVisionProvider creates a new OpenVisionProvider with LLaMA 3.2 Vision 3B defaults (no-op)
func NewOpenVisionProvider(modelPath string) (*OpenVisionProvider, error) {
	config := DefaultGGUFConfig(modelPath, ModelTypeVision)
	config.ContextSize = 8192
	config.MaxTokens = 1000
	config.Temperature = 0.3
	config.TopP = 0.9
	config.BatchSize = 256
	config.Threads = runtime.GOMAXPROCS(0)
	config.PoolSize = 2

	ggufProvider, err := NewGGUFProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create GGUFProvider: %w", err)
	}

	provider := &OpenVisionProvider{
		GGUFProvider: ggufProvider,
	}

	return provider, nil
}

// GenerateText generates vision-aware responses (placeholder for multimodal) (no-op)
func (p *OpenVisionProvider) GenerateText(ctx context.Context, prompt string, options ...interface{}) (string, error) {
	return p.GGUFProvider.GenerateText(ctx, prompt, options...)
}

// DescribeImage provides a placeholder for image description (future multimodal feature) (no-op)
func (p *OpenVisionProvider) DescribeImage(ctx context.Context, imageData []byte) (string, error) {
	return "", fmt.Errorf("image description not yet implemented")
}

// GetSupportedImageFormats returns supported image formats (placeholder) (no-op)
func (p *OpenVisionProvider) GetSupportedImageFormats() []string {
	return []string{"jpeg", "png", "webp"}
}
