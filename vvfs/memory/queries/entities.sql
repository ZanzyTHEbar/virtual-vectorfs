-- SQLC queries for graph_entities table
-- These queries will be generated into Go code for type-safe database operations

-- name: GetGraphEntity :one
SELECT id, kind, name, summary, attrs_json, created_at, updated_at
FROM graph_entities
WHERE id = ?;

-- name: ListGraphEntities :many
SELECT id, kind, name, summary, attrs_json, created_at, updated_at
FROM graph_entities
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CreateGraphEntity :one
INSERT INTO graph_entities (id, kind, name, summary, attrs_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO UPDATE SET
    kind = EXCLUDED.kind,
    name = EXCLUDED.name,
    summary = EXCLUDED.summary,
    attrs_json = EXCLUDED.attrs_json,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, kind, name, summary, attrs_json, created_at, updated_at;

-- name: UpdateGraphEntity :one
UPDATE graph_entities
SET kind = ?, name = ?, summary = ?, attrs_json = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, kind, name, summary, attrs_json, created_at, updated_at;

-- name: DeleteGraphEntity :exec
DELETE FROM graph_entities WHERE id = ?;

-- name: GetGraphEntitiesByKind :many
SELECT id, kind, name, summary, attrs_json, created_at, updated_at
FROM graph_entities
WHERE kind = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: SearchGraphEntitiesFTS :many
SELECT e.id, e.kind, e.name, e.summary, e.attrs_json, e.created_at, e.updated_at,
       highlight(graph_entities_fts, 0, '<mark>', '</mark>') as highlighted_name,
       highlight(graph_entities_fts, 1, '<mark>', '</mark>') as highlighted_summary,
       bm25(graph_entities_fts) as bm25_score
FROM graph_entities_fts
JOIN graph_entities e ON graph_entities_fts.rowid = e.rowid
WHERE graph_entities_fts MATCH ?
ORDER BY bm25(graph_entities_fts) DESC
LIMIT ? OFFSET ?;

-- name: CountGraphEntities :one
SELECT COUNT(*) FROM graph_entities;

-- name: CountGraphEntitiesByKind :one
SELECT COUNT(*) FROM graph_entities WHERE kind = ?;

-- name: GetGraphEntityAttrs :one
SELECT attrs_json FROM graph_entities WHERE id = ?;

-- name: UpdateGraphEntityAttrs :exec
UPDATE graph_entities
SET attrs_json = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetGraphEntitiesWithSummary :many
SELECT id, kind, name, summary, attrs_json, created_at, updated_at
FROM graph_entities
WHERE summary IS NOT NULL AND summary != ''
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;