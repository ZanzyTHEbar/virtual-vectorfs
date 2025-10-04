-- +goose Up
-- FTS5 virtual tables and triggers for full-text search
-- This migration adds FTS5 support for entities and files
-- Create FTS5 virtual table for entities (observations content)
CREATE VIRTUAL TABLE fts_entities USING fts5(
    entity_name,
    content,
    tokenize = 'unicode61',
    prefix = '2 3 4 5 6 7'
);
-- Create FTS5 virtual table for files (metadata content)
CREATE VIRTUAL TABLE fts_files USING fts5(
    workspace_id,
    file_path,
    metadata,
    tokenize = 'unicode61',
    prefix = '2 3 4 5 6 7'
);
-- +goose StatementBegin
CREATE TRIGGER trg_observations_fts_insert
AFTER
INSERT ON observations BEGIN
INSERT INTO fts_entities(rowid, entity_name, content)
VALUES (new.id, new.entity_name, new.content);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_observations_fts_delete
AFTER DELETE ON observations BEGIN
DELETE FROM fts_entities
WHERE rowid = old.id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_observations_fts_update
AFTER
UPDATE ON observations
    WHEN old.content != new.content
    OR old.entity_name != new.entity_name BEGIN
DELETE FROM fts_entities
WHERE rowid = old.id;
INSERT INTO fts_entities(rowid, entity_name, content)
VALUES (new.id, new.entity_name, new.content);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_files_fts_insert
AFTER
INSERT ON files BEGIN
INSERT INTO fts_files(rowid, workspace_id, file_path, metadata)
VALUES (
        new.rowid,
        new.workspace_id,
        new.file_path,
        COALESCE(new.metadata, '')
    );
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_files_fts_delete
AFTER DELETE ON files BEGIN
DELETE FROM fts_files
WHERE rowid = old.rowid;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_files_fts_update
AFTER
UPDATE OF file_path,
    metadata ON files BEGIN
DELETE FROM fts_files
WHERE rowid = old.rowid;
INSERT INTO fts_files(rowid, workspace_id, file_path, metadata)
VALUES (
        new.rowid,
        new.workspace_id,
        new.file_path,
        COALESCE(new.metadata, '')
    );
END;
-- +goose StatementEnd
-- +goose Down
-- Remove FTS5 tables and triggers
DROP TRIGGER IF EXISTS trg_observations_fts_update;
DROP TRIGGER IF EXISTS trg_observations_fts_delete;
DROP TRIGGER IF EXISTS trg_observations_fts_insert;
DROP TRIGGER IF EXISTS trg_files_fts_update;
DROP TRIGGER IF EXISTS trg_files_fts_delete;
DROP TRIGGER IF EXISTS trg_files_fts_insert;
DROP TABLE IF EXISTS fts_entities;
DROP TABLE IF EXISTS fts_files;