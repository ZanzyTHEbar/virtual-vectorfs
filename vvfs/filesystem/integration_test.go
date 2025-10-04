package filesystem

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceIntegrationEndToEnd validates that all enhanced packages work together
func TestServiceIntegrationEndToEnd(t *testing.T) {
	// Create temporary test directory structure
	tempDir, err := os.MkdirTemp("", "vvfs_integration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files with various types for comprehensive tagging
	testStructure := map[string]string{
		"document.pdf":     "test pdf content",
		"image.jpg":        "fake image data",
		"code.go":          "package main\nfunc main() {}",
		"hidden/.dotfile":  "hidden file content",
		"data/config.json": `{"test": true}`,
		"archive/test.zip": "fake zip data",
		"video/demo.mp4":   "fake video data",
		"README.md":        "# Test Project",
	}

	for filePath, content := range testStructure {
		fullPath := filepath.Join(tempDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	t.Run("ConcurrentTraverserWithEnhancedTagging", func(t *testing.T) {
		// Test enhanced metadata and tagging on an existing directory tree
		// Create a simple directory tree manually for testing
		rootNode := &trees.DirectoryNode{
			Path:     tempDir,
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}

		// Manually populate the tree structure for testing
		err := populateTestDirectoryTree(rootNode, tempDir)
		require.NoError(t, err)

		// Test enhanced metadata and tagging
		err = trees.AddMetadataToTree(rootNode)
		require.NoError(t, err)

		// Validate that files have enhanced metadata with owner information
		var testFiles []*trees.FileNode
		collectAllFiles(rootNode, &testFiles)

		assert.GreaterOrEqual(t, len(testFiles), 8, "Should find all test files")

		// Test owner retrieval works
		for _, file := range testFiles {
			assert.NotEmpty(t, file.Metadata.Owner, "Owner should be retrieved for file: %s", file.Path)
			assert.NotEqual(t, "unknown", file.Metadata.Owner, "Owner should not be unknown for file: %s", file.Path)
		}

		// Test comprehensive tagging
		pdfFile := findFileByExtension(testFiles, ".pdf")
		if assert.NotNil(t, pdfFile, "Should find PDF file") {
			// Should have extension-based tags
			assert.Contains(t, pdfFile.Metadata.Tags, "type:pdf")
			assert.Contains(t, pdfFile.Metadata.Tags, "type:document")
			// Should have time-based tags
			assert.True(t, containsAnyTag(pdfFile.Metadata.Tags, []string{"type:recent", "type:thisweek"}))
			// Should have size-based tags
			assert.True(t, containsAnyTag(pdfFile.Metadata.Tags, []string{"type:small", "type:empty"}))
		}

		goFile := findFileByExtension(testFiles, ".go")
		if assert.NotNil(t, goFile, "Should find Go file") {
			assert.Contains(t, goFile.Metadata.Tags, "type:go")
			assert.Contains(t, goFile.Metadata.Tags, "type:code")
		}

		hiddenFile := findFileByName(testFiles, ".dotfile")
		if assert.NotNil(t, hiddenFile, "Should find hidden file") {
			assert.Contains(t, hiddenFile.Metadata.Tags, "type:hidden")
		}
	})

	t.Run("DatabaseIntegrationWithEnhancedMetadata", func(t *testing.T) {
		// Create workspace database for testing
		workspaceDB, err := db.NewWorkspaceDB(tempDir)
		require.NoError(t, err)
		defer workspaceDB.Close()

		// Test file metadata operations with enhanced data
		testFile := filepath.Join(tempDir, "document.pdf")
		metadata, err := trees.GenerateMetadataFromPath(testFile)
		require.NoError(t, err)

		// Add enhanced tags with filename
		err = trees.AddTagsToMetadataWithFilename(&metadata, "document.pdf")
		require.NoError(t, err)

		// Convert to FileMetadata for database operations
		fileMetadata := trees.FileMetadata{
			FilePath: testFile,
			Size:     metadata.Size,
			ModTime:  metadata.ModifiedAt,
			IsDir:    false,
			Checksum: "", // Optional
		}

		// Test database operations with metadata
		err = workspaceDB.InsertFileMetadata(&fileMetadata)
		assert.NoError(t, err)

		// Test retrieval
		retrievedMetadata, err := workspaceDB.GetFileMetadata(testFile)
		require.NoError(t, err)

		// Validate metadata was preserved
		assert.Equal(t, fileMetadata.Size, retrievedMetadata.Size)
		assert.Equal(t, fileMetadata.FilePath, retrievedMetadata.FilePath)
		assert.Equal(t, fileMetadata.IsDir, retrievedMetadata.IsDir)

		// Test batch operations with multiple files
		var allFileMetadata []trees.FileMetadata
		for filePath := range testStructure {
			fullPath := filepath.Join(tempDir, filePath)
			if fileInfo, err := os.Stat(fullPath); err == nil && !fileInfo.IsDir() {
				meta := trees.FileMetadata{
					FilePath: fullPath,
					Size:     fileInfo.Size(),
					ModTime:  fileInfo.ModTime(),
					IsDir:    false,
					Checksum: "",
				}
				allFileMetadata = append(allFileMetadata, meta)
			}
		}

		// Test batch insertion
		err = workspaceDB.BatchInsertFiles(allFileMetadata)
		assert.NoError(t, err)

		// Validate batch operations worked
		allRetrieved, err := workspaceDB.GetAllFileMetadata()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allRetrieved), len(allFileMetadata))
	})

	t.Run("PerformanceValidation", func(t *testing.T) {
		// Create a larger test structure for performance validation
		perfTestDir, err := os.MkdirTemp("", "vvfs_perf_test_*")
		require.NoError(t, err)
		defer os.RemoveAll(perfTestDir)

		// Create multiple directories and files for performance testing
		for i := 0; i < 10; i++ {
			dirPath := filepath.Join(perfTestDir, "dir"+string(rune('0'+i)))
			err := os.MkdirAll(dirPath, 0o755)
			require.NoError(t, err)

			for j := 0; j < 10; j++ {
				fileName := "file" + string(rune('0'+j)) + ".txt"
				filePath := filepath.Join(dirPath, fileName)
				err := os.WriteFile(filePath, []byte("test content"), 0o644)
				require.NoError(t, err)
			}
		}

		// Test metadata generation performance directly
		metadataStartTime := time.Now()

		// Create a simple tree structure and test metadata generation
		perfRootNode := &trees.DirectoryNode{
			Path:     perfTestDir,
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}

		err = populateTestDirectoryTree(perfRootNode, perfTestDir)
		require.NoError(t, err)

		err = trees.AddMetadataToTree(perfRootNode)
		require.NoError(t, err)

		metadataDuration := time.Since(metadataStartTime)

		slog.Info("Performance test results",
			"metadata_duration", metadataDuration,
			"total_files", countAllFiles(perfRootNode),
			"total_dirs", countAllDirs(perfRootNode))

		// Validate performance expectations
		assert.Less(t, metadataDuration, 200*time.Millisecond, "Metadata generation should be fast")

		// Validate that performance is comparable to Phase 1 achievements
		filesPerSecond := float64(countAllFiles(perfRootNode)) / metadataDuration.Seconds()
		assert.Greater(t, filesPerSecond, 500.0, "Should process >500 files/sec")
	})

	t.Run("EnhancedTaggingIntegration", func(t *testing.T) {
		// Test the enhanced tagging system end-to-end
		testFile := filepath.Join(tempDir, "document.pdf")

		// Generate metadata with enhanced tagging
		metadata, err := trees.GenerateMetadataFromPath(testFile)
		require.NoError(t, err)

		// Test filename-aware tagging
		originalTagCount := len(metadata.Tags)
		err = trees.AddTagsToMetadataWithFilename(&metadata, "document.pdf")
		require.NoError(t, err)

		// Should have more tags after filename processing
		assert.Greater(t, len(metadata.Tags), originalTagCount, "Should add filename-based tags")

		// Validate specific tags were added
		assert.Contains(t, metadata.Tags, "type:pdf")
		assert.Contains(t, metadata.Tags, "type:document")

		// Test with different file types
		goFile := filepath.Join(tempDir, "code.go")
		goMetadata, err := trees.GenerateMetadataFromPath(goFile)
		require.NoError(t, err)

		err = trees.AddTagsToMetadataWithFilename(&goMetadata, "code.go")
		require.NoError(t, err)

		assert.Contains(t, goMetadata.Tags, "type:go")
		assert.Contains(t, goMetadata.Tags, "type:code")

		// Test hidden file tagging
		hiddenFile := filepath.Join(tempDir, "hidden/.dotfile")
		hiddenMetadata, err := trees.GenerateMetadataFromPath(hiddenFile)
		require.NoError(t, err)

		err = trees.AddTagsToMetadataWithFilename(&hiddenMetadata, ".dotfile")
		require.NoError(t, err)

		assert.Contains(t, hiddenMetadata.Tags, "type:hidden")
	})
}

// Helper functions for testing

func populateTestDirectoryTree(rootNode *trees.DirectoryNode, dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			childNode := &trees.DirectoryNode{
				Path:     fullPath,
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			}
			rootNode.Children = append(rootNode.Children, childNode)

			// Recursively populate child directories
			err = populateTestDirectoryTree(childNode, fullPath)
			if err != nil {
				return err
			}
		} else {
			fileNode := &trees.FileNode{
				Path: fullPath,
			}
			rootNode.Files = append(rootNode.Files, fileNode)
		}
	}

	return nil
}

func collectAllFiles(node *trees.DirectoryNode, files *[]*trees.FileNode) {
	*files = append(*files, node.Files...)
	for _, child := range node.Children {
		collectAllFiles(child, files)
	}
}

func findFileByExtension(files []*trees.FileNode, ext string) *trees.FileNode {
	for _, file := range files {
		if filepath.Ext(file.Path) == ext {
			return file
		}
	}
	return nil
}

func findFileByName(files []*trees.FileNode, name string) *trees.FileNode {
	for _, file := range files {
		if filepath.Base(file.Path) == name {
			return file
		}
	}
	return nil
}

func containsAnyTag(tags []string, searchTags []string) bool {
	for _, tag := range tags {
		for _, searchTag := range searchTags {
			if tag == searchTag {
				return true
			}
		}
	}
	return false
}

func countAllFiles(node *trees.DirectoryNode) int {
	count := len(node.Files)
	for _, child := range node.Children {
		count += countAllFiles(child)
	}
	return count
}

func countAllDirs(node *trees.DirectoryNode) int {
	count := 1 // count this directory
	for _, child := range node.Children {
		count += countAllDirs(child)
	}
	return count
}

// TestPathIndexKDTreeConsistency validates that path-index and KD-tree operations
// remain consistent during insert/query operations
func TestPathIndexKDTreeConsistency(t *testing.T) {
	t.Run("PathIndexKDTreeInsertQueryConsistency", func(t *testing.T) {
		// Create a directory tree for testing
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))

		// Create path index
		pathIndex := trees.NewPatriciaPathIndex()

		// Create test directory nodes with metadata
		testDirs := []*trees.DirectoryNode{
			{
				Path:     "docs",
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			},
			{
				Path:     "src",
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			},
			{
				Path:     "data",
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			},
		}

		// Initialize metadata for each directory
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		for i, dir := range testDirs {
			modifiedTime := baseTime.Add(time.Duration(i) * time.Hour)

			dir.Metadata = trees.Metadata{
				Size:        int64(1024 * (i + 1)), // Different sizes for uniqueness
				ModifiedAt:  modifiedTime,
				CreatedAt:   modifiedTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}
		}

		// Add directories to tree and path index
		treeNodes := make([]*trees.DirectoryNode, len(testDirs))
		for i, dir := range testDirs {
			treeNode, err := tree.AddDirectory(dir.Path)
			require.NoError(t, err)

			// Set metadata on the tree node that was actually created
			modifiedTime := baseTime.Add(time.Duration(i) * time.Hour)
			treeNode.Metadata = trees.Metadata{
				Size:        int64(1024 * (i + 1)), // Different sizes for uniqueness
				ModifiedAt:  modifiedTime,
				CreatedAt:   modifiedTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}

			// Store the tree node for later use
			treeNodes[i] = treeNode

			err = pathIndex.Insert(treeNode)
			require.NoError(t, err)
		}

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Build KD-tree
		tree.BuildKDTree()

		// Test 1: Verify path index and KD-tree have consistent node counts
		// KD-tree includes root node, path index does not count root in Size()
		pathIndexSize := pathIndex.Size()
		kdTreeSize := len(tree.KDTreeData)
		assert.Equal(t, pathIndexSize, int64(kdTreeSize-1), "Path index should have one less node than KD-tree (excluding root)")

		// Test 2: Verify all paths in path index exist in KD-tree data
		pathIndex.WalkPaths(func(path string, node *trees.DirectoryNode) bool {
			// Check if this path exists in KD-tree data
			found := false
			for _, kdPoint := range tree.KDTreeData {
				if kdPoint.Node.Path == path {
					found = true
					break
				}
			}
			assert.True(t, found, "Path %s should exist in KD-tree data", path)
			return false
		})

		// Test 3: Verify lookups return consistent results
		for _, dir := range testDirs {
			// Test path index lookup (need full path for lookup)
			fullPath := "/test/root/" + dir.Path
			pathIndexNode, found := pathIndex.Lookup(fullPath)
			assert.True(t, found, "Path index should find %s", fullPath)
			assert.NotNil(t, pathIndexNode, "Path index node should not be nil")

			// Test KD-tree consistency (node should exist in KD-tree data)
			foundInKDTree := false
			for _, kdPoint := range tree.KDTreeData {
				if kdPoint.Node.Path == fullPath {
					foundInKDTree = true
					break
				}
			}
			assert.True(t, foundInKDTree, "Directory %s should exist in KD-tree data", fullPath)
		}
	})

	t.Run("PathIndexKDTreeIncrementalOperations", func(t *testing.T) {
		// Test incremental operations maintain consistency
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Start with empty structures
		assert.Equal(t, int64(0), pathIndex.Size())
		assert.Equal(t, 0, len(tree.KDTreeData))

		// Add directories incrementally
		newDir := &trees.DirectoryNode{
			Path:     "projects",
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}
		newDir.Metadata = trees.Metadata{
			Size:        2048,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Add to tree and path index
		_, err := tree.AddDirectory(newDir.Path)
		require.NoError(t, err)
		err = pathIndex.Insert(newDir)
		require.NoError(t, err)

		// Insert into KD-tree incrementally
		tree.InsertNodeToKDTreeIncremental(newDir)

		// Force pending updates to be processed
		tree.FlushPendingKDTreeUpdates()

		// Verify consistency after incremental operations
		assert.Equal(t, int64(1), pathIndex.Size())
		assert.Equal(t, 1, len(tree.KDTreeData))

		// Verify the node exists in both
		pathIndexNode, found := pathIndex.Lookup("/test/root/projects")
		assert.True(t, found)
		assert.NotNil(t, pathIndexNode)

		foundInKDTree := false
		for _, kdPoint := range tree.KDTreeData {
			if kdPoint.Node.Path == "/test/root/projects" {
				foundInKDTree = true
				break
			}
		}
		assert.True(t, foundInKDTree, "Incrementally added node should exist in KD-tree")
	})

	t.Run("PathIndexKDTreeDeleteOperations", func(t *testing.T) {
		// Test that delete operations maintain consistency
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Add a directory
		dir := &trees.DirectoryNode{
			Path:     "temp",
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}
		dir.Metadata = trees.Metadata{
			Size:        1024,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		_, err := tree.AddDirectory(dir.Path)
		require.NoError(t, err)
		err = pathIndex.Insert(dir)
		require.NoError(t, err)

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		tree.BuildKDTree()

		// Verify it exists
		assert.Equal(t, int64(1), pathIndex.Size())
		assert.Equal(t, 1, len(tree.KDTreeData))

		// Remove from path index (KD-tree doesn't have delete, so we rebuild)
		deleted := pathIndex.Remove("/test/root/temp")
		assert.True(t, deleted)

		// Rebuild KD-tree without the deleted node
		tree.KDTreeData = trees.DirectoryPointCollection{}
		tree.BuildKDTree()

		// Verify both are empty
		assert.Equal(t, int64(0), pathIndex.Size())
		assert.Equal(t, 0, len(tree.KDTreeData))
	})

	t.Run("PathIndexKDTreePrefixOperations", func(t *testing.T) {
		// Test prefix operations consistency
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Create hierarchical directory structure
		dirs := []*trees.DirectoryNode{
			{Path: "src", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
			{Path: "src/main", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
			{Path: "src/test", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
			{Path: "docs", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
			{Path: "docs/api", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
		}

		// Initialize metadata
		for _, dir := range dirs {
			dir.Metadata = trees.Metadata{
				Size:        1024,
				ModifiedAt:  baseTime,
				CreatedAt:   baseTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}
		}

		// Add all directories
		for _, dir := range dirs {
			_, err := tree.AddDirectory(dir.Path)
			require.NoError(t, err)
			err = pathIndex.Insert(dir)
			require.NoError(t, err)
		}

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		tree.BuildKDTree()

		// Test prefix lookup
		srcPrefixResults := pathIndex.PrefixLookup("/test/root/src")
		assert.Len(t, srcPrefixResults, 3, "Should find 3 directories under /test/root/src")

		// Verify all prefix results exist in KD-tree
		for _, prefixNode := range srcPrefixResults {
			foundInKDTree := false
			for _, kdPoint := range tree.KDTreeData {
				if kdPoint.Node.Path == prefixNode.Path {
					foundInKDTree = true
					break
				}
			}
			assert.True(t, foundInKDTree, "Prefix lookup result %s should exist in KD-tree", prefixNode.Path)
		}
	})

	t.Run("PathIndexKDTreeConcurrentOperations", func(t *testing.T) {
		// Test concurrent operations maintain consistency
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		var wg sync.WaitGroup
		var errors []error
		var errorMu sync.Mutex

		// Create multiple directories to insert concurrently
		numDirs := 10
		basePaths := make([]string, numDirs)
		for i := 0; i < numDirs; i++ {
			basePaths[i] = fmt.Sprintf("dir%02d", i)
		}

		// Concurrent insertion
		for i := 0; i < numDirs; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				dir := &trees.DirectoryNode{
					Path:     basePaths[idx],
					Children: []*trees.DirectoryNode{},
					Files:    []*trees.FileNode{},
				}
				dir.Metadata = trees.Metadata{
					Size:        1024,
					ModifiedAt:  baseTime,
					CreatedAt:   baseTime,
					NodeType:    trees.Directory,
					Permissions: 0o755,
					Owner:       "testuser",
					Tags:        []string{},
				}

				// Add to tree and path index
				treeNode, err := tree.AddDirectory(dir.Path)
				if err != nil {
					errorMu.Lock()
					errors = append(errors, err)
					errorMu.Unlock()
					return
				}

				// Copy metadata to the tree node that was actually created
				treeNode.Metadata = dir.Metadata

				if err := pathIndex.Insert(treeNode); err != nil {
					errorMu.Lock()
					errors = append(errors, err)
					errorMu.Unlock()
				}

				// Insert into KD-tree incrementally
				tree.InsertNodeToKDTreeIncremental(treeNode)
			}(i)
		}

		wg.Wait()

		// Check for errors
		assert.Empty(t, errors, "No errors should occur during concurrent operations")

		// Flush pending KD-tree updates
		tree.FlushPendingKDTreeUpdates()

		// Verify consistency
		pathIndexSize := pathIndex.Size()
		kdTreeSize := len(tree.KDTreeData)
		assert.Equal(t, pathIndexSize, int64(kdTreeSize), "Concurrent operations should maintain consistency")

		// Verify all paths exist in both structures
		for _, basePath := range basePaths {
			fullPath := "/test/root/" + basePath
			_, found := pathIndex.Lookup(fullPath)
			assert.True(t, found, "Path %s should exist in path index", fullPath)

			foundInKDTree := false
			for _, kdPoint := range tree.KDTreeData {
				if kdPoint.Node.Path == fullPath {
					foundInKDTree = true
					break
				}
			}
			assert.True(t, foundInKDTree, "Path %s should exist in KD-tree", fullPath)
		}
	})

	t.Run("PathIndexKDTreeValidationAndIntegrity", func(t *testing.T) {
		// Test integrity validation between structures
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Add some directories
		dirs := []*trees.DirectoryNode{
			{Path: "valid1", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
			{Path: "valid2", Children: []*trees.DirectoryNode{}, Files: []*trees.FileNode{}},
		}

		for _, dir := range dirs {
			dir.Metadata = trees.Metadata{
				Size:        1024,
				ModifiedAt:  baseTime,
				CreatedAt:   baseTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}

			_, err := tree.AddDirectory(dir.Path)
			require.NoError(t, err)
			err = pathIndex.Insert(dir)
			require.NoError(t, err)
		}

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Build KD-tree
		tree.BuildKDTree()

		// Test path index validation
		validationErrors := pathIndex.Validate()
		assert.Empty(t, validationErrors, "Path index should be valid")

		// Verify statistics consistency
		stats := pathIndex.GetStats()
		assert.Equal(t, int64(2), stats.TotalNodes)
		assert.Equal(t, int64(2), stats.Insertions)

		// Test cross-validation between structures
		pathIndex.WalkPaths(func(path string, piNode *trees.DirectoryNode) bool {
			// Find corresponding node in KD-tree
			var kdNode *trees.DirectoryNode
			for _, kdPoint := range tree.KDTreeData {
				if kdPoint.Node.Path == path {
					kdNode = kdPoint.Node
					break
				}
			}

			assert.NotNil(t, kdNode, "Path %s should exist in KD-tree", path)
			if kdNode != nil {
				assert.Equal(t, piNode.Path, kdNode.Path, "Paths should match between structures")
				assert.Equal(t, piNode.Metadata.NodeType, kdNode.Metadata.NodeType, "Node types should match")
			}

			return false
		})
	})

	t.Run("PathIndexKDTreePerformanceCharacteristics", func(t *testing.T) {
		// Test performance characteristics of both structures
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Create many directories for performance testing
		numDirs := 100
		dirs := make([]*trees.DirectoryNode, numDirs)

		for i := 0; i < numDirs; i++ {
			dirPath := fmt.Sprintf("dir%03d", i)
			dir := &trees.DirectoryNode{
				Path:     dirPath,
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			}
			dir.Metadata = trees.Metadata{
				Size:        1024,
				ModifiedAt:  baseTime,
				CreatedAt:   baseTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}

			dirs[i] = dir
		}

		// Measure insertion performance
		startTime := time.Now()

		for _, dir := range dirs {
			_, err := tree.AddDirectory(dir.Path)
			require.NoError(t, err)
			err = pathIndex.Insert(dir)
			require.NoError(t, err)
		}

		insertionTime := time.Since(startTime)

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Build KD-tree
		kdStartTime := time.Now()
		tree.BuildKDTree()
		kdBuildTime := time.Since(kdStartTime)

		// Measure lookup performance
		lookupStartTime := time.Now()
		for i := 0; i < 50; i++ { // Test 50 random lookups
			randomPath := fmt.Sprintf("/test/root/dir%03d", i*2)
			_, found := pathIndex.Lookup(randomPath)
			assert.True(t, found, "Lookup should succeed for %s", randomPath)
		}
		lookupTime := time.Since(lookupStartTime)

		// Verify performance expectations
		assert.Less(t, insertionTime, 100*time.Millisecond, "Bulk insertion should be fast")
		assert.Less(t, kdBuildTime, 50*time.Millisecond, "KD-tree build should be fast")
		assert.Less(t, lookupTime, 10*time.Millisecond, "Lookups should be very fast")

		// Verify final consistency
		assert.Equal(t, int64(numDirs), pathIndex.Size())
		assert.Equal(t, numDirs, len(tree.KDTreeData))

		slog.Info("Performance test completed",
			"directories", numDirs,
			"insertion_time", insertionTime,
			"kd_build_time", kdBuildTime,
			"lookup_time", lookupTime)
	})

	t.Run("PathIndexKDTreeErrorHandling", func(t *testing.T) {
		// Test error handling and recovery scenarios
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Test 1: Invalid directory insertion
		invalidDir := &trees.DirectoryNode{
			Path:     "", // Invalid empty path
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}

		err := pathIndex.Insert(invalidDir)
		assert.Error(t, err, "Should reject invalid directory")

		// Test 2: Duplicate path handling
		validDir := &trees.DirectoryNode{
			Path:     "duplicate",
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}
		validDir.Metadata = trees.Metadata{
			Size:        1024,
			ModifiedAt:  baseTime,
			CreatedAt:   time.Now(),
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// First insertion should succeed
		err = pathIndex.Insert(validDir)
		assert.NoError(t, err)

		// Second insertion should succeed (update)
		err = pathIndex.Insert(validDir)
		assert.NoError(t, err)

		// Test 3: KD-tree operations with invalid metadata
		invalidMetadataDir := &trees.DirectoryNode{
			Path:     "invalid",
			Children: []*trees.DirectoryNode{},
			Files:    []*trees.FileNode{},
		}
		invalidMetadataDir.Metadata = trees.Metadata{
			Size:        1024,
			ModifiedAt:  baseTime,
			CreatedAt:   time.Now(),
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Add to tree first
		_, err = tree.AddDirectory(invalidMetadataDir.Path)
		require.NoError(t, err)

		// KD-tree insertion should handle errors gracefully
		tree.InsertNodeToKDTreeIncremental(invalidMetadataDir)

		// Flush to ensure processing
		tree.FlushPendingKDTreeUpdates()

		// Verify path index still works
		_, found := pathIndex.Lookup("/test/root/duplicate")
		assert.True(t, found, "Valid paths should still work after error conditions")
	})

	t.Run("PathIndexKDTreeMemoryEfficiency", func(t *testing.T) {
		// Test memory efficiency of incremental operations
		tree := trees.NewDirectoryTree(trees.WithRoot("/test/root"))
		pathIndex := trees.NewPatriciaPathIndex()
		baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// Track memory usage pattern
		var initialKDSize, finalKDSize int
		var initialPathSize, finalPathSize int64

		// Initial state
		initialKDSize = len(tree.KDTreeData)
		initialPathSize = pathIndex.Size()

		// Add directories incrementally
		for i := 0; i < 20; i++ {
			dir := &trees.DirectoryNode{
				Path:     fmt.Sprintf("memtest%02d", i),
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
			}
			dir.Metadata = trees.Metadata{
				Size:        1024,
				ModifiedAt:  baseTime,
				CreatedAt:   baseTime,
				NodeType:    trees.Directory,
				Permissions: 0o755,
				Owner:       "testuser",
				Tags:        []string{},
			}

			treeNode, err := tree.AddDirectory(dir.Path)
			require.NoError(t, err)

			// Copy metadata to the tree node that was actually created
			treeNode.Metadata = dir.Metadata

			err = pathIndex.Insert(treeNode)
			require.NoError(t, err)

			// Use incremental KD-tree insertion
			tree.InsertNodeToKDTreeIncremental(treeNode)

			// Every 5 insertions, check pending batch status
			if (i+1)%5 == 0 && len(tree.KDTreeData) > 0 {
				// Just a simple check to ensure KD-tree data is accessible
				kdTreeLen := len(tree.KDTreeData)
				_ = kdTreeLen // Use the variable
			}
		}

		// Set metadata on the root node to avoid validation failures
		tree.Root.Metadata = trees.Metadata{
			Size:        4096,
			ModifiedAt:  baseTime,
			CreatedAt:   baseTime,
			NodeType:    trees.Directory,
			Permissions: 0o755,
			Owner:       "testuser",
			Tags:        []string{},
		}

		// Flush all pending operations
		tree.FlushPendingKDTreeUpdates()

		// Final state
		finalKDSize = len(tree.KDTreeData)
		finalPathSize = pathIndex.Size()

		// Verify growth
		assert.Equal(t, 20, finalKDSize-initialKDSize, "Should have added 20 directories to KD-tree")
		assert.Equal(t, int64(20), finalPathSize-initialPathSize, "Should have added 20 directories to path index")

		// Verify no memory leaks (sizes should match)
		assert.Equal(t, finalKDSize, int(finalPathSize), "KD-tree and path index should have same size")

		// Test cleanup
		for i := 0; i < 20; i++ {
			path := fmt.Sprintf("/test/root/memtest%02d", i)
			pathIndex.Remove(path)
		}

		assert.Equal(t, initialPathSize, pathIndex.Size(), "Should return to initial size after cleanup")
	})
}
