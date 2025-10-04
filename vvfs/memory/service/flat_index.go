package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// FlatIndexImpl implements VectorIndex using brute-force search
// This is the simplest implementation and serves as a baseline
type FlatIndexImpl struct {
	db        *sql.DB
	dimension int
	mu        sync.RWMutex

	// In-memory cache for fast access (optional optimization)
	cache     map[string][]float64
	cacheSize int
}

// NewFlatIndexImpl creates a new flat vector index
func NewFlatIndexImpl(db *sql.DB, dimension int) *FlatIndexImpl {
	return &FlatIndexImpl{
		db:        db,
		dimension: dimension,
		cache:     make(map[string][]float64),
		cacheSize: 10000, // Cache up to 10k vectors
	}
}

// Upsert adds or updates a vector in the index
func (f *FlatIndexImpl) Upsert(ctx context.Context, id string, vector []float64) error {
	if len(vector) != f.dimension {
		return fmt.Errorf("vector dimension mismatch: expected %d, got %d", f.dimension, len(vector))
	}

	// Encode vector as BLOB
	vectorBlob, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to encode vector: %w", err)
	}

	query := `
		UPDATE memory_items
		SET embedding = ?
		WHERE id = ?
	`

	result, err := f.db.ExecContext(ctx, query, vectorBlob, id)
	if err != nil {
		return fmt.Errorf("failed to upsert vector: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("memory item not found: %s", id)
	}

	// Update cache
	f.mu.Lock()
	if len(f.cache) < f.cacheSize {
		f.cache[id] = vector
	}
	f.mu.Unlock()

	return nil
}

// Query performs k-NN search using brute force
func (f *FlatIndexImpl) Query(ctx context.Context, query []float64, k int) ([]SearchResult, error) {
	if len(query) != f.dimension {
		return nil, fmt.Errorf("query dimension mismatch: expected %d, got %d", f.dimension, len(query))
	}

	// Default metric is cosine
	metric := "cosine"

	// Fetch all vectors from database
	sqlQuery := `
		SELECT id, embedding
		FROM memory_items
		WHERE embedding IS NOT NULL
	`

	rows, err := f.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vectors: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id       string
		distance float64
	}

	var candidates []candidate

	for rows.Next() {
		var id string
		var embeddingBlob []byte

		if err := rows.Scan(&id, &embeddingBlob); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Decode vector
		var vector []float64
		if err := json.Unmarshal(embeddingBlob, &vector); err != nil {
			continue // Skip invalid vectors
		}

		if len(vector) != f.dimension {
			continue // Skip dimension mismatches
		}

		// Compute distance
		var distance float64
		switch metric {
		case "cosine":
			distance = 1.0 - cosineSimilarity(query, vector)
		case "l2":
			distance = euclideanDistance(query, vector)
		case "dot":
			distance = -dotProduct(query, vector) // Negate for sorting
		default:
			distance = 1.0 - cosineSimilarity(query, vector)
		}

		candidates = append(candidates, candidate{id: id, distance: distance})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Sort by distance (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Return top-k
	if k > len(candidates) {
		k = len(candidates)
	}

	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		// Convert distance to similarity score (higher is better)
		score := 1.0 / (1.0 + candidates[i].distance)
		results[i] = SearchResult{
			ID:         candidates[i].id,
			Score:      score,
			Provenance: "vector_flat",
		}
	}

	return results, nil
}

// Delete removes a vector from the index
func (f *FlatIndexImpl) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE memory_items
		SET embedding = NULL
		WHERE id = ?
	`

	_, err := f.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete vector: %w", err)
	}

	// Remove from cache
	f.mu.Lock()
	delete(f.cache, id)
	f.mu.Unlock()

	return nil
}

// Close cleans up resources
func (f *FlatIndexImpl) Close() error {
	f.mu.Lock()
	f.cache = nil
	f.mu.Unlock()
	return nil
}

// Distance metric helper functions

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProd, normA, normB float64
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProd / (math.Sqrt(normA) * math.Sqrt(normB))
}

func euclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}

	return sum
}
