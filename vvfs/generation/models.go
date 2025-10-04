package generation

// Default model configurations for supported LLMs
// These are the recommended production models for on-device generation

const (
	// Gemma 3 models - ONNX-ready for Hugot TextGeneration
	DefaultGemma270M = "onnx-community/gemma-3-270m-it-ONNX"
	DefaultGemma1B   = "onnx-community/gemma-3-1b-it-ONNX"

	// LFM2 models - requires ONNX export via HF Optimum
	// DefaultLFM2350M = "LiquidAI/LFM2-350M" // Use after ONNX export
)

// ModelConfig holds configuration for a specific model
type ModelConfig struct {
	Name               string
	Path               string
	ContextLength      int
	DefaultMaxTokens   int
	DefaultTemperature float32
	DefaultTopP        float32
	DefaultMinP        float32
	DefaultRepPenalty  float32
	RequiresONNX       bool
}

// GetModelConfig returns the default configuration for a model
func GetModelConfig(modelPath string) *ModelConfig {
	switch {
	case contains(modelPath, "gemma-3-270m"):
		return &ModelConfig{
			Name:               "Gemma 3 270M",
			Path:               modelPath,
			ContextLength:      8192,
			DefaultMaxTokens:   512,
			DefaultTemperature: 0.3,
			DefaultTopP:        0.9,
			DefaultMinP:        0.15,
			DefaultRepPenalty:  1.05,
			RequiresONNX:       true,
		}
	case contains(modelPath, "gemma-3-1b"):
		return &ModelConfig{
			Name:               "Gemma 3 1B",
			Path:               modelPath,
			ContextLength:      8192,
			DefaultMaxTokens:   512,
			DefaultTemperature: 0.3,
			DefaultTopP:        0.9,
			DefaultMinP:        0.15,
			DefaultRepPenalty:  1.05,
			RequiresONNX:       true,
		}
	case contains(modelPath, "lfm2-350m"):
		return &ModelConfig{
			Name:               "LFM2 350M",
			Path:               modelPath,
			ContextLength:      32768,
			DefaultMaxTokens:   512,
			DefaultTemperature: 0.3,
			DefaultTopP:        0.9,
			DefaultMinP:        0.15,
			DefaultRepPenalty:  1.05,
			RequiresONNX:       true, // Requires export from PyTorch to ONNX
		}
	default:
		// Default fallback configuration
		return &ModelConfig{
			Name:               "Unknown Model",
			Path:               modelPath,
			ContextLength:      4096,
			DefaultMaxTokens:   256,
			DefaultTemperature: 0.7,
			DefaultTopP:        0.9,
			DefaultMinP:        0.1,
			DefaultRepPenalty:  1.0,
			RequiresONNX:       true,
		}
	}
}

// GetDefaultGenerationRequest creates a generation request with model-appropriate defaults
func GetDefaultGenerationRequest(modelPath string, messages []Message) *GenerationRequest {
	config := GetModelConfig(modelPath)

	return &GenerationRequest{
		Messages:          messages,
		MaxTokens:         config.DefaultMaxTokens,
		Temperature:       config.DefaultTemperature,
		TopP:              config.DefaultTopP,
		MinP:              config.DefaultMinP,
		RepetitionPenalty: config.DefaultRepPenalty,
		Stream:            false,
	}
}
