package service

import (
	"context"
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGraphSearchImpl_SearchFromCenter tests center-based graph search
func TestGraphSearchImpl_SearchFromCenter(t *testing.T) {
	mockStore := new(MockGraphStore)
	config := &config.MemoryConfig{
		GraphDepth: 2,
		K:          5,
	}

	search := NewGraphSearch(mockStore, config)

	// Mock entity and edges
	entity := &Entity{ID: "center1", Kind: "person", Name: "John Doe"}
	mockStore.On("GetEntity", mock.Anything, "center1").Return(entity, nil)

	edges := []*Edge{
		{ID: "edge1", SourceID: "center1", TargetID: "entity2", Relation: "works_with"},
	}
	mockStore.On("ListEdges", mock.Anything, mock.Anything).Return(edges, nil)

	results, err := search.SearchFromCenter(context.Background(), "center1", "test query", 2, 5)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "entity2", results[0].EntityID)

	mockStore.AssertExpectations(t)
}

// TestGraphSearchImpl_calculateGraphScore tests graph score calculation
func TestGraphSearchImpl_calculateGraphScore(t *testing.T) {
	mockStore := new(MockGraphStore)
	config := &config.MemoryConfig{}
	search := NewGraphSearch(mockStore, config)

	edge := &Edge{Relation: "works_on"}
	score := search.calculateGraphScore(edge, 1)
	assert.Greater(t, score, 0.0)
}

// TestGraphSearchImpl_calculatePathWeight tests path weight calculation
func TestGraphSearchImpl_calculatePathWeight(t *testing.T) {
	mockStore := new(MockGraphStore)
	config := &config.MemoryConfig{}
	search := NewGraphSearch(mockStore, config)

	weight := search.calculatePathWeight(2)
	assert.Greater(t, weight, 0.0)
	assert.Less(t, weight, 1.0) // Should decay with distance
}
