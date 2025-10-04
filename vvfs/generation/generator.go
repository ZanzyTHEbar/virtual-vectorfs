package generation

import (
	"context"

	"github.com/spf13/viper"
)

// Message represents a chat message for generation
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // Message content
}

// LLMConfig holds LLM model configurations
type LLMConfig struct {
	Provider          string  `mapstructure:"provider"`
	ModelPath         string  `mapstructure:"model_path"`
	MaxNewTokens      int     `mapstructure:"max_new_tokens"`
	Temperature       float32 `mapstructure:"temperature"`
	TopP              float32 `mapstructure:"top_p"`
	MinP              float32 `mapstructure:"min_p"`
	RepetitionPenalty float32 `mapstructure:"repetition_penalty"`
}

// GenerationRequest represents a text generation request
type GenerationRequest struct {
	Messages          []Message `json:"messages"`           // Chat messages
	MaxTokens         int       `json:"max_tokens"`         // Maximum tokens to generate
	Temperature       float32   `json:"temperature"`        // Sampling temperature
	TopP              float32   `json:"top_p"`              // Nucleus sampling
	MinP              float32   `json:"min_p"`              // Minimum probability
	RepetitionPenalty float32   `json:"repetition_penalty"` // Repetition penalty
	Stream            bool      `json:"stream"`             // Whether to stream the response
}

// GenerationResponse represents the response from text generation
type GenerationResponse struct {
	Text     string    `json:"text"`            // Generated text
	Messages []Message `json:"messages"`        // Full conversation including generation
	Usage    *Usage    `json:"usage,omitempty"` // Token usage information
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Generator produces text completions from chat messages
type Generator interface {
	Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error)
	StreamGenerate(ctx context.Context, req *GenerationRequest) (<-chan *GenerationResponse, error)
}

// LoadLLMConfig loads LLM configuration from viper
func LoadLLMConfig() (*LLMConfig, error) {
	var config LLMConfig

	// Try to load from viper if available
	err := viper.UnmarshalKey("llm", &config)
	if err != nil || config.Provider == "" {
		// Fallback to defaults if viper not available or no config found
		config = LLMConfig{
			Provider:          "hugot",
			ModelPath:         "onnx-community/gemma-3-270m-it-ONNX",
			MaxNewTokens:      512,
			Temperature:       0.3,
			TopP:              0.9,
			MinP:              0.15,
			RepetitionPenalty: 1.05,
		}
	}

	// Validate configuration
	if config.MaxNewTokens <= 0 {
		config.MaxNewTokens = 512
	}
	if config.Temperature <= 0 {
		config.Temperature = 0.7
	}
	if config.TopP <= 0 || config.TopP > 1 {
		config.TopP = 0.9
	}
	if config.MinP < 0 {
		config.MinP = 0.1
	}
	if config.RepetitionPenalty <= 0 {
		config.RepetitionPenalty = 1.0
	}

	return &config, nil
}
