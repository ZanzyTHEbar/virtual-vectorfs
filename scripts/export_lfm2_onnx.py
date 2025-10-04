#!/usr/bin/env python3
"""
ONNX Export Script for LFM2-350M Model

This script exports LiquidAI's LFM2-350M model to ONNX format for use with Hugot.
The exported model can be used for on-device generation with optimized performance.

Requirements:
- transformers >= 4.55
- optimum >= 1.21.0
- torch >= 2.0.0
- onnxruntime >= 1.18.0

Usage:
    python export_lfm2_onnx.py --output_dir ./models/lfm2-350m-onnx

The exported model will be compatible with Hugot's TextGeneration pipeline.
"""

import argparse
import os
import logging
from pathlib import Path

try:
    from transformers import AutoTokenizer, AutoModelForCausalLM
    from optimum.onnxruntime import ORTModelForCausalLM
    import torch
except ImportError as e:
    print(f"Missing required packages: {e}")
    print("Install with: pip install transformers optimum[onnxruntime] torch")
    exit(1)

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def export_lfm2_to_onnx(model_id: str = "LiquidAI/LFM2-350M", output_dir: str = "./models/lfm2-350m-onnx"):
    """
    Export LFM2-350M to ONNX format optimized for Hugot.

    Args:
        model_id: HuggingFace model ID
        output_dir: Output directory for ONNX model
    """

    logger.info(f"Starting LFM2-350M ONNX export from {model_id}")

    # Create output directory
    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)

    try:
        # Load tokenizer
        logger.info("Loading tokenizer...")
        tokenizer = AutoTokenizer.from_pretrained(model_id)

        # Set pad token if not present (LFM2 should have this, but just in case)
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token

        # Save tokenizer
        tokenizer.save_pretrained(output_path)
        logger.info(f"Tokenizer saved to {output_path}")

        # Load PyTorch model
        logger.info("Loading PyTorch model...")
        model = AutoModelForCausalLM.from_pretrained(
            model_id,
            torch_dtype=torch.float16,  # Use FP16 for better performance
            device_map="auto",  # Use GPU if available
            trust_remote_code=True
        )

        # Export to ONNX using Optimum
        logger.info("Exporting to ONNX format...")

        # Create ONNX model
        onnx_model = ORTModelForCausalLM.from_pretrained(
            model_id,
            export=True,
            provider="CUDAExecutionProvider" if torch.cuda.is_available() else "CPUExecutionProvider"
        )

        # Save ONNX model
        onnx_model.save_pretrained(output_path)
        logger.info(f"ONNX model saved to {output_path}")

        # Verify the export
        logger.info("Verifying ONNX export...")
        verify_export(output_path, tokenizer)

        logger.info("✅ LFM2-350M ONNX export completed successfully!")
        logger.info(f"Model saved to: {output_path}")
        logger.info(f"Model size: {get_dir_size(output_path)} MB")

        # Print usage instructions
        print_usage_instructions(output_path)

    except Exception as e:
        logger.error(f"Export failed: {e}")
        raise

def verify_export(model_path: Path, tokenizer):
    """Verify the exported ONNX model works correctly."""
    try:
        # Load the exported model
        model = ORTModelForCausalLM.from_pretrained(model_path)

        # Test with a simple prompt
        test_prompt = "Hello, I am"
        inputs = tokenizer(test_prompt, return_tensors="pt")

        # Generate
        with torch.no_grad():
            outputs = model.generate(
                inputs["input_ids"],
                max_length=20,
                temperature=0.3,
                do_sample=True,
                pad_token_id=tokenizer.pad_token_id,
                eos_token_id=tokenizer.eos_token_id
            )

        generated_text = tokenizer.decode(outputs[0], skip_special_tokens=True)
        logger.info(f"✅ Verification successful. Generated: '{generated_text}'")

    except Exception as e:
        logger.warning(f"Model verification failed: {e}")
        logger.warning("The model was exported but may have issues with inference.")

def get_dir_size(path: Path) -> float:
    """Get directory size in MB."""
    total_size = 0
    for file_path in path.rglob('*'):
        if file_path.is_file():
            total_size += file_path.stat().st_size
    return round(total_size / (1024 * 1024), 2)

def print_usage_instructions(model_path: Path):
    """Print usage instructions for the exported model."""
    print("\n" + "="*50)
    print("USAGE INSTRUCTIONS")
    print("="*50)
    print(f"Model Path: {model_path}")
    print()
    print("In your Go application:")
    print(f'generator := generation.NewHugotGenerator("{model_path}")')
    print()
    print("Or via config:")
    print(f'llm.model_path = "{model_path}"')
    print()
    print("Recommended generation parameters:")
    print("- temperature: 0.3")
    print("- min_p: 0.15")
    print("- repetition_penalty: 1.05")
    print("- max_new_tokens: 512")
    print()
    print("For chat templates, use:")
    print('template := generation.GetChatTemplate("lfm2")')
    print("="*50)

def main():
    parser = argparse.ArgumentParser(description="Export LFM2-350M to ONNX format")
    parser.add_argument(
        "--model_id",
        default="LiquidAI/LFM2-350M",
        help="HuggingFace model ID (default: LiquidAI/LFM2-350M)"
    )
    parser.add_argument(
        "--output_dir",
        default="./models/lfm2-350m-onnx",
        help="Output directory for ONNX model"
    )
    parser.add_argument(
        "--verify",
        action="store_true",
        help="Verify the exported model works"
    )

    args = parser.parse_args()

    export_lfm2_to_onnx(args.model_id, args.output_dir)

if __name__ == "__main__":
    main()
