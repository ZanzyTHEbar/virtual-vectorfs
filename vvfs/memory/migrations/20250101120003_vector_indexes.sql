-- +goose Up
-- Vector indexes for entities, files, and observations using libSQL
-- Note: libSQL vector indexes use specialized functions for vector similarity search
-- These indexes enable efficient vector_top_k and vector similarity operations
-- Create vector index for entities embeddings using libSQL vector indexing
-- This creates an index that can be used with vector_top_k function
CREATE INDEX IF NOT EXISTS idx_entities_embedding ON entities(libsql_vector_idx(embedding));
-- Create vector index for files embeddings
CREATE INDEX IF NOT EXISTS idx_files_embedding ON files(libsql_vector_idx(embedding));
-- Create vector index for observations embeddings
CREATE INDEX IF NOT EXISTS idx_observations_embedding ON observations(libsql_vector_idx(embedding));
-- +goose Down
-- Remove vector indexes
DROP INDEX IF EXISTS idx_entities_embedding;
DROP INDEX IF EXISTS idx_files_embedding;
DROP INDEX IF EXISTS idx_observations_embedding;