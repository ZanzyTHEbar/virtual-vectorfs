package service

import (
	"context"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/stretchr/testify/assert"
)

// TestQueryRouterImpl_Route tests query routing logic
func TestQueryRouterImpl_Route(t *testing.T) {
	config := &config.MemoryConfig{
		WeightsBM25:   0.35,
		WeightsVector: 0.55,
		WeightsGraph:  0.10,
		GraphEnabled:  true,
	}

	router := NewQueryRouter(config)

	decision, err := router.Route(context.Background(), "test query", RoutingOptions{
		Query: "test query",
		Budget: CostBudget{
			MaxLatency: 100 * time.Millisecond,
		},
	})

	assert.NoError(t, err)
	assert.Len(t, decision.Indexes, 2) // BM25 and vector for semantic query
	assert.Contains(t, decision.Weights, "bm25")
	assert.Contains(t, decision.Weights, "vector")
}

// TestQueryRouterImpl_isSemanticQuery tests semantic query detection
func TestQueryRouterImpl_isSemanticQuery(t *testing.T) {
	router := NewQueryRouter(&config.MemoryConfig{})

	assert.True(t, router.isSemanticQuery("How does machine learning work?"))
	assert.False(t, router.isSemanticQuery("cat dog"))
}

// TestQueryRouterImpl_isGraphQuery tests graph query detection
func TestQueryRouterImpl_isGraphQuery(t *testing.T) {
	router := NewQueryRouter(&config.MemoryConfig{})

	assert.True(t, router.isGraphQuery("What projects is John working on?"))
	assert.False(t, router.isGraphQuery("simple search"))
}
