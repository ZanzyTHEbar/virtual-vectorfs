-- SQLC queries for graph_edges table
-- These queries will be generated into Go code for type-safe database operations

-- name: GetGraphGraphEdge :one
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE id = ?;

-- name: ListGraphGraphGraphEdges :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: CreateGraphGraphEdge :one
INSERT INTO graph_edges (id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    rel = EXCLUDED.rel,
    attrs_json = EXCLUDED.attrs_json,
    valid_to = EXCLUDED.valid_to,
    invalidated_at = EXCLUDED.invalidated_at,
    provenance_json = EXCLUDED.provenance_json
RETURNING id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json;

-- name: UpdateGraphEdge :one
UPDATE graph_edges
SET rel = ?, attrs_json = ?, valid_to = ?, invalidated_at = ?, provenance_json = ?
WHERE id = ?
RETURNING id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json;

-- name: DeleteGraphEdge :exec
DELETE FROM graph_edges WHERE id = ?;

-- name: InvalidateGraphEdge :exec
UPDATE graph_edges
SET invalidated_at = CURRENT_TIMESTAMP, valid_to = CURRENT_TIMESTAMP
WHERE id = ? AND invalidated_at IS NULL;

-- name: GetGraphGraphEdgesBySource :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE src_id = ?
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetGraphGraphEdgesByTarget :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE dst_id = ?
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetGraphGraphEdgesByRelation :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE rel = ?
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetGraphGraphEdgesBetweenEntities :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE (src_id = ? AND dst_id = ?) OR (src_id = ? AND dst_id = ?)
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetCurrentGraphGraphEdges :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges_current
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetGraphGraphEdgesAsOf :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges_asof(?)
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetInvalidatedGraphGraphEdges :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE invalidated_at IS NOT NULL
ORDER BY invalidated_at DESC
LIMIT ? OFFSET ?;

-- name: CountGraphGraphEdges :one
SELECT COUNT(*) FROM graph_edges;

-- name: CountGraphGraphEdgesByRelation :one
SELECT COUNT(*) FROM graph_edges WHERE rel = ?;

-- name: CountGraphGraphEdgesBySource :one
SELECT COUNT(*) FROM graph_edges WHERE src_id = ?;

-- name: CountGraphGraphEdgesByTarget :one
SELECT COUNT(*) FROM graph_edges WHERE dst_id = ?;

-- name: GetGraphEdgeAttrs :one
SELECT attrs_json FROM graph_edges WHERE id = ?;

-- name: UpdateGraphEdgeAttrs :exec
UPDATE graph_edges
SET attrs_json = ?
WHERE id = ?;

-- name: GetGraphEdgeProvenance :one
SELECT provenance_json FROM graph_edges WHERE id = ?;

-- name: UpdateGraphEdgeProvenance :exec
UPDATE graph_edges
SET provenance_json = ?
WHERE id = ?;

-- name: GetGraphGraphEdgesWithValidity :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE valid_from >= ? AND (valid_to IS NULL OR valid_to <= ?)
ORDER BY valid_from DESC
LIMIT ? OFFSET ?;

-- name: GetGraphGraphEdgesByTimeRange :many
SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
FROM graph_edges
WHERE ingested_at >= ? AND ingested_at <= ?
ORDER BY ingested_at DESC
LIMIT ? OFFSET ?;

-- name: GetEntityNeighbors :many
SELECT DISTINCT e.id, e.kind, e.name, e.summary, e.attrs_json, e.created_at, e.updated_at
FROM graph_entities e
JOIN graph_edges ed ON (ed.src_id = e.id OR ed.dst_id = e.id)
WHERE ed.src_id = ? OR ed.dst_id = ?
  AND ed.valid_to IS NULL AND ed.invalidated_at IS NULL
ORDER BY e.created_at DESC
LIMIT ? OFFSET ?;
