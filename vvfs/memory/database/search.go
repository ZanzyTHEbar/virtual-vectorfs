package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// SearchEntities performs FTS-backed (or LIKE-fallback) search over observations + entity_name
//
// Relations are expected to be fetched by caller as needed; return empty to avoid heavy join
func (dm *DBManager) SearchEntities(ctx context.Context, projectName string, query string, limit, offset int) ([]apptype.Entity, []apptype.Relation, error) {
	db, err := dm.getDB(projectName)
	if err != nil {
		return nil, nil, err
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return []apptype.Entity{}, []apptype.Relation{}, nil
	}

	dm.capMu.RLock()
	caps := dm.capsByProject[projectName]
	dm.capMu.RUnlock()
	var rows *sql.Rows
	if caps.fts5 {
		// Use matchinfo(bm25) ranking if available; fall back to default order otherwise
		stmt := `WITH ranked AS (
			SELECT entity_name AS name, max(rank) AS r
			FROM (
				SELECT rowid, entity_name, bm25(fts_observations, 1.2, 0.75) AS rank FROM fts_observations WHERE fts_observations MATCH ?
				UNION ALL
				SELECT id AS rowid, entity_name, 1.0 AS rank FROM observations WHERE content LIKE '%' || ? || '%'
			)
			GROUP BY entity_name
		)
		SELECT e.name, e.entity_type, e.embedding FROM ranked r JOIN entities e ON e.name = r.name ORDER BY r.r LIMIT ? OFFSET ?`
		rows, err = db.QueryContext(ctx, stmt, q, q, limit, offset)
	} else {
		stmt := `SELECT DISTINCT e.name, e.entity_type, e.embedding
			FROM entities e LEFT JOIN observations o ON o.entity_name = e.name
			WHERE e.name LIKE '%' || ? || '%' OR o.content LIKE '%' || ? || '%'
			LIMIT ? OFFSET ?`
		rows, err = db.QueryContext(ctx, stmt, q, q, limit, offset)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()
	var ents []apptype.Entity
	for rows.Next() {
		var name, et string
		var emb []byte
		if err := rows.Scan(&name, &et, &emb); err != nil {
			return nil, nil, err
		}
		vec, _ := dm.ExtractVector(ctx, emb)
		ents = append(ents, apptype.Entity{Name: name, EntityType: et, Embedding: vec})
	}
	// relations can be fetched by caller as needed; return empty to avoid heavy join
	return ents, []apptype.Relation{}, nil
}
