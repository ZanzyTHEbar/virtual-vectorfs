-- +goose Up
-- Clean initial schema following working examples
PRAGMA foreign_keys = ON;
-- Core entities table (knowledge graph nodes)
CREATE TABLE entities (
    name TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    embedding F32_BLOB(728),
    metadata TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_entities_entity_type ON entities(entity_type);
CREATE INDEX idx_entities_created_at ON entities(created_at);
-- Observations table (entity content)
CREATE TABLE observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_name TEXT NOT NULL,
    content TEXT NOT NULL,
    embedding F32_BLOB(728),
    created_at INTEGER NOT NULL,
    FOREIGN KEY(entity_name) REFERENCES entities(name)
);
CREATE INDEX idx_observations_entity ON observations(entity_name);
CREATE INDEX idx_observations_created_at ON observations(created_at);
-- Relations table (entity relationships)
CREATE TABLE relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    metadata TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(source) REFERENCES entities(name),
    FOREIGN KEY(target) REFERENCES entities(name)
);
CREATE INDEX idx_relations_source ON relations(source);
CREATE INDEX idx_relations_target ON relations(target);
CREATE INDEX idx_relations_src_tgt_type ON relations(source, target, relation_type);
CREATE INDEX idx_relations_type_source ON relations(relation_type, source);
CREATE INDEX idx_relations_confidence ON relations(confidence DESC);
-- Workspaces table (file system roots)
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    root_path TEXT NOT NULL UNIQUE,
    config TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_workspaces_root_path ON workspaces(root_path);
CREATE INDEX idx_workspaces_created_at ON workspaces(created_at);
-- Files table (file system metadata with embeddings)
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    size INTEGER,
    mod_time INTEGER,
    is_dir INTEGER DEFAULT 0,
    checksum TEXT,
    embedding F32_BLOB(728),
    metadata TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(workspace_id) REFERENCES workspaces(id),
    UNIQUE(workspace_id, file_path)
);
CREATE INDEX idx_files_workspace_path ON files(workspace_id, file_path);
CREATE INDEX idx_files_mod_time ON files(mod_time);
CREATE INDEX idx_files_size ON files(size);
CREATE INDEX idx_files_created_at ON files(created_at);
-- Entity-file relations (semantic linking)
CREATE TABLE entity_file_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_name TEXT NOT NULL,
    file_id TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    similarity_score REAL,
    metadata TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(entity_name) REFERENCES entities(name),
    FOREIGN KEY(file_id) REFERENCES files(id),
    UNIQUE(entity_name, file_id, relation_type)
);
CREATE INDEX idx_entity_file_entity ON entity_file_relations(entity_name);
CREATE INDEX idx_entity_file_file ON entity_file_relations(file_id);
CREATE INDEX idx_entity_file_confidence ON entity_file_relations(confidence DESC);
CREATE INDEX idx_entity_file_similarity ON entity_file_relations(similarity_score DESC);
-- Snapshots table (directory state backups)
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    taken_at INTEGER NOT NULL,
    directory_state BLOB,
    description TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(workspace_id) REFERENCES workspaces(id)
);
CREATE INDEX idx_snapshots_workspace_taken ON snapshots(workspace_id, taken_at DESC);
CREATE INDEX idx_snapshots_taken_at ON snapshots(taken_at DESC);
-- Operation history table (audit trail)
CREATE TABLE operation_history (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    operation_type TEXT NOT NULL,
    entity_path TEXT,
    old_path TEXT,
    new_path TEXT,
    metadata TEXT,
    performed_by TEXT,
    performed_at INTEGER NOT NULL,
    FOREIGN KEY(workspace_id) REFERENCES workspaces(id)
);
CREATE INDEX idx_history_workspace_time ON operation_history(workspace_id, performed_at DESC);
CREATE INDEX idx_history_operation_type ON operation_history(operation_type);
CREATE INDEX idx_history_performed_by ON operation_history(performed_by);
-- +goose Down
DROP TABLE IF EXISTS operation_history;
DROP TABLE IF EXISTS snapshots;
DROP TABLE IF EXISTS entity_file_relations;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS relations;
DROP TABLE IF EXISTS observations;
DROP TABLE IF EXISTS entities;