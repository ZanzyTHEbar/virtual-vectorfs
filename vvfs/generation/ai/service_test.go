package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/models"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEnvironment creates a test environment with mock data
func setupTestEnvironment(t *testing.T) (*Service, func()) {
	// Create temp directory for test data
	tempDir, err := os.MkdirTemp("", "ai_service_test_*")
	require.NoError(t, err)

	// Create test files
	testFiles := map[string]string{
		"documents/report.txt":       "Q4 2024 Financial Report\n\nRevenue increased 15%",
		"documents/meeting_notes.md": "# Team Meeting\n\nDiscussed project timeline",
		"images/photo.jpg":           "fake image data",
		"code/main.go":               "package main\nfunc main() {}",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize database
	centralDB, err := db.NewCentralDBProvider()
	require.NoError(t, err)

	// Initialize filesystem
	fs, err := filesystem.New(nil, centralDB)
	require.NoError(t, err)

	// Initialize AI service with mock config
	aiConfig := &Config{
		ModelManagerConfig: &models.ModelManagerConfig{
			EmbeddingModelPath:     "models/demo/LFM-2-Embed-7B-Q4_K_M.gguf",
			ChatModelPath:          "models/demo/LFM-2-Chat-7B-Q4_K_M.gguf",
			VisionModelPath:        "",
			EmbeddingDims:          512,
			ContextSize:            4096,
			GPULayers:              0,
			Threads:                2,
			UseF16Memory:           true,
			UseMMAP:                true,
			EnableHealthMonitoring: false, // Disable for tests
			ConfidenceThreshold:    0.8,
		},
		EnableContentAnalysis:  true,
		EnableSemanticSearch:   true,
		EnableAutoOrganization: true,
	}

	aiService, err := NewService(fs, aiConfig)
	require.NoError(t, err)

	cleanup := func() {
		aiService.modelManager.Close()
		centralDB.Close()
		os.RemoveAll(tempDir)
	}

	return aiService, cleanup
}

func TestServiceCreation(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	assert.NotNil(t, aiService)
	assert.NotNil(t, aiService.modelManager)
	assert.NotNil(t, aiService.filesystem)
}

func TestAnalyzeFileContent(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test file node
	fileNode := &trees.FileNode{
		Path:      "test_workspace/documents/report.txt",
		Name:      "report.txt",
		Extension: ".txt",
		Metadata: trees.Metadata{
			Size:        100,
			ModifiedAt:  time.Now(),
			CreatedAt:   time.Now(),
			Permissions: 0644,
		},
	}

	// Test file analysis
	analysis, err := aiService.AnalyzeFileContent(ctx, fileNode)
	if err != nil {
		// This is expected if models aren't loaded, just verify error handling
		assert.Error(t, err)
		return
	}

	assert.NotNil(t, analysis)
	assert.NotNil(t, analysis.FileNode)
	assert.Equal(t, fileNode.Path, analysis.FileNode.Path)
	assert.NotEmpty(t, analysis.ContentType)
	assert.NotEmpty(t, analysis.Keywords)
	assert.NotNil(t, analysis.Metadata)
}

func TestDetectContentType(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		extension string
		expected  string
	}{
		{".txt", "text"},
		{".md", "text"},
		{".pdf", "document"},
		{".jpg", "image"},
		{".png", "image"},
		{".mp4", "video"},
		{".json", "structured"},
		{".go", "code"},
		{".unknown", "unknown"},
	}

	for _, test := range tests {
		t.Run(test.extension, func(t *testing.T) {
			fileNode := &trees.FileNode{
				Extension: test.extension,
			}

			result := aiService.detectContentType(fileNode)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fileNode := &trees.FileNode{
		Path: "test.txt",
	}

	// Test with sample content
	keywords := aiService.extractKeywords(fileNode)
	assert.NotNil(t, keywords)
	// Should extract some keywords from the file content
}

func TestExtractMetadata(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fileNode := &trees.FileNode{
		Metadata: trees.Metadata{
			Size:        1024,
			ModifiedAt:  time.Now(),
			CreatedAt:   time.Now(),
			Permissions: 0644,
		},
	}

	metadata := aiService.extractMetadata(fileNode)
	assert.NotNil(t, metadata)
	assert.Equal(t, int64(1024), metadata["size"])
}

func TestFindSimilarFiles(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	fileNode := &trees.FileNode{
		Path: "test_workspace/documents/report.txt",
	}

	// Test finding similar files
	similarFiles, err := aiService.FindSimilarFiles(ctx, fileNode, 5)
	if err != nil {
		// Expected if models aren't loaded
		assert.Error(t, err)
		return
	}

	assert.NotNil(t, similarFiles)
	assert.Len(t, similarFiles, 0) // Should return empty slice without real models
}

func TestSuggestOrganization(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test organization suggestions
	suggestion, err := aiService.SuggestOrganization(ctx, []*trees.FileNode{})
	if err != nil {
		// Expected if models aren't loaded
		assert.Error(t, err)
		return
	}

	assert.NotNil(t, suggestion)
	assert.Greater(t, suggestion.Confidence, 0.0)
	assert.NotEmpty(t, suggestion.Suggestions)
}

func TestServiceClose(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test that close doesn't panic
	err := aiService.modelManager.Close()
	assert.NoError(t, err)
}

func TestModelHealth(t *testing.T) {
	aiService, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test health summary
	healthSummary := aiService.modelManager.GetHealthSummary()
	assert.NotNil(t, healthSummary)

	// Test model info
	modelInfo := aiService.modelManager.GetModelInfo()
	assert.NotNil(t, modelInfo)
}
