package service

import (
	"context"
	"fmt"
	"math"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// GraphSearchImpl implements GraphSearch for graph-based retrieval and reranking
type GraphSearchImpl struct {
	store  GraphStore
	config *config.MemoryConfig
}

// NewGraphSearch creates a new graph search implementation
func NewGraphSearch(store GraphStore, config *config.MemoryConfig) *GraphSearchImpl {
	return &GraphSearchImpl{
		store:  store,
		config: config,
	}
}

// SearchFromCenter performs search centered around a specific entity
func (gs *GraphSearchImpl) SearchFromCenter(ctx context.Context, centerID string, query string, depth int, k int) ([]GraphSearchResult, error) {
	if depth <= 0 {
		depth = gs.config.GraphDepth
	}
	if k <= 0 {
		k = gs.config.K
	}

	// Get the center entity
	_, err := gs.store.GetEntity(ctx, centerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get center entity: %w", err)
	}

	// Start BFS from center entity
	visited := make(map[string]bool)
	queue := []string{centerID}
	distances := map[string]int{centerID: 0}
	results := []GraphSearchResult{}

	for len(queue) > 0 && len(results) < k {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		currentDist := distances[currentID]

		// Get outgoing edges from current entity
		edges, err := gs.store.ListEdges(ctx, ListOptions{
			Filter: map[string]interface{}{"src_id": currentID},
			Limit:  100, // Reasonable limit per level
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get edges from %s: %w", currentID, err)
		}

		for _, edge := range edges {
			if edge.ValidTo != nil || edge.InvalidatedAt != nil {
				continue // Skip invalidated edges
			}

			targetID := edge.TargetID
			if !visited[targetID] {
				newDist := currentDist + 1
				if newDist <= depth {
					distances[targetID] = newDist
					queue = append(queue, targetID)

					// Create result
					result := GraphSearchResult{
						EntityID:   targetID,
						Score:      gs.calculateGraphScore(edge, newDist),
						PathLength: newDist,
						Relation:   edge.Relation,
					}
					results = append(results, result)
				}
			}
		}
	}

	// Sort results by score (descending)
	gs.sortGraphResults(results)

	// Return top k
	if len(results) > k {
		results = results[:k]
	}

	return results, nil
}

// SearchWithPathBoost performs search with path-based boosting
func (gs *GraphSearchImpl) SearchWithPathBoost(ctx context.Context, query string, opts GraphSearchOptions) ([]GraphSearchResult, error) {
	// Use center entity for graph traversal
	centerID := opts.CenterID
	if centerID == "" {
		// Fallback: find a relevant center entity based on query
		centerID = gs.findRelevantCenter(ctx, query)
	}

	depth := opts.Depth
	if depth <= 0 {
		depth = gs.config.GraphDepth
	}

	k := opts.K
	if k <= 0 {
		k = gs.config.K
	}

	// Perform BFS from center
	results, err := gs.SearchFromCenter(ctx, centerID, query, depth, k*2) // Overfetch for reranking
	if err != nil {
		return nil, err
	}

	// Apply path weighting if enabled
	if opts.PathWeights {
		for i := range results {
			pathWeight := gs.calculatePathWeight(results[i].PathLength)
			results[i].Score *= pathWeight
		}
	}

	// Sort and truncate
	gs.sortGraphResults(results)
	if len(results) > k {
		results = results[:k]
	}

	return results, nil
}

// calculateGraphScore computes a score for an edge based on distance and relation strength
func (gs *GraphSearchImpl) calculateGraphScore(edge *Edge, distance int) float64 {
	// Base score from relation (could be learned or predefined)
	baseScore := 1.0

	// Distance penalty (closer is better)
	distancePenalty := 1.0 / (1.0 + float64(distance))

	// Relation-specific boost (example: some relations are stronger)
	if relationBoost, ok := gs.getRelationBoost(edge.Relation); ok {
		baseScore *= relationBoost
	}

	return baseScore * distancePenalty
}

// calculatePathWeight computes weight for path length
func (gs *GraphSearchImpl) calculatePathWeight(pathLength int) float64 {
	// Exponential decay with path length
	return math.Exp(-float64(pathLength) * 0.1) // Configurable decay factor
}

// findRelevantCenter finds a relevant center entity for the query (placeholder implementation)
func (gs *GraphSearchImpl) findRelevantCenter(ctx context.Context, query string) string {
	// Placeholder: return a default center or implement entity search
	// For now, return empty string to indicate no center
	return ""
}

// getRelationBoost returns boost factor for specific relations (can be extended)
func (gs *GraphSearchImpl) getRelationBoost(relation string) (float64, bool) {
	boosts := map[string]float64{
		"works_on":   1.5,
		"mentions":   1.2,
		"related_to": 1.0,
		// Add more relations as needed
	}

	boost, ok := boosts[relation]
	return boost, ok
}

// sortGraphResults sorts results by score descending
func (gs *GraphSearchImpl) sortGraphResults(results []GraphSearchResult) {
	// Simple bubble sort for small lists (can be optimized)
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}
