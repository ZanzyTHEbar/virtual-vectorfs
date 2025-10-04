package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/common"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/services"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"

	"github.com/sourcegraph/conc/pool"
)

// ConcurrentTraverser implements high-performance concurrent directory traversal
// using the conc package for robust worker pool and job management
type ConcurrentTraverser struct {
	maxWorkers     int
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
	processedDirs  map[string]bool // Track processed directories to avoid duplicates
	pool           *pool.ContextPool
	osCapabilities *OSCapabilities // Cached OS capabilities for intelligent method selection
	capabilitiesMu sync.RWMutex    // Protect OS capabilities access
}

// OSCapabilities represents the detected OS-level capabilities
type OSCapabilities struct {
	OS              string
	HasFind         bool
	HasGfind        bool // GNU find (Linux)
	HasBfind        bool // BSD find (macOS)
	HasPwsh         bool // PowerShell (Windows)
	HasDir          bool // Windows dir command
	HasStat         bool // stat command availability
	HasDu           bool // du command availability
	HasTree         bool // tree command availability
	PreferredMethod string
	CacheEnabled    bool
}

// DirectoryStats represents comprehensive directory information from OS
type DirectoryStats struct {
	Path         string
	TotalFiles   int64
	TotalDirs    int64
	MaxDepth     int
	TotalSize    int64
	FileTypes    map[string]int64 // extension -> count
	DirDensity   float64          // directories as % of total entries
	LastModified time.Time
	MethodUsed   string // which method was used to gather stats
	Source       string // OS tool or "go_fallback"
}

// TraversalResult contains the result of processing a directory
type TraversalResult struct {
	Node     *trees.DirectoryNode
	Children []*trees.DirectoryNode
	Files    []*trees.FileNode
	Error    error
	Path     string
}

// TraversalStats tracks performance metrics during traversal
type TraversalStats struct {
	DirsProcessed  int64
	FilesProcessed int64
	ErrorsFound    int64
	StartTime      int64
	EndTime        int64
}

// NewConcurrentTraverser creates a new concurrent directory traverser
// with optimal worker count based on available CPU cores using conc.Pool
func NewConcurrentTraverser(ctx context.Context) *ConcurrentTraverser {
	// Optimal worker count: CPU cores * 2 for I/O bound operations
	// Minimum workers for responsiveness
	// Maximum to prevent resource exhaustion
	maxWorkers := min(max(runtime.NumCPU()*2, 4), 32)

	ctxWithCancel, cancel := context.WithCancel(ctx)

	return &ConcurrentTraverser{
		maxWorkers:    maxWorkers,
		ctx:           ctxWithCancel,
		cancel:        cancel,
		processedDirs: make(map[string]bool),
		pool:          pool.New().WithMaxGoroutines(maxWorkers).WithContext(ctxWithCancel),
	}
}

// TraverseDirectory performs concurrent directory traversal using conc.Pool
// for optimal performance and resource management
func (ct *ConcurrentTraverser) TraverseDirectory(rootPath string, recursive bool, maxDepth int, handler services.TraversalHandler) (*trees.DirectoryNode, error) {
	// Initialize root node
	rootNode := trees.NewDirectoryNode(rootPath, nil)

	// Create time utilities instance
	timeUtils := common.NewTimeUtils()

	// Track performance metrics with atomic operations
	stats := &TraversalStats{
		StartTime: timeUtils.GetCurrentTime(),
	}

	// Use the pool to enforce bounded concurrency per task submission

	// Process directories level by level using a BFS approach with conc.Pool
	currentLevel := []*trees.DirectoryNode{rootNode}

	for depth := 0; (maxDepth == -1 || depth <= maxDepth+1) && len(currentLevel) > 0; depth++ {
		if !recursive && depth > 0 {
			break
		}

		nextLevel := make([]*trees.DirectoryNode, 0)
		var nextLevelMu sync.Mutex

		// Create a new pool context for this level to avoid reusing closed pools
		// levelPool := pool.New().WithMaxGoroutines(ct.maxWorkers).WithContext(ct.ctx)

		// Use the shared pool to enforce bounded concurrency and a per-level
		// waitgroup so we only wait for the tasks we submitted for this level.
		var wg sync.WaitGroup

		// Process all directories at the current level concurrently via ct.pool
		for _, dirNode := range currentLevel {
			wg.Add(1)

			// Submit task to shared pool; pool limits actual goroutines to ct.maxWorkers
			ct.pool.Go(func(ctx context.Context) error {
				defer wg.Done()

				result := ct.processDirectoryNode(ctx, dirNode, depth, maxDepth, handler)

				// Update statistics atomically
				if result.Error == nil {
					atomic.AddInt64(&stats.DirsProcessed, 1)
					atomic.AddInt64(&stats.FilesProcessed, int64(len(result.Files)))
				} else {
					atomic.AddInt64(&stats.ErrorsFound, 1)
					slog.Error("Error processing directory",
						"path", result.Path,
						"error", result.Error)
				}

				// Add child directories to next level if within depth limits
				if recursive && (maxDepth == -1 || depth <= maxDepth) && result.Error == nil {
					nextLevelMu.Lock()
					nextLevel = append(nextLevel, result.Children...)
					nextLevelMu.Unlock()
				}

				return nil
			})
		}

		// Wait for all directories at this level to be processed
		wg.Wait()

		// Move to the next level
		currentLevel = nextLevel
	}

	stats.EndTime = timeUtils.GetCurrentTime()
	ct.logPerformanceStats(stats)

	return rootNode, nil
}

// processDirectoryNode processes a single directory node with optimized I/O operations
func (ct *ConcurrentTraverser) processDirectoryNode(ctx context.Context, dirNode *trees.DirectoryNode, depth, maxDepth int, handler services.TraversalHandler) TraversalResult {
	result := TraversalResult{
		Node: dirNode,
		Path: dirNode.Path,
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		return result
	default:
	}

	// Check depth limits (-1 means unlimited)
	// Allow processing up to maxDepth+1 to match TraverseDirectory logic
	if maxDepth != -1 && depth > maxDepth+1 {
		slog.Debug("Max depth reached",
			"maxDepth", maxDepth,
			"path", dirNode.Path)
		return result
	}

	// Check if already processed (prevent duplicates)
	ct.mu.RLock()
	if ct.processedDirs[dirNode.Path] {
		ct.mu.RUnlock()
		return result
	}
	ct.mu.RUnlock()

	// Mark as processed
	ct.mu.Lock()
	ct.processedDirs[dirNode.Path] = true
	ct.mu.Unlock()

	// Read directory entries with optimized I/O
	entries, err := os.ReadDir(dirNode.Path)
	if err != nil {
		result.Error = fmt.Errorf("failed to read directory %s: %w", dirNode.Path, err)
		return result
	}

	// Get ignore patterns
	ignored, err := handler.GetDesktopCleanerIgnore(dirNode.Path)
	if err != nil {
		slog.Warn("Failed to get ignore patterns",
			"path", dirNode.Path,
			"error", err)
	}

	// Process entries with intelligent allocation
	children, files := ct.preallocateSlicesIntelligent(dirNode.Path, entries)

	for _, entry := range entries {
		childPath := filepath.Join(dirNode.Path, entry.Name())

		// Skip ignored files and directories
		if ignored != nil && ignored.MatchesPath(childPath) {
			slog.Debug("Ignoring file",
				"path", childPath)
			continue
		}

		if entry.IsDir() {
			// For directories, create minimal metadata without syscall
			now := time.Now()
			childDir := &trees.DirectoryNode{
				Path:     childPath,
				Type:     trees.Directory,
				Parent:   dirNode,
				Children: []*trees.DirectoryNode{},
				Files:    []*trees.FileNode{},
				Metadata: trees.Metadata{
					Size:        4096, // Default directory size
					ModifiedAt:  now,
					CreatedAt:   now,
					NodeType:    trees.Directory,
					Permissions: 0o755,
					Owner:       "system", // Default owner
					Tags:        []string{},
				},
			}
			children = append(children, childDir)
			dirNode.Children = append(dirNode.Children, childDir)

			// Call handler for directory
			if err := handler.HandleDirectory(childDir); err != nil {
				slog.Warn("Handler error for directory",
					"path", childPath,
					"error", err)
			}
		} else {
			// For files, we need full metadata so syscall is necessary
			entryInfo, err := entry.Info()
			if err != nil {
				slog.Warn("Error getting file info",
					"name", entry.Name(),
					"error", err)
				continue
			}

			childFile := &trees.FileNode{
				Path:      childPath,
				Name:      entry.Name(),
				Extension: strings.ToLower(filepath.Ext(entry.Name())),
				Metadata:  trees.NewMetadata(entryInfo),
			}
			files = append(files, childFile)
			dirNode.AddFile(childFile)

			// Call handler for file
			if err := handler.HandleFile(childFile); err != nil {
				slog.Warn("Handler error for file",
					"path", childPath,
					"error", err)
			}
		}
	}

	result.Children = children
	result.Files = files
	return result
}

// preallocateSlices performs intelligent slice pre-allocation based on directory structure analysis
func (ct *ConcurrentTraverser) preallocateSlices(entries []os.DirEntry) ([]*trees.DirectoryNode, []*trees.FileNode) {
	if len(entries) == 0 {
		return make([]*trees.DirectoryNode, 0), make([]*trees.FileNode, 0)
	}

	// Quick pre-scan for accurate counts (negligible overhead for most cases)
	dirCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirCount++
		}
	}

	// Pre-allocate with exact counts + small buffer for dynamic additions
	children := make([]*trees.DirectoryNode, 0, dirCount+2)
	files := make([]*trees.FileNode, 0, len(entries)-dirCount+2)

	return children, files
}

// preallocateSlicesIntelligent chooses the best pre-allocation strategy based on context
func (ct *ConcurrentTraverser) preallocateSlicesIntelligent(dirPath string, entries []os.DirEntry) ([]*trees.DirectoryNode, []*trees.FileNode) {
	totalEntries := len(entries)
	if totalEntries == 0 {
		return make([]*trees.DirectoryNode, 0), make([]*trees.FileNode, 0)
	}

	// For small directories, use exact pre-scan (fastest)
	if len(entries) <= 100 {
		return ct.preallocateSlices(entries)
	}

	// For larger directories, try OS-level statistics first
	caps := ct.getOSCapabilities()
	if caps.PreferredMethod != "go_fallback" {
		stats, err := ct.getDirectoryStatsIntelligent(dirPath)
		if err == nil && stats.DirDensity > 0 {
			// Use OS-derived directory density for perfect pre-allocation
			dirCapacity := int(float64(len(entries))*stats.DirDensity/100) + 2
			fileCapacity := len(entries) - dirCapacity + 2

			slog.Debug("Using OS-level pre-allocation",
				"path", dirPath,
				"entries", len(entries),
				"dir_density", stats.DirDensity,
				"dir_capacity", dirCapacity,
				"file_capacity", fileCapacity,
				"method", stats.MethodUsed)

			return make([]*trees.DirectoryNode, 0, dirCapacity), make([]*trees.FileNode, 0, fileCapacity)
		}
	}

	// Fallback to adaptive estimation
	return ct.preallocateSlicesAdaptive(entries)
}

// preallocateSlicesAdaptive uses adaptive estimation based on historical patterns
func (ct *ConcurrentTraverser) preallocateSlicesAdaptive(entries []os.DirEntry) ([]*trees.DirectoryNode, []*trees.FileNode) {
	totalEntries := len(entries)
	if totalEntries == 0 {
		return make([]*trees.DirectoryNode, 0), make([]*trees.FileNode, 0)
	}

	// Adaptive estimation based on typical directory structures
	// Most directories: ~10-20% directories (code, docs, src/)
	// Node.js projects: ~5-10% directories
	// Large data dirs: ~1-5% directories
	// Use conservative estimate to avoid reallocations
	dirEstimate := max(totalEntries/8, 4) // 12.5% minimum, at least 4 capacity

	children := make([]*trees.DirectoryNode, 0, dirEstimate)
	files := make([]*trees.FileNode, 0, totalEntries-dirEstimate/2)

	return children, files
}

// getDirectoryStatsLinux uses GNU find (Linux) for optimal performance
func (ct *ConcurrentTraverser) getDirectoryStatsLinux(rootPath string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:       rootPath,
		FileTypes:  make(map[string]int64),
		MethodUsed: "find_gnu",
		Source:     "gnu_find",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use GNU find's advanced features for maximum efficiency
	findCmd := exec.CommandContext(ctx, "find", rootPath, "-type", "f", "-print0")
	findOutput, err := findCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("GNU find failed: %w", err)
	}

	// Parse files and count types
	files := strings.Split(strings.TrimRight(string(findOutput), "\x00"), "\x00")
	stats.TotalFiles = int64(len(files))

	for _, file := range files {
		if file == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file))
		stats.FileTypes[ext]++
	}

	// Directory count with GNU find
	dirCmd := exec.CommandContext(ctx, "find", rootPath, "-type", "d", "-print0")
	dirOutput, err := dirCmd.Output()
	if err == nil {
		dirs := strings.Split(strings.TrimRight(string(dirOutput), "\x00"), "\x00")
		stats.TotalDirs = int64(len(dirs))
	}

	// Max depth with GNU find
	stats.MaxDepth = ct.calculateMaxDepthLinux(rootPath)

	// Total size using du (disk usage)
	duCmd := exec.CommandContext(ctx, "du", "-sb", rootPath)
	duOutput, err := duCmd.Output()
	if err == nil {
		parts := strings.Fields(string(duOutput))
		if len(parts) >= 1 {
			if size, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				stats.TotalSize = size
			}
		}
	}

	// Calculate directory density
	if stats.TotalFiles+stats.TotalDirs > 0 {
		stats.DirDensity = float64(stats.TotalDirs) / float64(stats.TotalFiles+stats.TotalDirs) * 100
	}

	return stats, nil
}

// getDirectoryStatsMacOS uses BSD find (macOS) with compatible syntax
func (ct *ConcurrentTraverser) getDirectoryStatsMacOS(rootPath string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:       rootPath,
		FileTypes:  make(map[string]int64),
		MethodUsed: "find_bsd",
		Source:     "bsd_find",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// BSD find has different syntax - use compatible approach
	findCmd := exec.CommandContext(ctx, "find", rootPath,
		"-type", "f",
		"-print0")

	findOutput, err := findCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("BSD find failed: %w", err)
	}

	files := strings.Split(strings.TrimRight(string(findOutput), "\x00"), "\x00")
	stats.TotalFiles = int64(len(files))

	for _, file := range files {
		if file == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file))
		stats.FileTypes[ext]++
	}

	// Directory count
	dirCmd := exec.CommandContext(ctx, "find", rootPath, "-type", "d", "-print0")
	dirOutput, err := dirCmd.Output()
	if err == nil {
		dirs := strings.Split(strings.TrimRight(string(dirOutput), "\x00"), "\x00")
		stats.TotalDirs = int64(len(dirs))
	}

	// Max depth calculation (BSD compatible)
	stats.MaxDepth = ct.calculateMaxDepthMacOS(rootPath)

	// Directory density
	if stats.TotalFiles+stats.TotalDirs > 0 {
		stats.DirDensity = float64(stats.TotalDirs) / float64(stats.TotalFiles+stats.TotalDirs) * 100
	}

	return stats, nil
}

// getDirectoryStatsWindowsPwsh uses PowerShell for Windows directory analysis
func (ct *ConcurrentTraverser) getDirectoryStatsWindowsPwsh(rootPath string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:       rootPath,
		FileTypes:  make(map[string]int64),
		MethodUsed: "powershell",
		Source:     "pwsh",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// PowerShell command to get file statistics
	psScript := fmt.Sprintf(`
		$path = "%s"
		$files = Get-ChildItem -Path $path -File -Recurse -ErrorAction SilentlyContinue
		$dirs = Get-ChildItem -Path $path -Directory -Recurse -ErrorAction SilentlyContinue

		# Count files and types
		$fileCount = $files.Count
		$dirCount = $dirs.Count

		# File extensions
		$extensions = @{}
		$files | ForEach-Object {
			$ext = $_.Extension.ToLower()
			if ($extensions.ContainsKey($ext)) {
				$extensions[$ext]++
			} else {
				$extensions[$ext] = 1
			}
		}

		# Max depth calculation
		$maxDepth = 0
		$dirs | ForEach-Object {
			$relativePath = $_.FullName.Substring($path.Length).TrimStart('\\')
			$depth = ($relativePath -split '\\').Count
			if ($depth -gt $maxDepth) { $maxDepth = $depth }
		}

		# Total size
		$totalSize = ($files | Measure-Object -Property Length -Sum).Sum

		# Output as JSON
		@{
			FileCount = $fileCount
			DirCount = $dirCount
			MaxDepth = $maxDepth
			TotalSize = $totalSize
			Extensions = $extensions
		} | ConvertTo-Json -Compress
	`, rootPath)

	cmd := exec.CommandContext(ctx, "powershell", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PowerShell command failed: %w", err)
	}

	// Parse JSON output
	var psResult struct {
		FileCount  int64            `json:"FileCount"`
		DirCount   int64            `json:"DirCount"`
		MaxDepth   int              `json:"MaxDepth"`
		TotalSize  int64            `json:"TotalSize"`
		Extensions map[string]int64 `json:"Extensions"`
	}

	if err := json.Unmarshal(output, &psResult); err != nil {
		return nil, fmt.Errorf("failed to parse PowerShell output: %w", err)
	}

	stats.TotalFiles = psResult.FileCount
	stats.TotalDirs = psResult.DirCount
	stats.MaxDepth = psResult.MaxDepth
	stats.TotalSize = psResult.TotalSize
	stats.FileTypes = psResult.Extensions

	if stats.TotalFiles+stats.TotalDirs > 0 {
		stats.DirDensity = float64(stats.TotalDirs) / float64(stats.TotalFiles+stats.TotalDirs) * 100
	}

	return stats, nil
}

// getDirectoryStatsWindowsCmd uses Windows CMD for directory analysis (fallback)
func (ct *ConcurrentTraverser) getDirectoryStatsWindowsCmd(rootPath string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:       rootPath,
		FileTypes:  make(map[string]int64),
		MethodUsed: "cmd_dir",
		Source:     "windows_cmd",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use dir command with recursive option
	cmd := exec.CommandContext(ctx, "cmd", "/C", fmt.Sprintf(`dir "%s" /S /B`, rootPath))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dir command failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	stats.TotalFiles = 0
	stats.TotalDirs = 0
	maxDepth := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if it's a directory (ends with backslash or is a directory marker)
		if strings.HasSuffix(line, "\\") || filepath.Ext(line) == "" {
			// This is a rough heuristic - could be improved with more sophisticated parsing
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			ext := strings.ToLower(filepath.Ext(line))
			stats.FileTypes[ext]++
		}

		// Calculate depth
		relPath, _ := filepath.Rel(rootPath, line)
		depth := strings.Count(relPath, "\\") + 1
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	stats.MaxDepth = maxDepth

	if stats.TotalFiles+stats.TotalDirs > 0 {
		stats.DirDensity = float64(stats.TotalDirs) / float64(stats.TotalFiles+stats.TotalDirs) * 100
	}

	return stats, nil
}

// calculateMaxDepthLinux uses GNU find's depth calculation
func (ct *ConcurrentTraverser) calculateMaxDepthLinux(rootPath string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use GNU find's printf to get depth information
	cmd := exec.CommandContext(ctx, "find", rootPath, "-type", "d", "-printf", "%d\n")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	maxDepth := 0

	for _, line := range lines {
		if depth, err := strconv.Atoi(line); err == nil && depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// calculateMaxDepthMacOS uses BSD find compatible depth calculation
func (ct *ConcurrentTraverser) calculateMaxDepthMacOS(rootPath string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// BSD find doesn't have printf, so we use binary search approach
	low, high := 0, 50

	for low < high {
		mid := (low + high + 1) / 2
		cmd := exec.CommandContext(ctx, "find", rootPath,
			"-mindepth", strconv.Itoa(mid+1),
			"-maxdepth", strconv.Itoa(mid+1),
			"-type", "d")

		if err := cmd.Run(); err != nil {
			high = mid - 1
		} else {
			low = mid
		}
	}

	return low
}

// getDirectoryStatsGo falls back to Go implementation when OS tools unavailable
func (ct *ConcurrentTraverser) getDirectoryStatsGo(rootPath string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:      rootPath,
		FileTypes: make(map[string]int64),
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		if info.IsDir() {
			stats.TotalDirs++
			// Calculate depth
			relPath, _ := filepath.Rel(rootPath, path)
			depth := strings.Count(relPath, string(os.PathSeparator))
			if depth > stats.MaxDepth {
				stats.MaxDepth = depth
			}
		} else {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
			ext := strings.ToLower(filepath.Ext(path))
			stats.FileTypes[ext]++
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Calculate directory density
	if stats.TotalFiles+stats.TotalDirs > 0 {
		stats.DirDensity = float64(stats.TotalDirs) / float64(stats.TotalFiles+stats.TotalDirs) * 100
	}

	return stats, nil
}

// detectOSCapabilities detects available OS-level tools and capabilities
func (ct *ConcurrentTraverser) detectOSCapabilities() *OSCapabilities {
	caps := &OSCapabilities{
		OS:           runtime.GOOS,
		CacheEnabled: true,
	}

	// Test for available commands (with timeout to avoid hanging)
	testCommands := map[string]string{
		"find": "find --version",
		"stat": "stat --version",
		"du":   "du --version",
		"tree": "tree --version",
	}

	// Windows-specific commands
	if caps.OS == "windows" {
		testCommands["pwsh"] = "powershell -Command Get-Host"
		testCommands["dir"] = "dir /?"
		delete(testCommands, "find") // Windows find is different, use dir instead
	}

	// Test each command
	for cmd, testCmd := range testCommands {
		if ct.testCommandAvailability(testCmd) {
			switch cmd {
			case "find":
				caps.HasFind = true
				// Determine if GNU or BSD find
				if ct.isGNUFind() {
					caps.HasGfind = true
				} else {
					caps.HasBfind = true
				}
			case "stat":
				caps.HasStat = true
			case "du":
				caps.HasDu = true
			case "tree":
				caps.HasTree = true
			case "pwsh":
				caps.HasPwsh = true
			case "dir":
				caps.HasDir = true
			}
		}
	}

	// Set preferred method based on capabilities
	caps.PreferredMethod = ct.determinePreferredMethod(caps)

	slog.Info("OS capabilities detected",
		"os", caps.OS,
		"preferred_method", caps.PreferredMethod,
		"has_find", caps.HasFind,
		"has_pwsh", caps.HasPwsh)

	return caps
}

// testCommandAvailability tests if a command is available with a timeout
func (ct *ConcurrentTraverser) testCommandAvailability(cmd string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	execCmd := exec.CommandContext(ctx, "sh", "-c", cmd)
	err := execCmd.Run()
	return err == nil
}

// isGNUFind determines if find is GNU version (Linux) or BSD version (macOS)
func (ct *ConcurrentTraverser) isGNUFind() bool {
	cmd := exec.Command("find", "--version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(output)), "gnu")
}

// determinePreferredMethod selects the best method based on detected capabilities
func (ct *ConcurrentTraverser) determinePreferredMethod(caps *OSCapabilities) string {
	switch caps.OS {
	case "linux":
		if caps.HasGfind {
			return "find_gnu" // Full feature set
		}
		return "go_fallback"

	case "darwin": // macOS
		if caps.HasBfind {
			return "find_bsd" // Limited but available
		}
		return "go_fallback"

	case "windows":
		if caps.HasPwsh {
			return "powershell"
		}
		if caps.HasDir {
			return "cmd_dir"
		}
		return "go_fallback"

	default:
		return "go_fallback"
	}
}

// getOSCapabilities returns cached OS capabilities, detecting if not already done
func (ct *ConcurrentTraverser) getOSCapabilities() *OSCapabilities {
	ct.capabilitiesMu.RLock()
	if ct.osCapabilities != nil {
		caps := ct.osCapabilities
		ct.capabilitiesMu.RUnlock()
		return caps
	}
	ct.capabilitiesMu.RUnlock()

	// Detect capabilities
	ct.capabilitiesMu.Lock()
	defer ct.capabilitiesMu.Unlock()

	// Double-check in case another goroutine detected capabilities
	if ct.osCapabilities != nil {
		return ct.osCapabilities
	}

	ct.osCapabilities = ct.detectOSCapabilities()
	return ct.osCapabilities
}

// getDirectoryStatsIntelligent intelligently chooses the best method based on OS capabilities
func (ct *ConcurrentTraverser) getDirectoryStatsIntelligent(rootPath string) (*DirectoryStats, error) {
	caps := ct.getOSCapabilities()

	// Try OS methods first if available
	if caps.PreferredMethod != "go_fallback" {
		stats, err := ct.getDirectoryStatsOSIntelligent(rootPath, caps)
		if err == nil {
			return stats, nil
		}
		slog.Debug("OS method failed, falling back to Go", "method", caps.PreferredMethod, "error", err)
	}

	// Fallback to Go implementation
	return ct.getDirectoryStatsGo(rootPath)
}

// getDirectoryStatsOSIntelligent uses the detected OS capabilities intelligently
func (ct *ConcurrentTraverser) getDirectoryStatsOSIntelligent(rootPath string, caps *OSCapabilities) (*DirectoryStats, error) {
	switch caps.PreferredMethod {
	case "find_gnu":
		return ct.getDirectoryStatsLinux(rootPath)
	case "find_bsd":
		return ct.getDirectoryStatsMacOS(rootPath)
	case "powershell":
		return ct.getDirectoryStatsWindowsPwsh(rootPath)
	case "cmd_dir":
		return ct.getDirectoryStatsWindowsCmd(rootPath)
	default:
		return nil, fmt.Errorf("no suitable OS method available")
	}
}

// logPerformanceStats logs traversal performance metrics
func (ct *ConcurrentTraverser) logPerformanceStats(stats *TraversalStats) {
	duration := stats.EndTime - stats.StartTime
	dirsProcessed := atomic.LoadInt64(&stats.DirsProcessed)
	filesProcessed := atomic.LoadInt64(&stats.FilesProcessed)
	errors := atomic.LoadInt64(&stats.ErrorsFound)

	if duration > 0 {
		dirsPerSec := float64(dirsProcessed) / float64(duration) * 1000 // Convert to per second
		filesPerSec := float64(filesProcessed) / float64(duration) * 1000

		slog.Info("Traversal completed",
			"dirs", dirsProcessed,
			"files", filesProcessed,
			"duration_ms", duration,
			"dirs_per_sec", dirsPerSec,
			"files_per_sec", filesPerSec,
			"errors", errors)
	}
}

// Cleanup releases resources used by the traverser
func (ct *ConcurrentTraverser) Cleanup() {
	if ct.cancel != nil {
		ct.cancel() // Cancel the context
	}
}
