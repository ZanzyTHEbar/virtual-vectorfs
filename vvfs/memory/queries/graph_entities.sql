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
       '' as highlighted_name,
       '' as highlighted_summary,
       0.0 as bm25_score
FROM graph_entities e
WHERE e.name LIKE '%' || ? || '%' OR e.summary LIKE '%' || ? || '%'
ORDER BY e.created_at DESC
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