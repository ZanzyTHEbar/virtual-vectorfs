package service

import (
	"context"
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/stretchr/testify/assert"
)

// MockLexicalIndex for testing
type MockLexicalIndex struct {
	Results []SearchResult
}

func (m *MockLexicalIndex) Query(ctx context.Context, query string, k int) ([]SearchResult, error) {
	return m.Results, nil
}

// MockVectorIndex for testing
type MockVectorIndex struct {
	Results []SearchResult
}

func (m *MockVectorIndex) Upsert(ctx context.Context, id string, vector []float64) error {
	return nil
}

func (m *MockVectorIndex) Query(ctx context.Context, query []float64, k int) ([]SearchResult, error) {
	return m.Results, nil
}

func (m *MockVectorIndex) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockVectorIndex) Close() error {
	return nil
}

// MockGraphSearch for testing
type MockGraphSearch struct {
	Results []GraphSearchResult
}

func (m *MockGraphSearch) SearchFromCenter(ctx context.Context, centerID string, query string, depth int, k int) ([]GraphSearchResult, error) {
	return m.Results, nil
}

func (m *MockGraphSearch) SearchWithPathBoost(ctx context.Context, query string, opts GraphSearchOptions) ([]GraphSearchResult, error) {
	return m.Results, nil
}

// MockQueryRouter for testing
type MockQueryRouter struct {
	Decision *RoutingDecision
}

func (m *MockQueryRouter) Route(ctx context.Context, query string, opts RoutingOptions) (*RoutingDecision, error) {
	return m.Decision, nil
}

// MockFusionRanker for testing
type MockFusionRanker struct {
	Results []SearchResult
}

func (m *MockFusionRanker) Fuse(ctx context.Context, results []EnsembleResult, strategy FusionStrategy) ([]SearchResult, error) {
	return m.Results, nil
}

// TestIndexEnsembleImpl_Search tests ensemble search coordination
func TestIndexEnsembleImpl_Search(t *testing.T) {
	mockBM25 := &MockLexicalIndex{Results: []SearchResult{{ID: "doc1", Score: 0.8}}}
	mockVector := &MockVectorIndex{Results: []SearchResult{{ID: "doc2", Score: 0.9}}}
	mockGraph := &MockGraphSearch{Results: []GraphSearchResult{{EntityID: "entity1", Score: 0.7}}}
	mockRouter := &MockQueryRouter{Decision: &RoutingDecision{}}
	mockFusion := &MockFusionRanker{Results: []SearchResult{{ID: "doc1", Score: 1.0}, {ID: "doc2", Score: 0.9}}}

	config := &config.MemoryConfig{}
	ensemble := NewIndexEnsemble(mockBM25, mockVector, mockGraph, config, mockRouter, mockFusion)

	results, err := ensemble.Search(context.Background(), "test query", EnsembleSearchOptions{
		Query:    "test query",
		K:        5,
		Strategy: FusionWeightedRRF,
	})

	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestFusionRankerImpl_Fuse tests fusion strategies
func TestFusionRankerImpl_Fuse(t *testing.T) {
	config := &config.MemoryConfig{
		WeightsBM25:   0.35,
		WeightsVector: 0.55,
		WeightsGraph:  0.10,
	}

	fusion := NewFusionRanker(config)

	ensembleResults := []EnsembleResult{
		{
			Source:  "bm25",
			Results: []SearchResult{{ID: "doc1", Score: 0.8}},
		},
		{
			Source:  "vector",
			Results: []SearchResult{{ID: "doc2", Score: 0.9}},
		},
	}

	results, err := fusion.Fuse(context.Background(), ensembleResults, FusionWeightedRRF)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}
