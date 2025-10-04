# ONNX Export Guide for LFM2-350M

This guide explains how to export LiquidAI's LFM2-350M model to ONNX format for use with Hugot's TextGeneration pipeline.

## Overview

LFM2-350M is a 350M parameter language model optimized for on-device inference. While it can run directly in PyTorch, exporting to ONNX format enables:

- **Accelerated Inference**: Use ONNX Runtime with hardware acceleration
- **Cross-Platform**: Deploy on CPU, GPU, mobile, and edge devices
- **Hugot Compatibility**: Native integration with Hugot's TextGeneration pipeline
- **Performance**: Optimized execution graphs for production use

## Prerequisites

### Python Environment

```bash
# Create virtual environment
python -m venv lfm2-export
source lfm2-export/bin/activate  # Linux/Mac
# or: lfm2-export\Scripts\activate  # Windows

# Install dependencies
pip install transformers optimum[onnxruntime] torch onnxruntime
```

### System Requirements

- **RAM**: 8GB minimum (16GB recommended)
- **Disk**: 2GB free space for exported model
- **GPU**: Optional but recommended for faster export

## Export Process

### Method 1: Automated Script (Recommended)

Use the provided export script:

```bash
# Clone or navigate to your project
cd /path/to/virtual-vectorfs

# Make script executable
chmod +x scripts/export_lfm2_onnx.py

# Run export
python scripts/export_lfm2_onnx.py --output_dir ./models/lfm2-350m-onnx
```

**Command Options:**
- `--model_id`: HuggingFace model ID (default: LiquidAI/LFM2-350M)
- `--output_dir`: Output directory (default: ./models/lfm2-350m-onnx)
- `--verify`: Test the exported model after export

### Method 2: Manual Export

For custom export options:

```python
from transformers import AutoTokenizer, AutoModelForCausalLM
from optimum.onnxruntime import ORTModelForCausalLM
import torch

# Load components
model_id = "LiquidAI/LFM2-350M"
tokenizer = AutoTokenizer.from_pretrained(model_id)
model = AutoModelForCausalLM.from_pretrained(model_id, torch_dtype=torch.float16)

# Export to ONNX
onnx_model = ORTModelForCausalLM.from_pretrained(
    model_id,
    export=True,
    provider="CUDAExecutionProvider" if torch.cuda.is_available() else "CPUExecutionProvider"
)

# Save
output_dir = "./models/lfm2-350m-onnx"
tokenizer.save_pretrained(output_dir)
onnx_model.save_pretrained(output_dir)
```

## Exported Model Structure

After export, your directory will contain:

```
lfm2-350m-onnx/
├── config.json          # Model configuration
├── generation_config.json  # Generation settings
├── model.onnx          # ONNX model (main file)
├── special_tokens_map.json
├── tokenizer.json      # Tokenizer configuration
├── tokenizer_config.json
└── vocab.json          # Vocabulary
```

**File Sizes (Approximate):**
- `model.onnx`: ~700MB (FP16 quantized)
- `tokenizer.json`: ~2MB
- Total: ~750MB

## Hugot Integration

### Basic Usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation"
)

func main() {
    // Initialize generator
    generator := generation.NewHugotGenerator("./models/lfm2-350m-onnx")

    // Create request
    req := generation.GetDefaultGenerationRequest(
        "./models/lfm2-350m-onnx",
        []generation.Message{
            {Role: "user", Content: "Explain quantum computing in simple terms"},
        },
    )

    // Generate
    ctx := context.Background()
    response, err := generator.Generate(ctx, req)
    if err != nil {
        panic(err)
    }

    fmt.Println(response.Text)
}
```

### Configuration

```yaml
# config.yaml
llm:
  provider: "hugot"
  model_path: "./models/lfm2-350m-onnx"
  max_new_tokens: 512
  temperature: 0.3
  top_p: 0.9
  min_p: 0.15
  repetition_penalty: 1.05

onnx:
  backend: "ort"
  ep: "cpu"  # or "cuda", "tensorrt", etc.
  inter_op_threads: 1
  intra_op_threads: 4
```

## Performance Optimization

### Execution Providers

Choose the right execution provider for your hardware:

```yaml
# CPU (default)
onnx:
  ep: "cpu"

# NVIDIA GPU
onnx:
  ep: "cuda"

# TensorRT (NVIDIA)
onnx:
  ep: "tensorrt"

# DirectML (Windows)
onnx:
  ep: "dml"

# CoreML (macOS)
onnx:
  ep: "coreml"
```

### Memory Optimization

```yaml
# Optimize for memory efficiency
onnx:
  inter_op_threads: 1
  intra_op_threads: 1  # Match CPU cores
  cpu_mem_arena: false
  mem_pattern: false
```

### Quantization

For smaller model size and faster inference:

```bash
# Export with quantization (reduces size by ~50%)
python scripts/export_lfm2_onnx.py --quantize int8 --output_dir ./models/lfm2-350m-int8
```

## Chat Templates

LFM2 uses ChatML format. Templates are automatically applied:

```go
// Automatic template selection
template := generation.GetChatTemplate("lfm2") // Uses LFM2 template

// Or use with Hugot directly
messages := [][]pipelines.Message{
    {
        {Role: "user", Content: "Hello, how are you?"},
    },
}
result := pipeline.RunWithTemplate(messages)
```

**Chat Format:**
```
<|im_start|>user
Hello, how are you?
<|im_end|>
<|im_start|>assistant
```

## Troubleshooting

### Common Issues

**1. Out of Memory During Export**
```bash
# Use smaller batch size or CPU-only
export CUDA_VISIBLE_DEVICES=""
python scripts/export_lfm2_onnx.py
```

**2. ONNX Export Fails**
```python
# Try with different provider
onnx_model = ORTModelForCausalLM.from_pretrained(
    model_id,
    export=True,
    provider="CPUExecutionProvider"  # Force CPU
)
```

**3. Model Verification Fails**
- Check tokenizer configuration
- Ensure model was exported correctly
- Try different generation parameters

### Performance Tuning

**GPU Issues:**
```bash
# Check GPU memory
nvidia-smi

# Set CUDA device
export CUDA_VISIBLE_DEVICES=0
```

**CPU Optimization:**
```yaml
onnx:
  inter_op_threads: 1
  intra_op_threads: 4  # Match physical cores
```

## Model Specifications

| Model     | Parameters | Context | Size (ONNX) | Performance     |
| --------- | ---------- | ------- | ----------- | --------------- |
| LFM2-350M | 350M       | 32K     | ~750MB      | 15-25 tok/s CPU |

### Generation Parameters

Based on LFM2 recommendations:

- **Temperature**: 0.3 (creativity vs coherence balance)
- **Min P**: 0.15 (quality filtering)
- **Repetition Penalty**: 1.05 (reduce repetition)
- **Max Tokens**: 512 (response length)

## Deployment

### Production Setup

1. **Export Model**: Use the script to export ONNX model
2. **Optimize**: Choose appropriate execution provider
3. **Test**: Verify generation quality and performance
4. **Deploy**: Integrate with Hugot in your application
5. **Monitor**: Track latency, throughput, and errors

### Cloud Deployment

For cloud environments:

```yaml
# AWS/GCP/Azure optimized
onnx:
  ep: "cuda"
  inter_op_threads: 1
  intra_op_threads: 2
```

### Edge Deployment

For mobile/edge devices:

```yaml
# Mobile optimized
onnx:
  ep: "coreml"  # iOS
  # or
  ep: "nnapi"   # Android
```

## Next Steps

1. **Export**: Run the export script
2. **Test**: Verify model works with Hugot
3. **Optimize**: Tune execution provider for your hardware
4. **Deploy**: Integrate into your application
5. **Monitor**: Track performance and adjust as needed

For issues or questions, check the [LFM2 repository](https://huggingface.co/LiquidAI/LFM2-350M) or [Hugot documentation](https://github.com/knights-analytics/hugot).
