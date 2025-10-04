-- Migration: Graph entities and edges for bi-temporal knowledge graph
-- Adds support for graph_entities (nodes) and graph_edges (relationships) with temporal validity
-- FTS5 for entity search, indexes for performance, views for temporal queries
-- Note: Uses "graph_" prefix to avoid conflict with existing entities table

-- Graph entities table: Represents people, projects, concepts, etc. in knowledge graph
CREATE TABLE graph_entities (
    id TEXT PRIMARY KEY, -- UUID
    kind TEXT NOT NULL, -- e.g., 'person', 'project', 'concept'
    name TEXT NOT NULL, -- Human-readable name
    summary TEXT, -- Short description or summary
    attrs_json TEXT, -- JSON object for additional attributes
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- FTS5 virtual table for graph entity search (name and summary)
CREATE VIRTUAL TABLE graph_entities_fts USING fts5(
    id UNINDEXED,
    kind,
    name,
    summary,
    content='graph_entities',
    content_rowid='rowid'
);

-- Trigger to keep FTS5 in sync on insert/update
CREATE TRIGGER graph_entities_fts_insert AFTER INSERT ON graph_entities
BEGIN
    INSERT INTO graph_entities_fts (rowid, id, kind, name, summary)
    VALUES (new.rowid, new.id, new.kind, new.name, new.summary);
END;

CREATE TRIGGER graph_entities_fts_delete AFTER DELETE ON graph_entities
BEGIN
    DELETE FROM graph_entities_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER graph_entities_fts_update AFTER UPDATE ON graph_entities
BEGIN
    UPDATE graph_entities_fts SET
        kind = new.kind,
        name = new.name,
        summary = new.summary
    WHERE rowid = new.rowid;
END;

-- Graph edges table: Represents relationships between graph entities with bi-temporal validity
-- valid_from: When the fact became true (event time)
-- valid_to: When the fact ceased to be true (NULL if still valid)
-- ingested_at: When we learned this fact (knowledge time)
-- invalidated_at: When we invalidated this edge due to contradiction (NULL if valid)
CREATE TABLE graph_edges (
    id TEXT PRIMARY KEY, -- UUID
    src_id TEXT NOT NULL REFERENCES graph_entities(id) ON DELETE CASCADE,
    dst_id TEXT NOT NULL REFERENCES graph_entities(id) ON DELETE CASCADE,
    rel TEXT NOT NULL, -- Relationship type, e.g., 'works_on', 'mentions'
    attrs_json TEXT, -- JSON object for edge attributes (e.g., confidence, weight)
    valid_from DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Event time
    valid_to DATETIME, -- NULL if still valid
    ingested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Knowledge time
    invalidated_at DATETIME, -- NULL if not invalidated
    provenance_json TEXT, -- JSON object for source provenance (e.g., episode_id, extractor_version)
    CHECK (src_id != dst_id), -- No self-loops
    CHECK (valid_to IS NULL OR valid_to > valid_from),
    CHECK (invalidated_at IS NULL OR invalidated_at >= ingested_at)
);

-- Indexes for efficient queries
CREATE INDEX graph_edges_src_idx ON graph_edges(src_id);
CREATE INDEX graph_edges_dst_idx ON graph_edges(dst_id);
CREATE INDEX graph_edges_rel_idx ON graph_edges(rel);
CREATE INDEX graph_edges_valid_idx ON graph_edges(valid_from, valid_to) WHERE valid_to IS NULL; -- Current edges
CREATE INDEX graph_edges_ingested_idx ON graph_edges(ingested_at);
CREATE INDEX graph_edges_invalidated_idx ON graph_edges(invalidated_at) WHERE invalidated_at IS NOT NULL;

-- Views for temporal queries
-- Current graph edges (still valid, not invalidated)
CREATE VIEW graph_edges_current AS
SELECT * FROM graph_edges
WHERE valid_to IS NULL
  AND invalidated_at IS NULL;

-- Graph edges as of a specific time (point-in-time query)
-- Returns edges that were valid at the given time
CREATE VIEW graph_edges_asof(timepoint) AS
SELECT * FROM graph_edges
WHERE valid_from <= timepoint
  AND (valid_to IS NULL OR valid_to > timepoint)
  AND (invalidated_at IS NULL OR invalidated_at > timepoint);

-- Trigger to update updated_at on graph entity changes
CREATE TRIGGER graph_entities_updated_at AFTER UPDATE ON graph_entities
BEGIN
    UPDATE graph_entities SET updated_at = CURRENT_TIMESTAMP WHERE id = new.id;
END;

-- Trigger to prevent invalidating already invalidated graph edges (optional, for data integrity)
-- This is a no-op trigger for now, but can be extended for business logic
CREATE TRIGGER graph_edges_invalidation_check BEFORE UPDATE OF invalidated_at ON graph_edges
WHEN new.invalidated_at IS NOT NULL AND old.invalidated_at IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'Edge already invalidated');
END;
