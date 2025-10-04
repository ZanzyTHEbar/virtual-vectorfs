-- +goose Up
-- JSON metadata indexes using json_extract
-- This migration adds efficient indexing for JSON metadata queries
-- Create JSON indexes for entities metadata
CREATE INDEX idx_entities_metadata_type ON entities(json_extract(metadata, '$.type'));
CREATE INDEX idx_entities_metadata_created ON entities(json_extract(metadata, '$.created_at'));
CREATE INDEX idx_entities_metadata_tags ON entities(json_extract(metadata, '$.tags'));
-- Create JSON indexes for files metadata
CREATE INDEX idx_files_metadata_type ON files(json_extract(metadata, '$.file_type'));
CREATE INDEX idx_files_metadata_mime ON files(json_extract(metadata, '$.mime_type'));
CREATE INDEX idx_files_metadata_hash ON files(json_extract(metadata, '$.content_hash'));
-- Create JSON indexes for relations metadata
CREATE INDEX idx_relations_metadata_strength ON relations(json_extract(metadata, '$.strength'));
CREATE INDEX idx_relations_metadata_evidence ON relations(json_extract(metadata, '$.evidence_count'));
-- Create JSON indexes for entity-file relations metadata
CREATE INDEX idx_entity_file_relations_metadata_context ON entity_file_relations(json_extract(metadata, '$.context'));
CREATE INDEX idx_entity_file_relations_metadata_score ON entity_file_relations(json_extract(metadata, '$.relevance_score'));
-- Create JSON indexes for operation_history metadata
CREATE INDEX idx_operation_history_metadata_user ON operation_history(json_extract(metadata, '$.user_id'));
CREATE INDEX idx_operation_history_metadata_session ON operation_history(json_extract(metadata, '$.session_id'));
-- +goose Down
-- Remove JSON metadata indexes
DROP INDEX IF EXISTS idx_entities_metadata_type;
DROP INDEX IF EXISTS idx_entities_metadata_created;
DROP INDEX IF EXISTS idx_entities_metadata_tags;
DROP INDEX IF EXISTS idx_files_metadata_type;
DROP INDEX IF EXISTS idx_files_metadata_mime;
DROP INDEX IF EXISTS idx_files_metadata_hash;
DROP INDEX IF EXISTS idx_relations_metadata_strength;
DROP INDEX IF EXISTS idx_relations_metadata_evidence;
DROP INDEX IF EXISTS idx_entity_file_relations_metadata_context;
DROP INDEX IF EXISTS idx_entity_file_relations_metadata_score;
DROP INDEX IF EXISTS idx_operation_history_metadata_user;
DROP INDEX IF EXISTS idx_operation_history_metadata_session;