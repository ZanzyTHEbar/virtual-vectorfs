package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	internal "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs"

	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variables.
type Config struct {
	VVFS      VVFSConfig      `mapstructure:"vvfs"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	LLM       LLMConfig       `mapstructure:"llm"`
	ONNX      ONNXConfig      `mapstructure:"onnx"`
	Harness   HarnessConfig   `mapstructure:"harness"`
	Memory    MemoryConfig    `mapstructure:"memory"`
}

// DatabaseConfig stores database connection details.
type DatabaseConfig struct {
	DSN  string `mapstructure:"dsn"`
	Type string `mapstructure:"type"`
	// Embedded-only configuration
	LibSQLDataDir string `mapstructure:"libsql_data_dir"` // Directory for database files
}

// VVFSConfig stores vvfs specific configurations.
type VVFSConfig struct {
	TargetDir              string         `mapstructure:"targetDir"`
	CacheDir               string         `mapstructure:"cacheDir"`
	Database               DatabaseConfig `mapstructure:"database"`
	OrganizeTimeoutMinutes int            `mapstructure:"organizeTimeoutMinutes"`
}

// EmbeddingConfig stores embedding model configurations.
type EmbeddingConfig struct {
	Provider  string `mapstructure:"provider"`   // "hugot", "onnx", etc.
	ModelPath string `mapstructure:"model_path"` // Path or HF repo ID
	Dims      int    `mapstructure:"dims"`       // Target embedding dimensions
	Pooling   string `mapstructure:"pooling"`    // "mean", "cls"
	BatchSize int    `mapstructure:"batch_size"` // Batch size for inference
}

// LLMConfig stores language model configurations.
type LLMConfig struct {
	Provider          string  `mapstructure:"provider"`           // "hugot"
	ModelPath         string  `mapstructure:"model_path"`         // Path or HF repo ID
	MaxNewTokens      int     `mapstructure:"max_new_tokens"`     // Max tokens to generate
	Temperature       float32 `mapstructure:"temperature"`        // Sampling temperature
	TopP              float32 `mapstructure:"top_p"`              // Nucleus sampling
	MinP              float32 `mapstructure:"min_p"`              // Minimum probability
	RepetitionPenalty float32 `mapstructure:"repetition_penalty"` // Repetition penalty
}

// ONNXConfig stores ONNX runtime configurations.
type ONNXConfig struct {
	Backend        string `mapstructure:"backend"`          // "ort", "xla", "go"
	EP             string `mapstructure:"ep"`               // Execution provider: "cpu", "cuda", "tensorrt", "dml", "coreml", "openvino"
	LibraryPath    string `mapstructure:"library_path"`     // Path to onnxruntime library
	CUDADeviceID   int    `mapstructure:"cuda_device_id"`   // CUDA device ID
	InterOpThreads int    `mapstructure:"inter_op_threads"` // Inter-op parallelism
	IntraOpThreads int    `mapstructure:"intra_op_threads"` // Intra-op parallelism
	CPUMemArena    bool   `mapstructure:"cpu_mem_arena"`    // CPU memory arena
	MemPattern     bool   `mapstructure:"mem_pattern"`      // Memory pattern optimization
}

// HarnessConfig stores LLM harness configurations.
type HarnessConfig struct {
	// Cache settings
	CacheEnabled    bool `mapstructure:"cache_enabled"`     // Enable prompt caching
	CacheCapacity   int  `mapstructure:"cache_capacity"`    // LRU cache capacity
	CacheTTLSeconds int  `mapstructure:"cache_ttl_seconds"` // Cache entry TTL

	// Rate limiting
	RateLimitEnabled    bool          `mapstructure:"rate_limit_enabled"`     // Enable rate limiting
	RateLimitCapacity   int           `mapstructure:"rate_limit_capacity"`    // Token bucket capacity
	RateLimitRefillRate time.Duration `mapstructure:"rate_limit_refill_rate"` // Refill rate

	// Policies
	MaxToolDepth  int `mapstructure:"max_tool_depth"`  // Maximum recursive tool calls
	MaxIterations int `mapstructure:"max_iterations"`  // Maximum orchestration iterations
	MaxOutputSize int `mapstructure:"max_output_size"` // Maximum output size in bytes

	// Safety and validation
	EnableGuardrails bool     `mapstructure:"enable_guardrails"` // Enable safety checks
	BlockedWords     []string `mapstructure:"blocked_words"`     // Words to block in output
	AllowedTools     []string `mapstructure:"allowed_tools"`     // Whitelist of allowed tool names

	// Telemetry
	EnableTracing bool `mapstructure:"enable_tracing"` // Enable structured logging/tracing

	// Performance
	ToolConcurrency int `mapstructure:"tool_concurrency"` // Max concurrent tool executions
}

// MemoryConfig stores memory system configurations.
type MemoryConfig struct {
	// Retrieval settings
	Alpha     float64 `mapstructure:"alpha"`      // Fusion alpha for hybrid search (0.0-1.0)
	K         int     `mapstructure:"k"`          // Top-k results to return
	Threshold float64 `mapstructure:"threshold"`  // Distance threshold for filtering
	Lambda    float64 `mapstructure:"lambda"`     // Time decay factor for recency boosting
	Autocut   bool    `mapstructure:"autocut"`    // Enable autocut for knee detection
	TimeDecay bool    `mapstructure:"time_decay"` // Enable time-based decay
	Rerank    bool    `mapstructure:"rerank"`     // Enable reranking

	// Vector index settings
	VectorIndex string `mapstructure:"vector_index"` // "flat", "hnsw", "leann", "external"

	// HNSW settings (for hnsw index)
	HNSWM              int `mapstructure:"hnsw_m"`               // Max connections per node (16-64)
	HNSWEFConstruction int `mapstructure:"hnsw_ef_construction"` // Construction time ef (64-256)
	HNSWEFSearch       int `mapstructure:"hnsw_ef_search"`       // Search time ef (32-256)

	// LEANN settings (for low-storage mode)
	LeannEnabled       bool    `mapstructure:"leann_enabled"`        // Enable LEANN mode
	LeannPQDim         int     `mapstructure:"leann_pq_dim"`         // PQ dimension (for approx search)
	LeannBatchSize     int     `mapstructure:"leann_batch_size"`     // Recomputation batch size
	LeannStorageBudget float64 `mapstructure:"leann_storage_budget"` // Storage budget fraction (0.01-0.1)

	// Ensemble settings
	EnsembleEnabled  bool   `mapstructure:"ensemble_enabled"`  // Enable ensemble search
	EnsembleStrategy string `mapstructure:"ensemble_strategy"` // "rrf", "weighted_rrf", "relative_score", "ltr"
	RouterMode       string `mapstructure:"router_mode"`       // "rules", "ml"

	// Per-index weights for ensemble (if weighted_rrf)
	WeightsBM25   float64 `mapstructure:"weights_bm25"`   // Weight for BM25 results
	WeightsVector float64 `mapstructure:"weights_vector"` // Weight for vector results
	WeightsGraph  float64 `mapstructure:"weights_graph"`  // Weight for graph results

	// Graph settings
	GraphEnabled      bool   `mapstructure:"graph_enabled"`       // Enable knowledge graph
	GraphDepth        int    `mapstructure:"graph_depth"`         // Max graph traversal depth (1-3)
	GraphCenterPolicy string `mapstructure:"graph_center_policy"` // "top_entity", "explicit"
	GraphRerankOnly   bool   `mapstructure:"graph_rerank_only"`   // Use graph only for reranking

	// Knowledge extraction settings
	ExtractorProvider    string        `mapstructure:"extractor_provider"`    // "openai", "gemini", "ollama"
	ExtractorConcurrency int           `mapstructure:"extractor_concurrency"` // Max concurrent extractions
	ExtractorTimeout     time.Duration `mapstructure:"extractor_timeout"`     // Timeout per extraction

	// Performance and limits
	MaxLatency      time.Duration `mapstructure:"max_latency"`       // Max retrieval latency budget
	IngestBatchSize int           `mapstructure:"ingest_batch_size"` // Batch size for parallel ingest
	CacheCapacity   int           `mapstructure:"cache_capacity"`    // Cache capacity for embeddings/summaries

	// Observability
	EnableMetrics bool `mapstructure:"enable_metrics"` // Enable detailed metrics collection
	EnableTracing bool `mapstructure:"enable_tracing"` // Enable tracing for memory operations
}

var AppConfig Config

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(configPath string) (*Config, error) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("..")
		viper.AddConfigPath(filepath.Join("etc", internal.DefaultAppName))
		viper.AddConfigPath(internal.DefaultConfigPath)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Set default values
	viper.SetDefault("vvfs.targetDir", ".")
	viper.SetDefault("vvfs.cacheDir", internal.DefaultCacheDir)
	viper.SetDefault("vvfs.database.dsn", internal.DefaultDatabaseDSN)
	viper.SetDefault("vvfs.database.type", internal.DefaultDatabaseType)
	viper.SetDefault("vvfs.organizeTimeoutMinutes", 10)

	// LibSQL embedded defaults only
	viper.SetDefault("vvfs.database.libsql_data_dir", internal.DefaultDatabaseDir)
	viper.SetDefault("vvfs.organizeTimeoutMinutes", 10)

	// Embedding defaults
	viper.SetDefault("embedding.provider", "hugot")
	viper.SetDefault("embedding.model_path", "google/gemma-3-1b") // EmbeddingGemma when available
	viper.SetDefault("embedding.dims", 768)
	viper.SetDefault("embedding.pooling", "mean")
	viper.SetDefault("embedding.batch_size", 32)

	// LLM defaults (Gemma 270M)
	viper.SetDefault("llm.provider", "hugot")
	viper.SetDefault("llm.model_path", "onnx-community/gemma-3-270m-it-ONNX")
	viper.SetDefault("llm.max_new_tokens", 512)
	viper.SetDefault("llm.temperature", 0.3)
	viper.SetDefault("llm.top_p", 0.9)
	viper.SetDefault("llm.min_p", 0.15)
	viper.SetDefault("llm.repetition_penalty", 1.05)

	// ONNX defaults (optimized for performance)
	viper.SetDefault("onnx.backend", "ort")
	viper.SetDefault("onnx.ep", "cpu")
	viper.SetDefault("onnx.inter_op_threads", 1)  // Optimized for multi-goroutine fan-out
	viper.SetDefault("onnx.intra_op_threads", 1)  // Optimized for multi-goroutine fan-out
	viper.SetDefault("onnx.cpu_mem_arena", false) // Disable for high-throughput
	viper.SetDefault("onnx.mem_pattern", false)   // Disable for high-throughput

	// Harness defaults (production-optimized)
	viper.SetDefault("harness.cache_enabled", true)
	viper.SetDefault("harness.cache_capacity", 1000)
	viper.SetDefault("harness.cache_ttl_seconds", 3600) // 1 hour
	viper.SetDefault("harness.rate_limit_enabled", true)
	viper.SetDefault("harness.rate_limit_capacity", 10)
	viper.SetDefault("harness.rate_limit_refill_rate", "1s")
	viper.SetDefault("harness.max_tool_depth", 3)
	viper.SetDefault("harness.max_iterations", 10)
	viper.SetDefault("harness.max_output_size", 10000) // 10KB
	viper.SetDefault("harness.enable_guardrails", true)
	viper.SetDefault("harness.blocked_words", []string{"password", "secret", "key", "token", "credential"})
	viper.SetDefault("harness.allowed_tools", []string{}) // Empty means allow all by default
	viper.SetDefault("harness.enable_tracing", true)
	viper.SetDefault("harness.tool_concurrency", 5)

	// Memory defaults (retrieval-optimized)
	viper.SetDefault("memory.alpha", 0.5)     // Balanced fusion
	viper.SetDefault("memory.k", 8)           // Top-8 results
	viper.SetDefault("memory.threshold", 0.8) // Cosine similarity threshold
	viper.SetDefault("memory.lambda", 0.1)    // Gentle time decay
	viper.SetDefault("memory.autocut", true)
	viper.SetDefault("memory.time_decay", true)
	viper.SetDefault("memory.rerank", false) // Disabled by default for performance

	viper.SetDefault("memory.vector_index", "flat") // Start with simple flat index

	// HNSW defaults (tuned for 768-dim embeddings)
	viper.SetDefault("memory.hnsw_m", 32)
	viper.SetDefault("memory.hnsw_ef_construction", 128)
	viper.SetDefault("memory.hnsw_ef_search", 64)

	// LEANN defaults (experimental low-storage mode)
	viper.SetDefault("memory.leann_enabled", false)
	viper.SetDefault("memory.leann_pq_dim", 64) // PQ subvector dim
	viper.SetDefault("memory.leann_batch_size", 64)
	viper.SetDefault("memory.leann_storage_budget", 0.05) // 5% of raw data

	// Ensemble defaults (off by default for simplicity)
	viper.SetDefault("memory.ensemble_enabled", false)
	viper.SetDefault("memory.ensemble_strategy", "weighted_rrf")
	viper.SetDefault("memory.router_mode", "rules")

	viper.SetDefault("memory.weights_bm25", 0.35)
	viper.SetDefault("memory.weights_vector", 0.55)
	viper.SetDefault("memory.weights_graph", 0.10)

	// Graph defaults (off by default)
	viper.SetDefault("memory.graph_enabled", false)
	viper.SetDefault("memory.graph_depth", 2)
	viper.SetDefault("memory.graph_center_policy", "top_entity")
	viper.SetDefault("memory.graph_rerank_only", true) // Use only for reranking

	// Knowledge extraction defaults
	viper.SetDefault("memory.extractor_provider", "openai") // Requires API key
	viper.SetDefault("memory.extractor_concurrency", 3)
	viper.SetDefault("memory.extractor_timeout", "30s")

	// Performance defaults
	viper.SetDefault("memory.max_latency", "200ms")
	viper.SetDefault("memory.ingest_batch_size", 32)
	viper.SetDefault("memory.cache_capacity", 1000)

	// Observability defaults
	viper.SetDefault("memory.enable_metrics", true)
	viper.SetDefault("memory.enable_tracing", true)

	viper.AutomaticEnv()
	// Replace dots with underscores in env var names e.g. genkit.plugins.googleAI.apiKey becomes GENKIT_PLUGINS_GOOGLEAI_APIKEY
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// TODO: Create default config file if not found
			// Config file not found; defaults will be used. This is not an error for the application to halt on.
			// It's good practice to log this situation if a logger is available here.
			// fmt.Printf("Warning: Config file not found at expected locations. Using default values. Searched: %s\n", viper.ConfigFileUsed())
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	err := viper.Unmarshal(&AppConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &AppConfig, nil
}
