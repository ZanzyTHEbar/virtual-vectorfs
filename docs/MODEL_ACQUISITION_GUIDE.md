# 🧠 GGUF Model Acquisition Guide for File4You

## Overview

To make File4You production-ready, you need to embed **real GGUF models** instead of placeholder files. This guide covers all legal and technical approaches to acquire and embed production-ready models.

## ⚖️ Licensing Considerations

### LFM-2 Models (RESTRICTED)

**Liquid.ai LFM-2 models use LFM Open License v1.0** with significant commercial restrictions.

**CRITICAL RESTRICTIONS:**

1. **Revenue Threshold**: Commercial use ONLY permitted for organizations with < $10M annual revenue
2. **Non-Profit Exemption**: Qualified 501(c)(3) non-profits exempt for research purposes
3. **No Redistribution**: Cannot redistribute models if revenue > $10M threshold

**License Options:**

1. **LFM Open License v1.0**: Free but restricted (current license)
2. **Commercial License**: Contact Liquid.ai for full redistribution rights
3. **API Integration**: Use Liquid.ai API instead of embedding
4. **Open-Source Alternatives**: Use production-ready open-source models

### Open-Source Models (Recommended for Development)

Use these production-ready alternatives with permissive licenses:

| Model Type | Recommended Model | Size | License | Performance |
|------------|------------------|------|---------|-------------|
| **Embedding** | nomic-embed-text-v1.5 | ~200MB | Apache 2.0 | Excellent |
| **Chat** | Llama-3.2-3B-Instruct | ~2GB | Llama 2 | Very Good |
| **Vision** | llava-phi-3-mini | ~2.5GB | MIT/Apache | Good |

## 📥 Download Methods

### Method 1: Automated Scripts (Recommended)

#### Option A: LFM-2 Models (Requires License)

```bash
# Download LFM-2 models (requires Liquid.ai license)
chmod +x scripts/download_lfm2_models.sh
./scripts/download_lfm2_models.sh
```

#### Option B: Open-Source Models (Immediate)

```bash
# Download open-source alternatives
chmod +x scripts/download_open_source_models.sh
./scripts/download_open_source_models.sh
```

### Method 2: Manual Download

#### Step 1: Install HuggingFace CLI

```bash
pip install huggingface_hub
huggingface-cli login  # Or set HF_TOKEN
```

#### Step 2: Download Models

```bash
# Create directories
mkdir -p models/liquid-ai vvfs/generation/embedded

# Download LFM-2 models (if licensed)
huggingface-cli download LiquidAI/LFM-2-Embed-7B \
    --local-dir models/liquid-ai/embed \
    --local-dir-use-symlinks False

# Download open-source alternatives
huggingface-cli download nomic-ai/nomic-embed-text-v1.5-GGUF \
    nomic-embed-text-v1.5.Q4_K_M.gguf \
    --local-dir models/open-source

# Copy to embedded directory
cp models/liquid-ai/embed/*.gguf vvfs/generation/embedded/
# OR for open-source:
cp models/open-source/nomic-embed-text-v1.5.Q4_K_M.gguf vvfs/generation/embedded/lfm2-embed-7b.gguf
```

### Method 3: Direct Model Sources

#### Official Repositories

- **Liquid.ai**: <https://huggingface.co/LiquidAI>
- **Nomic.ai**: <https://huggingface.co/nomic-ai>
- **Meta Llama**: <https://huggingface.co/meta-llama>
- **Microsoft Phi**: <https://huggingface.co/microsoft>

#### GGUF Conversion Tools

If you have PyTorch models, convert them:

```bash
pip install llama.cpp
python -m llama.cpp.convert --help
```

## 🚀 Production Deployment Strategies

### Strategy 1: Embedded Models (Current Implementation)

```go
//go:embed embedded/lfm2-embed-7b.gguf
var modelData []byte
```

**Pros**: Single binary, no external dependencies
**Cons**: Large binary size, model updates require rebuild
**Best For**: Desktop applications, air-gapped environments

### Strategy 2: Model Registry (Recommended for Production)

```go
// Dynamic model loading from configurable paths
type ModelRegistry struct {
    basePath string
    models   map[string]string
}

func (r *ModelRegistry) LoadModel(name string) ([]byte, error) {
    path := filepath.Join(r.basePath, r.models[name])
    return os.ReadFile(path)
}
```

**Pros**: Smaller binaries, runtime model updates, flexible deployment
**Cons**: Requires model files on filesystem
**Best For**: Server applications, cloud deployments

### Strategy 3: Hybrid Approach

```go
// Fallback system: embedded → external → download
func (p *GGUFProvider) LoadModel() ([]byte, error) {
    // Try embedded first
    if data, err := embedded.GetEmbeddedModelData(modelType); err == nil {
        return data, nil
    }

    // Try external path
    if data, err := os.ReadFile(p.config.ModelPath); err == nil {
        return data, nil
    }

    // Try download/cache
    return p.downloadAndCacheModel()
}
```

## 📊 Model Performance Comparison

### Embedding Models

| Model | Dimensions | Size | Quality | Speed | License |
|-------|------------|------|---------|-------|---------|
| **nomic-embed-text-v1.5** | 768 | 200MB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Apache 2.0 |
| **text-embedding-ada-002** | 1536 | N/A | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Proprietary |
| **LFM-2-Embed** | 512 | ~4GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Commercial |

### Chat Models

| Model | Size | Context | Quality | Speed | License |
|-------|------|---------|---------|-------|---------|
| **Llama-3.2-3B** | 2GB | 4K | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Llama 2 |
| **Phi-3-mini** | 2GB | 4K | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | MIT |
| **LFM-2-Chat** | ~4GB | 32K | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Commercial |

### Vision Models

| Model | Size | Quality | Speed | Multimodal | License |
|-------|------|---------|-------|------------|---------|
| **llava-phi-3-mini** | 2.5GB | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Yes | MIT |
| **LFM-2-VL** | ~7GB | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Yes | Commercial |

## ⚙️ Configuration Updates

### Update Model Manager Config

```go
func DefaultModelManagerConfig() *ModelManagerConfig {
    return &ModelManagerConfig{
        EmbeddingModelPath: "models/lfm2-embed-7b.gguf", // Update path
        ChatModelPath:      "models/lfm2-chat-7b.gguf",
        VisionModelPath:    "models/lfm2-vl-7b.gguf",

        // Adjust for actual model capabilities
        EmbeddingDims: 768,  // nomic-embed-text-v1.5
        ContextSize:   4096, // Llama-3.2-3B
        GPULayers:    -1,
        Threads:      runtime.NumCPU(),
    }
}
```

### Update Provider Configs

```go
func NewLFM2EmbedProvider(modelPath string, embeddingDims int) (*LFM2EmbedProvider, error) {
    config := DefaultGGUFConfig(modelPath, ModelTypeEmbedding)
    // Adjust config for nomic-embed-text-v1.5
    config.ContextSize = 2048  // nomic works well with smaller context
    config.MaxTokens = 1       // embeddings don't need generation
    // ... rest of implementation
}
```

## 🧪 Testing with Real Models

### Validate Model Loading

```bash
# Test embedded models
go test ./vvfs/generation/embedded/... -v

# Test full AI service
go test ./vvfs/generation/ai/... -v
```

### Performance Benchmarking

```bash
# Run benchmarks
go test -bench=. ./vvfs/generation/models/... -benchmem

# Profile memory usage
go test -memprofile=mem.prof ./vvfs/generation/models/...
go tool pprof mem.prof
```

## 📋 Production Checklist

### Pre-Deployment

- [ ] **License Verification**: Confirm redistribution rights for chosen models
- [ ] **Model Validation**: Test all models load and function correctly
- [ ] **Performance Testing**: Benchmark latency and memory usage
- [ ] **Binary Size Check**: Verify final binary size is acceptable
- [ ] **Security Audit**: Scan models for vulnerabilities

### Deployment Configuration

- [ ] **Model Paths**: Update configuration files with correct paths
- [ ] **Hardware Requirements**: Document minimum RAM/GPU requirements
- [ ] **Fallback Strategy**: Configure model cascade behavior
- [ ] **Monitoring Setup**: Configure health checks and metrics

### Post-Deployment

- [ ] **Model Health Monitoring**: Set up alerts for model failures
- [ ] **Performance Monitoring**: Track inference latency and throughput
- [ ] **Update Strategy**: Plan for model updates and A/B testing
- [ ] **Backup Models**: Maintain fallback model versions

## 🚨 Important Legal Notes

### LFM-2 Commercial Licensing

- **Liquid.ai models require explicit permission** for redistribution
- Contact <sales@liquid.ai> for commercial licensing
- Consider API integration as alternative to embedding

### Open-Source Compliance

- Verify license compatibility with your application
- Include license notices in distributed binaries
- Credit original model authors appropriately

### Distribution Rights

- **Embedded models become part of your binary**
- Distribution requires compliance with all model licenses
- Consider user opt-in for model downloads

## 🎯 Recommended Approach

**Given LFM-2 license restrictions, use open-source models for immediate development:**

```bash
# Use open-source models (recommended for unrestricted commercial use)
./scripts/download_open_source_models.sh

# Test thoroughly
go test ./vvfs/generation/... -v

# For LFM-2 models (only if revenue < $10M OR qualified non-profit)
# Contact legal@liquid.ai for license verification
# Then use: ./scripts/download_lfm2_models.sh
```

**Why Open-Source First:**

- ✅ **No revenue restrictions** - fully commercial-friendly
- ✅ **Permissive licenses** - Apache 2.0, MIT, etc.
- ✅ **Production-ready** - tested and optimized models
- ✅ **Cost-effective** - no licensing fees

**LFM-2 Only If:**

- Organization revenue < $10M annually, OR
- Qualified 501(c)(3) non-profit for research purposes

This approach provides immediate production-ready functionality without license complications.

---

**Next Steps**: Run the download script and test with real models to validate the complete pipeline.
