package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

var (
	// DefaultConfigPath is the default path to the config file
	DefaultAppName             = "vvfs"
	DefaultAppCMDShortCut      = "vvfs"
	DefaultConfigPath          = filepath.Join(getHomeDir(), ".config", DefaultAppName)
	DefaultCacheDir            = filepath.Join(DefaultConfigPath, ".cache")
	DefaultCentralDBPath       = filepath.Join(DefaultConfigPath, "central.db")
	DefaultDatabaseDir         = filepath.Join(DefaultConfigPath, "databases")
	DefaultWorkspaceDotDir     = fmt.Sprintf(".%s", DefaultAppName)
	DefaultWorkspaceDBPath     = filepath.Join(DefaultWorkspaceDotDir, "workspace.db")
	DefaultWorkspaceConfigFile = filepath.Join(DefaultWorkspaceDotDir, "config.toml")
	DefaultGlobalConfigFile    = filepath.Join(DefaultConfigPath, "config.toml")

	// Default Database settings
	DefaultDatabaseDSN  = "file::memory:?cache=shared" // Default to in-memory SQLite
	DefaultDatabaseType = "libsql"
	// Version will be set at build time using -ldflags
	Version = "development"
)

func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current working directory if home directory is unavailable
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			// Last resort - use tmp directory
			log.Printf("Unable to get home or working directory, using /tmp: %v", err)
			return "/tmp"
		}
		log.Printf("Unable to get home directory, using current working directory: %v", err)
		return cwd
	}
	return homeDir
}

// GetLogger returns a properly configured zerolog logger instance
func GetLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}
