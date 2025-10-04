//go:build !llama

package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewOpenEmbedProvider tests provider creation and configuration
func TestNewOpenEmbedProvider(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-embed.gguf")

	// Create a minimal GGUF file (just the header)
	ggufData := []byte("GGUF" + string(make([]byte, 100))) // Minimal header + padding
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenEmbedProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenEmbedProvider: %v", err)
	}
	defer provider.Close()

	// Check configuration
	config := provider.GetConfig()
	if config.ModelType != ModelTypeEmbedding {
		t.Errorf("Expected ModelTypeEmbedding, got %v", config.ModelType)
	}
	if config.Temperature != 0.0 {
		t.Errorf("Expected temperature 0.0, got %f", config.Temperature)
	}
	if config.MaxTokens != 1 {
		t.Errorf("Expected MaxTokens 1, got %d", config.MaxTokens)
	}
}

// TestOpenEmbedProvider_EmbedText tests embedding generation (mocked)
func TestOpenEmbedProvider_EmbedText(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-embed.gguf")

	// Create a minimal GGUF file
	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenEmbedProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenEmbedProvider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test embedding generation (will fail due to no real model, but tests the flow)
	_, err = provider.EmbedText(ctx, "test text")
	// We expect this to fail because the model isn't real, but the provider should handle it gracefully
	if err == nil {
		t.Error("Expected error for non-functional model, got nil")
	}
}

// TestOpenEmbedProvider_SetMatryoshkaDims tests dimension adjustment
func TestOpenEmbedProvider_SetMatryoshkaDims(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-embed.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenEmbedProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenEmbedProvider: %v", err)
	}
	defer provider.Close()

	// Test dimension adjustment
	provider.SetMatryoshkaDims(512)

	// Test adjustToDims logic
	tests := []struct {
		input    []float32
		target   int
		expected int
	}{
		{[]float32{1, 2, 3}, 3, 3},
		{[]float32{1, 2, 3, 4}, 2, 2}, // Truncate
		{[]float32{1, 2}, 4, 4},       // Pad
	}

	for _, test := range tests {
		result := provider.adjustToDims(test.input, test.target)
		if len(result) != test.expected {
			t.Errorf("adjustToDims(%v, %d) = %d, expected %d", test.input, test.target, len(result), test.expected)
		}
	}
}

// TestNewOpenChatProvider tests chat provider creation
func TestNewOpenChatProvider(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-chat.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenChatProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenChatProvider: %v", err)
	}
	defer provider.Close()

	// Check configuration
	config := provider.GetConfig()
	if config.ModelType != ModelTypeChat {
		t.Errorf("Expected ModelTypeChat, got %v", config.ModelType)
	}
	if config.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", config.Temperature)
	}
}

// TestOpenChatProvider_GenerateText tests text generation (mocked)
func TestOpenChatProvider_GenerateText(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-chat.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenChatProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenChatProvider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test text generation (will fail due to no real model, but tests the flow)
	_, err = provider.GenerateText(ctx, "Hello")
	// We expect this to fail because the model isn't real, but the provider should handle it gracefully
	if err == nil {
		t.Error("Expected error for non-functional model, got nil")
	}
}

// TestNewOpenVisionProvider tests vision provider creation
func TestNewOpenVisionProvider(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-vision.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenVisionProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenVisionProvider: %v", err)
	}
	defer provider.Close()

	// Check configuration
	config := provider.GetConfig()
	if config.ModelType != ModelTypeVision {
		t.Errorf("Expected ModelTypeVision, got %v", config.ModelType)
	}
	if config.ContextSize != 8192 {
		t.Errorf("Expected ContextSize 8192, got %d", config.ContextSize)
	}
}

// TestOpenVisionProvider_DescribeImage tests image description (placeholder)
func TestOpenVisionProvider_DescribeImage(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-vision.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenVisionProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenVisionProvider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()
	imageData := []byte("fake image data")

	// Test image description (should return not implemented error)
	_, err = provider.DescribeImage(ctx, imageData)
	if err == nil {
		t.Error("Expected 'not yet implemented' error, got nil")
	}
}

// TestOpenVisionProvider_GetSupportedImageFormats tests supported formats
func TestOpenVisionProvider_GetSupportedImageFormats(t *testing.T) {
	// Create a temporary GGUF file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-vision.gguf")

	ggufData := []byte("GGUF" + string(make([]byte, 100)))
	if err := os.WriteFile(modelPath, ggufData, 0o644); err != nil {
		t.Fatalf("Failed to create test GGUF file: %v", err)
	}

	provider, err := NewOpenVisionProvider(modelPath)
	if err != nil {
		t.Fatalf("Failed to create OpenVisionProvider: %v", err)
	}
	defer provider.Close()

	formats := provider.GetSupportedImageFormats()
	expected := []string{"jpeg", "png", "webp"}

	if len(formats) != len(expected) {
		t.Errorf("Expected %d formats, got %d", len(expected), len(formats))
	}

	for i, format := range expected {
		if i >= len(formats) || formats[i] != format {
			t.Errorf("Expected format %s at index %d, got %v", format, i, formats)
		}
	}
}
