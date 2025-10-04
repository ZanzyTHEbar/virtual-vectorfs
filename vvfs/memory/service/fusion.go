package service

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// FusionRankerImpl implements FusionRanker for combining results from multiple sources
type FusionRankerImpl struct {
	config *config.MemoryConfig
}

// NewFusionRanker creates a new fusion ranker
func NewFusionRanker(config *config.MemoryConfig) *FusionRankerImpl {
	return &FusionRankerImpl{config: config}
}

// Fuse combines results from multiple sources using the specified strategy
func (fr *FusionRankerImpl) Fuse(ctx context.Context, results []EnsembleResult, strategy FusionStrategy) ([]SearchResult, error) {
	switch strategy {
	case FusionRRF:
		return fr.fuseRRF(results)
	case FusionWeightedRRF:
		return fr.fuseWeightedRRF(results)
	case FusionRelativeScore:
		return fr.fuseRelativeScore(results)
	case FusionLTR:
		return fr.fuseLTR(results)
	default:
		return nil, fmt.Errorf("unsupported fusion strategy: %s", strategy)
	}
}

// fuseRRF implements Reciprocal Rank Fusion
func (fr *FusionRankerImpl) fuseRRF(results []EnsembleResult) ([]SearchResult, error) {
	// Collect all unique results with their ranks
	rankMap := make(map[string]float64)

	for _, ensembleResult := range results {
		for rank, result := range ensembleResult.Results {
			// RRF score: 1 / (rank + k) where k is a constant (usually 60)
			rrfScore := 1.0 / (float64(rank) + 60.0)
			if existingScore, exists := rankMap[result.ID]; exists {
				rankMap[result.ID] = existingScore + rrfScore
			} else {
				rankMap[result.ID] = rrfScore
			}
		}
	}

	// Convert to SearchResult slice and sort by fused score
	var fusedResults []SearchResult
	for id, score := range rankMap {
		fusedResults = append(fusedResults, SearchResult{
			ID:    id,
			Score: score,
			// Provenance could be combined from multiple sources
		})
	}

	sort.Slice(fusedResults, func(i, j int) bool {
		return fusedResults[i].Score > fusedResults[j].Score
	})

	return fusedResults, nil
}

// fuseWeightedRRF implements Weighted Reciprocal Rank Fusion
func (fr *FusionRankerImpl) fuseWeightedRRF(results []EnsembleResult) ([]SearchResult, error) {
	// Get weights from config or routing decision
	weights := map[string]float64{
		"bm25":   fr.config.WeightsBM25,
		"vector": fr.config.WeightsVector,
		"graph":  fr.config.WeightsGraph,
	}

	// Collect weighted RRF scores
	rankMap := make(map[string]float64)

	for _, ensembleResult := range results {
		weight := weights[ensembleResult.Source]
		if weight == 0 {
			weight = 1.0 // Default weight if not specified
		}

		for rank, result := range ensembleResult.Results {
			// Weighted RRF score
			rrfScore := weight / (float64(rank) + 60.0)
			if existingScore, exists := rankMap[result.ID]; exists {
				rankMap[result.ID] = existingScore + rrfScore
			} else {
				rankMap[result.ID] = rrfScore
			}
		}
	}

	// Convert to SearchResult slice and sort
	var fusedResults []SearchResult
	for id, score := range rankMap {
		fusedResults = append(fusedResults, SearchResult{
			ID:    id,
			Score: score,
		})
	}

	sort.Slice(fusedResults, func(i, j int) bool {
		return fusedResults[i].Score > fusedResults[j].Score
	})

	return fusedResults, nil
}

// fuseRelativeScore implements Relative Score Fusion
func (fr *FusionRankerImpl) fuseRelativeScore(results []EnsembleResult) ([]SearchResult, error) {
	// Normalize scores per source and combine
	rankMap := make(map[string]float64)

	for _, ensembleResult := range results {
		// Normalize scores within this source (min-max normalization)
		scores := make([]float64, len(ensembleResult.Results))
		for i, result := range ensembleResult.Results {
			scores[i] = result.Score
		}

		minScore := math.Inf(1)
		maxScore := math.Inf(-1)
		for _, score := range scores {
			if score < minScore {
				minScore = score
			}
			if score > maxScore {
				maxScore = score
			}
		}

		// Avoid division by zero
		if maxScore == minScore {
			maxScore = minScore + 1
		}

		for i, result := range ensembleResult.Results {
			normalizedScore := (scores[i] - minScore) / (maxScore - minScore)
			if existingScore, exists := rankMap[result.ID]; exists {
				rankMap[result.ID] = existingScore + normalizedScore
			} else {
				rankMap[result.ID] = normalizedScore
			}
		}
	}

	// Convert to SearchResult slice and sort
	var fusedResults []SearchResult
	for id, score := range rankMap {
		fusedResults = append(fusedResults, SearchResult{
			ID:    id,
			Score: score,
		})
	}

	sort.Slice(fusedResults, func(i, j int) bool {
		return fusedResults[i].Score > fusedResults[j].Score
	})

	return fusedResults, nil
}

// fuseLTR implements Learning to Rank (placeholder for ML-based fusion)
func (fr *FusionRankerImpl) fuseLTR(results []EnsembleResult) ([]SearchResult, error) {
	// Placeholder: for now, fall back to weighted RRF
	// In a full implementation, this would use a trained model to predict relevance
	return fr.fuseWeightedRRF(results)
}
