# 📚 Virtual Vector Filesystem - API Reference

## Overview

This document provides a comprehensive reference for the Virtual Vector Filesystem (VVFS) API. VVFS is built using hexagonal architecture with clean interfaces and dependency injection.

## Core Interfaces

### FileSystemManager

The main interface for filesystem operations.

```go
type FileSystemManager interface {
    // Directory operations
    IndexDirectory(ctx context.Context, rootPath string, opts options.IndexOptions) error
    BuildDirectoryTree(ctx context.Context, rootPath string, opts options.TraversalOptions) (*trees.DirectoryNode, error)
    BuildDirectoryTreeWithAnalysis(ctx context.Context, rootPath string, opts options.TraversalOptions) (*trees.DirectoryNode, *types.DirectoryAnalysis, error)
    AnalyzeDirectory(ctx context.Context, rootPath string) (*types.DirectoryAnalysis, error)

    // File operations
    CopyFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
    MoveFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
    DeleteFile(ctx context.Context, path string) error

    // Directory operations
    CopyDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
    MoveDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
    DeleteDirectory(ctx context.Context, path string, recursive bool) error

    // Organization
    OrganizeDirectory(ctx context.Context, sourceDir, targetDir string, opts options.OrganizationOptions) (*types.OrganizationResult, error)
    PreviewOrganization(ctx context.Context, opts options.OrganizationOptions) (*types.OrganizationPreview, error)
    ExecuteOrganization(ctx context.Context, preview *types.OrganizationPreview, opts options.OrganizationOptions) error
    OrganizeWithOptions(ctx context.Context, opts options.OrganizationOptions) error

    // Conflict resolution
    ResolveConflict(ctx context.Context, srcPath, dstPath string, strategy options.ConflictStrategy) (string, error)
    DetectConflict(ctx context.Context, srcPath, dstPath string) (*types.ConflictInfo, error)

    // Git operations
    IsGitRepo(dir string) bool
    InitGitRepo(dir string) error
    GitRewind(dir string, stepsOrSha string) error
    GitAddAndCommit(dir, message string) error
    GitHasUncommittedChanges(dir string) (bool, error)

    // Utilities
    CalculateMaxDepth(rootPath string) (int, error)
    GetFileType(path string) string
    ValidatePath(path string) error
    GetDirectoryTree() *trees.DirectoryTree
}
```

### DirectoryService

Interface for directory management and traversal operations.

```go
type DirectoryService interface {
    IndexDirectory(ctx context.Context, rootPath string, opts options.IndexOptions) error
    BuildDirectoryTree(ctx context.Context, rootPath string, opts options.TraversalOptions) (*trees.DirectoryNode, error)
    BuildDirectoryTreeWithAnalysis(ctx context.Context, rootPath string, opts options.TraversalOptions) (*trees.DirectoryNode, *types.DirectoryAnalysis, error)
    CalculateMaxDepth(ctx context.Context, rootPath string) (int, error)
    AnalyzeDirectory(ctx context.Context, rootPath string) (*types.DirectoryAnalysis, error)
}
```

### OrganizationService

Interface for file organization and workflow operations.

```go
type OrganizationService interface {
    OrganizeFiles(ctx context.Context, opts options.OrganizationOptions) error
    OrganizeDirectory(ctx context.Context, sourcePath, targetPath string, opts options.OrganizationOptions) (*types.OrganizationResult, error)
    DetermineTargetPath(ctx context.Context, fileNode *trees.FileNode, opts options.OrganizationOptions) (string, bool, error)
    PreviewOrganization(ctx context.Context, opts options.OrganizationOptions) (*types.OrganizationPreview, error)
    ExecuteOrganization(ctx context.Context, preview *types.OrganizationPreview, opts options.OrganizationOptions) error
}
```

### ConflictResolver

Interface for file conflict resolution strategies.

```go
type ConflictResolver interface {
    ResolveConflict(ctx context.Context, srcPath, dstPath string, strategy options.ConflictStrategy) (string, error)
    DetectConflict(ctx context.Context, srcPath, dstPath string) (*types.ConflictInfo, error)
    GenerateUniqueFilename(path string) string
}
```

### GitService

Interface for git repository operations.

```go
type GitService interface {
    InitRepository(ctx context.Context, dir string) error
    IsRepository(dir string) bool
    AddFiles(ctx context.Context, repoDir string, paths ...string) error
    CommitChanges(ctx context.Context, repoDir, message string) error
    HasUncommittedChanges(ctx context.Context, repoDir string) (bool, error)
    StashCreate(ctx context.Context, repoDir, message string) error
    StashPop(ctx context.Context, repoDir string, forceOverwrite bool) error
    CheckoutFile(ctx context.Context, repoDir, path string) error
}
```

## Database Interfaces

### ICentralDBProvider

Central database operations for metadata and configuration.

```go
type ICentralDBProvider interface {
    Connect(dsn string) (*sql.DB, error)
    Close() error
    InitSchema() error

    // Snapshot operations
    InsertSnapshot(snapshot *Snapshot) (uuid.UUID, error)
    GetSnapshot(id uuid.UUID) (*Snapshot, error)
    GetLatestSnapshot() (*Snapshot, error)

    // Workspace operations
    AddWorkspace(rootPath, config string) (*Workspace, error)
    GetWorkspace(id uuid.UUID) (*Workspace, error)
    ListWorkspaces() ([]Workspace, error)
    DeleteWorkspace(workspaceID uuid.UUID) error
}
```

### WorkspaceDBProvider

Workspace-specific database operations.

```go
type WorkspaceDBProvider interface {
    Connect(dsn string) (*sql.DB, error)
    Close() error
    InitSchema() error
    InsertFileMetadata(meta *trees.FileMetadata) error
    GetFileMetadata(filePath string) (*trees.FileMetadata, error)
    UpdateFileMetadata(meta *trees.FileMetadata) error
    DeleteFileMetadata(filePath string) error
}
```

## Embedding Providers

### Provider

Interface for generating embeddings from text.

```go
type Provider interface {
    Dimensions() int
    Embed(ctx context.Context, inputs []string) ([][]float32, error)
}
```

### Provider Selection

```go
// Hash-based embeddings (deterministic)
provider := embedding.NewProvider("hash", 384, "")

// Hugging Face models via Hugot
provider := embedding.NewProvider("hugot", 768, "google/gemma-3-1b")

// ONNX models (requires ORT)
provider := embedding.NewProvider("onnx", 768, "path/to/model.onnx")
```

## Data Types

### DirectoryAnalysis

Analysis results for a directory structure.

```go
type DirectoryAnalysis struct {
    TotalFiles       int               `json:"total_files"`
    TotalDirectories int               `json:"total_directories"`
    TotalSize        int64             `json:"total_size"`
    MaxDepth         int               `json:"max_depth"`
    FileTypes        map[string]int    `json:"file_types"`
    SizeDistribution map[string]int    `json:"size_distribution"`
    AgeDistribution  map[string]int    `json:"age_distribution"`
    LargestFiles     []*trees.FileNode `json:"largest_files"`
    OldestFiles      []*trees.FileNode `json:"oldest_files"`
    NewestFiles      []*trees.FileNode `json:"newest_files"`
    Duration         time.Duration     `json:"duration"`
}
```

### DirectoryNode

Hierarchical directory structure node.

```go
type DirectoryNode struct {
    ID       string           `json:"id"`
    Path     string           `json:"path"`
    Type     NodeType         `json:"type"`
    Parent   *DirectoryNode   `json:"-"`
    Children []*DirectoryNode `json:"-"`
    Files    []*FileNode      `json:"files"`
    Metadata Metadata         `json:"metadata"`
}
```

### FileNode

File metadata and content information.

```go
type FileNode struct {
    ID        uuid.UUID `json:"id"`
    Path      string    `json:"path"`
    Name      string    `json:"name"`
    Extension string    `json:"extension"`
    Metadata  Metadata  `json:"metadata"`
}
```

## Options

### TraversalOptions

Options for directory traversal operations.

```go
type TraversalOptions struct {
    MaxDepth         int           `json:"max_depth"`
    MaxWorkers       int           `json:"max_workers"`
    FollowSymlinks   bool          `json:"follow_symlinks"`
    SkipHidden       bool          `json:"skip_hidden"`
    SkipSystem       bool          `json:"skip_system"`
    FileFilters      []string      `json:"file_filters"`
    DirFilters       []string      `json:"dir_filters"`
    Timeout          time.Duration `json:"timeout"`
}
```

### OrganizationOptions

Options for file organization operations.

```go
type OrganizationOptions struct {
    Strategy      string            `json:"strategy"`
    TargetBase    string            `json:"target_base"`
    CreateDirs    bool              `json:"create_dirs"`
    DryRun        bool              `json:"dry_run"`
    PreserveTree  bool              `json:"preserve_tree"`
    Overwrite     bool              `json:"overwrite"`
    CustomRules   map[string]string `json:"custom_rules"`
    Exclusions    []string          `json:"exclusions"`
}
```

### IndexOptions

Options for indexing operations.

```go
type IndexOptions struct {
    ForceRebuild    bool          `json:"force_rebuild"`
    MaxFileSize     int64         `json:"max_file_size"`
    SkipExtensions  []string      `json:"skip_extensions"`
    IncludePatterns []string      `json:"include_patterns"`
    ExcludePatterns []string      `json:"exclude_patterns"`
    Workers         int           `json:"workers"`
    BatchSize       int           `json:"batch_size"`
}
```

## Usage Examples

### Basic Directory Analysis

```go
fs, err := filesystem.New(nil, centralDB)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
analysis, err := fs.AnalyzeDirectory(ctx, "/path/to/directory")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Files: %d, Directories: %d, Size: %d bytes\n",
    analysis.TotalFiles, analysis.TotalDirectories, analysis.TotalSize)
```

### Tree Building with Analysis

```go
dirService := fs.GetDirectoryService()
tree, analysis, err := dirService.BuildDirectoryTreeWithAnalysis(
    ctx, "/path/to/directory", options.DefaultTraversalOptions())
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Tree depth: %d, Files: %d\n",
    tree.MaxDepth(), analysis.TotalFiles)
```

### Custom Organization

```go
opts := options.OrganizationOptions{
    Strategy:   "by_extension",
    TargetBase: "/organized",
    CreateDirs: true,
    DryRun:     false,
}

preview, err := fs.PreviewOrganization(ctx, opts)
if err != nil {
    log.Fatal(err)
}

err = fs.ExecuteOrganization(ctx, preview, opts)
if err != nil {
    log.Fatal(err)
}
```

### Vector Search Integration

```go
// Generate embeddings
provider := embedding.NewProvider("hash", 384, "")
embeddings, err := provider.Embed(ctx, fileContents)

// Store for similarity search
for i, file := range files {
    _, err := centralDB.Exec(`
        INSERT INTO file_embeddings (file_path, embedding)
        VALUES (?, vector32(?))`,
        file.Path, embeddings[i])
}
```

### Conflict Resolution

```go
resolver := fs.GetConflictResolver()
strategy := options.ConflictStrategyOverwrite

newPath, err := resolver.ResolveConflict(ctx, srcPath, dstPath, strategy)
if err != nil {
    log.Fatal(err)
}
```

## Error Handling

### Common Error Types

```go
// Path validation errors
if err := fs.ValidatePath("/invalid/path"); err != nil {
    // Handle invalid path
}

// Database connection errors
if db, err := db.NewCentralDBProvider(); err != nil {
    // Handle database connection failure
}

// Context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

analysis, err := fs.AnalyzeDirectory(ctx, largeDirectory)
if err != nil && ctx.Err() == context.DeadlineExceeded {
    // Handle timeout
}
```

### Error Wrapping

```go
if err != nil {
    return fmt.Errorf("filesystem operation failed: %w", err)
}
```

## Performance Considerations

### Concurrency

- All operations support concurrent processing
- Use appropriate worker counts based on CPU cores
- Context cancellation is supported for all long-running operations

### Memory Management

- Large datasets are processed with streaming operations
- Eytzinger layout optimizes cache locality
- Connection pooling prevents database bottlenecks

### Indexing Strategy

- Spatial indexing for location-based files
- Bitmap indexing for set operations
- Hierarchical embeddings for multi-scale similarity

## Testing

### Running Tests

```bash
# Core component tests
go test ./vvfs/db/... -v
go test ./vvfs/filesystem/... -v
go test ./vvfs/trees/... -v
go test ./vvfs/embedding/... -v

# Integration tests
go test ./... -tags=integration

# Benchmarks
go test -bench=. ./vvfs/filesystem/...
```

### Writing Tests

```go
func TestMyFeature(t *testing.T) {
    // Setup
    fs, err := filesystem.New(nil, mockDB)
    require.NoError(t, err)

    // Test
    result, err := fs.SomeOperation(ctx, params)
    assert.NoError(t, err)
    assert.Equal(t, expected, result)

    // Cleanup
}
```

## Best Practices

### Resource Management

```go
// Always close database connections
defer centralDB.Close()

// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### Error Handling

```go
// Check for specific error types
if errors.Is(err, fs.ErrPathNotFound) {
    // Handle not found
} else if err != nil {
    // Handle other errors
}
```

### Configuration

```go
// Use configuration for flexibility
config := &config.VVFSConfig{
    Database: config.DatabaseConfig{
        DSN: "file:~/vvfs.db",
    },
    Filesystem: config.FilesystemConfig{
        MaxConcurrentOperations: 10,
    },
}
```

### Logging

```go
// Structured logging with zerolog
logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

// Log operations
logger.Info().
    Str("operation", "analyze_directory").
    Str("path", directoryPath).
    Int("files", analysis.TotalFiles).
    Msg("Directory analysis completed")
```

## Migration Guide

### From v0.x to v1.x

- `Database` interface renamed to `ICentralDBProvider`
- `WorkspaceDB` interface renamed to `WorkspaceDBProvider`
- Added context support to all operations
- Options structs are now immutable
- Added support for custom embedding providers

### Breaking Changes

```go
// Old (v0.x)
db := db.NewDatabase(dsn)
analysis := db.AnalyzeDirectory(path)

// New (v1.x)
centralDB, err := db.NewCentralDBProvider()
fs, err := filesystem.New(nil, centralDB)
analysis, err := fs.AnalyzeDirectory(ctx, path)
```

## Contributing

### Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./...

# Format code
go fmt ./...
```

### Code Style

- Follow standard Go formatting (`go fmt`)
- Write table-driven tests
- Use descriptive variable names
- Document public APIs
- Handle errors appropriately

---

This API reference provides comprehensive documentation for integrating with the Virtual Vector Filesystem. For additional examples and use cases, see the `examples/` directory and the main README.
