package database

import (
	"os"
	"strconv"
)

// Config holds the database configuration
type Config struct {
	URL              string
	AuthToken        string
	ProjectsDir      string
	MultiProjectMode bool
	EmbeddingDims    int
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxIdleSec   int
	ConnMaxLifeSec   int
	// PRAGMA settings
	EnableWAL   bool
	SyncMode    string // NORMAL, FULL, OFF
	CacheSize   int    // pages, negative for KB
	TempStore   string // MEMORY, FILE, DEFAULT
	JournalMode string // WAL, DELETE, TRUNCATE, PERSIST, MEMORY, OFF
}

// NewConfig creates a new Config from environment variables
func NewConfig() *Config {
	url := os.Getenv("LIBSQL_URL")
	if url == "" {
		url = "file:./libsql.db"
	}

	authToken := os.Getenv("LIBSQL_AUTH_TOKEN")
	dims := 4
	if v := os.Getenv("EMBEDDING_DIMS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dims = n
		}
	}

	maxOpen := 0
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			maxOpen = n
		}
	}
	maxIdle := 0
	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			maxIdle = n
		}
	}
	idleSec := 0
	if v := os.Getenv("DB_CONN_MAX_IDLE_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			idleSec = n
		}
	}
	lifeSec := 0
	if v := os.Getenv("DB_CONN_MAX_LIFETIME_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			lifeSec = n
		}
	}

	// PRAGMA settings
	enableWAL := false
	if v := os.Getenv("DB_ENABLE_WAL"); v != "" {
		enableWAL = v == "true" || v == "1"
	}

	syncMode := "NORMAL"
	if v := os.Getenv("DB_SYNC_MODE"); v != "" {
		syncMode = v
	}

	cacheSize := -64000 // 64MB default
	if v := os.Getenv("DB_CACHE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cacheSize = n
		}
	}

	tempStore := "MEMORY"
	if v := os.Getenv("DB_TEMP_STORE"); v != "" {
		tempStore = v
	}

	journalMode := "WAL"
	if v := os.Getenv("DB_JOURNAL_MODE"); v != "" {
		journalMode = v
	}

	return &Config{
		URL:            url,
		AuthToken:      authToken,
		EmbeddingDims:  dims,
		MaxOpenConns:   maxOpen,
		MaxIdleConns:   maxIdle,
		ConnMaxIdleSec: idleSec,
		ConnMaxLifeSec: lifeSec,
		// PRAGMA settings
		EnableWAL:   enableWAL,
		SyncMode:    syncMode,
		CacheSize:   cacheSize,
		TempStore:   tempStore,
		JournalMode: journalMode,
	}
}
