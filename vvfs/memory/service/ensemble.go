package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// IndexEnsembleImpl implements IndexEnsemble for coordinating multiple indexes
type IndexEnsembleImpl struct {
	bm25   LexicalIndex
	vector VectorIndex
	graph  GraphSearch
	config *config.MemoryConfig
	router QueryRouter
	fusion FusionRanker
}

// NewIndexEnsemble creates a new index ensemble
func NewIndexEnsemble(bm25 LexicalIndex, vector VectorIndex, graph GraphSearch, config *config.MemoryConfig, router QueryRouter, fusion FusionRanker) *IndexEnsembleImpl {
	return &IndexEnsembleImpl{
		bm25:   bm25,
		vector: vector,
		graph:  graph,
		config: config,
		router: router,
		fusion: fusion,
	}
}

// Search performs ensemble search across multiple indexes
func (ie *IndexEnsembleImpl) Search(ctx context.Context, query string, opts EnsembleSearchOptions) ([]EnsembleResult, error) {
	// Route to determine which indexes to query and with what parameters
	decision, err := ie.router.Route(ctx, query, RoutingOptions{
		Query: query,
		Budget: CostBudget{
			MaxLatency: ie.config.MaxLatency,
			MaxCost:    100.0, // Placeholder cost budget
		},
		Filters: nil, // No filters for now
	})
	if err != nil {
		return nil, fmt.Errorf("failed to route query: %w", err)
	}

	// Fan out to selected indexes in parallel
	var wg sync.WaitGroup
	resultsChan := make(chan EnsembleResult, len(decision.Indexes))

	for _, indexConfig := range decision.Indexes {
		if !indexConfig.Enabled {
			continue
		}

		wg.Add(1)
		go func(config IndexConfig) {
			defer wg.Done()

			var searchResults []SearchResult
			var err error

			switch config.Name {
			case "bm25":
				searchResults, err = ie.bm25.Query(ctx, query, opts.K)
			case "vector":
				// For vector search, we need an embedding of the query
				// This is a placeholder - in practice, embed the query first
				// For now, assume we have a query vector or embed it here
				// searchResults, err = ie.vector.Query(ctx, queryVector, opts.K)
				searchResults = []SearchResult{} // Placeholder
			case "graph":
				// Graph search requires a center entity
				// This is a placeholder implementation
				graphResults, err := ie.graph.SearchWithPathBoost(ctx, query, GraphSearchOptions{
					Query: query,
					K:     opts.K,
				})
				if err != nil {
					return
				}
				// Convert GraphSearchResult to SearchResult
				for _, gr := range graphResults {
					searchResults = append(searchResults, SearchResult{
						ID:         gr.EntityID,
						Score:      gr.Score,
						Provenance: "graph",
					})
				}
			default:
				// Handle other indexes (e.g., external ANN)
			}

			if err != nil {
				// Log error but don't fail the entire ensemble
				return
			}

			resultsChan <- EnsembleResult{
				Source:  config.Name,
				Results: searchResults,
				Metadata: map[string]interface{}{
					"config": config,
				},
			}
		}(indexConfig)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var ensembleResults []EnsembleResult
	for result := range resultsChan {
		ensembleResults = append(ensembleResults, result)
	}

	return ensembleResults, nil
}
