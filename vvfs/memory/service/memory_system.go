package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// MemorySystem is the main entry point for the memory subsystem
// It coordinates all memory-related services and provides a unified interface
type MemorySystem struct {
	config *config.MemoryConfig

	// Core components
	embedder    Embedder
	vectorIndex VectorIndex
	lexical     LexicalIndex
	retriever   Retriever
	scorer      Scorer
	reranker    Reranker
	summarizer  Summarizer

	// Graph components
	extractor   KnowledgeExtractor
	graphStore  GraphStore
	graphSearch GraphSearch

	// Ensemble components
	ensemble     IndexEnsemble
	router       QueryRouter
	fusionRanker FusionRanker

	// Storage
	memoryStore  MemoryStore
	sessionStore SessionStore

	// Infrastructure
	ingester *Ingester
	metrics  *MetricsCollector

	// Database connection
	db *sql.DB
}

// MemorySystemConfig holds all configuration for initializing the memory system
type MemorySystemConfig struct {
	Config   *config.MemoryConfig
	DB       *sql.DB
	Embedder Embedder // Optional: if nil, will use default

	// Optional overrides for testing/customization
	VectorIndex  VectorIndex
	LexicalIndex LexicalIndex
	GraphStore   GraphStore
	MemoryStore  MemoryStore
	SessionStore SessionStore
}

// NewMemorySystem creates a fully configured memory system
func NewMemorySystem(ctx context.Context, cfg MemorySystemConfig) (*MemorySystem, error) {
	if cfg.Config == nil {
		return nil, fmt.Errorf("memory config is required")
	}
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	ms := &MemorySystem{
		config:  cfg.Config,
		db:      cfg.DB,
		metrics: NewMetricsCollector(),
	}

	// Initialize embedder
	if cfg.Embedder != nil {
		ms.embedder = cfg.Embedder
	} else {
		// Use default embedder (placeholder - to be implemented)
		ms.embedder = NewDefaultEmbedder()
	}

	// Initialize vector index based on config
	if cfg.VectorIndex != nil {
		ms.vectorIndex = cfg.VectorIndex
	} else {
		var err error
		ms.vectorIndex, err = ms.createVectorIndex()
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
	}

	// Initialize lexical index
	if cfg.LexicalIndex != nil {
		ms.lexical = cfg.LexicalIndex
	} else {
		ms.lexical = NewLexicalIndexImpl(cfg.DB, cfg.Config)
	}

	// Initialize scorer
	ms.scorer = NewScorer(cfg.Config)

	// Initialize retriever
	ms.retriever = NewRetriever(
		cfg.Config,
		ms.lexical,
		ms.vectorIndex,
		nil, // graphSearch will be set later
		ms.scorer,
		ms.metrics,
	)

	// Initialize summarizer
	ms.summarizer = NewSummarizer(cfg.Config)

	// Initialize graph components if enabled
	if cfg.Config.GraphEnabled {
		if err := ms.initializeGraphComponents(cfg); err != nil {
			return nil, fmt.Errorf("failed to initialize graph components: %w", err)
		}
	}

	// Initialize ensemble if enabled
	if cfg.Config.EnsembleEnabled {
		if err := ms.initializeEnsembleComponents(); err != nil {
			return nil, fmt.Errorf("failed to initialize ensemble components: %w", err)
		}
	}

	// Initialize storage
	if cfg.MemoryStore != nil {
		ms.memoryStore = cfg.MemoryStore
	} else {
		ms.memoryStore = NewMemoryStoreImpl(cfg.DB)
	}

	if cfg.SessionStore != nil {
		ms.sessionStore = cfg.SessionStore
	} else {
		ms.sessionStore = NewSessionStoreImpl(cfg.DB)
	}

	// Initialize ingester
	ms.ingester = NewIngester(
		cfg.Config,
		ms.vectorIndex,
		ms.lexical,
		ms.graphStore,
		ms.extractor,
		ms.metrics,
	)

	return ms, nil
}

// createVectorIndex creates the appropriate vector index based on config
func (ms *MemorySystem) createVectorIndex() (VectorIndex, error) {
	switch ms.config.VectorIndex {
	case "flat":
		return NewFlatIndexImpl(ms.db, ms.embedder.Dimension()), nil
	case "hnsw":
		return NewHNSWIndex(ms.config)
	case "leann":
		// LEANN mode - experimental - not yet implemented
		return nil, fmt.Errorf("LEANN vector index not yet implemented")
	case "external":
		// External ANN adapter - to be implemented
		return nil, fmt.Errorf("external vector index not yet implemented")
	default:
		return NewFlatIndexImpl(ms.db, ms.embedder.Dimension()), nil
	}
}

// initializeGraphComponents sets up the knowledge graph subsystem
func (ms *MemorySystem) initializeGraphComponents(cfg MemorySystemConfig) error {
	// Initialize graph store
	if cfg.GraphStore != nil {
		ms.graphStore = cfg.GraphStore
	} else {
		ms.graphStore = &GraphStoreImpl{db: ms.db}
	}

	// Initialize knowledge extractor
	ms.extractor = &KnowledgeExtractorImpl{
		// LLM client would be injected here based on extractor.provider config
		llmClient: nil, // Placeholder
		config:    ms.config,
	}

	// Initialize graph search
	ms.graphSearch = &GraphSearchImpl{
		store:  ms.graphStore,
		config: ms.config,
	}

	// Update retriever with graph search
	if retrieverImpl, ok := ms.retriever.(*RetrieverImpl); ok {
		retrieverImpl.graphSearch = ms.graphSearch
	}

	return nil
}

// initializeEnsembleComponents sets up the ensemble search subsystem
func (ms *MemorySystem) initializeEnsembleComponents() error {
	// Initialize router
	ms.router = &QueryRouterImpl{
		config: ms.config,
	}

	// Initialize fusion ranker
	ms.fusionRanker = &FusionRankerImpl{
		config: ms.config,
	}

	// Initialize ensemble
	ms.ensemble = &IndexEnsembleImpl{
		bm25:   ms.lexical,
		vector: ms.vectorIndex,
		graph:  ms.graphSearch,
		config: ms.config,
		router: ms.router,
		fusion: ms.fusionRanker,
	}

	// Initialize reranker if configured
	ms.reranker = &LTRankerImpl{
		config: ms.config,
	}

	return nil
}

// Ingest adds a memory item to the system
func (ms *MemorySystem) Ingest(ctx context.Context, item *MemoryItem) error {
	return ms.ingester.IngestMemoryItem(ctx, item)
}

// IngestWithEpisode ingests a memory item along with graph extraction
func (ms *MemorySystem) IngestWithEpisode(ctx context.Context, item *MemoryItem, episode *Episode) error {
	return ms.ingester.IngestWithPriority(ctx, item, episode, 0)
}

// Search performs hybrid retrieval
func (ms *MemorySystem) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if ms.config.EnsembleEnabled && ms.ensemble != nil {
		// Use ensemble search
		ensembleOpts := EnsembleSearchOptions{
			Query:    query,
			K:        opts.K,
			Strategy: FusionStrategy(ms.config.EnsembleStrategy),
		}
		ensembleResults, err := ms.ensemble.Search(ctx, query, ensembleOpts)
		if err != nil {
			return nil, err
		}

		// Fuse ensemble results
		var allResults []EnsembleResult
		allResults = append(allResults, ensembleResults...)
		return ms.fusionRanker.Fuse(ctx, allResults, ensembleOpts.Strategy)
	}

	// Use basic hybrid retrieval
	return ms.retriever.Search(ctx, query, opts)
}

// Summarize creates a structured summary of conversation messages
func (ms *MemorySystem) Summarize(ctx context.Context, messages []ConversationMessage) (Summary, error) {
	return ms.summarizer.Summarize(ctx, messages)
}

// GetMetrics returns current metrics
func (ms *MemorySystem) GetMetrics() MetricsSummary {
	return ms.metrics.GetSummary()
}

// Close gracefully shuts down the memory system
func (ms *MemorySystem) Close() error {
	// Stop ingester
	if ms.ingester != nil {
		if err := ms.ingester.Stop(); err != nil {
			return fmt.Errorf("failed to stop ingester: %w", err)
		}
	}

	// Close vector index
	if ms.vectorIndex != nil {
		if err := ms.vectorIndex.Close(); err != nil {
			return fmt.Errorf("failed to close vector index: %w", err)
		}
	}

	return nil
}

// GetMemoryStore returns the memory store for direct access
func (ms *MemorySystem) GetMemoryStore() MemoryStore {
	return ms.memoryStore
}

// GetSessionStore returns the session store for direct access
func (ms *MemorySystem) GetSessionStore() SessionStore {
	return ms.sessionStore
}

// GetGraphStore returns the graph store for direct access (if graph is enabled)
func (ms *MemorySystem) GetGraphStore() GraphStore {
	return ms.graphStore
}

// DefaultEmbedder is a placeholder embedder implementation
type DefaultEmbedder struct {
	dimension int
}

// NewDefaultEmbedder creates a default embedder
func NewDefaultEmbedder() *DefaultEmbedder {
	return &DefaultEmbedder{dimension: 384} // Common embedding dimension
}

// Embed generates embeddings (placeholder implementation)
func (e *DefaultEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	// Placeholder: In a real implementation, this would call an embedding model
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, e.dimension)
		// Zero embeddings for now
	}
	return result, nil
}

// Dimension returns the embedding dimension
func (e *DefaultEmbedder) Dimension() int {
	return e.dimension
}
