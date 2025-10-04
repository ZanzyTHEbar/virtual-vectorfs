package service

import (
	"context"
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/stretchr/testify/assert"
)

// TestFusionRankerImpl_fuseRRF tests RRF fusion
func TestFusionRankerImpl_fuseRRF(t *testing.T) {
	config := &config.MemoryConfig{}
	fusion := NewFusionRanker(config)

	ensembleResults := []EnsembleResult{
		{
			Source:  "bm25",
			Results: []SearchResult{{ID: "doc1", Score: 0.8}, {ID: "doc2", Score: 0.6}},
		},
		{
			Source:  "vector",
			Results: []SearchResult{{ID: "doc1", Score: 0.9}, {ID: "doc3", Score: 0.7}},
		},
	}

	results, err := fusion.Fuse(context.Background(), ensembleResults, FusionRRF)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// doc1 should have highest score (appears in both)
	assert.Equal(t, "doc1", results[0].ID)
}

// TestFusionRankerImpl_fuseWeightedRRF tests weighted RRF fusion
func TestFusionRankerImpl_fuseWeightedRRF(t *testing.T) {
	config := &config.MemoryConfig{
		WeightsBM25:   0.3,
		WeightsVector: 0.7,
	}
	fusion := NewFusionRanker(config)

	ensembleResults := []EnsembleResult{
		{
			Source:  "bm25",
			Results: []SearchResult{{ID: "doc1", Score: 0.8}},
		},
		{
			Source:  "vector",
			Results: []SearchResult{{ID: "doc1", Score: 0.9}},
		},
	}

	results, err := fusion.Fuse(context.Background(), ensembleResults, FusionWeightedRRF)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "doc1", results[0].ID)

	// Score should be weighted combination
	expectedScore := 0.3*(1.0/61.0) + 0.7*(1.0/61.0) // RRF scores
	assert.InDelta(t, expectedScore, results[0].Score, 0.001)
}

// TestFusionRankerImpl_fuseRelativeScore tests relative score fusion
func TestFusionRankerImpl_fuseRelativeScore(t *testing.T) {
	config := &config.MemoryConfig{}
	fusion := NewFusionRanker(config)

	ensembleResults := []EnsembleResult{
		{
			Source:  "bm25",
			Results: []SearchResult{{ID: "doc1", Score: 0.8}, {ID: "doc2", Score: 0.6}},
		},
		{
			Source:  "vector",
			Results: []SearchResult{{ID: "doc1", Score: 0.9}, {ID: "doc3", Score: 0.7}},
		},
	}

	results, err := fusion.Fuse(context.Background(), ensembleResults, FusionRelativeScore)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// Scores should be normalized and combined
	assert.Greater(t, results[0].Score, results[1].Score)
}
