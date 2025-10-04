# LibSQL Static Build Configuration

## Pinned Version
- **LibSQL Version**: `v0.9.0` (commit: `4630acad2010bc3f35f6db9c52550fddd81bdb6f`)
- **Pinned Date**: 2025-01-27
- **Reason**: Latest stable release with vector support and stable API

## Build Configuration

### Enabled Features
- ✅ **FTS5**: Full-text search (built-in)
- ✅ **JSON1**: JSON functions (built-in)
- ✅ **R*Tree**: Spatial indexing (enabled via `-DLIBSQL_ENABLE_RTREE=ON`)
- ✅ **Vector**: Native vector operations (enabled via `-DLIBSQL_ENABLE_VECTOR=ON`)
- ✅ **SQLean Modules**:
  - Math: Statistical and mathematical functions
  - Stats: Statistical aggregations (median, percentile, etc.)
  - Text: Text processing and normalization
  - Fuzzy: String similarity and distance functions
  - Crypto: Cryptographic hash functions

### SQLean Module Selection
Only essential modules are included to minimize binary size:
- **Included**: Math, Stats, Text, Fuzzy, Crypto
- **Excluded**: Regexp (use native SQLite regex if needed)
- **Excluded**: UUID (use native Go UUID generation)

## Build Targets
- **linux/amd64**: Primary target platform
- **linux/arm64**: Secondary target for ARM deployments

## Build Process
1. Clone libsql at pinned commit
2. Clone SQLean and copy required modules
3. Apply build patches for static linking
4. Compile with CMake
5. Extract static library (libsql.a)
6. Link into go-libsql via CGO

## sqlite-vec Decision
**Deferred**: We are using LibSQL's native vector implementation instead of sqlite-vec extension. LibSQL provides built-in vector functions (`vector32`, `vector_distance_cos`, etc.) that are more tightly integrated and don't require additional extensions. This decision:
- Reduces build complexity
- Maintains single-binary deployment
- Leverages LibSQL's optimized vector implementation
- Avoids potential compatibility issues with external extensions

## Verification
After build, verify all features:
- FTS5 virtual tables
- JSON functions (json_extract, etc.)
- Vector functions (vector32, vector_distance_cos)
- SQLean functions (sqrt, median, concat_ws, damerau_levenshtein, sha256)
- R*Tree virtual tables

## Dependencies
- CMake 3.16+
- Clang/LLVM 11+
- Rust toolchain (for sqld components)
- SQLite3 development headers

## Build Artifacts
- `libsql-amd64.a`: Static library for x86_64
- `libsql-arm64.a`: Static library for ARM64
- Build logs and verification reports
