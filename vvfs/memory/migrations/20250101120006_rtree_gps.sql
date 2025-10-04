-- +goose Up
-- R*Tree spatial indexing for GPS coordinates in files
-- Enables efficient spatial queries for photos and location-based files
-- Create R*Tree virtual table for GPS coordinates
-- R*Tree provides efficient spatial indexing for 2D bounding boxes
CREATE VIRTUAL TABLE IF NOT EXISTS file_gps_rtree USING rtree(
    rowid,
    -- Links to files.rowid
    min_lat,
    -- Minimum latitude (bounding box)
    max_lat,
    -- Maximum latitude
    min_lon,
    -- Minimum longitude
    max_lon -- Maximum longitude
);
-- Create trigger to automatically maintain R*Tree index when files are inserted
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_files_gps_insert_rtree
AFTER
INSERT ON files
    WHEN json_extract(new.metadata, '$.gps.latitude') IS NOT NULL
    AND json_extract(new.metadata, '$.gps.longitude') IS NOT NULL BEGIN
INSERT INTO file_gps_rtree(rowid, min_lat, max_lat, min_lon, max_lon)
VALUES (
        new.rowid,
        json_extract(new.metadata, '$.gps.latitude') - 0.001,
        -- Small bounding box around point
        json_extract(new.metadata, '$.gps.latitude') + 0.001,
        json_extract(new.metadata, '$.gps.longitude') - 0.001,
        json_extract(new.metadata, '$.gps.longitude') + 0.001
    );
END;
-- +goose StatementEnd
-- Create trigger for updates to GPS metadata
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_files_gps_update_rtree
AFTER
UPDATE OF metadata ON files
    WHEN json_extract(old.metadata, '$.gps.latitude') IS NOT NULL
    OR json_extract(new.metadata, '$.gps.latitude') IS NOT NULL BEGIN -- Remove old GPS entry if it exists
DELETE FROM file_gps_rtree
WHERE rowid = old.rowid;
-- Insert new GPS entry if new metadata has GPS
INSERT INTO file_gps_rtree(rowid, min_lat, max_lat, min_lon, max_lon)
SELECT new.rowid,
    json_extract(new.metadata, '$.gps.latitude') - 0.001,
    json_extract(new.metadata, '$.gps.latitude') + 0.001,
    json_extract(new.metadata, '$.gps.longitude') - 0.001,
    json_extract(new.metadata, '$.gps.longitude') + 0.001
WHERE json_extract(new.metadata, '$.gps.latitude') IS NOT NULL
    AND json_extract(new.metadata, '$.gps.longitude') IS NOT NULL;
END;
-- +goose StatementEnd
-- Create trigger for file deletions
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_files_gps_delete_rtree
AFTER DELETE ON files BEGIN
DELETE FROM file_gps_rtree
WHERE rowid = old.rowid;
END;
-- +goose StatementEnd
-- +goose Down
-- Remove R*Tree GPS indexing
DROP TRIGGER IF EXISTS trg_files_gps_delete_rtree;
DROP TRIGGER IF EXISTS trg_files_gps_update_rtree;
DROP TRIGGER IF EXISTS trg_files_gps_insert_rtree;
DROP TABLE IF EXISTS file_gps_rtree;