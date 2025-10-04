# Build Tags Guide for Virtual VectorFS Models

## Overview

The models package uses Go build tags to support both CGO-dependent (llama.cpp) and pure-Go builds. This allows the codebase to compile and run in environments where CGO is not available (e.g., CI/CD pipelines, cross-compilation).

## Build Tags

### `llama`

Enables full llama.cpp functionality via CGO bindings.

**Files compiled:**

- `gguf_provider_llama.go` - Full GGUFProvider implementation with llama.cpp
- `open_providers_llama.go` - Full Open*Provider implementations with llama.cpp
- `embed.go` (with `embed_models`) - Embedded model support

**Usage:**

```bash
# Build with llama support (requires CGO and llama.cpp dependencies)
go build -tags=llama ./vvfs/generation/models/...

# Test with llama support
go test -tags=llama ./vvfs/generation/models/...
```

### `no_llama` / default (no tags)

Provides no-op implementations that allow compilation without CGO dependencies.

**Files compiled:**

- `gguf_provider_no_llama.go` - No-op GGUFProvider returning errors
- `open_providers_no_llama.go` - No-op Open*Provider returning errors
- `embed_noembed.go` - No embedded model support

**Usage:**

```bash
# Build without llama support (pure Go, no CGO)
go build ./vvfs/generation/models/...

# Or explicitly with no_llama tag
go build -tags=no_llama ./vvfs/generation/models/...

# Test without llama support
go test ./vvfs/generation/models/...
```

### `embed_models`

Embeds GGUF model files into the binary at compile time.

**Files compiled:**

- `embed.go` - Provides `readEmbeddedModelBytes` function

**Without this tag:**

- `embed_noembed.go` - Disables embedded models, requires file-path loading

**Usage:**

```bash
# Build with embedded models (requires models in gguf/ directory)
go build -tags="llama,embed_models" ./vvfs/generation/models/...

# Build without embedded models (loads from file paths at runtime)
go build -tags=llama ./vvfs/generation/models/...
```

### `netgo`

Pure Go networking implementation (no CGO for networking).

**Usage:**

```bash
# Build with pure Go networking (useful for static binaries)
go build -tags=netgo ./vvfs/generation/models/...

# Test with netgo (ensures no CGO dependencies leak in)
go test -tags=netgo ./vvfs/generation/models/...
```

## Common Build Scenarios

### 1. Development Build (Full Features, CGO Required)

```bash
# Build with llama.cpp support, loading models from files
go build -tags=llama -o vvfs-dev ./cmd/vvfs/

# Environment variables for model paths
export VVFS_EMBED_MODEL_PATH=/path/to/open-embed.gguf
export VVFS_CHAT_MODEL_PATH=/path/to/open-chat.gguf
export VVFS_VISION_MODEL_PATH=/path/to/open-vision.gguf
```

### 2. Production Build with Embedded Models (CGO Required)

```bash
# Ensure models are in vvfs/generation/models/gguf/
# Then build with both llama and embed_models tags
go build -tags="llama,embed_models" -o vvfs-prod ./cmd/vvfs/

# This creates a single binary with models embedded
# No external model files needed at runtime
```

### 3. CI/CD Build (No CGO, Unit Tests Only)

```bash
# Build and test without CGO dependencies
go build -tags=netgo ./...
go test -tags=netgo ./...

# This allows builds in Docker containers or cross-compilation
# Providers will return "llama.cpp not available" errors at runtime
```

### 4. Static Binary for Deployment (CGO, No Embedded Models)

```bash
# Build with llama support but load models from environment paths
CGO_ENABLED=1 go build -tags=llama -ldflags="-w -s" -o vvfs ./cmd/vvfs/

# Deploy with model files and set environment variables
export VVFS_EMBED_MODEL_PATH=/opt/vvfs/models/open-embed.gguf
# etc.
```

## File Structure

```
vvfs/generation/models/
├── gguf_provider_shared.go      # Shared structs/config (no build tags)
├── gguf_provider_llama.go       # CGO implementation (tag: llama && !no_llama)
├── gguf_provider_no_llama.go    # No-op implementation (tag: !llama || no_llama)
├── open_providers_llama.go      # CGO Open*Providers (tag: llama && !no_llama)
├── open_providers_no_llama.go   # No-op Open*Providers (tag: !llama || no_llama)
├── embed.go                     # Embedded models (tag: embed_models)
├── embed_noembed.go             # No embedded models (tag: !embed_models)
├── model_manager.go             # Orchestrator (no build tags, works with both)
└── gguf/                        # Model files directory (for embedding)
    ├── open-embed.gguf
    ├── open-chat-qwen3-1_7b.gguf
    └── open-vision.gguf
```

## Testing Strategy

### Unit Tests (No CGO)

Run in all environments, including CI/CD:

```bash
go test -tags=netgo ./vvfs/generation/models/...
```

Tests verify:

- Provider initialization
- Configuration validation
- Error handling
- Interface contracts

### Integration Tests (CGO Required)

Run only in environments with CGO and real models:

```bash
go test -tags=llama ./vvfs/generation/models/... -run Integration
```

Tests verify:

- Actual model loading
- Real inference
- Performance benchmarks

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VVFS_EMBED_MODEL_PATH` | Path to embedding model | `vvfs/generation/models/gguf/open-embed.gguf` |
| `VVFS_CHAT_MODEL_PATH` | Path to chat model | `vvfs/generation/models/gguf/open-chat-qwen3-1_7b.gguf` |
| `VVFS_VISION_MODEL_PATH` | Path to vision model | `vvfs/generation/models/gguf/open-vision.gguf` |
| `VVFS_THREADS` | Number of CPU threads | Auto (NumCPU) |
| `VVFS_GPU_LAYERS` | GPU layers to offload | `0` (CPU only) |

## Troubleshooting

### Error: "llama.cpp not available in this build"

**Cause:** Built without `llama` tag or CGO disabled.

**Solution:**

```bash
# Rebuild with llama tag and CGO enabled
CGO_ENABLED=1 go build -tags=llama ./...
```

### Error: "embedded models disabled; use file-path loading"

**Cause:** Built without `embed_models` tag.

**Solution:**
Either rebuild with `embed_models` tag:

```bash
go build -tags="llama,embed_models" ./...
```

Or provide model paths via environment variables:

```bash
export VVFS_EMBED_MODEL_PATH=/path/to/model.gguf
```

### Error: "common.h: No such file or directory"

**Cause:** Missing llama.cpp CGO dependencies.

**Solution:**

1. Install llama.cpp dependencies (see main README)
2. Or build without llama support:

```bash
go build -tags=netgo ./...
```

## Performance Considerations

- **Embedded models** increase binary size (~1-2GB) but eliminate external dependencies
- **File-path models** keep binaries small (~10-20MB) but require model files at runtime
- **CGO builds** offer full performance but limit deployment flexibility
- **No-CGO builds** allow static binaries and cross-compilation but disable AI features

## Best Practices

1. **Development:** Use `llama` tag with file-path loading for fast iteration
2. **CI/CD:** Use `netgo` for fast unit tests without CGO overhead
3. **Production:** Choose based on deployment requirements:
   - **Embedded** for simplicity (single binary)
   - **File-path** for flexibility (model hot-swapping)
4. **Integration tests:** Guard with `//go:build llama` and skip in CI
5. **Environment overrides:** Always support runtime model path configuration
