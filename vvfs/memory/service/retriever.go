package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// RetrieverImpl implements Retriever for hybrid search with fusion and filters
type RetrieverImpl struct {
	config       *config.MemoryConfig
	lexicalIndex LexicalIndex
	vectorIndex  VectorIndex
	graphSearch  GraphSearch
	scorer       Scorer
	metrics      *MetricsCollector
}

// NewRetriever creates a new retriever
func NewRetriever(config *config.MemoryConfig, lexicalIndex LexicalIndex, vectorIndex VectorIndex, graphSearch GraphSearch, scorer Scorer, metrics *MetricsCollector) *RetrieverImpl {
	return &RetrieverImpl{
		config:       config,
		lexicalIndex: lexicalIndex,
		vectorIndex:  vectorIndex,
		graphSearch:  graphSearch,
		scorer:       scorer,
		metrics:      metrics,
	}
}

// Search performs hybrid retrieval with fusion and filtering
func (ret *RetrieverImpl) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	start := time.Now()

	// 1. Get candidate sets from lexical and vector indexes
	var lexicalResults, vectorResults []SearchResult
	var err error

	// Lexical search (BM25/FTS5)
	if ret.lexicalIndex != nil {
		lexicalResults, err = ret.lexicalIndex.Query(ctx, query, opts.K*2) // Overfetch for fusion
		if err != nil {
			return nil, fmt.Errorf("lexical search failed: %w", err)
		}
	}

	// Vector search (requires embedding - placeholder for now)
	if ret.vectorIndex != nil {
		// In a full implementation, embed the query first
		// For now, use placeholder
		vectorResults, err = ret.vectorIndex.Query(ctx, []float64{}, opts.K*2)
		if err != nil {
			return nil, fmt.Errorf("vector search failed: %w", err)
		}
	}

	// 2. Fuse scores using alpha
	fusedResults := ret.fuseResults(lexicalResults, vectorResults, opts.Alpha)

	// 3. Apply filters and boosters
	filteredResults := ret.applyFiltersAndBoosters(fusedResults, opts)

	// 4. Apply thresholds and autocut
	thresholdedResults := ret.scorer.ApplyThresholds(filteredResults, opts.Threshold)
	autocutResults := ret.scorer.ApplyAutocut(thresholdedResults)

	// 5. Optional reranking
	if opts.Rerank && ret.graphSearch != nil {
		autocutResults, err = ret.applyGraphReranking(ctx, query, autocutResults, opts)
		if err != nil {
			return nil, fmt.Errorf("reranking failed: %w", err)
		}
	}

	// 6. Truncate to final k
	finalResults := ret.truncateResults(autocutResults, opts.K)

	duration := time.Since(start)
	ret.metrics.RecordRetrieval("hybrid", duration, nil)

	return finalResults, nil
}

// fuseResults combines lexical and vector results using alpha fusion
func (ret *RetrieverImpl) fuseResults(lexicalResults, vectorResults []SearchResult, alpha float64) []SearchResult {
	// Normalize scores per source
	lexicalNorm := ret.normalizeScores(lexicalResults)
	vectorNorm := ret.normalizeScores(vectorResults)

	// Combine results with deduplication
	resultMap := make(map[string]*SearchResult)

	// Add lexical results
	for _, result := range lexicalNorm {
		if existing, exists := resultMap[result.ID]; exists {
			// Fuse scores: alpha * vector_score + (1-alpha) * lexical_score
			existing.Score = alpha*existing.Score + (1-alpha)*result.Score
			existing.Provenance += ",lexical"
		} else {
			result.Score = (1 - alpha) * result.Score
			result.Provenance = "lexical"
			resultMap[result.ID] = &result
		}
	}

	// Add vector results
	for _, result := range vectorNorm {
		if existing, exists := resultMap[result.ID]; exists {
			existing.Score = alpha*result.Score + (1-alpha)*existing.Score
			existing.Provenance += ",vector"
		} else {
			result.Score = alpha * result.Score
			result.Provenance = "vector"
			resultMap[result.ID] = &result
		}
	}

	// Convert back to slice
	var fusedResults []SearchResult
	for _, result := range resultMap {
		fusedResults = append(fusedResults, *result)
	}

	// Sort by fused score
	sort.Slice(fusedResults, func(i, j int) bool {
		return fusedResults[i].Score > fusedResults[j].Score
	})

	return fusedResults
}

// normalizeScores normalizes scores to [0,1] range per source
func (ret *RetrieverImpl) normalizeScores(results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// Find min/max scores
	minScore := math.Inf(1)
	maxScore := math.Inf(-1)
	for _, result := range results {
		if result.Score < minScore {
			minScore = result.Score
		}
		if result.Score > maxScore {
			maxScore = result.Score
		}
	}

	// Avoid division by zero
	if maxScore == minScore {
		maxScore = minScore + 1
	}

	// Normalize
	normalized := make([]SearchResult, len(results))
	for i, result := range results {
		normalizedScore := (result.Score - minScore) / (maxScore - minScore)
		normalized[i] = SearchResult{
			ID:         result.ID,
			Score:      normalizedScore,
			Metadata:   result.Metadata,
			Provenance: result.Provenance,
		}
	}

	return normalized
}

// applyFiltersAndBoosters applies metadata filters, time decay, and spatial boosters
func (ret *RetrieverImpl) applyFiltersAndBoosters(results []SearchResult, opts SearchOptions) []SearchResult {
	// Apply metadata filters
	filtered := ret.applyMetadataFilters(results, opts.MetadataFilters)

	// Apply time decay if enabled
	if opts.TimeDecay {
		filtered = ret.scorer.ApplyTimeDecay(filtered, opts.Lambda)
	}

	// Apply spatial boost if center provided
	if len(opts.SpatialCenter) > 0 && opts.SpatialRadius > 0 {
		filtered = ret.scorer.ApplySpatialBoost(filtered, opts.SpatialCenter, opts.SpatialRadius)
	}

	return filtered
}

// applyMetadataFilters filters results based on metadata criteria
func (ret *RetrieverImpl) applyMetadataFilters(results []SearchResult, filters map[string]interface{}) []SearchResult {
	var filtered []SearchResult

	for _, result := range results {
		include := true

		// Check each filter criterion
		for key, expectedValue := range filters {
			actualValue, exists := result.Metadata[key]
			if !exists {
				include = false
				break
			}

			// Simple equality check (can be extended for ranges, etc.)
			if actualValue != expectedValue {
				include = false
				break
			}
		}

		if include {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// applyGraphReranking applies graph-based reranking
func (ret *RetrieverImpl) applyGraphReranking(ctx context.Context, query string, results []SearchResult, opts SearchOptions) ([]SearchResult, error) {
	if ret.graphSearch == nil {
		return results, nil
	}

	// Find center entity for graph search
	centerID := ret.findCenterEntity(results, opts)

	if centerID == "" {
		return results, nil // No suitable center found
	}

	// Perform graph search from center
	graphResults, err := ret.graphSearch.SearchFromCenter(ctx, centerID, query, opts.GraphDepth, len(results))
	if err != nil {
		return results, err // Continue without graph reranking
	}

	// Boost results that appear in graph search
	resultMap := make(map[string]*SearchResult)
	for _, result := range results {
		resultMap[result.ID] = &result
	}

	for _, graphResult := range graphResults {
		if existing, exists := resultMap[graphResult.EntityID]; exists {
			// Boost score based on graph distance
			boost := 1.0 / (1.0 + float64(graphResult.PathLength))
			existing.Score *= (1.0 + boost)
		}
	}

	// Re-sort after boosting
	var boostedResults []SearchResult
	for _, result := range resultMap {
		boostedResults = append(boostedResults, *result)
	}
	sort.Slice(boostedResults, func(i, j int) bool {
		return boostedResults[i].Score > boostedResults[j].Score
	})

	return boostedResults, nil
}

// findCenterEntity selects a center entity for graph search
func (ret *RetrieverImpl) findCenterEntity(results []SearchResult, opts SearchOptions) string {
	// Simple heuristic: use the top result if it's an entity ID
	if len(results) > 0 {
		topResult := results[0]
		// Check if ID looks like an entity ID (could be UUID or specific pattern)
		if len(topResult.ID) > 10 { // Simple heuristic
			return topResult.ID
		}
	}
	return ""
}

// truncateResults limits results to k
func (ret *RetrieverImpl) truncateResults(results []SearchResult, k int) []SearchResult {
	if len(results) <= k {
		return results
	}
	return results[:k]
}
