package service

import (
	"context"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// QueryRouterImpl implements QueryRouter for deciding which indexes to query
type QueryRouterImpl struct {
	config *config.MemoryConfig
}

// NewQueryRouter creates a new query router
func NewQueryRouter(config *config.MemoryConfig) *QueryRouterImpl {
	return &QueryRouterImpl{config: config}
}

// Route determines which indexes to query based on query characteristics
func (qr *QueryRouterImpl) Route(ctx context.Context, query string, opts RoutingOptions) (*RoutingDecision, error) {
	decision := &RoutingDecision{
		Indexes: []IndexConfig{},
		Weights: map[string]float64{},
	}

	// Simple rule-based routing (can be extended with ML)
	queryLower := strings.ToLower(query)

	// Always include BM25 for keyword-heavy queries
	if qr.isKeywordHeavy(queryLower) {
		decision.Indexes = append(decision.Indexes, IndexConfig{
			Name:    "bm25",
			Enabled: true,
			Options: map[string]interface{}{"boost": 1.2},
		})
		decision.Weights["bm25"] = qr.config.WeightsBM25 * 1.2
	} else {
		decision.Indexes = append(decision.Indexes, IndexConfig{
			Name:    "bm25",
			Enabled: true,
		})
		decision.Weights["bm25"] = qr.config.WeightsBM25
	}

	// Include vector search for semantic queries
	if qr.isSemanticQuery(queryLower) {
		decision.Indexes = append(decision.Indexes, IndexConfig{
			Name:    "vector",
			Enabled: true,
			Options: map[string]interface{}{"ef_search": qr.config.HNSWEFSearch},
		})
		decision.Weights["vector"] = qr.config.WeightsVector
	}

	// Include graph search if graph is enabled and query suggests relationships
	if qr.config.GraphEnabled && qr.isGraphQuery(queryLower) {
		decision.Indexes = append(decision.Indexes, IndexConfig{
			Name:    "graph",
			Enabled: true,
			Options: map[string]interface{}{
				"depth":       qr.config.GraphDepth,
				"rerank_only": qr.config.GraphRerankOnly,
			},
		})
		decision.Weights["graph"] = qr.config.WeightsGraph
	}

	// Apply budget constraints
	if opts.Budget.MaxLatency > 0 {
		qr.applyLatencyBudget(decision, opts.Budget.MaxLatency)
	}

	return decision, nil
}

// isKeywordHeavy checks if query is keyword-focused
func (qr *QueryRouterImpl) isKeywordHeavy(query string) bool {
	// Simple heuristic: queries with quotes, specific terms, or short length
	return strings.Contains(query, "\"") || len(strings.Fields(query)) <= 3
}

// isSemanticQuery checks if query is semantic/conceptual
func (qr *QueryRouterImpl) isSemanticQuery(query string) bool {
	// Semantic queries are longer, more descriptive
	return len(query) > 20 || strings.Contains(query, "how") || strings.Contains(query, "why")
}

// isGraphQuery checks if query suggests relationships or entities
func (qr *QueryRouterImpl) isGraphQuery(query string) bool {
	// Queries mentioning entities, relationships, or complex concepts
	graphKeywords := []string{"related to", "works with", "project", "team", "concept"}
	for _, keyword := range graphKeywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

// applyLatencyBudget adjusts decision based on latency constraints
func (qr *QueryRouterImpl) applyLatencyBudget(decision *RoutingDecision, maxLatency time.Duration) {
	// If latency budget is tight, disable slower indexes
	if maxLatency < 100*time.Millisecond {
		// Disable graph search which might be slower
		for i, idx := range decision.Indexes {
			if idx.Name == "graph" {
				decision.Indexes[i].Enabled = false
				delete(decision.Weights, "graph")
			}
		}
	}
}
