package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// SearchNodes routes query by type: string -> text search; []float32 -> vector search
func (dm *DBManager) SearchNodes(ctx context.Context, projectName string, query interface{}, limit, offset int) ([]apptype.Entity, []apptype.Relation, error) {
	switch q := query.(type) {
	case string:
		return dm.SearchEntities(ctx, projectName, q, limit, offset)
	case []float32:
		res, err := dm.SearchSimilar(ctx, projectName, q, limit, offset)
		if err != nil {
			return nil, nil, err
		}
		ents := make([]apptype.Entity, len(res))
		for i, r := range res {
			ents[i] = r.Entity
		}
		rels, err := dm.GetRelationsForEntities(ctx, projectName, ents)
		if err != nil {
			return nil, nil, err
		}
		return ents, rels, nil
	default:
		return nil, nil, fmt.Errorf("unsupported query type %T", query)
	}
}

// SearchSimilar returns entities ranked by vector similarity to the provided embedding
func (dm *DBManager) SearchSimilar(ctx context.Context, projectName string, embedding []float32, limit, offset int) ([]apptype.SearchResult, error) {
	db, err := dm.getDB(projectName)
	if err != nil {
		return nil, err
	}
	if len(embedding) == 0 {
		return nil, fmt.Errorf("search embedding cannot be empty")
	}
	vecStr, err := dm.vectorToString(embedding)
	if err != nil {
		return nil, err
	}
	dm.capMu.RLock()
	caps := dm.capsByProject[projectName]
	dm.capMu.RUnlock()
	var rows *sql.Rows
	if caps.vectorTopK {
		k := limit + offset
		if k <= 0 {
			k = limit
		}
		topK := `WITH vt AS (
			SELECT id FROM vector_top_k('idx_entities_embedding', vector32(?), ?)
		)
		SELECT e.name, e.entity_type, e.embedding,
			(1 - (dot_product(cast(e.embedding as vector32), vector32(?)) / (vector_norm(cast(e.embedding as vector32)) * vector_norm(vector32(?))))) AS distance
		FROM entities e
		JOIN vt ON vt.id = e.rowid
		LIMIT ?`
		rows, err = db.QueryContext(ctx, topK, vecStr, k, vecStr, vecStr, k)
	} else {
		// fallback: compute cosine distances client-side (for small datasets)
		stmt := `SELECT name, entity_type, embedding FROM entities`
		rows, err = db.QueryContext(ctx, stmt)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []apptype.SearchResult
	if caps.vectorTopK {
		for rows.Next() {
			var name, et string
			var emb []byte
			var dist float64
			if err := rows.Scan(&name, &et, &emb, &dist); err != nil {
				return nil, err
			}
			vec, _ := dm.ExtractVector(ctx, emb)
			results = append(results, apptype.SearchResult{Entity: apptype.Entity{Name: name, EntityType: et, Embedding: vec}, Distance: dist})
		}
		// pagination handled by query
		return results, nil
	}
	// client-side cosine
	qnorm := 0.0
	for _, v := range embedding {
		qnorm += float64(v) * float64(v)
	}
	qnorm = math.Sqrt(qnorm)
	for rows.Next() {
		var name, et string
		var emb []byte
		if err := rows.Scan(&name, &et, &emb); err != nil {
			return nil, err
		}
		vec, _ := dm.ExtractVector(ctx, emb)
		dot := 0.0
		vnorm := 0.0
		for i := 0; i < len(vec) && i < len(embedding); i++ {
			dot += float64(vec[i]) * float64(embedding[i])
			vnorm += float64(vec[i]) * float64(vec[i])
		}
		if qnorm == 0 || vnorm == 0 {
			continue
		}
		sim := dot / (qnorm * math.Sqrt(vnorm))
		dist := 1.0 - sim
		results = append(results, apptype.SearchResult{Entity: apptype.Entity{Name: name, EntityType: et, Embedding: vec}, Distance: dist})
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Distance < results[j].Distance })
	// paginate
	start := offset
	end := start + limit
	if start > len(results) {
		start = len(results)
	}
	if end > len(results) {
		end = len(results)
	}
	return results[start:end], nil
}

// ensureFTSSchema creates FTS5 virtual table and triggers if supported (best-effort)
func (dm *DBManager) ensureFTSSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`DROP TRIGGER IF EXISTS trg_obs_ai`,
		`DROP TRIGGER IF EXISTS trg_obs_ad`,
		`DROP TRIGGER IF EXISTS trg_obs_au`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_observations USING fts5(
            entity_name,
            content,
            tokenize = 'unicode61 tokenchars=:-_@./',
            prefix = '2 3 4 5 6 7'
        )`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_ai AFTER INSERT ON observations BEGIN
            INSERT INTO fts_observations(rowid, entity_name, content) VALUES (new.id, new.entity_name, new.content);
        END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_ad AFTER DELETE ON observations BEGIN
            INSERT INTO fts_observations(fts_observations, rowid, entity_name, content) VALUES ('delete', old.id, old.entity_name, old.content);
        END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_au AFTER UPDATE ON observations BEGIN
            INSERT INTO fts_observations(fts_observations, rowid, entity_name, content) VALUES ('delete', old.id, old.entity_name, old.content);
            INSERT INTO fts_observations(rowid, entity_name, content) VALUES (new.id, new.entity_name, new.content);
        END;`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return nil
		}
	}
	return nil
}
