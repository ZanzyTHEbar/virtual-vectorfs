# LLM Harness Memory System

A production-ready, spatio-temporal hybrid memory system for LLM applications with support for semantic search, knowledge graphs, and multi-index ensemble retrieval.

## Features

### Core Capabilities

- **Hybrid Retrieval**: Combines BM25 lexical search with dense vector embeddings
- **Spatio-Temporal Awareness**: Time-based decay, geospatial filtering via RTree
- **Knowledge Graph**: Bi-temporal entity-relationship graph with temporal invalidation
- **Multi-Index Ensemble**: Intelligent routing and fusion across BM25, HNSW, and graph indexes
- **Working Memory**: Conversation summarization with guardrails
- **Flexible Architecture**: Pluggable components via hexagonal architecture

### Memory Types

1. **Episodic**: Individual experiences, events, conversations
2. **Semantic**: General knowledge, documents, RAG corpus
3. **Procedural**: Rules, tools, prompts, workflows
4. **Entity**: Knowledge graph of people, projects, concepts, and relationships

## Quick Start

### Installation

```bash
go get github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory
```

### Basic Usage

```go
package main

import (
    "context"
    "database/sql"
    "log"
    
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/service"
    
    _ "github.com/mattn/go-sqlite3"
)

func main() {
    // 1. Open database
    db, err := sql.Open("sqlite3", "memory.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 2. Configure memory system
    memCfg := &config.MemoryConfig{
        VectorIndex: "flat",
        Alpha:       0.5,
        K:           10,
    }
    
    // 3. Initialize
    ctx := context.Background()
    memSys, err := service.NewMemorySystem(ctx, service.MemorySystemConfig{
        Config: memCfg,
        DB:     db,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer memSys.Close()
    
    // 4. Ingest content
    item := &service.MemoryItem{
        Type: "document",
        Text: "Machine learning is a subset of artificial intelligence.",
        Metadata: map[string]interface{}{
            "category": "AI",
        },
    }
    
    if err := memSys.Ingest(ctx, item); err != nil {
        log.Fatal(err)
    }
    
    // 5. Search
    results, err := memSys.Search(ctx, "what is machine learning", service.SearchOptions{
        K:     5,
        Alpha: 0.5,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    for _, result := range results {
        log.Printf("Found: %s (score: %.4f)", result.ID, result.Score)
    }
}
```

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Memory System                         │
│                                                          │
│  ┌───────────┐  ┌───────────┐  ┌──────────────┐       │
│  │  Ingester │  │ Retriever │  │ Summarizer   │       │
│  └─────┬─────┘  └─────┬─────┘  └──────┬───────┘       │
│        │              │                │                │
│        ▼              ▼                ▼                │
│  ┌──────────────────────────────────────────────┐      │
│  │           Index Ensemble                     │      │
│  │  ┌─────────┐ ┌──────────┐ ┌──────────────┐ │      │
│  │  │  BM25   │ │  Vector  │ │  Graph       │ │      │
│  │  │  FTS5   │ │  Index   │ │  Search      │ │      │
│  │  └─────────┘ └──────────┘ └──────────────┘ │      │
│  │         │          │             │          │      │
│  │         └──────────┴─────────────┘          │      │
│  │                    │                        │      │
│  │              Fusion Ranker                  │      │
│  └──────────────────────────────────────────────┘      │
│                       │                                │
│               ┌───────┴────────┐                       │
│               │                │                        │
│         Memory Store      Session Store                │
│         (SQLite/libsql)   (SQLite/libsql)              │
└─────────────────────────────────────────────────────────┘
```

### Hexagonal Architecture

All components implement well-defined interfaces (ports), allowing for:

- **Testability**: Easy mocking and testing
- **Flexibility**: Swap implementations without breaking code
- **Maintainability**: Clear separation of concerns

Core ports:

- `Embedder`: Text → vector embeddings
- `VectorIndex`: Vector storage and k-NN search
- `LexicalIndex`: BM25/FTS5 search
- `GraphStore`: Entity/edge CRUD with temporal semantics
- `Retriever`: Hybrid search orchestration
- `Reranker`: Cross-encoder re-ranking
- `Summarizer`: Working memory summarization

## Configuration

### Memory Config Options

```go
type MemoryConfig struct {
    // Vector index selection
    VectorIndex string // "flat", "hnsw", "leann", "external"
    
    // HNSW configuration
    HNSWConfig *HNSWConfig
    
    // Ensemble configuration
    EnsembleEnabled  bool
    EnsembleStrategy string // "rrf", "weighted_rrf", "relative", "ltr"
    RouterMode       string // "rules", "ml"
    IndexWeights     map[string]float64
    
    // Graph configuration
    GraphEnabled      bool
    GraphDepth        int
    GraphCenterPolicy string // "hybrid_top", "metadata_filter"
    
    // Retrieval parameters
    Alpha             float64 // BM25 vs vector fusion weight (0-1)
    K                 int     // Top-K results
    Lambda            float64 // Time decay factor
    DistanceThreshold float64 // Similarity threshold
    Autocut           bool    // Enable autocut
    Rerank            bool    // Enable reranking
    TimeDecay         bool    // Enable time-based decay
    
    // Performance tuning
    AsyncIngest      bool
    IngestWorkers    int
    IngestBufferSize int
    
    // Observability
    MetricsEnabled bool
    TracingEnabled bool
}
```

### Recommended Configurations

#### Development / Testing

```go
&config.MemoryConfig{
    VectorIndex:  "flat",
    GraphEnabled: false,
    Alpha:        0.5,
    K:            10,
}
```

#### Production (Small Scale <100k items)

```go
&config.MemoryConfig{
    VectorIndex:      "hnsw",
    HNSWConfig:       &config.HNSWConfig{M: 16, EfConstruction: 100, EfSearch: 50},
    EnsembleEnabled:  true,
    EnsembleStrategy: "weighted_rrf",
    GraphEnabled:     true,
    GraphDepth:       2,
    Alpha:            0.5,
    K:                10,
    AsyncIngest:      true,
    IngestWorkers:    4,
}
```

#### Production (Large Scale >100k items)

```go
&config.MemoryConfig{
    VectorIndex:      "hnsw",
    HNSWConfig:       &config.HNSWConfig{M: 32, EfConstruction: 128, EfSearch: 64},
    EnsembleEnabled:  true,
    EnsembleStrategy: "weighted_rrf",
    GraphEnabled:     true,
    GraphDepth:       2,
    Alpha:            0.6, // Favor vector search at scale
    K:                10,
    AsyncIngest:      true,
    IngestWorkers:    8,
    IngestBufferSize: 2000,
    MetricsEnabled:   true,
    TracingEnabled:   true,
}
```

## Advanced Features

### Knowledge Graph Extraction

```go
episode := &service.Episode{
    Content: `
        Alice works on the ML platform.
        Bob is the tech lead for ML platform.
        The ML platform uses PyTorch.
    `,
}

item := &service.MemoryItem{
    Type: "episode",
    Text: episode.Content,
}

// Automatically extracts entities and relationships
memSys.IngestWithEpisode(ctx, item, episode)

// Entities: Alice, Bob, ML platform, PyTorch
// Edges: Alice-[works_on]->ML platform, Bob-[tech_lead]->ML platform, ML platform-[uses]->PyTorch
```

### Temporal Queries

```go
// Get edges as of a specific time
edges, err := graphStore.GetEdgesAsOf(ctx, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), opts)

// Get only current (non-invalidated) edges
currentEdges, err := graphStore.GetCurrentEdges(ctx, opts)

// Invalidate contradictory information
graphStore.InvalidateEdge(ctx, edgeID, "contradicted by new information")
```

### Ensemble Search Strategies

1. **Reciprocal Rank Fusion (RRF)**: Simple, parameter-free fusion
2. **Weighted RRF**: Assign importance weights to each index
3. **Relative Score Fusion**: Normalize scores relative to max per index
4. **Learning-to-Rank (LTR)**: Train a model on labeled data (future)

### Metrics and Observability

```go
metrics := memSys.GetMetrics()
// Returns: MetricsSummary{
//   IngestQPS: 150.2,
//   SearchQPS: 45.8,
//   AvgSearchLatencyMs: 23.5,
//   RecallAtK: map[int]float64{5: 0.85, 10: 0.92},
//   IndexContributions: map[string]float64{
//     "bm25": 0.35,
//     "vector": 0.55,
//     "graph": 0.10,
//   },
// }
```

## Performance Characteristics

| Operation | Latency (p99) | Throughput | Scale |
|-----------|---------------|------------|-------|
| Ingest (sync) | ~150ms | ~100 items/s | <1M items |
| Ingest (async) | ~50ms | ~500 items/s | <1M items |
| Search (flat) | ~30ms | ~150 queries/s | <100k items |
| Search (HNSW) | ~10ms | ~300 queries/s | <1M items |
| Graph traversal (D=2) | ~20ms | ~100 queries/s | <100k entities |

*Benchmarked on typical hardware (4 cores, 16GB RAM)*

## Migration Guide

### From v0.x to v1.0

1. Update imports:

```go
// Old
import "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory"

// New
import "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/service"
```

2. Configuration changes:

```go
// Old
cfg := &memory.Config{
    IndexType: "flat",
    TopK: 10,
}

// New
cfg := &config.MemoryConfig{
    VectorIndex: "flat",
    K: 10,
}
```

3. API changes:

```go
// Old
results, _ := mem.Query(ctx, "query", 10)

// New
results, _ := mem.Search(ctx, "query", service.SearchOptions{K: 10})
```

## Testing

Run the test suite:

```bash
go test ./vvfs/memory/... -v
```

Run integration tests:

```bash
go test ./vvfs/memory/service -tags=integration -v
```

Run benchmarks:

```bash
go test ./vvfs/memory/service -bench=. -benchmem
```

## Contributing

1. Follow TDD: Write tests first
2. Use table-driven tests
3. Document all public APIs
4. Run linter: `go vet ./vvfs/memory/...`
5. Format code: `go fmt ./vvfs/memory/...`

## License

See [LICENSE](../../LICENSE)

## References

- [Plan Document](../../plan.md)
- [API Reference](../../docs/API_REFERENCE.md)
- [Production Guide](../../docs/PRODUCTION_README.md)

