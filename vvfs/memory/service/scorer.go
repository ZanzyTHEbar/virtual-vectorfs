package service

import (
	"math"
	"sort"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// ScorerImpl implements Scorer for fusion, thresholding, and boosters
type ScorerImpl struct {
	config *config.MemoryConfig
}

// NewScorer creates a new scorer
func NewScorer(config *config.MemoryConfig) *ScorerImpl {
	return &ScorerImpl{config: config}
}

// FuseScores combines results using alpha fusion (already implemented in retriever)
// This method provides the interface for the scorer
func (sc *ScorerImpl) FuseScores(results []SearchResult, alpha float64) []SearchResult {
	// This is implemented in the retriever for hybrid search
	// For standalone use, implement alpha fusion here
	return results
}

// ApplyThresholds filters results below threshold
func (sc *ScorerImpl) ApplyThresholds(results []SearchResult, threshold float64) []SearchResult {
	var filtered []SearchResult
	for _, result := range results {
		if result.Score >= threshold {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// ApplyAutocut applies knee detection to cut off low-relevance results
func (sc *ScorerImpl) ApplyAutocut(results []SearchResult) []SearchResult {
	if len(results) < 3 {
		return results // Need at least 3 points for knee detection
	}

	// Calculate score differences to find knee point
	diffs := make([]float64, len(results)-1)
	for i := 0; i < len(results)-1; i++ {
		diffs[i] = results[i].Score - results[i+1].Score
	}

	// Find the largest difference (knee)
	maxDiff := 0.0
	kneeIndex := len(results) - 1

	for i, diff := range diffs {
		if diff > maxDiff {
			maxDiff = diff
			kneeIndex = i + 1 // Cut after this index
		}
	}

	// Cut at knee point, but ensure we keep at least some results
	minResults := 3
	if kneeIndex < minResults {
		kneeIndex = minResults
	}

	return results[:kneeIndex]
}

// ApplyTimeDecay applies exponential decay based on recency
func (sc *ScorerImpl) ApplyTimeDecay(results []SearchResult, lambda float64) []SearchResult {
	now := time.Now()

	for i := range results {
		// Extract creation time from metadata (assuming it's stored there)
		var createdAt time.Time
		if createdAtVal, ok := results[i].Metadata["created_at"]; ok {
			if t, ok := createdAtVal.(time.Time); ok {
				createdAt = t
			}
		}

		// Calculate age in days
		age := now.Sub(createdAt).Hours() / 24

		// Apply exponential decay: score *= exp(-lambda * age)
		decayFactor := math.Exp(-lambda * age)
		results[i].Score *= decayFactor

		// Update metadata to include decay info
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}
		results[i].Metadata["decay_factor"] = decayFactor
		results[i].Metadata["age_days"] = age
	}

	return results
}

// ApplySpatialBoost applies spatial proximity boost
func (sc *ScorerImpl) ApplySpatialBoost(results []SearchResult, center []float64, radius float64) []SearchResult {
	// Simple implementation: boost based on Euclidean distance to center
	// In a full implementation, this would use RTree or spatial indexing

	for i := range results {
		// Extract spatial coordinates from metadata
		var coords []float64
		if coordsVal, ok := results[i].Metadata["coordinates"]; ok {
			if c, ok := coordsVal.([]float64); ok {
				coords = c
			}
		}

		if len(coords) >= 2 && len(center) >= 2 {
			// Calculate Euclidean distance
			distance := math.Sqrt(
				math.Pow(coords[0]-center[0], 2) +
					math.Pow(coords[1]-center[1], 2),
			)

			// Boost factor: closer = higher boost
			if distance <= radius {
				boostFactor := 1.0 + (radius-distance)/radius
				results[i].Score *= boostFactor

				// Update metadata
				if results[i].Metadata == nil {
					results[i].Metadata = make(map[string]interface{})
				}
				results[i].Metadata["spatial_boost"] = boostFactor
				results[i].Metadata["distance_to_center"] = distance
			}
		}
	}

	return results
}

// CalculateScoreDistribution returns statistics about score distribution
func (sc *ScorerImpl) CalculateScoreDistribution(results []SearchResult) map[string]float64 {
	if len(results) == 0 {
		return map[string]float64{}
	}

	scores := make([]float64, len(results))
	for i, result := range results {
		scores[i] = result.Score
	}

	// Simple statistics (can be extended)
	min := math.Inf(1)
	max := math.Inf(-1)
	sum := 0.0

	for _, score := range scores {
		if score < min {
			min = score
		}
		if score > max {
			max = score
		}
		sum += score
	}

	mean := sum / float64(len(scores))

	// Calculate variance
	variance := 0.0
	for _, score := range scores {
		variance += math.Pow(score-mean, 2)
	}
	variance /= float64(len(scores))
	stddev := math.Sqrt(variance)

	return map[string]float64{
		"min":    min,
		"max":    max,
		"mean":   mean,
		"stddev": stddev,
		"count":  float64(len(scores)),
	}
}

// SortByScore sorts results by score descending
func (sc *ScorerImpl) SortByScore(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}
