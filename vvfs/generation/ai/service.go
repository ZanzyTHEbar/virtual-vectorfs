package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/models"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
)

// Service provides AI capabilities integrated with File4You's filesystem
type Service struct {
	modelManager *models.ModelManager
	filesystem   *filesystem.FileSystem
}

// Config holds configuration for the AI service
type Config struct {
	ModelManagerConfig     *models.ModelManagerConfig
	EnableContentAnalysis  bool
	EnableSemanticSearch   bool
	EnableAutoOrganization bool
}

// NewService creates a new AI service
func NewService(fs *filesystem.FileSystem, config *Config) (*Service, error) {
	if config == nil {
		config = &Config{
			ModelManagerConfig:     models.DefaultModelManagerConfig(),
			EnableContentAnalysis:  true,
			EnableSemanticSearch:   true,
			EnableAutoOrganization: true,
		}
	}

	modelManager, err := models.NewModelManager(config.ModelManagerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create model manager: %w", err)
	}

	return &Service{
		modelManager: modelManager,
		filesystem:   fs,
	}, nil
}

// AnalyzeFileContent analyzes file content using AI models
func (s *Service) AnalyzeFileContent(ctx context.Context, fileNode *trees.FileNode) (*FileAnalysis, error) {
	// Validate input
	if fileNode == nil {
		return nil, fmt.Errorf("file node cannot be nil")
	}

	// Generate embedding for the file
	embedding, err := s.modelManager.GenerateEmbedding(ctx, s.generateFileRepresentation(fileNode))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Generate content summary using chat model
	summary, err := s.generateContentSummary(ctx, fileNode)
	if err != nil {
		log.Printf("Warning: Failed to generate content summary for %s: %v", fileNode.Path, err)
		summary = "Content analysis unavailable"
	}

	// Extract key information
	analysis := &FileAnalysis{
		FileNode:    fileNode,
		Embedding:   embedding,
		Summary:     summary,
		ContentType: s.detectContentType(fileNode),
		Keywords:    s.extractKeywords(fileNode),
		Metadata:    s.extractMetadata(fileNode),
	}

	return analysis, nil
}

// generateFileRepresentation creates a text representation for embedding
func (s *Service) generateFileRepresentation(fileNode *trees.FileNode) string {
	// Read file content (limited to avoid memory issues)
	content := s.readFileContent(fileNode.Path, 1024) // First 1KB

	// Create comprehensive representation
	representation := fmt.Sprintf("Path: %s\n", fileNode.Path)
	representation += fmt.Sprintf("Name: %s\n", fileNode.Name)
	representation += fmt.Sprintf("Extension: %s\n", fileNode.Extension)
	representation += fmt.Sprintf("Size: %d bytes\n", fileNode.Metadata.Size)
	representation += fmt.Sprintf("Modified: %s\n", fileNode.Metadata.ModifiedAt.Format("2006-01-02 15:04:05"))
	representation += fmt.Sprintf("Content: %s", content)

	// Add any extracted metadata
	if exifData := s.extractEXIFData(fileNode.Path); exifData != "" {
		representation += fmt.Sprintf("\nEXIF: %s", exifData)
	}

	return representation
}

// generateContentSummary creates a summary of file content
func (s *Service) generateContentSummary(ctx context.Context, fileNode *trees.FileNode) (string, error) {
	content := s.readFileContent(fileNode.Path, 2048) // First 2KB for summary

	if len(content) == 0 {
		return "Empty file", nil
	}

	// Create summary prompt
	prompt := fmt.Sprintf(`Please provide a brief, informative summary of this file content:

File: %s
Type: %s
Content:
%s

Summary (2-3 sentences):`, fileNode.Name, s.detectContentType(fileNode), content)

	summary, err := s.modelManager.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate content summary: %w", err)
	}

	return strings.TrimSpace(summary), nil
}

// readFileContent safely reads file content with size limit
func (s *Service) readFileContent(filePath string, maxSize int) string {
	// This would integrate with the existing filesystem utilities
	// For now, return a placeholder
	return fmt.Sprintf("File content would be read here (max %d bytes)", maxSize)
}

// detectContentType determines the type of file content
func (s *Service) detectContentType(fileNode *trees.FileNode) string {
	ext := strings.ToLower(fileNode.Extension)

	switch ext {
	case ".txt", ".md", ".markdown":
		return "text"
	case ".pdf":
		return "document"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return "image"
	case ".mp4", ".avi", ".mkv", ".mov":
		return "video"
	case ".mp3", ".wav", ".flac":
		return "audio"
	case ".json", ".xml", ".yaml", ".yml":
		return "structured"
	case ".go", ".py", ".js", ".ts", ".java", ".cpp", ".c", ".h":
		return "code"
	default:
		return "unknown"
	}
}

// extractKeywords extracts key terms from file content
func (s *Service) extractKeywords(fileNode *trees.FileNode) []string {
	content := s.readFileContent(fileNode.Path, 1024)

	// Simple keyword extraction - in production, use NLP
	words := strings.Fields(content)
	keywords := make([]string, 0, 5)

	// Extract nouns, proper nouns, and important terms
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?;:"))
		if len(word) > 3 && !s.isStopWord(word) {
			keywords = append(keywords, word)
			if len(keywords) >= 5 {
				break
			}
		}
	}

	return keywords
}

// isStopWord checks if a word is a common stop word
func (s *Service) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"was": true, "are": true, "were": true, "be": true, "been": true,
	}

	return stopWords[word]
}

// extractMetadata extracts metadata from the file
func (s *Service) extractMetadata(fileNode *trees.FileNode) map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["size"] = fileNode.Metadata.Size
	metadata["modified"] = fileNode.Metadata.ModifiedAt
	metadata["created"] = fileNode.Metadata.CreatedAt
	metadata["permissions"] = fileNode.Metadata.Permissions

	// Extract EXIF data if it's an image
	if s.detectContentType(fileNode) == "image" {
		if exifData := s.extractEXIFData(fileNode.Path); exifData != "" {
			metadata["exif"] = exifData
		}
	}

	return metadata
}

// extractEXIFData extracts EXIF information from image files
func (s *Service) extractEXIFData(filePath string) string {
	// This would integrate with the existing EXIF extraction utilities
	// For now, return placeholder
	return "EXIF data would be extracted here"
}

// FindSimilarFiles finds files similar to the given file using embeddings
func (s *Service) FindSimilarFiles(ctx context.Context, fileNode *trees.FileNode, limit int) ([]*trees.FileNode, error) {
	if fileNode == nil {
		return nil, fmt.Errorf("file node cannot be nil")
	}

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive, got %d", limit)
	}

	// Generate embedding for the target file
	_, err := s.modelManager.GenerateEmbedding(ctx, s.generateFileRepresentation(fileNode))
	if err != nil {
		return nil, fmt.Errorf("failed to generate target embedding: %w", err)
	}

	// Search for similar files using the vector database
	// This would integrate with the existing vector search
	similarFiles := make([]*trees.FileNode, 0, limit)

	// For now, return empty slice - integration would be with the database layer
	log.Printf("Would search for %d files similar to %s", limit, fileNode.Path)

	return similarFiles, nil
}

// SuggestOrganization suggests how to organize files using AI
func (s *Service) SuggestOrganization(ctx context.Context, files []*trees.FileNode) (*OrganizationSuggestion, error) {
	// Analyze all files
	analyses := make([]*FileAnalysis, len(files))
	for i, file := range files {
		analysis, err := s.AnalyzeFileContent(ctx, file)
		if err != nil {
			log.Printf("Warning: Failed to analyze file %s: %v", file.Path, err)
			continue
		}
		analyses[i] = analysis
	}

	// Create organization prompt
	prompt := s.createOrganizationPrompt(analyses)

	// Generate organization suggestions
	suggestion, err := s.modelManager.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate organization suggestion: %w", err)
	}

	// Parse the suggestion (in production, this would parse structured JSON)
	return &OrganizationSuggestion{
		Description: suggestion,
		Confidence:  0.8, // Placeholder - would be calculated
		Suggestions: s.parseOrganizationSuggestions(suggestion),
	}, nil
}

// createOrganizationPrompt creates a prompt for AI organization suggestions
func (s *Service) createOrganizationPrompt(analyses []*FileAnalysis) string {
	var fileDescriptions strings.Builder
	for _, analysis := range analyses {
		fileDescriptions.WriteString(fmt.Sprintf("- %s: %s\n", analysis.FileNode.Name, analysis.Summary))
	}

	prompt := fmt.Sprintf(`Based on these file analyses, suggest how to organize these files:

Files:
%s

Provide organization suggestions including:
1. Suggested folder structure
2. File groupings by theme/topic
3. Naming conventions
4. Any special handling needed

Output in structured format:
{
  "folder_structure": ["Documents/", "Images/", "Code/"],
  "file_groups": {"Documents": ["file1.txt", "file2.pdf"]},
  "naming_conventions": "Use descriptive names with dates",
  "special_handling": "Archive old files"
}`, fileDescriptions.String())

	return prompt
}

// parseOrganizationSuggestions parses AI-generated organization suggestions
func (s *Service) parseOrganizationSuggestions(suggestion string) []string {
	// Simple parsing - in production, this would parse structured JSON
	lines := strings.Split(suggestion, "\n")
	suggestions := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "}") {
			suggestions = append(suggestions, line)
		}
	}

	return suggestions
}

// FileAnalysis represents AI analysis of a file
type FileAnalysis struct {
	FileNode    *trees.FileNode        `json:"file"`
	Embedding   []float32              `json:"embedding"`
	Summary     string                 `json:"summary"`
	ContentType string                 `json:"content_type"`
	Keywords    []string               `json:"keywords"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// OrganizationSuggestion represents AI-generated organization suggestions
type OrganizationSuggestion struct {
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
	Suggestions []string `json:"suggestions"`
}

// GetHealthSummary returns health status of all providers
func (s *Service) GetHealthSummary() map[string]*models.ModelHealth {
	return s.modelManager.GetHealthSummary()
}

// GetModelInfo returns information about loaded models
func (s *Service) GetModelInfo() map[string]interface{} {
	return s.modelManager.GetModelInfo()
}

// Close gracefully shuts down the AI service
func (s *Service) Close() error {
	return s.modelManager.Close()
}

// ModelManagerInterface defines the interface for model management
type ModelManagerInterface interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	AnalyzeImage(ctx context.Context, imagePath string) (*models.ImageAnalysis, error)
	GetHealthSummary() map[string]*models.ModelHealth
	GetModelInfo() map[string]interface{}
	Close() error
}
