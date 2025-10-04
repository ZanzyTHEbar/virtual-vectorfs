-- name: CreateFile :one
-- Create a new file with upsert semantics
INSERT INTO files (
        id,
        workspace_id,
        file_path,
        size,
        mod_time,
        is_dir,
        checksum,
        embedding,
        metadata,
        created_at,
        updated_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(workspace_id, file_path) DO
UPDATE
SET size = excluded.size,
    mod_time = excluded.mod_time,
    checksum = excluded.checksum,
    embedding = excluded.embedding,
    metadata = excluded.metadata,
    updated_at = excluded.updated_at
RETURNING id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at;
-- name: GetFile :one
-- Get file by ID
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE id = ?;
-- name: GetFileByPath :one
-- Get a file by workspace and exact path
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND file_path = ?;
-- name: UpdateFile :one
-- Update file metadata
UPDATE files
SET size = ?,
    mod_time = ?,
    checksum = ?,
    embedding = ?,
    metadata = ?,
    updated_at = ?
WHERE id = ?
RETURNING id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at;
-- name: DeleteFile :exec
-- Delete file by ID
DELETE FROM files
WHERE id = ?;
-- name: ListFilesByWorkspace :many
-- List all files in a workspace with pagination
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
ORDER BY file_path
LIMIT ? OFFSET ?;
-- name: ListFilesByDirectory :many
-- List files whose path has a given directory prefix
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND file_path LIKE ? || '/%'
ORDER BY file_path ASC;
-- name: SearchFiles :many
-- Search files by path or metadata content
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND (
        file_path LIKE '%' || ? || '%'
        OR metadata LIKE '%' || ? || '%'
    )
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;
-- name: GetFilesWithEmbeddings :many
-- Get files that have embeddings (for semantic search)
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND embedding IS NOT NULL
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;
-- name: GetFileStatsByWorkspace :one
-- Get file statistics for a workspace
SELECT COUNT(*) as total_files,
    SUM(size) as total_size,
    AVG(size) as avg_file_size,
    COUNT(
        CASE
            WHEN is_dir THEN 1
        END
    ) as directory_count,
    COUNT(
        CASE
            WHEN NOT is_dir THEN 1
        END
    ) as file_count,
    COUNT(
        CASE
            WHEN embedding IS NOT NULL THEN 1
        END
    ) as files_with_embeddings,
    MAX(mod_time) as latest_modification,
    MIN(created_at) as oldest_file
FROM files
WHERE workspace_id = ?;
-- name: GetFilesBySizeRange :many
-- Get files within a size range
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND size BETWEEN ? AND ?
ORDER BY size DESC
LIMIT ? OFFSET ?;
-- name: GetRecentlyModifiedFiles :many
-- Get files modified within a time range
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    embedding,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND mod_time >= ?
ORDER BY mod_time DESC
LIMIT ? OFFSET ?;
-- name: BatchCreateFiles :exec
-- Batch insert multiple files
INSERT INTO files (
        id,
        workspace_id,
        file_path,
        size,
        mod_time,
        is_dir,
        checksum,
        embedding,
        metadata,
        created_at,
        updated_at
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: BatchUpdateFiles :exec
-- Batch update multiple files
UPDATE files
SET size = ?,
    mod_time = ?,
    checksum = ?,
    embedding = ?,
    metadata = ?,
    updated_at = ?
WHERE id = ?;
-- name: BatchDeleteFiles :exec
-- Batch delete multiple files
DELETE FROM files
WHERE id = ?;
-- name: BulkUpdateFileEmbeddings :exec
-- Bulk update embeddings for multiple files
UPDATE files
SET embedding = ?,
    updated_at = ?
WHERE id = ?;
-- name: GetFilesWithoutEmbeddings :many
-- Get files that need embedding generation
SELECT id,
    workspace_id,
    file_path,
    size,
    mod_time,
    is_dir,
    checksum,
    metadata,
    created_at,
    updated_at
FROM files
WHERE workspace_id = ?
    AND embedding IS NULL
    AND NOT is_dir
ORDER BY size DESC
LIMIT ? OFFSET ?;