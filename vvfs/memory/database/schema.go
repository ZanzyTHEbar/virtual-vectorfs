package database

import "fmt"

// dynamicSchema returns schema DDL using the configured embedding dimension
func dynamicSchema(embeddingDims int) []string {
	if embeddingDims <= 0 {
		embeddingDims = 4
	}
	return []string{
		// entities
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS entities (
			name TEXT PRIMARY KEY,
			entity_type TEXT NOT NULL,
			embedding F32_BLOB(%d),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`, embeddingDims),
		// observations
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_name TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (entity_name) REFERENCES entities(name)
		)`,
		// relations
		`CREATE TABLE IF NOT EXISTS relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			target TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source) REFERENCES entities(name),
			FOREIGN KEY (target) REFERENCES entities(name)
		)`,
		// indexes
		`CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name)`,
		`CREATE INDEX IF NOT EXISTS idx_entities_created_at ON entities(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_observations_entity ON observations(entity_name)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_source ON relations(source)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_target ON relations(target)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_src_tgt_type ON relations(source, target, relation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_type_source ON relations(relation_type, source)`,
		// vector index
		`CREATE INDEX IF NOT EXISTS idx_entities_embedding ON entities(libsql_vector_idx(embedding))`,
		// soft cascade trigger
		`CREATE TRIGGER IF NOT EXISTS trg_entities_delete_cascade AFTER DELETE ON entities BEGIN
		    DELETE FROM observations WHERE entity_name = old.name;
		    DELETE FROM relations WHERE source = old.name OR target = old.name;
		END;`,
	}
}


