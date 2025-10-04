-- +goose Up
-- SQLean Text normalization for efficient search and deduplication
-- Adds normalized text columns using SQLean text processing functions
-- Add normalized columns to entities table for efficient search
ALTER TABLE entities
ADD COLUMN name_normalized TEXT;
ALTER TABLE entities
ADD COLUMN entity_type_normalized TEXT;
-- Create indexes on normalized columns
CREATE INDEX IF NOT EXISTS idx_entities_name_normalized ON entities(name_normalized);
CREATE INDEX IF NOT EXISTS idx_entities_type_normalized ON entities(entity_type_normalized);
-- Add normalized columns to files table
ALTER TABLE files
ADD COLUMN file_path_normalized TEXT;
ALTER TABLE files
ADD COLUMN file_name_normalized TEXT;
-- Create indexes on normalized file columns
CREATE INDEX IF NOT EXISTS idx_files_path_normalized ON files(file_path_normalized);
CREATE INDEX IF NOT EXISTS idx_files_name_normalized ON files(file_name_normalized);
-- Add normalized columns to observations table
ALTER TABLE observations
ADD COLUMN content_normalized TEXT;
-- Create index on normalized observation content
CREATE INDEX IF NOT EXISTS idx_observations_content_normalized ON observations(content_normalized);
-- Populate initial normalized data for existing records
-- Note: Using basic SQLite functions, SQLean regexp_replace not available
UPDATE entities
SET name_normalized = lower(trim(name)),
    entity_type_normalized = lower(trim(entity_type))
WHERE name_normalized IS NULL;
UPDATE files
SET file_path_normalized = lower(trim(file_path)),
    file_name_normalized = lower(
        trim(
            CASE
                WHEN instr(file_path, '/') > 0 THEN substr(file_path, instr(file_path, '/') + 1)
                WHEN instr(file_path, '\\') > 0 THEN substr(file_path, instr(file_path, '\\') + 1)
                ELSE file_path
            END
        )
    )
WHERE file_path_normalized IS NULL;
UPDATE observations
SET content_normalized = lower(trim(content))
WHERE content_normalized IS NULL;
-- Note: Triggers disabled due to SQLite limitations with self-updates
-- Normalization will be handled at application level for now
-- TODO: Re-enable triggers when SQLean regexp_replace is available
-- +goose Down
-- Remove text normalization (no triggers were created)
DROP INDEX IF EXISTS idx_observations_content_normalized;
DROP INDEX IF EXISTS idx_files_name_normalized;
DROP INDEX IF EXISTS idx_files_path_normalized;
DROP INDEX IF EXISTS idx_entities_type_normalized;
DROP INDEX IF EXISTS idx_entities_name_normalized;
-- Note: ALTER TABLE DROP COLUMN is not supported in older SQLite versions
-- The columns will remain but indexes will be removed
-- Manual cleanup may be needed if downgrading