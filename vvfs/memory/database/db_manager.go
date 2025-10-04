// Package database implements the core database operations with sqlc + goose
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/tursodatabase/go-libsql"
)

const defaultProject = "default"

// DBManager handles all database operations with sqlc integration
type DBManager struct {
	config        *Config
	dbs           map[string]*sql.DB
	mu            sync.RWMutex
	capsByProject map[string]capFlags
	capMu         sync.RWMutex        // mutex for capabilities
	queries       map[string]*Queries // sqlc generated queriers
}

// NewDBManager creates a new database manager with sqlc integration
func NewDBManager(config *Config) (*DBManager, error) {
	if config.EmbeddingDims <= 0 || config.EmbeddingDims > 65536 {
		return nil, fmt.Errorf("EMBEDDING_DIMS must be between 1 and 65536 inclusive: %d", config.EmbeddingDims)
	}

	manager := &DBManager{
		config:        config,
		dbs:           make(map[string]*sql.DB),
		capsByProject: make(map[string]capFlags),
		queries:       make(map[string]*Queries),
	}

	// initialize default DB in single-project mode
	if !config.MultiProjectMode {
		if _, err := manager.getDB(defaultProject); err != nil {
			return nil, fmt.Errorf("failed to initialize default database: %w", err)
		}
	}

	return manager, nil
}

// Close closes all database connections.
func (dm *DBManager) Close() error {
	// close dbs
	dm.mu.Lock()
	for name, db := range dm.dbs {
		_ = db.Close()
		delete(dm.dbs, name)
	}
	dm.mu.Unlock()

	return nil
}

// getDB retrieves or creates a DB connection for a project
func (dm *DBManager) getDB(projectName string) (*sql.DB, error) {
	dm.mu.RLock()
	db, ok := dm.dbs[projectName]
	dm.mu.RUnlock()
	if ok {
		return db, nil
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()
	if db, ok = dm.dbs[projectName]; ok {
		return db, nil
	}

	var dbURL string
	if dm.config.MultiProjectMode {
		if projectName == "" {
			return nil, fmt.Errorf("project name cannot be empty in multi-project mode")
		}
		dbPath := filepath.Join(dm.config.ProjectsDir, projectName, "libsql.db")
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create project directory for %s: %w", projectName, err)
		}
		dbURL = fmt.Sprintf("file:%s", dbPath)
	} else {
		dbURL = dm.config.URL
	}

	var newDb *sql.DB
	var err error
	if strings.HasPrefix(dbURL, "file:") {
		newDb, err = sql.Open("libsql", dbURL)
	} else {
		authURL := dbURL
		if dm.config.AuthToken != "" {
			if u, perr := url.Parse(dbURL); perr == nil {
				q := u.Query()
				q.Set("authToken", dm.config.AuthToken)
				u.RawQuery = q.Encode()
				authURL = u.String()
			} else {
				if strings.Contains(dbURL, "?") {
					authURL = dbURL + "&authToken=" + url.QueryEscape(dm.config.AuthToken)
				} else {
					authURL = dbURL + "?authToken=" + url.QueryEscape(dm.config.AuthToken)
				}
			}
		}
		newDb, err = sql.Open("libsql", authURL)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create database connector for project %s: %w", projectName, err)
	}

	if err := dm.initialize(newDb); err != nil {
		newDb.Close()
		return nil, fmt.Errorf("failed to initialize database for project %s: %w", projectName, err)
	}

	// Configure connection pooling for optimal performance
	dm.configureConnectionPooling(newDb)

	// reconcile embedding dims with DB if needed
	if dbDims := detectDBEmbeddingDims(newDb); dbDims > 0 && dbDims != dm.config.EmbeddingDims {
		log.Printf("Embedding dims mismatch: DB=%d, Config=%d. Adopting DB dims.", dbDims, dm.config.EmbeddingDims)
		dm.config.EmbeddingDims = dbDims
	}

	dm.dbs[projectName] = newDb

	// detect caps
	dm.detectCapabilitiesForProject(context.Background(), projectName, newDb)

	// prepare sqlc querier with prepared statements
	ctx := context.Background()
	querier, err := Prepare(ctx, newDb)
	if err != nil {
		newDb.Close()
		return nil, fmt.Errorf("failed to prepare sqlc querier: %w", err)
	}
	dm.queries[projectName] = querier

	_ = newDb.Stats() // touch stats (future metrics)
	return newDb, nil
}

// detectDBEmbeddingDims introspects F32_BLOB size for entities.embedding
func detectDBEmbeddingDims(db *sql.DB) int {
	var sqlText string
	_ = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='entities'").Scan(&sqlText)
	if sqlText != "" {
		low := strings.ToLower(sqlText)
		idx := strings.Index(low, "f32_blob(")
		if idx >= 0 {
			rest := low[idx+len("f32_blob("):]
			end := strings.Index(rest, ")")
			if end > 0 {
				num := strings.TrimSpace(rest[:end])
				if n, err := strconv.Atoi(num); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	var blob []byte
	_ = db.QueryRow("SELECT embedding FROM entities LIMIT 1").Scan(&blob)
	if len(blob) > 0 && len(blob)%4 == 0 {
		return len(blob) / 4
	}
	return 0
}

// initialize creates schema using goose and prepares sqlc querier
func (dm *DBManager) initialize(db *sql.DB) error {
	// Run goose migrations to ensure schema is up to date
	if err := dm.runGooseMigrations(db); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	// Configure PRAGMA settings for optimal performance
	if err := dm.configurePragmaSettings(db); err != nil {
		return fmt.Errorf("failed to configure PRAGMA settings: %w", err)
	}

	return nil
}

// runGooseMigrations runs goose migrations on the provided database
func (dm *DBManager) runGooseMigrations(db *sql.DB) error {
	// Use absolute path to migrations directory
	// Get the current working directory and construct the path
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check if we're in a subdirectory and need to go up to project root
	migrationsPath := filepath.Join(wd, "vvfs", "memory", "migrations")

	// If the path doesn't exist, try going up levels (for test execution from subdirs)
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		// Try going up one level
		migrationsPath = filepath.Join(wd, "..", "vvfs", "memory", "migrations")
		if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
			// Try going up two levels
			migrationsPath = filepath.Join(wd, "..", "..", "vvfs", "memory", "migrations")
		}
	}

	// Set goose dialect to SQLite (required for proper migration execution)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run all pending migrations
	if err := goose.Up(db, migrationsPath); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}
	return nil
}

// configurePragmaSettings applies PRAGMA settings to the database
func (dm *DBManager) configurePragmaSettings(db *sql.DB) error {
	// Journal mode (WAL, DELETE, etc.)
	if dm.config.JournalMode != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s", dm.config.JournalMode)); err != nil {
			return fmt.Errorf("failed to set journal_mode: %w", err)
		}
	}

	// Synchronous mode (NORMAL, FULL, OFF)
	if dm.config.SyncMode != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous = %s", dm.config.SyncMode)); err != nil {
			return fmt.Errorf("failed to set synchronous: %w", err)
		}
	}

	// Cache size (negative values in KB, positive in pages)
	if dm.config.CacheSize != 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size = %d", dm.config.CacheSize)); err != nil {
			return fmt.Errorf("failed to set cache_size: %w", err)
		}
	}

	// Temporary storage location
	if dm.config.TempStore != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA temp_store = %s", dm.config.TempStore)); err != nil {
			return fmt.Errorf("failed to set temp_store: %w", err)
		}
	}

	// Additional performance PRAGMAs
	pragmaSettings := []struct {
		name  string
		value string
	}{
		{"mmap_size", "268435456"},     // 256MB memory map
		{"wal_autocheckpoint", "1000"}, // Checkpoint every 1000 pages
		{"busy_timeout", "5000"},       // 5 second timeout
		{"foreign_keys", "ON"},         // Enable foreign key constraints
	}

	for _, setting := range pragmaSettings {
		// Some PRAGMA statements return values, so we need to handle them differently
		query := fmt.Sprintf("PRAGMA %s = %s", setting.name, setting.value)
		if _, err := db.Exec(query); err != nil {
			// If Exec fails due to returning rows, try Query instead
			if strings.Contains(err.Error(), "returned rows") {
				if _, err := db.Query(query); err != nil {
					return fmt.Errorf("failed to set %s: %w", setting.name, err)
				}
			} else {
				return fmt.Errorf("failed to set %s: %w", setting.name, err)
			}
		}
	}

	return nil
}

// configureConnectionPooling sets up optimal connection pooling parameters
func (dm *DBManager) configureConnectionPooling(db *sql.DB) {
	// Set max open connections (default: 25 for SQLite)
	maxOpen := dm.config.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25 // SQLite default
	}
	db.SetMaxOpenConns(maxOpen)

	// Set max idle connections (default: 25 for SQLite)
	maxIdle := dm.config.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 25 // SQLite default
	}
	db.SetMaxIdleConns(maxIdle)

	// Set connection max idle time (default: 5 minutes)
	idleTime := time.Duration(dm.config.ConnMaxIdleSec) * time.Second
	if idleTime <= 0 {
		idleTime = 5 * time.Minute
	}
	db.SetConnMaxIdleTime(idleTime)

	// Set connection max lifetime (default: 1 hour)
	lifeTime := time.Duration(dm.config.ConnMaxLifeSec) * time.Second
	if lifeTime <= 0 {
		lifeTime = time.Hour
	}
	db.SetConnMaxLifetime(lifeTime)

	// Log connection pool configuration
	log.Printf("Connection pool configured: max_open=%d, max_idle=%d, max_idle_time=%v, max_lifetime=%v",
		maxOpen, maxIdle, idleTime, lifeTime)
}

// GetQuerier returns the sqlc querier for a project
func (dm *DBManager) GetQuerier(projectName string) (*Queries, error) {
	dm.mu.RLock()
	querier, ok := dm.queries[projectName]
	dm.mu.RUnlock()
	if !ok {
		// Try to get DB to initialize querier
		_, err := dm.getDB(projectName)
		if err != nil {
			return nil, err
		}
		dm.mu.RLock()
		querier = dm.queries[projectName]
		dm.mu.RUnlock()
	}
	return querier, nil
}

// WithTx executes a function within a database transaction
func (dm *DBManager) WithTx(ctx context.Context, projectName string, fn func(*Queries) error) error {
	// Get the base querier
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return fmt.Errorf("failed to get querier: %w", err)
	}

	// Get the underlying database connection
	dm.mu.RLock()
	db, ok := dm.dbs[projectName]
	dm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("database connection not found for project: %s", projectName)
	}

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create querier with transaction
	txQuerier := querier.WithTx(tx)

	// Execute the function
	if err := fn(txQuerier); err != nil {
		// Rollback on error
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed and rollback failed: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}

	// Commit on success
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithTxReadOnly executes a read-only function within a transaction
func (dm *DBManager) WithTxReadOnly(ctx context.Context, projectName string, fn func(*Queries) error) error {
	// Get the base querier
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return fmt.Errorf("failed to get querier: %w", err)
	}

	// Get the underlying database connection
	dm.mu.RLock()
	db, ok := dm.dbs[projectName]
	dm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("database connection not found for project: %s", projectName)
	}

	// Begin read-only transaction
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to begin read-only transaction: %w", err)
	}

	// Create querier with transaction
	txQuerier := querier.WithTx(tx)

	// Execute the function
	if err := fn(txQuerier); err != nil {
		// Rollback on error
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("read-only transaction failed and rollback failed: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}

	// Commit on success (read-only commits are safe)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit read-only transaction: %w", err)
	}

	return nil
}
