package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/tursodatabase/go-libsql"
)

func TestFilesCRUD(t *testing.T) {
	// Create a test database
	db := createTestDB(t)
	defer db.Close()

	ctx := context.Background()
	// Use non-prepared queries to isolate prepared-statement behavior
	queries := New(db)

	// Create a workspace first
	workspaceID := "test-workspace"
	_, err := queries.CreateWorkspace(ctx, CreateWorkspaceParams{
		ID:        workspaceID,
		RootPath:  "/test/workspace",
		Config:    `{"test": true}`,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	// Test Create File
	embed := make([]byte, 728*4)
	filePath := "/test/file.txt"
	fileID := "test-file-1"
	size := int64(1024)
	modTime := time.Now().Unix()
	checksum := "abc123"

	file, err := queries.CreateFile(ctx, CreateFileParams{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FilePath:    filePath,
		Size:        size,
		ModTime:     modTime,
		IsDir:       0,
		Checksum:    checksum,
		Embedding:   embed,
		Metadata:    `{"mime_type": "text/plain"}`,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	})
	require.NoError(t, err)
	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, filePath, file.FilePath)

	// Debug: ensure row exists via COUNT(*)
	var cnt int64
	countRow := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE workspace_id = ? AND file_path = ?`, workspaceID, filePath)
	require.NoError(t, countRow.Scan(&cnt))
	t.Logf("row count for workspace_id=%s path=%s => %d", workspaceID, filePath, cnt)

	// Sanity check: fetch by ID first
	byID, err := queries.GetFile(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, fileID, byID.ID)
	assert.Equal(t, filePath, byID.FilePath)

	// Test Get File By Path
	// Unconditional raw scan to compare behavior
	rawRow := db.QueryRowContext(ctx, `SELECT id, workspace_id, file_path, size, mod_time, is_dir, checksum, embedding, metadata, created_at, updated_at FROM files WHERE workspace_id = ? AND file_path = ?`, workspaceID, filePath)
	var rid, rws, rpath, rsum, rmeta string
	var rsz, rmt, risd, rca, rua int64
	var remb []byte
	rerr := rawRow.Scan(&rid, &rws, &rpath, &rsz, &rmt, &risd, &rsum, &remb, &rmeta, &rca, &rua)
	t.Logf("unconditional raw scan id=%s ws=%s path=%s size=%d mod=%d isdir=%d checksum=%s emb_len=%d meta=%s created=%d updated=%d err=%v", rid, rws, rpath, rsz, rmt, risd, rsum, len(remb), rmeta, rca, rua, rerr)

	// Raw scan with LIMIT 1
	rawRow2 := db.QueryRowContext(ctx, `SELECT id, workspace_id, file_path, size, mod_time, is_dir, checksum, embedding, metadata, created_at, updated_at FROM files WHERE workspace_id = ? AND file_path = ? LIMIT 1`, workspaceID, filePath)
	var rid2, rws2, rpath2, rsum2, rmeta2 string
	var rsz2, rmt2, risd2, rca2, rua2 int64
	var remb2 []byte
	rerr2 := rawRow2.Scan(&rid2, &rws2, &rpath2, &rsz2, &rmt2, &risd2, &rsum2, &remb2, &rmeta2, &rca2, &rua2)
	t.Logf("raw scan LIMIT1 id=%s ws=%s path=%s size=%d mod=%d isdir=%d checksum=%s emb_len=%d meta=%s created=%d updated=%d err=%v", rid2, rws2, rpath2, rsz2, rmt2, risd2, rsum2, len(remb2), rmeta2, rca2, rua2, rerr2)

	retrievedFile, err := queries.GetFileByPath(ctx, GetFileByPathParams{
		WorkspaceID: workspaceID,
		FilePath:    filePath,
	})
	if err != nil {
		// Already printed raw scan above
	}
	require.NoError(t, err)
	assert.Equal(t, fileID, retrievedFile.ID)
	assert.Equal(t, filePath, retrievedFile.FilePath)

	// Test Update File
	newSize := int64(2048)
	updatedFile, err := queries.UpdateFile(ctx, UpdateFileParams{
		ID:        fileID,
		Size:      newSize,
		ModTime:   time.Now().Unix(),
		Checksum:  "def456",
		Embedding: embed,
		Metadata:  `{"mime_type": "text/plain", "updated": true}`,
		UpdatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	assert.Equal(t, newSize, updatedFile.Size)

	// Test List Files By Directory
	dirPath := "/test"
	filesInDir, err := queries.ListFilesByDirectory(ctx, ListFilesByDirectoryParams{
		WorkspaceID: workspaceID,
		Column2:     sql.NullString{String: dirPath, Valid: true},
	})
	require.NoError(t, err)
	assert.Len(t, filesInDir, 1)
	assert.Equal(t, fileID, filesInDir[0].ID)

	// Test Delete File
	err = queries.DeleteFile(ctx, fileID)
	require.NoError(t, err)

	// Verify deletion
	_, err = queries.GetFileByPath(ctx, GetFileByPathParams{
		WorkspaceID: workspaceID,
		FilePath:    filePath,
	})
	assert.Error(t, err) // Should return error for not found
}

func TestFilesBatchOperations(t *testing.T) {
	// Create a test database
	db := createTestDB(t)
	defer db.Close()

	ctx := context.Background()
	queries := New(db)

	// Create workspace
	workspaceID := "batch-workspace"
	_, err := queries.CreateWorkspace(ctx, CreateWorkspaceParams{
		ID:        workspaceID,
		RootPath:  "/batch/workspace",
		Config:    `{"batch": true}`,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	embed := make([]byte, 728*4)

	// Create multiple files
	testFiles := []struct {
		id       string
		path     string
		size     int64
		isDir    bool
		checksum string
	}{
		{"file1", "/batch/file1.txt", 100, false, "hash1"},
		{"file2", "/batch/file2.txt", 200, false, "hash2"},
		{"dir1", "/batch/subdir", 0, true, ""},
	}

	for _, tf := range testFiles {
		_, err := queries.CreateFile(ctx, CreateFileParams{
			ID:          tf.id,
			WorkspaceID: workspaceID,
			FilePath:    tf.path,
			Size:        tf.size,
			ModTime:     time.Now().Unix(),
			IsDir: func() int64 {
				if tf.isDir {
					return 1
				}
				return 0
			}(),
			Checksum:  tf.checksum,
			Embedding: embed,
			Metadata:  `{"batch": true}`,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		})
		require.NoError(t, err)
	}

	// Test ListFilesByDirectory with files and directories
	files, err := queries.ListFilesByDirectory(ctx, ListFilesByDirectoryParams{
		WorkspaceID: workspaceID,
		Column2:     sql.NullString{String: "/batch", Valid: true},
	})
	require.NoError(t, err)
	assert.Len(t, files, 3)

	// Test filtering by file type
	regularFiles := 0
	directories := 0
	for _, f := range files {
		if f.IsDir != 0 {
			directories++
		} else {
			regularFiles++
		}
	}
	assert.Equal(t, 2, regularFiles)
	assert.Equal(t, 1, directories)

	// Test subdirectory listing
	subDirFiles, err := queries.ListFilesByDirectory(ctx, ListFilesByDirectoryParams{
		WorkspaceID: workspaceID,
		Column2:     sql.NullString{String: "/batch/subdir", Valid: true},
	})
	require.NoError(t, err)
	assert.Len(t, subDirFiles, 0) // Empty directory
}

// createTestDB is shared helper from entities_test.go
