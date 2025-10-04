# 🚀 Virtual Vector Filesystem - Quick Start Guide

## Overview

Virtual Vector Filesystem (VVFS) is a high-performance, AI-enhanced virtual filesystem implementation in Go with embedded LibSQL. It provides advanced indexing, concurrent operations, and machine learning capabilities for modern file organization and management.

## Key Features

- **🔍 Advanced Search**: Vector search, full-text search, spatial queries
- **⚡ High Performance**: Concurrent processing, memory-efficient operations
- **🧠 AI/ML Integration**: Embedding providers, ONNX runtime support
- **💾 Embedded Database**: Single-binary deployment with LibSQL
- **🌳 Hierarchical Trees**: Advanced tree structures with spatial indexing
- **🔧 Production Ready**: Comprehensive testing, structured logging

## Quick Start

### 1. Prerequisites

```bash
# Go 1.25 or later
go version

# SQLite3 development libraries (optional, for enhanced performance)
# Most systems have this pre-installed
```

### 2. Clone and Setup

```bash
git clone https://github.com/ZanzyTHEbar/virtual-vectorfs.git
cd virtual-vectorfs
go mod download
```

### 3. Run Tests (Recommended)

```bash
# Run core tests
go test ./vvfs/db/... -v
go test ./vvfs/filesystem/... -v
go test ./vvfs/trees/... -v

# Run comprehensive test suite
go test ./...
```

### 4. Create Your First Application

```go
package main

import (
    "context"
    "log"

    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
)

func main() {
    // Create embedded LibSQL database
    centralDB, err := db.NewCentralDBProvider()
    if err != nil {
        log.Fatal(err)
    }
    defer centralDB.Close()

    // Create filesystem manager
    fs, err := filesystem.New(nil, centralDB)
    if err != nil {
        log.Fatal(err)
    }

    // Analyze a directory
    ctx := context.Background()
    analysis, err := fs.AnalyzeDirectory(ctx, "/path/to/your/directory")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found %d files, %d directories",
        analysis.TotalFiles, analysis.TotalDirectories)
}
```

## Database Features

VVFS includes embedded LibSQL with all advanced features:

### Vector Operations

```sql
-- Create vectors
SELECT vector32('[1,2,3,4,5]');

-- Calculate similarity
SELECT vector_distance_cos(
    vector32('[1,2,3]'),
    vector32('[1,2,4]')
);
```

### Full-Text Search

```sql
-- Create FTS5 virtual table
CREATE VIRTUAL TABLE documents_fts
USING fts5(title, content);

-- Search documents
SELECT * FROM documents_fts
WHERE documents_fts MATCH 'database vector';
```

### Spatial Queries

```sql
-- R*Tree for GPS coordinates
CREATE VIRTUAL TABLE locations
USING rtree(id, min_lat, max_lat, min_lon, max_lon);

-- Find nearby locations
SELECT * FROM locations
WHERE min_lat <= 40.7 AND max_lat >= 40.7
  AND min_lon <= -74.0 AND max_lon >= -74.0;
```

### SQLean Extensions

```sql
-- Mathematical functions
SELECT sqrt(16), pow(2, 8), median(scores);

-- Text processing
SELECT concat_ws(' ', 'Hello', 'World');

-- Fuzzy matching
SELECT damerau_levenshtein('hello', 'helo');
```

## Configuration

### Basic Configuration

Create `~/.config/vvfs/config.toml`:

```toml
[database]
type = "libsql"
dsn = "file:~/vvfs/central.db"

[filesystem]
cache_dir = "~/.config/vvfs/.cache"
max_concurrent_operations = 10

[embedding]
provider = "hugot"
model_path = "google/gemma-3-1b"
dims = 768

[logging]
level = "info"
format = "json"
```

### Environment Variables

```bash
export VVFS_DATABASE_DSN="file:~/my-vvfs.db"
export VVFS_LOG_LEVEL="debug"
```

## Performance Tips

### Concurrent Operations

```go
// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Operations automatically use all CPU cores
analysis, err := fs.AnalyzeDirectory(ctx, largeDirectory)
```

### Memory Management

- Large file sets are processed with streaming operations
- Eytzinger layout optimizes cache locality
- Connection pooling prevents database bottlenecks

### Indexing Strategy

- Use spatial indexing for location-based files
- Enable bitmap indexing for set operations
- Consider hierarchical embedding representations (Matryoshka)

## Advanced Usage

### Custom Embedding Providers

```go
// Use hash-based embeddings (deterministic)
provider := embedding.NewProvider("hash", 384, "")

// Use ONNX models (requires ORT)
provider := embedding.NewProvider("onnx", 768, "path/to/model.onnx")
```

### Custom File Organization

```go
opts := options.OrganizationOptions{
    Strategy:     "by_extension",
    TargetBase:   "/organized",
    CreateDirs:   true,
    DryRun:       false,
}

err := fs.OrganizeWithOptions(ctx, opts)
```

### Vector Search Integration

```go
// Generate embeddings for files
embeddings, err := embeddingProvider.Embed(ctx, fileContents)

// Store in database for similarity search
for i, file := range files {
    _, err := db.Exec(`
        INSERT INTO file_embeddings (file_path, embedding)
        VALUES (?, vector32(?))
    `, file.Path, embeddings[i])
}
```

## Troubleshooting

### Common Issues

**Database Connection Errors**

```bash
# Check database file permissions
ls -la ~/.config/vvfs/central.db

# Test database connectivity
go run -c 'db, err := db.NewCentralDBProvider(); if err != nil { log.Fatal(err) }'
```

**Performance Issues**

```bash
# Enable debug logging
export VVFS_LOG_LEVEL=debug

# Check system resources
htop  # or top
```

**Memory Usage**

```bash
# Monitor memory usage
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Build Issues

**Custom LibSQL Build Fails**

```bash
# Use embedded LibSQL instead (recommended)
# The system works perfectly with go-libsql

# If you need the custom build:
make clean-libsql
# Update LibSQL version in Makefile
# Try newer Rust toolchain
```

## Production Deployment

### Single Binary

```bash
# Build optimized binary
go build -ldflags="-s -w" -o vvfs .

# Run with configuration
./vvfs --config=/path/to/config.toml
```

### Docker Deployment

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o vvfs .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/vvfs /usr/local/bin/vvfs
CMD ["vvfs"]
```

### System Service

```bash
# Create systemd service
sudo cp vvfs.service /etc/systemd/system/
sudo systemctl enable vvfs
sudo systemctl start vvfs
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write comprehensive tests
4. Follow conventional commits
5. Submit a pull request

## Support

- **Issues**: [GitHub Issues](https://github.com/ZanzyTHEbar/virtual-vectorfs/issues)
- **Documentation**: Check the `docs/` directory
- **Discussions**: Join community discussions

---

**Built with ❤️ in Go - Ready for production!**
