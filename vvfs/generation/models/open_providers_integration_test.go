//go:build llama

package models

import (
	"testing"
)

// TestOpenEmbedProvider_Integration tests actual model loading and inference
func TestOpenEmbedProvider_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require actual GGUF files and CGO setup
	t.Skip("Integration test requires CGO and real model files")
}

// TestOpenChatProvider_Integration tests chat model inference
func TestOpenChatProvider_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require actual GGUF files and CGO setup
	t.Skip("Integration test requires CGO and real model files")
}
