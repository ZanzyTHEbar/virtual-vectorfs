package trees

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gonum.org/v1/gonum/spatial/kdtree"
)

type DirectoryTree struct {
	Root       *DirectoryNode
	KDTree     *kdtree.Tree              // KD-Tree structure for fast metadata-based searches
	KDTreeData DirectoryPointCollection  // Holds DirectoryPoint references
	kdManager  *IncrementalKDTreeManager // Manages incremental updates to avoid O(N log N) rebuilds
	multiIndex *MultiIndex               // Multi-index system for optimized queries
	pathIndex  *PatriciaPathIndex        // Fast path lookups
	metrics    *TreeMetrics
	logger     *slog.Logger
	closeOnce  sync.Once
	cleanup    []func() error
	mu         sync.Mutex
	// Cache map[string]*DirectoryNode
}

// WithRoot sets the root directory for the DirectoryTree
func WithRoot(root string) TreeOption {
	return func(dt *DirectoryTree) {
		dt.Root = NewDirectoryNode(root, nil)
	}
}

func NewDirectoryTree(opts ...TreeOption) *DirectoryTree {
	dt := &DirectoryTree{
		metrics: &TreeMetrics{
			OperationCounts: make(map[string]int64),
			LastUpdated:     time.Now(),
		},
		logger:     slog.Default(),
		kdManager:  NewIncrementalKDTreeManager(),
		multiIndex: NewMultiIndex(),
		pathIndex:  NewPatriciaPathIndex(),
		// Cache: make(map[string]*DirectoryNode),
		Root: NewDirectoryNode("/", nil),
	}

	// Link the spatial index
	dt.multiIndex.SetSpatialIndex(dt)

	for _, opt := range opts {
		opt(dt)
	}

	return dt
}

// TreeOption allows for customization of DirectoryTree
type TreeOption func(*DirectoryTree)

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) TreeOption {
	return func(dt *DirectoryTree) {
		dt.logger = logger
	}
}

// Walk implements TreeWalker interface with context and metrics
func (dt *DirectoryTree) Walk(ctx context.Context) error {
	start := time.Now()
	defer func() {
		dt.mu.Lock()
		dt.metrics.ProcessingTime = time.Since(start)
		dt.metrics.LastUpdated = time.Now()
		dt.mu.Unlock()
	}()

	dt.logger.Info("starting tree walk",
		"root", dt.Root.Path,
		"operation", "walk",
		"timestamp", start)

	return dt.walkNode(ctx, dt.Root, 0)
}

func (dt *DirectoryTree) walkNode(ctx context.Context, node *DirectoryNode, depth int) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	dt.mu.Lock()
	dt.metrics.TotalNodes++
	dt.metrics.MaxDepth = max(dt.metrics.MaxDepth, depth)
	dt.mu.Unlock()

	for _, child := range node.Children {
		if err := dt.walkNode(ctx, child, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// AddFile adds a file node to the tree at a specified path.
// If intermediate directories don't exist, it creates them.
func (tree *DirectoryTree) AddFile(path string, filePath string, size int64, modifiedAt time.Time) error {
	// Split the path into directories and then find or create the path
	targetNode := tree.FindOrCreatePath(splitPathSegments(path))

	// Now that we're at the target directory, add the file node
	fileNode := &FileNode{
		Path:      filePath,
		Name:      filepath.Base(filePath),
		Extension: filepath.Ext(filePath),
	}
	targetNode.AddFile(fileNode)

	// Update the directory's metadata and add to indexes
	targetNode.Metadata.Size += size
	targetNode.Metadata.ModifiedAt = modifiedAt

	// Add the updated directory to all indexes
	if err := tree.AddNode(targetNode); err != nil {
		return fmt.Errorf("failed to re-index directory %s after adding file: %w", targetNode.Path, err)
	}

	return nil
}

// Flatten recursively collects all directories and files in a flat list of paths
func (tree *DirectoryTree) Flatten() []string {
	var paths []string
	tree.flattenNode(tree.Root, tree.Root.Path, &paths)
	return paths
}

// SafeCacheSet safely sets a value in the Cache map
//func (tree *DirectoryTree) SafeCacheSet(key string, value *DirectoryNode) {
//	tree.mu.Lock()
//	defer tree.mu.Unlock()
//
//	tree.Cache[key] = value
//}

// SafeCacheGet safely retrieves a value from the Cache map
//func (tree *DirectoryTree) SafeCacheGet(key string) (*DirectoryNode, bool) {
//	tree.mu.Lock()
//	defer tree.mu.Unlock()
//
//	value, exists := tree.Cache[key]
//	return value, exists
//}

// AddDirectory adds a directory node to the tree at a specified path
func (tree *DirectoryTree) AddDirectory(path string) (*DirectoryNode, error) {
	if path == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}

	node := tree.Root
	segments := splitPathSegments(path)
	for _, segment := range segments {
		found := false
		candidateFull := filepath.Clean(filepath.Join(node.Path, segment))
		for _, child := range node.Children {
			if child.Path == candidateFull && child.Type == Directory {
				node = child
				found = true
				break
			}
		}
		if !found {
			// Create missing directories in path with full-path semantics
			now := time.Now()
			newDir := &DirectoryNode{
				Path:     candidateFull,
				Type:     Directory,
				Parent:   node,
				Children: []*DirectoryNode{},
				Files:    []*FileNode{},
				Metadata: Metadata{
					Size:        4096, // Default directory size
					ModifiedAt:  now,
					CreatedAt:   now,
					NodeType:    Directory,
					Permissions: 0o755,
					Owner:       "system", // Default owner
					Tags:        []string{},
				},
			}
			node.Children = append(node.Children, newDir)

			// Add the new directory to all indexes
			if err := tree.AddNode(newDir); err != nil {
				return nil, fmt.Errorf("failed to index new directory %s: %w", candidateFull, err)
			}

			node = newDir
		}
	}
	return node, nil
}

// FindOrCreatePath traverses the tree to find or create a directory path
func (tree *DirectoryTree) FindOrCreatePath(path []string) *DirectoryNode {
	current := tree.Root
	for _, dir := range path {
		candidateFull := filepath.Clean(filepath.Join(current.Path, dir))
		var next *DirectoryNode
		for _, child := range current.Children {
			if child.Path == candidateFull {
				next = child
				break
			}
		}
		if next == nil {
			next, _ = current.AddChildDirectory(dir)
		}
		current = next
	}
	return current
}

// Cleanup performs necessary cleanup operations on the DirectoryTree
func (dt *DirectoryTree) Cleanup() error {
	// Clear the root node
	if dt.Root != nil {
		dt.Root.Children = nil
		dt.Root.Files = nil
	}

	var err error
	dt.closeOnce.Do(func() {
		dt.logger.Info("cleaning up directory tree")
		for _, cleanup := range dt.cleanup {
			if cleanErr := cleanup(); cleanErr != nil {
				dt.logger.Error("cleanup error",
					"error", cleanErr)
				err = cleanErr
			}
		}
	})
	dt.logger.Info("directory tree cleanup complete")
	// Clear any other resources that need cleanup
	dt.Root = nil
	dt.KDTree = nil
	dt.KDTreeData = nil
	dt.metrics = nil
	dt.logger = nil
	dt.cleanup = nil
	dt.mu = sync.Mutex{}
	// dt.Cache = nil
	return err
}

// GetMetrics returns current metrics with concurrency safety
func (dt *DirectoryTree) GetMetrics(ctx context.Context) (*TreeMetrics, error) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return a copy to prevent external modifications
	return &TreeMetrics{
		TotalNodes:      dt.metrics.TotalNodes,
		TotalSize:       dt.metrics.TotalSize,
		MaxDepth:        dt.metrics.MaxDepth,
		LastUpdated:     dt.metrics.LastUpdated,
		ProcessingTime:  dt.metrics.ProcessingTime,
		OperationCounts: maps.Clone(dt.metrics.OperationCounts),
	}, nil
}

// flattenNode is a helper function for Flatten, processing each node recursively
func (tree *DirectoryTree) flattenNode(node *DirectoryNode, currentPath string, paths *[]string) {
	// Add current directory path to paths
	*paths = append(*paths, currentPath)

	// Recursively process each child directory
	for _, child := range node.Children {
		// child.Path is already a full path
		tree.flattenNode(child, child.Path, paths)
	}

	// Add all files in this directory to paths
	for _, file := range node.Files {
		// file.Path is full path
		*paths = append(*paths, file.Path)
	}
}

func (tree *DirectoryTree) String() string {
	return tree.Root.String()
}

func (tree *DirectoryTree) MarshalJSON() ([]byte, error) {
	return tree.Root.MarshalJSON()
}

func (tree *DirectoryTree) UnMarshalJSON(data []byte) error {
	return tree.Root.UnMarshalJSON(data)
}

// Multi-Index Query Methods

// FindByPath performs O(k) path lookup using patricia tree
func (dt *DirectoryTree) FindByPath(path string) (*DirectoryNode, bool) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.pathIndex != nil {
		return dt.pathIndex.Lookup(path)
	}

	// Fallback to traditional search
	return dt.findNodeByPath(dt.Root, path), dt.findNodeByPath(dt.Root, path) != nil
}

// FindByPathPrefix finds all nodes with paths starting with the given prefix
func (dt *DirectoryTree) FindByPathPrefix(prefix string) []*DirectoryNode {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.pathIndex != nil {
		return dt.pathIndex.PrefixLookup(prefix)
	}

	// Fallback to traditional search
	var results []*DirectoryNode
	dt.walkAndCollect(dt.Root, func(node *DirectoryNode) bool {
		return strings.HasPrefix(node.Path, prefix)
	}, &results)
	return results
}

// FindBySizeRange finds directories within a size range
func (dt *DirectoryTree) FindBySizeRange(minSize, maxSize int64) []*DirectoryNode {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.multiIndex != nil {
		return dt.multiIndex.QueryBySizeRange(minSize, maxSize)
	}

	// Fallback to traditional search
	var results []*DirectoryNode
	dt.walkAndCollect(dt.Root, func(node *DirectoryNode) bool {
		return node.Metadata.Size >= minSize && node.Metadata.Size <= maxSize
	}, &results)
	return results
}

// FindByTimeRange finds directories modified within a time range
func (dt *DirectoryTree) FindByTimeRange(start, end time.Time) []*DirectoryNode {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.multiIndex != nil {
		return dt.multiIndex.QueryByTimeRange(start, end)
	}

	// Fallback to traditional search
	var results []*DirectoryNode
	dt.walkAndCollect(dt.Root, func(node *DirectoryNode) bool {
		modTime := node.Metadata.ModifiedAt
		return !modTime.Before(start) && !modTime.After(end)
	}, &results)
	return results
}

// FindByExtension finds directories containing files with specific extension
func (dt *DirectoryTree) FindByExtension(extension string) []*DirectoryNode {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.multiIndex != nil {
		return dt.multiIndex.QueryByExtension(extension)
	}

	// Fallback to traditional search
	var results []*DirectoryNode
	dt.walkAndCollect(dt.Root, func(node *DirectoryNode) bool {
		for _, file := range node.Files {
			if strings.EqualFold(file.Extension, extension) {
				return true
			}
		}
		return false
	}, &results)
	return results
}

// FindByCategory finds directories containing files of specific category
func (dt *DirectoryTree) FindByCategory(category string) []*DirectoryNode {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.multiIndex != nil {
		return dt.multiIndex.QueryByCategory(category)
	}

	// Fallback to traditional search - would need category logic
	return []*DirectoryNode{}
}

// AddNode adds a directory node to all indexes
func (dt *DirectoryTree) AddNode(node *DirectoryNode) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// Add to traditional KD-Tree
	dt.InsertNodeToKDTreeIncremental(node)

	// Add to path index
	if dt.pathIndex != nil {
		if err := dt.pathIndex.Insert(node); err != nil {
			dt.logger.Error("Failed to insert node into path index", "error", err, "path", node.Path)
		}
	}

	// Add to multi-index
	if dt.multiIndex != nil {
		if err := dt.multiIndex.Insert(node); err != nil {
			dt.logger.Error("Failed to insert node into multi-index", "error", err, "path", node.Path)
		}
	}

	return nil
}

// RemoveNode removes a directory node from all indexes
func (dt *DirectoryTree) RemoveNode(path string) bool {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// Remove from path index
	removed := false

	// Remove from path index
	if dt.pathIndex != nil {
		removed = dt.pathIndex.Remove(path)
	}

	// Remove from multi-index system
	if dt.multiIndex != nil {
		dt.multiIndex.Remove(path)
	}

	// Remove from KD-tree (handled by rebalancing during next insert)
	// Note: KD-tree removal is deferred to avoid expensive rebalancing operations
	// The incremental manager will handle this during the next rebalancing cycle

	return removed
}

// GetIndexStats returns performance statistics from all indexes
func (dt *DirectoryTree) GetIndexStats() (map[string]interface{}, error) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	stats := make(map[string]interface{})

	if dt.pathIndex != nil {
		stats["path_index"] = dt.pathIndex.GetStats()
	}

	if dt.multiIndex != nil {
		stats["multi_index"] = dt.multiIndex.GetStats()
	}

	return stats, nil
}

// ValidateIndexes performs integrity checking across all indexes
func (dt *DirectoryTree) ValidateIndexes() []error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	var errors []error

	if dt.pathIndex != nil {
		pathErrors := dt.pathIndex.Validate()
		errors = append(errors, pathErrors...)
	}

	if dt.multiIndex != nil {
		multiErrors := dt.multiIndex.Validate()
		errors = append(errors, multiErrors...)
	}

	return errors
}

// Helper methods

// findNodeByPath performs traditional recursive path search (fallback)
func (dt *DirectoryTree) findNodeByPath(node *DirectoryNode, targetPath string) *DirectoryNode {
	if node == nil {
		return nil
	}

	if node.Path == targetPath {
		return node
	}

	for _, child := range node.Children {
		if result := dt.findNodeByPath(child, targetPath); result != nil {
			return result
		}
	}

	return nil
}

// walkAndCollect performs traditional recursive walk with predicate (fallback)
func (dt *DirectoryTree) walkAndCollect(node *DirectoryNode, predicate func(*DirectoryNode) bool, results *[]*DirectoryNode) {
	if node == nil {
		return
	}

	if predicate(node) {
		*results = append(*results, node)
	}

	for _, child := range node.Children {
		dt.walkAndCollect(child, predicate, results)
	}
}

// normalizePath provides central path normalization using filepath.Clean + filepath.ToSlash
func normalizePath(path string) string {
	if path == "" {
		return path
	}
	// First replace backslashes with forward slashes (for Windows paths)
	normalized := strings.ReplaceAll(path, "\\", "/")
	// Then clean the path to resolve . and .. elements
	normalized = filepath.ToSlash(filepath.Clean(normalized))
	// Remove trailing slash unless it's the root
	if len(normalized) > 1 && strings.HasSuffix(normalized, "/") {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}

// splitPathSegments splits a filesystem path into clean, non-empty segments.
// It avoids misuse of filepath.SplitList which is for PATH lists, not components.
func splitPathSegments(p string) []string {
	cleaned := filepath.Clean(p)
	slashed := filepath.ToSlash(cleaned)
	parts := strings.Split(slashed, "/")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || s == "." {
			continue
		}
		out = append(out, s)
	}
	return out
}
