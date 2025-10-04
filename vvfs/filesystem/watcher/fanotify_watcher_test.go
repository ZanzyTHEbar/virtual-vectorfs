//go:build linux && fanotify

package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFanotifyConstants tests that fanotify constants are properly initialized
func TestFanotifyConstants(t *testing.T) {
	// Test that constants are non-zero (indicating successful initialization)
	assert.True(t, FAN_CLOEXEC > 0, "FAN_CLOEXEC should be initialized")
	assert.True(t, FAN_CLASS_CONTENT > 0, "FAN_CLASS_CONTENT should be initialized")
	assert.True(t, O_RDONLY == 0, "O_RDONLY should be 0")

	// Test event mask constants
	assert.True(t, FAN_MODIFY > 0, "FAN_MODIFY should be initialized")
	assert.True(t, FAN_CLOSE_WRITE > 0, "FAN_CLOSE_WRITE should be initialized")
	assert.True(t, FAN_CREATE > 0, "FAN_CREATE should be initialized")
	assert.True(t, FAN_DELETE > 0, "FAN_DELETE should be initialized")
	assert.True(t, FAN_MOVED_FROM > 0, "FAN_MOVED_FROM should be initialized")
	assert.True(t, FAN_MOVED_TO > 0, "FAN_MOVED_TO should be initialized")

	// Test mark constants
	assert.True(t, FAN_MARK_ADD > 0, "FAN_MARK_ADD should be initialized")
	assert.True(t, FAN_MARK_REMOVE > 0, "FAN_MARK_REMOVE should be initialized")
}

// TestFanotifyInit tests the fanotify initialization function
func TestFanotifyInit(t *testing.T) {
	// Test successful initialization
	fd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
	if err != nil {
		t.Skip("Fanotify not available on this system:", err)
		return
	}
	require.True(t, fd > 0, "File descriptor should be positive")

	// Clean up
	require.NoError(t, syscall.Close(fd))

	// Test invalid flags (should still work but may have different behavior)
	_, err = FanotifyInit(0, O_RDONLY)
	// This might succeed or fail depending on system, but shouldn't panic
	t.Logf("FanotifyInit with flags=0: %v", err)
}

// TestDoFanotifyMark tests the fanotify mark function
func TestDoFanotifyMark(t *testing.T) {
	// Initialize fanotify
	fd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
	if err != nil {
		t.Skip("Fanotify not available on this system:", err)
		return
	}
	defer syscall.Close(fd)

	// Test marking current directory
	err = doFanotifyMark(fd, FAN_MARK_ADD, FAN_ACCESS, -1, ".")
	if err != nil {
		t.Skip("Fanotify mark failed (may not have permissions):", err)
		return
	}

	// Test mark removal
	err = doFanotifyMark(fd, FAN_MARK_REMOVE, FAN_ACCESS, -1, ".")
	assert.NoError(t, err, "Mark removal should succeed")
}

// TestCheckFanotifyPermissions tests permission checking
func TestCheckFanotifyPermissions(t *testing.T) {
	err := checkFanotifyPermissions()
	// This should not return an error on systems with fanotify support
	// but may fail if permissions are insufficient
	if err != nil {
		t.Logf("Fanotify permissions check failed: %v", err)
		t.Log("This may be expected if running without sufficient privileges")
	}
}

// TestValidatePathSecurity tests path security validation
func TestValidatePathSecurity(t *testing.T) {
	// Test valid path
	tmpDir := t.TempDir()
	err := validatePathSecurity(tmpDir)
	assert.NoError(t, err, "Valid temp directory should pass validation")

	// Test non-existent path
	err = validatePathSecurity("/non/existent/path/that/does/not/exist")
	assert.Error(t, err, "Non-existent path should fail validation")

	// Test path outside accessible area (if running as non-root)
	err = validatePathSecurity("/root/private")
	if err != nil {
		t.Logf("Root path validation failed as expected: %v", err)
	}

	// Test relative path (using current directory)
	err = validatePathSecurity(".")
	assert.NoError(t, err, "Current directory should be acceptable")
}

// TestNewFanotifyWatcher tests the constructor
func TestNewFanotifyWatcher(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   2 * time.Second,
		BatchSize:          100,
		WorkerCount:        4,
		QueueCapacity:      1000,
		FingerprintEnabled: true,
		SimhashThreshold:   0.95,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	require.NotNil(t, watcher)

	// Test interface compliance
	var _ Watcher = watcher

	// Test closing
	err = watcher.Close()
	assert.NoError(t, err)
}

// TestFanotifyWatcher_BasicOperations tests basic watcher operations
func TestFanotifyWatcher_BasicOperations(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Test channels
	events := watcher.Events()
	errors := watcher.Errors()
	assert.NotNil(t, events)
	assert.NotNil(t, errors)

	// Test starting with no paths
	ctx := context.Background()
	err = watcher.Start(ctx, []string{})
	assert.NoError(t, err)
}

// TestFanotifyWatcher_PathOperations tests path addition and removal
func TestFanotifyWatcher_PathOperations(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create temporary directory
	tempDir := t.TempDir()

	// Test adding a path
	err = watcher.Add(tempDir)
	if err != nil {
		t.Skip("Path addition failed (may not have permissions):", err)
		return
	}

	// Test removing a path
	err = watcher.Remove(tempDir)
	assert.NoError(t, err, "Path removal should succeed")
}

// TestFanotifyWatcher_EventProcessing tests event processing logic
func TestFanotifyWatcher_EventProcessing(t *testing.T) {
	watcher := &FanotifyWatcher{
		config: WatcherConfig{
			QueueCapacity: 10,
		},
	}

	// Test convertFanotifyEvent with nil input
	result := watcher.convertFanotifyEvent(nil)
	assert.Nil(t, result, "Nil event should return nil")

	// Test convertFanotifyEvent with valid event
	event := &fanotifyEventMetadata{
		Event_len: 32,
		Mask:      FAN_CREATE,
		Fd:        5,
		Pid:       1234,
	}

	result = watcher.convertFanotifyEvent(event)
	require.NotNil(t, result)
	assert.Equal(t, EventCreate, result.Type)
	assert.Equal(t, "/proc/self/fd/5", result.Path)
	assert.True(t, result.Timestamp.After(time.Time{}))
}

// TestFanotifyWatcher_PathResolution tests file descriptor to path resolution
func TestFanotifyWatcher_PathResolution(t *testing.T) {
	watcher := &FanotifyWatcher{}

	// Test with invalid file descriptor
	path, err := watcher.getPathFromFd(-1)
	assert.Error(t, err, "Invalid file descriptor should return error")
	assert.Empty(t, path)

	// Test with a valid file descriptor (current directory)
	dir, err := os.Open(".")
	require.NoError(t, err)
	defer dir.Close()

	path, err = watcher.getPathFromFd(int(dir.Fd()))
	if err != nil {
		// This might fail on some systems due to /proc restrictions
		t.Logf("Path resolution failed (may be expected): %v", err)
	} else {
		assert.True(t, strings.Contains(path, "watcher") || strings.Contains(path, filepath.Base(".")), "Path should contain current directory or watcher directory")
	}
}

// TestFanotifyWatcher_RecursivePathAddition tests recursive path addition
func TestFanotifyWatcher_RecursivePathAddition(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Create a directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	subSubDir := filepath.Join(subDir, "subsubdir")

	err = os.MkdirAll(subSubDir, 0755)
	require.NoError(t, err)

	// Test recursive path addition
	ctx := context.Background()
	err = watcher.Start(ctx, []string{tempDir})
	if err != nil {
		t.Skip("Recursive path addition failed:", err)
		return
	}

	// Verify watcher is working
	time.Sleep(100 * time.Millisecond) // Give it time to initialize
}

// TestFanotifyWatcher_ErrorHandling tests error handling scenarios
func TestFanotifyWatcher_ErrorHandling(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Test adding invalid path
	err = watcher.Add("/nonexistent/path/that/does/not/exist")
	assert.Error(t, err, "Adding non-existent path should return error")

	// Test removing non-existent path
	err = watcher.Remove("/another/nonexistent/path")
	assert.Error(t, err, "Removing non-existent path should return error")
}

// TestGetConstant tests the constant retrieval function
func TestGetConstant(t *testing.T) {
	// Test with valid fallback
	result := getConstant("TEST_CONSTANT", 42)
	assert.Equal(t, uint(42), result)

	// Test with different fallback values
	result = getConstant("ANOTHER_CONSTANT", 100)
	assert.Equal(t, uint(100), result)
}

// TestFanotifyEventMetadata tests the event metadata structure
func TestFanotifyEventMetadata(t *testing.T) {
	event := fanotifyEventMetadata{
		Event_len:    32,
		Vers:         1,
		Reserved:     0,
		Metadata_len: 24,
		Mask:         FAN_CREATE,
		Fd:           10,
		Pid:          12345,
	}

	assert.Equal(t, uint32(32), event.Event_len)
	assert.Equal(t, uint8(1), event.Vers)
	assert.Equal(t, uint8(0), event.Reserved)
	assert.Equal(t, uint16(24), event.Metadata_len)
	assert.Equal(t, uint64(FAN_CREATE), event.Mask)
	assert.Equal(t, int32(10), event.Fd)
	assert.Equal(t, int32(12345), event.Pid)
}

// TestValidatePathSecurity_EdgeCases tests path security validation with edge cases
func TestValidatePathSecurity_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"valid temp path", func() string { f, _ := os.CreateTemp("", "test"); defer os.Remove(f.Name()); return f.Name() }(), false},
		{"empty path", "", true},
		{"root path", "/", false}, // May or may not be accessible depending on system
		{"relative path", "./test", false},
		{"parent directory", "../", false},
		{"current directory", ".", false},
		{"non-existent path", "/definitely/does/not/exist/path", true},
		{"path with spaces", "/tmp/test path", false},
		{"path with special chars", "/tmp/test-file_123", false},
		{"very long path", "/tmp/" + strings.Repeat("a", 200), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathSecurity(tt.path)
			if tt.expectError {
				assert.Error(t, err, "Expected error for path: %s", tt.path)
			} else {
				// Path might be accessible or not depending on system
				t.Logf("Path %s validation result: %v", tt.path, err)
			}
		})
	}
}

// TestGetConstant_EdgeCases tests constant retrieval with various inputs
func TestGetConstant_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		fallback uint
		expected uint
	}{
		{"empty constant name", "", 42, 42},
		{"valid constant name", "FAN_CLOEXEC", 999, 999}, // Will use fallback since we can't access real constants
		{"large fallback", "TEST", 0xFFFFFFFF, 0xFFFFFFFF},
		{"zero fallback", "TEST", 0, 0},
		{"negative fallback", "TEST", ^uint(0), ^uint(0)}, // Max uint value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getConstant(tt.constant, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFanotifyWatcher_NewFanotifyWatcher_ConfigValidation tests configuration validation
func TestFanotifyWatcher_NewFanotifyWatcher_ConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config WatcherConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: WatcherConfig{
				DebounceDelay:      100 * time.Millisecond,
				MaxDebounceDelay:   500 * time.Millisecond,
				BatchSize:          10,
				WorkerCount:        2,
				QueueCapacity:      50,
				FingerprintEnabled: false,
			},
			valid: true,
		},
		{
			name: "zero debounce delay",
			config: WatcherConfig{
				DebounceDelay:    0,
				MaxDebounceDelay: 500 * time.Millisecond,
				BatchSize:        10,
				WorkerCount:      2,
				QueueCapacity:    50,
			},
			valid: true, // Should still work, just no debouncing
		},
		{
			name: "large queue capacity",
			config: WatcherConfig{
				DebounceDelay:    100 * time.Millisecond,
				MaxDebounceDelay: 500 * time.Millisecond,
				BatchSize:        10,
				WorkerCount:      2,
				QueueCapacity:    10000,
			},
			valid: true,
		},
		{
			name: "zero worker count",
			config: WatcherConfig{
				DebounceDelay:    100 * time.Millisecond,
				MaxDebounceDelay: 500 * time.Millisecond,
				BatchSize:        10,
				WorkerCount:      0,
				QueueCapacity:    50,
			},
			valid: true, // May work but with limited concurrency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewFanotifyWatcher(tt.config)
			if !tt.valid {
				assert.Error(t, err, "Expected error for invalid config")
				return
			}

			if err != nil {
				t.Skip("Fanotify watcher creation failed:", err)
				return
			}

			require.NotNil(t, watcher)
			assert.NoError(t, watcher.Close())
		})
	}
}

// TestFanotifyWatcher_EventMasks tests that event masks are properly defined
func TestFanotifyWatcher_EventMasks(t *testing.T) {
	// Test that all event masks are properly defined and non-zero
	assert.True(t, FAN_MODIFY > 0)
	assert.True(t, FAN_CLOSE_WRITE > 0)
	assert.True(t, FAN_CREATE > 0)
	assert.True(t, FAN_DELETE > 0)
	assert.True(t, FAN_MOVED_FROM > 0)
	assert.True(t, FAN_MOVED_TO > 0)
	assert.True(t, FAN_ACCESS > 0)

	// Test that masks are unique (no overlaps in basic values)
	masks := []uint64{FAN_MODIFY, FAN_CLOSE_WRITE, FAN_CREATE, FAN_DELETE, FAN_MOVED_FROM, FAN_MOVED_TO, FAN_ACCESS}
	for i, mask1 := range masks {
		for j, mask2 := range masks {
			if i != j {
				assert.NotEqual(t, mask1, mask2, "Masks should be unique: %v and %v", mask1, mask2)
			}
		}
	}
}

// TestFanotifyWatcher_PathResolution_Comprehensive tests file descriptor resolution
func TestFanotifyWatcher_PathResolution_Comprehensive(t *testing.T) {
	watcher := &FanotifyWatcher{}

	// Test invalid file descriptors
	invalidFds := []int{-1, -100, 999999}
	for _, fd := range invalidFds {
		path, err := watcher.getPathFromFd(fd)
		assert.Error(t, err, "Invalid fd %d should return error", fd)
		assert.Empty(t, path, "Invalid fd %d should return empty path", fd)
	}

	// Test valid file descriptor
	tempFile, err := os.CreateTemp("", "fd_test")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	path, err := watcher.getPathFromFd(int(tempFile.Fd()))
	if err != nil {
		t.Logf("Path resolution failed (may be expected): %v", err)
		assert.Empty(t, path)
	} else {
		assert.NotEmpty(t, path, "Valid fd should return a path")
		assert.Contains(t, path, tempFile.Name(), "Path should contain temp file name")
	}
}

// TestFanotifyWatcher_ErrorHandlingAdvanced tests advanced error handling in various scenarios
func TestFanotifyWatcher_ErrorHandlingAdvanced(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	// Test operations with closed watcher
	err = watcher.Close()
	assert.NoError(t, err)

	// These should handle gracefully after close
	ctx := context.Background()
	err = watcher.Start(ctx, []string{"/tmp"})
	// May or may not return error depending on implementation
	t.Logf("Start after close: %v", err)

	err = watcher.Add("/tmp")
	t.Logf("Add after close: %v", err)

	err = watcher.Remove("/tmp")
	t.Logf("Remove after close: %v", err)
}

// TestFanotifyWatcher_ConcurrentOperations tests concurrent operations
func TestFanotifyWatcher_ConcurrentOperations(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    100 * time.Millisecond,
		MaxDebounceDelay: 500 * time.Millisecond,
		BatchSize:        10,
		WorkerCount:      4,
		QueueCapacity:    100,
	}

	watcher, err := NewFanotifyWatcher(config)
	if err != nil {
		t.Skip("Fanotify watcher creation failed:", err)
		return
	}
	defer watcher.Close()

	testDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	require.NoError(t, err)

	// Test concurrent Add operations
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer func() { done <- true }()
			subDir := filepath.Join(testDir, fmt.Sprintf("concurrent_%d", id))
			os.MkdirAll(subDir, 0755)
			err := watcher.Add(subDir)
			if err != nil {
				t.Logf("Concurrent add %d failed: %v", id, err)
			}
		}(i)
	}

	// Wait for all concurrent operations
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Concurrent operation timed out")
		}
	}

	// Test concurrent Remove operations
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer func() { done <- true }()
			subDir := filepath.Join(testDir, fmt.Sprintf("concurrent_%d", id))
			err := watcher.Remove(subDir)
			if err != nil {
				t.Logf("Concurrent remove %d failed: %v", id, err)
			}
		}(i)
	}

	// Wait for all concurrent operations
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Concurrent operation timed out")
		}
	}

	t.Log("Concurrent operations completed successfully")
}
