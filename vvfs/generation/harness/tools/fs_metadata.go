package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// FSMetadataSchema defines the JSON schema for filesystem metadata tool parameters.
const FSMetadataSchema = `{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "The file or directory path to get metadata for"
    },
    "include_contents": {
      "type": "boolean",
      "description": "Include file contents in response (for text files only)",
      "default": false
    },
    "max_content_size": {
      "type": "integer",
      "description": "Maximum content size to include (in bytes)",
      "minimum": 1,
      "maximum": 1048576,
      "default": 8192
    },
    "recursive": {
      "type": "boolean",
      "description": "For directories, include metadata for all nested files and directories",
      "default": false
    }
  },
  "required": ["path"]
}`

// FileMetadata represents metadata for a file or directory.
type FileMetadata struct {
	Path        string         `json:"path"`
	Name        string         `json:"name"`
	Type        string         `json:"type"` // "file" or "directory"
	Size        int64          `json:"size"`
	Permissions string         `json:"permissions"`
	ModifiedAt  time.Time      `json:"modified_at"`
	CreatedAt   time.Time      `json:"created_at,omitempty"`
	IsHidden    bool           `json:"is_hidden"`
	Extension   string         `json:"extension,omitempty"`
	MimeType    string         `json:"mime_type,omitempty"`
	Contents    string         `json:"contents,omitempty"`
	Children    []FileMetadata `json:"children,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// FSMetadataTool implements a tool for retrieving filesystem metadata.
type FSMetadataTool struct {
	basePath string // Optional base path for security
}

// NewFSMetadataTool creates a new filesystem metadata tool.
func NewFSMetadataTool(basePath string) *FSMetadataTool {
	return &FSMetadataTool{
		basePath: basePath,
	}
}

// Name returns the tool name.
func (t *FSMetadataTool) Name() string {
	return "fs_metadata"
}

// Schema returns the JSON schema for tool parameters.
func (t *FSMetadataTool) Schema() []byte {
	return []byte(FSMetadataSchema)
}

// Invoke executes the filesystem metadata tool.
func (t *FSMetadataTool) Invoke(ctx context.Context, args json.RawMessage) (any, error) {
	// Parse arguments
	var params struct {
		Path            string `json:"path"`
		IncludeContents bool   `json:"include_contents"`
		MaxContentSize  int    `json:"max_content_size"`
		Recursive       bool   `json:"recursive"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate required fields
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Validate and set defaults
	if params.MaxContentSize <= 0 {
		params.MaxContentSize = 8192
	}
	if params.MaxContentSize > 1048576 {
		params.MaxContentSize = 1048576
	}

	// Validate path for security (prevent directory traversal)
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path contains directory traversal: %s", params.Path)
	}

	// Resolve path (add base path if configured)
	fullPath := cleanPath
	if t.basePath != "" {
		fullPath = filepath.Join(t.basePath, cleanPath)
	}

	// Get metadata
	metadata, err := t.getMetadata(fullPath, params.IncludeContents, params.MaxContentSize, params.Recursive)
	if err != nil {
		return FileMetadata{
			Path:  params.Path,
			Error: err.Error(),
		}, nil
	}

	return metadata, nil
}

// getMetadata retrieves metadata for a file or directory.
func (t *FSMetadataTool) getMetadata(path string, includeContents bool, maxContentSize int, recursive bool) (FileMetadata, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("failed to stat path: %w", err)
	}

	metadata := FileMetadata{
		Path:        path,
		Name:        info.Name(),
		Size:        info.Size(),
		Permissions: info.Mode().String(),
		ModifiedAt:  info.ModTime(),
		IsHidden:    strings.HasPrefix(info.Name(), "."),
	}

	// Determine type and additional metadata
	if info.IsDir() {
		metadata.Type = "directory"

		if recursive {
			entries, err := os.ReadDir(path)
			if err != nil {
				return metadata, fmt.Errorf("failed to read directory: %w", err)
			}

			children := make([]FileMetadata, 0, len(entries))
			for _, entry := range entries {
				childPath := filepath.Join(path, entry.Name())
				childMetadata, err := t.getMetadata(childPath, includeContents, maxContentSize, false)
				if err != nil {
					childMetadata = FileMetadata{
						Path:  childPath,
						Name:  entry.Name(),
						Error: err.Error(),
					}
				}
				children = append(children, childMetadata)
			}
			metadata.Children = children
		}
	} else {
		metadata.Type = "file"
		metadata.Extension = filepath.Ext(path)

		// Get MIME type (simplified)
		if metadata.Extension != "" {
			metadata.MimeType = t.getMimeType(metadata.Extension)
		}

		// Include contents if requested and it's a text file
		if includeContents && t.isTextFile(path) {
			content, err := t.readFileContent(path, maxContentSize)
			if err != nil {
				metadata.Error = fmt.Sprintf("failed to read contents: %v", err)
			} else {
				metadata.Contents = content
			}
		}
	}

	return metadata, nil
}

// isTextFile checks if a file is likely to contain text.
func (t *FSMetadataTool) isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExtensions := []string{".txt", ".md", ".go", ".py", ".js", ".ts", ".json", ".yaml", ".yml", ".xml", ".html", ".css", ".scss", ".less", ".sh", ".bat", ".ps1"}

	for _, textExt := range textExtensions {
		if ext == textExt {
			return true
		}
	}
	return false
}

// readFileContent reads file content with size limit.
func (t *FSMetadataTool) readFileContent(path string, maxSize int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Check file size first
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	if stat.Size() > int64(maxSize) {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", stat.Size(), maxSize)
	}

	// Read content
	buf := make([]byte, maxSize)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}

	return string(buf[:n]), nil
}

// getMimeType returns a simple MIME type based on file extension.
func (t *FSMetadataTool) getMimeType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".go":
		return "text/plain" // Go files are text
	case ".py":
		return "text/plain" // Python files are text
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

// Ensure FSMetadataTool implements the Tool interface.
var _ ports.Tool = (*FSMetadataTool)(nil)
