# Virtual Vector Filesystem (vvfs)

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub Repo](https://img.shields.io/badge/GitHub-virtual--vectorfs-181717.svg)](https://github.com/ZanzyTHEbar/virtual-vectorfs)

A high-performance, AI-enhanced virtual filesystem implementation in Go with **embedded LibSQL**, designed for modern file organization and management with advanced indexing, concurrent operations, and machine learning capabilities.

## 🚀 Key Features

### **Embedded Database Engine**

- **Single Binary**: No external database server required
- **LibSQL**: SQLite fork with modern features and vector support
- **Compiled Extensions**: FTS5, JSON1, R*Tree, Vector, SQLean modules
- **Production Ready**: Optimized for performance and reliability

### **Advanced Search & AI Features**

- **Vector Search**: Native LibSQL vector operations for semantic similarity
- **Full-Text Search**: FTS5 virtual tables for document content indexing
- **Spatial Queries**: R*Tree indexing for GPS-enabled files
- **Text Processing**: SQLean text normalization and fuzzy matching
- **Statistical Analysis**: SQLean statistical functions for search ranking

## 🌟 Features

### Core Filesystem Operations

- **Hierarchical Directory Structures** - Advanced tree-based file organization
- **Concurrent File Operations** - High-performance parallel processing using goroutines
- **Intelligent File Organization** - Automated categorization and workflow management
- **Conflict Resolution** - Smart handling of file conflicts with multiple strategies
- **Git Integration** - Seamless version control operations within the filesystem

### Advanced Indexing & Search

- **Spatial Indexing** - KD-tree based spatial indexing for efficient file location
- **Bitmap Indexing** - Roaring bitmaps for ultra-fast set operations
- **Multi-dimensional Indexing** - Eytzinger layout optimization for cache efficiency
- **Path-based Indexing** - Hierarchical path indexing for rapid traversal

### AI/ML Integration (LFM-2 ONLY)

- **Liquid.ai LFM-2 Models** - Enterprise-grade GGUF models (Embed, Chat, VL)
- **Native GGUF Support** - Direct llama.cpp integration for optimal performance
- **Hardware Acceleration** - GPU/CPU optimization with automatic detection
- **Production Hardened** - Comprehensive error handling and resource management
- **Commercial Licensing** - Requires Liquid.ai commercial license for redistribution

### Database & Persistence

- **Embedded LibSQL Integration** - Single-binary embedded database
- **Workspace Management** - Multi-workspace support with isolated configurations
- **Metadata Persistence** - Comprehensive file metadata storage
- **Central Database** - Shared metadata across workspaces

### Developer Experience

- **Hexagonal Architecture** - Clean, testable, and maintainable code structure
- **Comprehensive Testing** - Extensive test suites with table-driven tests
- **Structured Logging** - Zerolog integration for observability
- **Configuration Management** - Viper-based configuration with multiple sources
- **CLI Integration** - Command-line interface for filesystem operations

## 🚀 Quick Start

### Prerequisites

- Go 1.25 or later
- SQLite3 development libraries (optional, for enhanced performance)

### Installation

```bash
# Clone the repository
git clone https://github.com/ZanzyTHEbar/virtual-vectorfs.git
cd virtual-vectorfs

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the project
go build ./...
```

### Database Setup (Embedded LibSQL)

Virtual VectorFS uses **embedded LibSQL** with all advanced features compiled into the single binary.

#### **Quick Start (Embedded)**

```bash
# Build with all features
make build-libsql-amd64
make build-app-amd64

# Run the single binary
./bin/vvfs-amd64
```

#### **Custom Build**

```bash
# Build LibSQL static libraries
make build-libsql-amd64  # or build-libsql-arm64

# Build application
make build-app-amd64

# Run smoke tests
make smoke-test
```

### **Compiled Database Features**

Virtual VectorFS includes these **statically compiled** features:

### **LFM-2 AI Model Setup**

Virtual VectorFS uses **Liquid.ai LFM-2 models exclusively** for AI/ML capabilities. These are enterprise-grade proprietary models that require commercial licensing.

#### **Prerequisites**

- **Commercial License**: Contact <sales@liquid.ai> for LFM-2 redistribution rights
- **HuggingFace CLI**: `pip install huggingface_hub`
- **Hardware Requirements**: 16GB+ RAM, NVIDIA GPU recommended

#### **Download LFM-2 Models**

```bash
# Download and embed LFM-2 models (includes validation)
./scripts/download_lfm2_models.sh
```

#### **Build with LFM-2 Models**

```bash
# Build production binary with embedded LFM-2 models
go build -o file4you-lfm2 -ldflags="-s -w" .

# Expected binary size: ~15-20GB with embedded models
ls -lh file4you-lfm2
```

#### **LFM-2 Model Specifications**

| Model | Purpose | Size | Context | Performance |
|-------|---------|------|---------|-------------|
| **LFM-2-Embed-7B** | Text Embeddings | ~4GB | 2K tokens | <100ms/query |
| **LFM-2-Chat-7B** | Conversational AI | ~7GB | 32K tokens | <500ms/response |
| **LFM-2-VL-7B** | Vision-Language | ~7GB | 16K tokens | <1s/analysis |

### **AI/ML Features**

#### **Core SQLite Features**

- ✅ **FTS5**: Full-text search with virtual tables and ranking
- ✅ **JSON1**: Complete JSON manipulation and querying
- ✅ **R*Tree**: Spatial indexing for GPS coordinates

#### **LibSQL Native Features**

- ✅ **Vector Operations**: Native vector data types and similarity functions
- ✅ **Vector Search**: Cosine, L2, and other distance metrics
- ✅ **Vector Indexing**: Efficient storage and retrieval

#### **SQLean Extensions (Compiled-in)**

- ✅ **Math**: `sqrt()`, `pow()`, `ceil()`, `floor()`, `exp()`, `log()`
- ✅ **Stats**: `median()`, `percentile()`, `stddev()`, advanced aggregations
- ✅ **Text**: `concat_ws()`, `trim()`, text normalization functions
- ✅ **Fuzzy**: `damerau_levenshtein()`, `jaro_winkler()`, string similarity
- ✅ **Crypto**: `sha256()`, `md5()`, cryptographic hash functions

### **Advanced Usage Examples**

#### **Vector Search**

```sql
-- Vector similarity search
SELECT * FROM files
WHERE vector_distance_cos(embedding, vector32('[1,2,3]')) < 0.8;
```

#### **Full-Text Search**

```sql
-- FTS5 content search
SELECT * FROM files_fts WHERE files_fts MATCH 'database vector';
```

#### **Spatial Queries**

```sql
-- R*Tree GPS queries
SELECT * FROM file_gps_rtree
WHERE min_lat <= 40.7 AND max_lat >= 40.7
  AND min_lon <= -74.0 AND max_lon >= -74.0;
```

#### **SQLean Text Processing**

```sql
-- Normalized text search
SELECT * FROM files
WHERE file_name_normalized LIKE concat_ws('%', 'report', '%');
```

#### **Statistical Analysis**

```sql
-- Statistical aggregations
SELECT median(vector_distance_cos(embedding, query_vector)) as median_distance
FROM search_results;
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/ports"
)

func main() {
    // Create database provider
    centralDB, err := db.NewCentralDBProvider()
    if err != nil {
        log.Fatal(err)
    }
    defer centralDB.Close()

    // Create terminal interactor
    interactor := ports.NewTerminalInteractor()

    // Create filesystem manager
    fs, err := filesystem.New(interactor, centralDB)
    if err != nil {
        log.Fatal(err)
    }

    // Index a directory
    ctx := context.Background()
    err = fs.IndexDirectory(ctx, "/path/to/directory", filesystem.DefaultIndexOptions())
    if err != nil {
        log.Fatal(err)
    }

    // Build directory tree with analysis
    tree, analysis, err := fs.BuildDirectoryTreeWithAnalysis(ctx, "/path/to/directory", filesystem.DefaultTraversalOptions())
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Indexed %d files, %d directories", analysis.FileCount, analysis.DirectoryCount)
    _ = tree
}
```

## 📖 Documentation

### Architecture Overview

The project follows a **hexagonal architecture** (ports and adapters) pattern:

```
├── ports/           # Application ports (interfaces)
├── filesystem/      # Core filesystem business logic
│   ├── interfaces/  # Service interfaces
│   ├── services/    # Service implementations
│   ├── types/       # Data types and DTOs
│   ├── options/     # Configuration options
│   └── common/      # Shared utilities
├── trees/           # Tree data structures and algorithms
├── indexing/        # Advanced indexing implementations
├── embedding/       # AI/ML embedding providers
├── db/              # Database providers and interfaces
├── memory/          # In-memory data structures
└── config/          # Configuration management
```

### Key Components

#### Filesystem Services

- **DirectoryService** - Directory indexing and tree building
- **FileOperations** - File manipulation operations
- **OrganizationService** - Intelligent file organization
- **ConflictResolver** - File conflict detection and resolution
- **GitService** - Git repository operations

#### Advanced Features

- **ConcurrentTraverser** - High-performance parallel directory traversal
- **KDTree** - Spatial indexing for file locations
- **RoaringBitmaps** - Efficient set operations for file indexing
- **ONNX Providers** - Machine learning model execution

## 🔧 Configuration

Create a configuration file at `~/.config/vvfs/config.toml`:

```toml
[database]
type = "sqlite3"
dsn = "file:~/vvfs/central.db"

[filesystem]
cache_dir = "~/.config/vvfs/.cache"
max_concurrent_operations = 10

[embedding]
default_provider = "onnx"
model_path = "~/.config/vvfs/models"

[logging]
level = "info"
format = "json"
```

## 🧪 Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...

# Run specific test
go test -run TestConcurrentTraverser ./vvfs/filesystem/
```

## 📊 Performance

The filesystem is optimized for high-performance operations:

- **Concurrent Processing** - Utilizes all available CPU cores
- **Memory-Efficient** - Streaming operations for large file sets
- **Cache-Optimized** - Eytzinger layout for improved cache locality
- **Database Performance** - Connection pooling and prepared statements

## 🤝 Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Write tests for your changes
4. Ensure all tests pass: `go test ./...`
5. Follow conventional commit format for commits
6. Submit a pull request

### Development Guidelines

- **Code Style** - Follow standard Go formatting (`go fmt`)
- **Testing** - Write table-driven tests for new functionality
- **Documentation** - Update documentation for API changes
- **Performance** - Include benchmarks for performance-critical code
- **Security** - Validate inputs and handle errors properly

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Roaring Bitmaps** - For efficient bitmap operations
- **ONNX Runtime** - For machine learning model execution
- **Turso** - For distributed SQLite database
- **Go Community** - For the excellent standard library and ecosystem

## 🔗 Related Projects

- [go-fuse](https://github.com/hanwen/go-fuse) - FUSE filesystem implementation
- [bleve](https://github.com/blevesearch/bleve) - Full-text search library
- [badger](https://github.com/dgraph-io/badger) - Key-value database

## 📞 Support

For questions and support:

- Open an issue on [GitHub](https://github.com/ZanzyTHEbar/virtual-vectorfs/issues)
- Check the [documentation](docs/) for detailed guides
- Join our community discussions

---

**Built with ❤️ in Go**
