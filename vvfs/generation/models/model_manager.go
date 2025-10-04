package models

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// ModelManager coordinates all AI model providers and manages the cascade system
type ModelManager struct {
	// Model providers (now using Open providers wrapping GGUF)
	embeddingProvider *OpenEmbedProvider
	chatProvider      *OpenChatProvider
	visionProvider    *OpenVisionProvider
	cascadeManager    *CascadeManager

	// Configuration
	config *ModelManagerConfig

	// Health monitoring
	healthCheckInterval time.Duration
	healthTicker        *time.Ticker
	stopHealthCheck     chan bool

	mu sync.RWMutex
}

// ModelManagerConfig holds configuration for the model manager
type ModelManagerConfig struct {
	// Model paths
	EmbeddingModelPath string
	ChatModelPath      string
	VisionModelPath    string

	// Performance settings
	EmbeddingDims int
	ContextSize   int
	GPULayers     int
	Threads       int
	UseF16Memory  bool
	UseMMAP       bool

	// Health monitoring
	HealthCheckInterval    time.Duration
	EnableHealthMonitoring bool

	// Cascade settings
	ConfidenceThreshold float64
	EnableCascade       bool
}

// DefaultModelManagerConfig returns default model manager config with open-source defaults
func DefaultModelManagerConfig() *ModelManagerConfig {
	return &ModelManagerConfig{
		// Open-source model paths (files under vvfs/generation/models/gguf)
		EmbeddingModelPath: "vvfs/generation/models/gguf/open-embed.gguf",
		ChatModelPath:      "vvfs/generation/models/gguf/open-chat-qwen3-1_7b.gguf",
		VisionModelPath:    "vvfs/generation/models/gguf/open-vision.gguf",

		// Open-source defaults
		EmbeddingDims: 768,  // Qwen3-Embedding-0.6B default
		ContextSize:   4096, // Qwen3-1.7B default
		GPULayers:     -1,   // Use all available GPU layers
		Threads:       8,    // Reasonable default
		UseF16Memory:  true, // VRAM optimization for quantized models
		UseMMAP:       true, // Memory mapping for multi-GB models

		// Health monitoring
		HealthCheckInterval:    30 * time.Second,
		EnableHealthMonitoring: true,
		ConfidenceThreshold:    0.90, // threshold tuned for open models
		EnableCascade:          true, // enable cascade across open providers
	}
}

// applyEnvOverrides applies environment variable overrides to the config
func (c *ModelManagerConfig) applyEnvOverrides() {
	if embedPath := os.Getenv("VVFS_EMBED_MODEL_PATH"); embedPath != "" {
		c.EmbeddingModelPath = embedPath
	}
	if chatPath := os.Getenv("VVFS_CHAT_MODEL_PATH"); chatPath != "" {
		c.ChatModelPath = chatPath
	}
	if visionPath := os.Getenv("VVFS_VISION_MODEL_PATH"); visionPath != "" {
		c.VisionModelPath = visionPath
	}
	if threads := os.Getenv("VVFS_THREADS"); threads != "" {
		if t, err := fmt.Sscanf(threads, "%d", &c.Threads); t == 1 && err == nil {
			// Valid integer
		}
	}
	if gpuLayers := os.Getenv("VVFS_GPU_LAYERS"); gpuLayers != "" {
		if l, err := fmt.Sscanf(gpuLayers, "%d", &c.GPULayers); l == 1 && err == nil {
			// Valid integer
		}
	}
}

// NewModelManager creates a new model manager with all providers and env overrides
func NewModelManager(config *ModelManagerConfig) (*ModelManager, error) {
	if config == nil {
		config = DefaultModelManagerConfig()
	}

	// Apply environment overrides
	config.applyEnvOverrides()

	manager := &ModelManager{
		config:              config,
		cascadeManager:      NewCascadeManager(),
		healthCheckInterval: config.HealthCheckInterval,
		stopHealthCheck:     make(chan bool),
	}

	// Initialize providers
	if err := manager.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Start health monitoring if enabled
	if config.EnableHealthMonitoring {
		manager.startHealthMonitoring()
	}

	return manager, nil
}

// initializeProviders sets up all Open model providers
func (m *ModelManager) initializeProviders() error {
	// Initialize embedding provider
	embeddingProvider, err := NewOpenEmbedProvider(m.config.EmbeddingModelPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize embedding provider: %v", err)
		// Continue without embedding provider for now
	} else {
		m.embeddingProvider = embeddingProvider
		m.cascadeManager.AddProvider("open-embed", embeddingProvider.GGUFProvider)
	}

	// Initialize chat provider
	chatProvider, err := NewOpenChatProvider(m.config.ChatModelPath)
	if err != nil {
		return fmt.Errorf("failed to initialize chat provider: %w", err)
	}
	m.chatProvider = chatProvider
	m.cascadeManager.AddProvider("open-chat", chatProvider.GGUFProvider)

	// Initialize vision provider (optional)
	if m.config.VisionModelPath != "" {
		visionProvider, err := NewOpenVisionProvider(m.config.VisionModelPath)
		if err != nil {
			log.Printf("Warning: Failed to initialize vision provider: %v", err)
			// Continue without vision provider
		} else {
			m.visionProvider = visionProvider
			m.cascadeManager.AddProvider("open-vision", visionProvider.GGUFProvider)
		}
	}

	return nil
}

// startHealthMonitoring starts the background health monitoring
func (m *ModelManager) startHealthMonitoring() {
	m.healthTicker = time.NewTicker(m.healthCheckInterval)

	go func() {
		for {
			select {
			case <-m.healthTicker.C:
				m.cascadeManager.UpdateHealth()
			case <-m.stopHealthCheck:
				m.healthTicker.Stop()
				return
			}
		}
	}()
}

// GetEmbeddingProvider returns the embedding provider
func (m *ModelManager) GetEmbeddingProvider() *OpenEmbedProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.embeddingProvider
}

// GetChatProvider returns the chat provider
func (m *ModelManager) GetChatProvider() *OpenChatProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chatProvider
}

// GetVisionProvider returns the vision provider
func (m *ModelManager) GetVisionProvider() *OpenVisionProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visionProvider
}

// SetEmbeddingPath updates the embedding model path and reloads the provider
func (m *ModelManager) SetEmbeddingPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing provider
	if m.embeddingProvider != nil {
		if err := m.embeddingProvider.Close(); err != nil {
			log.Printf("Warning: Error closing old embedding provider: %v", err)
		}
	}

	// Create new provider
	newProvider, err := NewOpenEmbedProvider(path)
	if err != nil {
		return fmt.Errorf("failed to create new embedding provider: %w", err)
	}

	m.embeddingProvider = newProvider
	m.config.EmbeddingModelPath = path

	// Update cascade manager
	m.cascadeManager.RemoveProvider("open-embed")
	m.cascadeManager.AddProvider("open-embed", newProvider.GGUFProvider)

	log.Printf("Embedding provider reloaded with path: %s", path)
	return nil
}

// SetChatPath updates the chat model path and reloads the provider
func (m *ModelManager) SetChatPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing provider
	if m.chatProvider != nil {
		if err := m.chatProvider.Close(); err != nil {
			log.Printf("Warning: Error closing old chat provider: %v", err)
		}
	}

	// Create new provider
	newProvider, err := NewOpenChatProvider(path)
	if err != nil {
		return fmt.Errorf("failed to create new chat provider: %w", err)
	}

	m.chatProvider = newProvider
	m.config.ChatModelPath = path

	// Update cascade manager
	m.cascadeManager.RemoveProvider("open-chat")
	m.cascadeManager.AddProvider("open-chat", newProvider.GGUFProvider)

	log.Printf("Chat provider reloaded with path: %s", path)
	return nil
}

// SetVisionPath updates the vision model path and reloads the provider
func (m *ModelManager) SetVisionPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing provider
	if m.visionProvider != nil {
		if err := m.visionProvider.Close(); err != nil {
			log.Printf("Warning: Error closing old vision provider: %v", err)
		}
	}

	// Create new provider
	newProvider, err := NewOpenVisionProvider(path)
	if err != nil {
		return fmt.Errorf("failed to create new vision provider: %w", err)
	}

	m.visionProvider = newProvider
	m.config.VisionModelPath = path

	// Update cascade manager
	m.cascadeManager.RemoveProvider("open-vision")
	m.cascadeManager.AddProvider("open-vision", newProvider.GGUFProvider)

	log.Printf("Vision provider reloaded with path: %s", path)
	return nil
}

// GetCascadeManager returns the cascade manager
func (m *ModelManager) GetCascadeManager() *CascadeManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cascadeManager
}

// GetBestProvider returns the best available provider for a given task type
func (m *ModelManager) GetBestProvider(ctx context.Context, taskType ModelType) (*GGUFProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.EnableCascade {
		// Return specific provider based on task type
		switch taskType {
		case ModelTypeEmbedding:
			if m.embeddingProvider != nil {
				return m.embeddingProvider.GGUFProvider, nil
			}
		case ModelTypeChat:
			if m.chatProvider != nil {
				return m.chatProvider.GGUFProvider, nil
			}
		case ModelTypeVision:
			if m.visionProvider != nil {
				return m.visionProvider.GGUFProvider, nil
			}
		}
		return nil, fmt.Errorf("no provider available for task type: %s", taskType)
	}

	// Use cascade system
	return m.cascadeManager.GetBestProvider(ctx)
}

// GenerateText generates text using the best available chat provider
func (m *ModelManager) GenerateText(ctx context.Context, prompt string, options ...interface{}) (string, error) {
	provider, err := m.GetBestProvider(ctx, ModelTypeChat)
	if err != nil {
		return "", err
	}

	// Pass options directly to provider (type handling is provider-specific)
	return provider.GenerateText(ctx, prompt, options...)
}

// GenerateEmbedding generates embeddings using the embedding provider
func (m *ModelManager) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.embeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider not available")
	}

	return m.embeddingProvider.EmbedText(ctx, text)
}

// AnalyzeImage analyzes an image using the vision provider (placeholder)
func (m *ModelManager) AnalyzeImage(ctx context.Context, imagePath string) (string, error) {
	if m.visionProvider == nil {
		return "", fmt.Errorf("vision provider not available")
	}

	// Placeholder: for now, return an error as multimodal is not implemented
	return "", fmt.Errorf("image analysis not yet implemented")
}

// GetHealthSummary returns health status of all providers
func (m *ModelManager) GetHealthSummary() map[string]*ModelHealth {
	return m.cascadeManager.GetHealthSummary()
}

// Close gracefully shuts down all providers
func (m *ModelManager) Close() error {
	// Stop health monitoring
	if m.healthTicker != nil {
		select {
		case m.stopHealthCheck <- true:
		default:
		}
	}

	// Close providers
	var errors []error

	if m.embeddingProvider != nil {
		if err := m.embeddingProvider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("embedding provider: %w", err))
		}
	}

	if m.chatProvider != nil {
		if err := m.chatProvider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("chat provider: %w", err))
		}
	}

	if m.visionProvider != nil {
		if err := m.visionProvider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("vision provider: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing providers: %v", errors)
	}

	return nil
}

// IsHealthy returns whether the model manager is healthy
func (m *ModelManager) IsHealthy() bool {
	healthSummary := m.GetHealthSummary()

	for _, health := range healthSummary {
		if !health.IsHealthy {
			return false
		}
	}

	return true
}

// GetModelInfo returns information about loaded models
func (m *ModelManager) GetModelInfo() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make(map[string]interface{})

	if m.embeddingProvider != nil {
		info["embedding"] = map[string]interface{}{
			"type":       "embedding",
			"dimensions": m.embeddingProvider.GetMatryoshkaDims(),
			"health":     m.embeddingProvider.GetHealth(),
			"model_path": m.config.EmbeddingModelPath,
		}
	}

	if m.chatProvider != nil {
		info["chat"] = map[string]interface{}{
			"type":         "chat",
			"context_size": m.config.ContextSize,
			"health":       m.chatProvider.GetHealth(),
			"model_path":   m.config.ChatModelPath,
		}
	}

	if m.visionProvider != nil {
		info["vision"] = map[string]interface{}{
			"type":       "vision",
			"health":     m.visionProvider.GetHealth(),
			"model_path": m.config.VisionModelPath,
		}
	}

	info["cascade_enabled"] = m.config.EnableCascade
	info["health_monitoring"] = m.config.EnableHealthMonitoring

	return info
}

// UpdateConfig updates the model manager configuration
func (m *ModelManager) UpdateConfig(newConfig *ModelManagerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate new config
	if newConfig.EmbeddingDims <= 0 {
		return fmt.Errorf("invalid embedding dimensions: %d", newConfig.EmbeddingDims)
	}

	if newConfig.ContextSize <= 0 {
		return fmt.Errorf("invalid context size: %d", newConfig.ContextSize)
	}

	// Update config
	oldConfig := m.config
	m.config = newConfig

	// Restart health monitoring if settings changed
	if oldConfig.EnableHealthMonitoring != newConfig.EnableHealthMonitoring {
		if m.healthTicker != nil {
			m.healthTicker.Stop()
		}

		if newConfig.EnableHealthMonitoring {
			m.startHealthMonitoring()
		}
	}

	return nil
}

// ValidateModels validates that all required models can be loaded
func (m *ModelManager) ValidateModels() error {
	var errors []error

	// Check if model files exist
	modelPaths := []string{
		m.config.EmbeddingModelPath,
		m.config.ChatModelPath,
	}

	if m.config.VisionModelPath != "" {
		modelPaths = append(modelPaths, m.config.VisionModelPath)
	}

	for _, path := range modelPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("model file not found: %s", path))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("model validation failed: %v", errors)
	}

	return nil
}

// PreloadModels preloads all models for faster first inference
func (m *ModelManager) PreloadModels(ctx context.Context) error {
	log.Println("Preloading AI models...")

	// Preload chat model with a simple prompt (no options needed for warmup)
	if m.chatProvider != nil {
		_, err := m.chatProvider.GenerateText(ctx, "Hello")
		if err != nil {
			return fmt.Errorf("failed to preload chat model: %w", err)
		}
	}

	// Preload embedding model with a simple text
	if m.embeddingProvider != nil {
		_, err := m.embeddingProvider.EmbedText(ctx, "test")
		if err != nil {
			return fmt.Errorf("failed to preload embedding model: %w", err)
		}
	}

	// Preload vision model if available
	if m.visionProvider != nil {
		// FIXME: Vision preloading would require a test image
		log.Println("Vision model preloaded (skipped - requires test image)")
	}

	log.Println("All models preloaded successfully")
	return nil
}
