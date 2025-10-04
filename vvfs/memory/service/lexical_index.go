package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// LexicalIndexImpl implements the LexicalIndex interface using SQLite FTS5
type LexicalIndexImpl struct {
	db     *sql.DB
	config *config.MemoryConfig
}

// NewLexicalIndexImpl creates a new FTS5-based lexical index
func NewLexicalIndexImpl(db *sql.DB, cfg *config.MemoryConfig) *LexicalIndexImpl {
	return &LexicalIndexImpl{
		db:     db,
		config: cfg,
	}
}

// Query performs BM25 search using FTS5
func (l *LexicalIndexImpl) Query(ctx context.Context, query string, k int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Escape FTS5 query syntax
	ftsQuery := escapeFTS5Query(query)

	// Build the SQL query
	sqlQuery := `
		SELECT 
			mi.id,
			mi.type,
			mi.text,
			mi.metadata_json,
			mi.created_at,
			bm25(mif) as bm25_score
		FROM memory_items_fts mif
		JOIN memory_items mi ON mif.rowid = mi.rowid
		WHERE mif MATCH ?
		ORDER BY bm25_score DESC
		LIMIT ?
	`

	args := []interface{}{ftsQuery, k}

	rows, err := l.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("FTS5 search query failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var itemType string
		var text string
		var metadataJSON sql.NullString
		var createdAt sql.NullTime
		var bm25Score float64

		err := rows.Scan(
			&r.ID,
			&itemType,
			&text,
			&metadataJSON,
			&createdAt,
			&bm25Score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan FTS5 result: %w", err)
		}

		r.Score = bm25Score
		r.Provenance = "bm25_fts5"

		// Store metadata as JSON
		if metadataJSON.Valid {
			r.Metadata = map[string]interface{}{
				"type":       itemType,
				"text":       text,
				"created_at": createdAt.Time.String(),
			}
		}

		results = append(results, r)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating FTS5 results: %w", err)
	}

	return results, nil
}

// Close cleans up resources
func (l *LexicalIndexImpl) Close() error {
	// No specific cleanup needed for FTS5
	return nil
}

// escapeFTS5Query escapes special FTS5 query characters
// FTS5 special characters: " ( ) : * - AND OR NOT NEAR
func escapeFTS5Query(query string) string {
	// Remove or escape special characters
	query = strings.ReplaceAll(query, "\"", "\"\"")
	query = strings.TrimSpace(query)

	// Wrap in quotes for exact phrase search if it contains spaces
	if strings.Contains(query, " ") {
		return fmt.Sprintf("\"%s\"", query)
	}

	return query
}

// GetStatistics returns statistics about the FTS5 index
func (l *LexicalIndexImpl) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total indexed items
	var count int64
	err := l.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_items_fts").Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count FTS5 items: %w", err)
	}
	stats["total_items"] = count

	// Get FTS5 index size (approximate)
	var size int64
	err = l.db.QueryRowContext(ctx, `
		SELECT SUM(pgsize) 
		FROM dbstat 
		WHERE name LIKE 'memory_items_fts%'
	`).Scan(&size)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get FTS5 index size: %w", err)
	}
	stats["index_size_bytes"] = size

	return stats, nil
}
