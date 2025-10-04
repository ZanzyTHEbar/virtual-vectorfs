# 🤖 File4You AI/ML Integration

## Overview

The AI/ML integration provides intelligent file management capabilities for File4You, including content analysis, semantic search, and AI-powered organization suggestions.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Liquid.ai     │    │   Model         │    │   Vector        │
│   LFM-2 Models  │───▶│   Manager       │───▶│   Store         │
│   (GGUF Format) │    │   (Cascade)     │    │   (libsql)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Content       │    │   AI Service    │    │   File          │
│   Analysis      │    │   Integration   │    │   Operations    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Key Components

### 1. GGUF Model Infrastructure

**Liquid.ai LFM-2 Models:**

- **LFM-2-Embed**: Primary embedding model (512 dims, GGUF format)
- **LFM-2-Chat**: Primary conversational model (4096 context, structured output)
- **LFM-2-VL**: Vision-language model for multimodal operations

**GGUF-First Design:**

- Native GGUF loading via `go-llama.cpp`
- Temp file extraction for model loading
- Hardware optimization (GPU layers, F16 memory, threading)

### 2. Model Management

**Cascade System:**

```
Liquid.ai LFM-2 → Gemma 3 → Nomic → Cloud
    ↓              ↓        ↓      ↓
Best Performance  Fallback  Backup  Ultimate
```

**Health Monitoring:**

- Real-time performance tracking
- Success rate monitoring
- Automatic model selection based on health metrics

### 3. AI Service Integration

**Content Analysis:**

- File type detection and metadata extraction
- Content summarization and keyword extraction
- Embedding generation for semantic search

**Semantic Search:**

- Vector similarity search using LFM-2 embeddings
- Hybrid search (FTS5 + Cosine similarity)
- Natural language query understanding

**AI Organization:**

- Operational transforms for safe file operations
- AI-powered folder structure suggestions
- Content-based similarity clustering

## Usage

### Basic Setup

```go
import (
    "github.com/ZanzyTHEbar/virtual-vectorfs/internal/ai"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
)

// Initialize AI service
aiService, err := ai.NewService(filesystem, &ai.Config{
    ModelManagerConfig: &models.ModelManagerConfig{
        EmbeddingModelPath: "models/liquid-ai/LFM-2-Embed-7B.gguf",
        ChatModelPath:      "models/liquid-ai/LFM-2-Chat-7B.gguf",
        EmbeddingDims:      512,
        ContextSize:        4096,
        GPULayers:         -1, // Use all available GPU
    },
    EnableContentAnalysis: true,
    EnableSemanticSearch: true,
})
```

### Content Analysis

```go
// Analyze file content
analysis, err := aiService.AnalyzeFileContent(ctx, fileNode)
if err != nil {
    return err
}

// Access analysis results
fmt.Printf("Content Type: %s\n", analysis.ContentType)
fmt.Printf("Summary: %s\n", analysis.Summary)
fmt.Printf("Keywords: %v\n", analysis.Keywords)
```

### Semantic Search

```go
// Find similar files
similarFiles, err := aiService.FindSimilarFiles(ctx, targetFile, 10)
if err != nil {
    return err
}

// Process results
for _, file := range similarFiles {
    fmt.Printf("Similar: %s\n", file.Path)
}
```

### AI Organization

```go
// Get organization suggestions
suggestion, err := aiService.SuggestOrganization(ctx, files)
if err != nil {
    return err
}

// Apply suggestions
for _, op := range suggestion.Operations {
    executeOperation(op)
}
```

## Configuration

### Model Configuration

```toml
[ai.models]
# Liquid.ai LFM-2 models (GGUF format)
embedding_model_path = "models/liquid-ai/LFM-2-Embed-7B.gguf"
chat_model_path = "models/liquid-ai/LFM-2-Chat-7B.gguf"
vision_model_path = "models/liquid-ai/LFM-2-VL-7B.gguf"

# Performance settings
embedding_dims = 512
context_size = 4096
gpu_layers = -1  # Use all available GPU layers
threads = 4
use_f16_memory = true
use_mmap = true

# Health monitoring
health_check_interval = "30s"
confidence_threshold = 0.85
enable_cascade = true
```

### Runtime Configuration

```go
// Runtime configuration
config := &ai.Config{
    ModelManagerConfig: &models.ModelManagerConfig{
        // Model paths
        EmbeddingModelPath: "models/liquid-ai/LFM-2-Embed-7B.gguf",
        ChatModelPath:      "models/liquid-ai/LFM-2-Chat-7B.gguf",
        VisionModelPath:    "models/liquid-ai/LFM-2-VL-7B.gguf",

        // Performance
        EmbeddingDims: 512,
        ContextSize:   4096,
        GPULayers:    -1,
        Threads:      4,

        // Memory optimization
        UseF16Memory: true,
        UseMMAP:      true,

        // Monitoring
        HealthCheckInterval: 30 * time.Second,
        ConfidenceThreshold: 0.85,
    },

    // Feature flags
    EnableContentAnalysis:  true,
    EnableSemanticSearch:   true,
    EnableAutoOrganization: true,
}
```

## Performance Characteristics

| Operation | Model | Latency | Hardware | Memory |
|-----------|-------|---------|----------|--------|
| **Embedding Generation** | LFM-2-Embed | <80ms/file | Consumer CPU | ~200MB |
| **Text Generation** | LFM-2-Chat | <300ms/simple | Consumer CPU | ~300MB |
| **Image Analysis** | LFM-2-VL | <500ms/image | Consumer GPU | ~400MB |
| **Vector Search** | libsql + LFM-2 | <40ms/query | Embedded DB | N/A |
| **Hybrid Search** | FTS5 + Cosine | <50ms/query | Embedded DB | N/A |

## Error Handling

The AI service includes comprehensive error handling:

```go
// Automatic fallback on model failures
result, err := aiService.AnalyzeFileContent(ctx, fileNode)
if err != nil {
    // Service automatically tries cascade fallback
    // Returns detailed error information
    log.Printf("Analysis failed: %v", err)
}
```

## Health Monitoring

Real-time model health tracking:

```go
// Check model health
health := aiService.GetHealthSummary()
for model, status := range health {
    fmt.Printf("%s: Healthy=%v, Success=%.1f%%, Latency=%v\n",
        model, status.IsHealthy, status.SuccessRate*100, status.AverageLatency)
}
```

## Testing

```bash
# Run AI service tests
go test ./internal/ai/... -v

# Run integration tests
go test ./internal/ai/... -tags=integration -v

# Benchmark performance
go test ./internal/ai/... -bench=. -benchmem
```

## Troubleshooting

### Common Issues

**Model Loading Failures:**

- Ensure GGUF files are in the correct paths
- Check file permissions and disk space
- Verify hardware compatibility (GPU drivers, etc.)

**Performance Issues:**

- Monitor memory usage during model loading
- Check GPU memory availability
- Adjust thread counts based on CPU cores

**Accuracy Problems:**

- Verify model files are not corrupted
- Check embedding dimensions match expectations
- Monitor health metrics for degraded performance

### Debug Logging

```go
// Enable debug logging
log.SetLevel(log.DebugLevel)

// Monitor model operations
aiService.SetDebugMode(true)
```

## Future Enhancements

- **Model Fine-tuning**: Domain-specific fine-tuning for file management
- **Plugin System**: Extensible model provider architecture
- **Advanced Multimodal**: Video/audio content analysis
- **Federated Learning**: Distributed model improvement
- **Hardware Acceleration**: NPU and custom accelerator support

## Security Considerations

- **Local Processing**: All AI operations happen locally (no cloud dependencies)
- **Input Sanitization**: All file content is sanitized before processing
- **Memory Safety**: Proper resource cleanup prevents memory leaks
- **Model Validation**: GGUF files are validated before loading

## Dependencies

- **go-llama.cpp**: GGUF model inference
- **libsql**: Vector database with hybrid search
- **Liquid.ai LFM-2**: Primary AI models (GGUF format)

## Contributing

When adding new AI features:

1. Update configuration schemas
2. Add comprehensive tests
3. Document performance characteristics
4. Consider cascade fallback strategies
5. Update this documentation

## License

This AI/ML integration follows the same license as the main File4You project.
