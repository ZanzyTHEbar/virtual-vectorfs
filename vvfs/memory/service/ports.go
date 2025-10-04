package service

import (
	"context"
	"time"
)

// Embedder generates embeddings for text content
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	Dimension() int
}

// VectorIndex manages vector storage and similarity search
type VectorIndex interface {
	Upsert(ctx context.Context, id string, vector []float64) error
	Query(ctx context.Context, query []float64, k int) ([]SearchResult, error)
	Delete(ctx context.Context, id string) error
	Close() error
}

// LexicalIndex manages BM25/FTS5 search
type LexicalIndex interface {
	Query(ctx context.Context, query string, k int) ([]SearchResult, error)
}

// Retriever orchestrates hybrid retrieval
type Retriever interface {
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}

// Reranker optionally reranks results
type Reranker interface {
	Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error)
}

// Summarizer handles working memory summarization
type Summarizer interface {
	Summarize(ctx context.Context, messages []ConversationMessage) (Summary, error)
}

// Scorer handles fusion, thresholding, and boosters
type Scorer interface {
	FuseScores(results []SearchResult, alpha float64) []SearchResult
	ApplyThresholds(results []SearchResult, threshold float64) []SearchResult
	ApplyAutocut(results []SearchResult) []SearchResult
	ApplyTimeDecay(results []SearchResult, lambda float64) []SearchResult
	ApplySpatialBoost(results []SearchResult, center []float64, radius float64) []SearchResult
}

// MemoryStore manages memory items and sessions
type MemoryStore interface {
	GetItem(ctx context.Context, id string) (*MemoryItem, error)
	PutItem(ctx context.Context, item *MemoryItem) error
	DeleteItem(ctx context.Context, id string) error
	ListItems(ctx context.Context, opts ListOptions) ([]*MemoryItem, error)
}

// SessionStore manages conversation sessions
type SessionStore interface {
	GetSession(ctx context.Context, id string) (*Session, error)
	PutSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, id string) error
}

// NEW: KnowledgeExtractor extracts entities and edges from episodes
type KnowledgeExtractor interface {
	Extract(ctx context.Context, episode Episode) (*ExtractionResult, error)
}

// GraphStore manages entities and edges with temporal semantics
type GraphStore interface {
	GetEntity(ctx context.Context, id string) (*Entity, error)
	UpsertEntity(ctx context.Context, entity *Entity) error
	DeleteEntity(ctx context.Context, id string) error
	ListEntities(ctx context.Context, opts ListOptions) ([]*Entity, error)

	GetEdge(ctx context.Context, id string) (*Edge, error)
	UpsertEdge(ctx context.Context, edge *Edge) error
	InvalidateEdge(ctx context.Context, id string, reason string) error
	ListEdges(ctx context.Context, opts ListOptions) ([]*Edge, error)
	// Temporal queries
	GetEdgesAsOf(ctx context.Context, timepoint time.Time, opts ListOptions) ([]*Edge, error)
	GetCurrentEdges(ctx context.Context, opts ListOptions) ([]*Edge, error)
}

// GraphSearch performs graph-based retrieval and reranking
type GraphSearch interface {
	SearchFromCenter(ctx context.Context, centerID string, query string, depth int, k int) ([]GraphSearchResult, error)
	SearchWithPathBoost(ctx context.Context, query string, opts GraphSearchOptions) ([]GraphSearchResult, error)
}

// IndexEnsemble coordinates multiple indexes
type IndexEnsemble interface {
	Search(ctx context.Context, query string, opts EnsembleSearchOptions) ([]EnsembleResult, error)
}

// QueryRouter decides which indexes to query and with what weights
type QueryRouter interface {
	Route(ctx context.Context, query string, opts RoutingOptions) (*RoutingDecision, error)
}

// FusionRanker combines results from multiple sources
type FusionRanker interface {
	Fuse(ctx context.Context, results []EnsembleResult, strategy FusionStrategy) ([]SearchResult, error)
}

// CostPolicy enforces budgets on search operations
type CostPolicy interface {
	ApplyBudget(ctx context.Context, decision *RoutingDecision) (*RoutingDecision, error)
}

// Supporting types and structs

// SearchResult represents a search hit
type SearchResult struct {
	ID         string                 `json:"id"`
	Score      float64                `json:"score"`
	Metadata   map[string]interface{} `json:"metadata"`
	Provenance string                 `json:"provenance"` // Which index/source
}

// ConversationMessage represents a message in working memory
type ConversationMessage struct {
	Role    string `json:"role"` // user/assistant/tool
	Content string `json:"content"`
}

// Summary represents a summarized conversation
type Summary struct {
	Content  string            `json:"content"`
	Sections map[string]string `json:"sections"`
}

// MemoryItem represents a stored memory item
type MemoryItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	Metadata  map[string]interface{} `json:"metadata"`
	Embedding []float64              `json:"embedding"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt *time.Time             `json:"expires_at"`
	SourceRef string                 `json:"source_ref"`
}

// Session represents a conversation session
type Session struct {
	ID        string    `json:"id"`
	Messages  []string  `json:"messages"` // Serialized messages
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Episode represents input for knowledge extraction
type Episode struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	EventTime *time.Time             `json:"event_time"` // When the episode occurred
}

// ExtractionResult from knowledge extraction
type ExtractionResult struct {
	Entities []Entity `json:"entities"`
	Edges    []Edge   `json:"edges"`
}

// Entity represents a node in the knowledge graph
type Entity struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Name      string                 `json:"name"`
	Summary   string                 `json:"summary"`
	Attrs     map[string]interface{} `json:"attrs"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Edge represents a relationship in the knowledge graph
type Edge struct {
	ID            string                 `json:"id"`
	SourceID      string                 `json:"source_id"`
	TargetID      string                 `json:"target_id"`
	Relation      string                 `json:"relation"`
	Attrs         map[string]interface{} `json:"attrs"`
	ValidFrom     time.Time              `json:"valid_from"`
	ValidTo       *time.Time             `json:"valid_to"`
	IngestedAt    time.Time              `json:"ingested_at"`
	InvalidatedAt *time.Time             `json:"invalidated_at"`
	Provenance    map[string]interface{} `json:"provenance"`
}

// GraphSearchResult represents a result from graph search
type GraphSearchResult struct {
	EntityID   string  `json:"entity_id"`
	Score      float64 `json:"score"`
	PathLength int     `json:"path_length"`
	Relation   string  `json:"relation"`
}

// EnsembleResult represents results from one index in the ensemble
type EnsembleResult struct {
	Source   string                 `json:"source"` // e.g., "bm25", "hnsw", "graph"
	Results  []SearchResult         `json:"results"`
	Metadata map[string]interface{} `json:"metadata"`
}

// SearchOptions for retrieval
type SearchOptions struct {
	K               int                    `json:"k"`
	Alpha           float64                `json:"alpha"`
	Threshold       float64                `json:"threshold"`
	Lambda          float64                `json:"lambda"`
	SpatialCenter   []float64              `json:"spatial_center"`
	SpatialRadius   float64                `json:"spatial_radius"`
	MetadataFilters map[string]interface{} `json:"metadata_filters"`
	TimeDecay       bool                   `json:"time_decay"`
	Autocut         bool                   `json:"autocut"`
	Rerank          bool                   `json:"rerank"`
	GraphDepth      int                    `json:"graph_depth"`
}

// EnsembleSearchOptions for ensemble search
type EnsembleSearchOptions struct {
	Query    string                 `json:"query"`
	K        int                    `json:"k"`
	Strategy FusionStrategy         `json:"strategy"`
	Options  map[string]interface{} `json:"options"` // Per-index options
}

// RoutingOptions for query routing
type RoutingOptions struct {
	Query   string                 `json:"query"`
	Budget  CostBudget             `json:"budget"`
	Filters map[string]interface{} `json:"filters"`
}

// RoutingDecision from the router
type RoutingDecision struct {
	Indexes []IndexConfig      `json:"indexes"`
	Weights map[string]float64 `json:"weights"`
}

// IndexConfig for each index in the ensemble
type IndexConfig struct {
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Options map[string]interface{} `json:"options"`
}

// CostBudget for limiting search costs
type CostBudget struct {
	MaxLatency time.Duration `json:"max_latency"`
	MaxCost    float64       `json:"max_cost"`
}

// FusionStrategy for combining results
type FusionStrategy string

const (
	FusionRRF           FusionStrategy = "rrf"
	FusionWeightedRRF   FusionStrategy = "weighted_rrf"
	FusionRelativeScore FusionStrategy = "relative_score"
	FusionLTR           FusionStrategy = "ltr"
)

// GraphSearchOptions for graph search
type GraphSearchOptions struct {
	CenterID    string `json:"center_id"`
	Query       string `json:"query"`
	Depth       int    `json:"depth"`
	K           int    `json:"k"`
	PathWeights bool   `json:"path_weights"`
}

// ListOptions for listing operations
type ListOptions struct {
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Filter map[string]interface{} `json:"filter"`
	Sort   string                 `json:"sort"`
}
