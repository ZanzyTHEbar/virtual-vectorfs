package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// LibSQLEmbeddedConfig holds configuration for embedded libsql connections
type LibSQLEmbeddedConfig struct {
	DatabasePath string // Path to .db file
}

func ConnectToDB(path string) (*sql.DB, error) {
	cfg := &LibSQLEmbeddedConfig{DatabasePath: path}
	return ConnectToDBWithConfig(cfg)
}

func ConnectToDBWithConfig(config *LibSQLEmbeddedConfig) (*sql.DB, error) {
	// Ensure database directory exists for embedded mode
	dir := filepath.Dir(config.DatabasePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create database directory %s: %v", dir, err)
	}

	// Ensure database file exists for embedded mode
	if _, err := os.Stat(config.DatabasePath); os.IsNotExist(err) {
		slog.Info("Database not found, creating a new one", "path", config.DatabasePath)
		file, err := os.Create(config.DatabasePath)
		if err != nil {
			return nil, fmt.Errorf("could not create db at path %s: %v", config.DatabasePath, err)
		}
		file.Close()
	}

	// Embedded mode with enhanced pragmas
	dsn := fmt.Sprintf("file:%s?_foreign_keys=1&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-64000&_temp_store=memory",
		config.DatabasePath)

	slog.Info("Connecting to embedded libsql", "dsn", dsn)

	db, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open libsql connection: %w", err)
	}

	// Verify built-in capabilities only (no dynamic loading)
	if err := verifyEmbeddedLibSQL(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// verifyEmbeddedLibSQL ensures built-in features are present; it does not load extensions
func verifyEmbeddedLibSQL(db *sql.DB) error {
	ctx := context.Background()

	// Basic connectivity
	var result int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("basic connectivity test failed: %w", err)
	}
	if result != 1 {
		return fmt.Errorf("basic connectivity test failed: unexpected result %d", result)
	}

	// FTS5 should be present in our build
	if _, err := db.ExecContext(ctx, "CREATE VIRTUAL TABLE IF NOT EXISTS temp._fts5_test USING fts5(content)"); err != nil {
		slog.Warn("FTS5 test failed", "error", err)
	} else {
		slog.Info("FTS5 extension verified")
		_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS temp._fts5_test")
	}

	// JSON1 should be present
	var jsonResult string
	if err := db.QueryRowContext(ctx, "SELECT json_extract('{\"test\":\"value\"}', '$.test')").Scan(&jsonResult); err != nil {
		slog.Warn("JSON1 test failed", "error", err)
	} else if jsonResult == "value" {
		slog.Info("JSON1 extension verified")
	} else {
		slog.Warn("JSON1 test returned unexpected result", "result", jsonResult)
	}

	// Probe vector functions (do not fail if not present in the build)
	vectorTests := []string{
		"SELECT typeof(vector32('[1,2,3]'))",
		"SELECT typeof(vector_distance_cos(vector32('[1,2,3]'), vector32('[1,2,3]')))",
	}
	for _, test := range vectorTests {
		var vtype string
		if err := db.QueryRowContext(ctx, test).Scan(&vtype); err != nil {
			slog.Debug("Vector function not available", "query", test, "error", err)
		} else {
			slog.Info("Vector function verified", "query", test, "type", vtype)
		}
	}

	return nil
}
