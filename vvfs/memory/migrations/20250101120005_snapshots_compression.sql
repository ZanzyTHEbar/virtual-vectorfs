-- +goose Up
-- Add compression support to snapshots table
-- This migration adds a compressed directory state column
-- Add compression column to snapshots table
ALTER TABLE snapshots
ADD COLUMN directory_state_compressed BLOB;
-- Create index for compressed snapshots
CREATE INDEX idx_snapshots_compressed ON snapshots(length(directory_state_compressed));
-- Update existing snapshots to have compressed data (placeholder)
-- In a real implementation, this would compress existing directory_state data
-- For now, we'll copy the existing data as-is
UPDATE snapshots
SET directory_state_compressed = directory_state
WHERE directory_state IS NOT NULL;
-- +goose Down
-- Remove compression support from snapshots table
DROP INDEX IF EXISTS idx_snapshots_compressed;
ALTER TABLE snapshots DROP COLUMN directory_state_compressed;