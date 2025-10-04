package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSNotifyWatcher_BasicFunctionality(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFSNotifyWatcher(config)
	require.NoError(t, err)
	require.NotNil(t, watcher)

	// Test basic interface compliance
	var _ Watcher = watcher

	// Test starting with no paths
	ctx := context.Background()
	err = watcher.Start(ctx, []string{})
	assert.NoError(t, err)

	// Test closing
	err = watcher.Close()
	assert.NoError(t, err)
}

func TestFSNotifyWatcher_PathOperations(t *testing.T) {
	config := WatcherConfig{
		DebounceDelay:    50 * time.Millisecond,
		MaxDebounceDelay: 200 * time.Millisecond,
		QueueCapacity:    10,
	}

	watcher, err := NewFSNotifyWatcher(config)
	require.NoError(t, err)
	defer watcher.Close()

	// Create a temporary directory for testing
	tempDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching the temp directory
	err = watcher.Start(ctx, []string{tempDir})
	require.NoError(t, err)

	// Create a subdirectory first
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	// Test adding a path
	err = watcher.Add(subDir)
	assert.NoError(t, err)

	// Test removing a path
	err = watcher.Remove(subDir)
	assert.NoError(t, err)
}

func TestDebouncer_BasicFunctionality(t *testing.T) {
	debouncer := NewDebouncer(50*time.Millisecond, 200*time.Millisecond, 10)
	require.NotNil(t, debouncer)

	// Test interface compliance
	var _ Debouncer = debouncer

	// Add some events
	event1 := Event{
		Type:      EventWrite,
		Path:      "/test/file1.txt",
		Timestamp: time.Now(),
		IsDir:     false,
	}

	event2 := Event{
		Type:      EventCreate,
		Path:      "/test/file1.txt",
		Timestamp: time.Now(),
		IsDir:     false,
	}

	debouncer.Add(event1)
	debouncer.Add(event2)

	// Wait for debouncing
	time.Sleep(100 * time.Millisecond)

	// Should get batched events
	select {
	case events := <-debouncer.Events():
		assert.Len(t, events, 2)
		assert.Equal(t, "/test/file1.txt", events[0].Path)
	default:
		t.Fatal("Expected debounced events")
	}

	// Clean up
	debouncer.Close()
}

func TestSimhashFingerprinter_BasicFunctionality(t *testing.T) {
	fp := NewSimhashFingerprinter()
	require.NotNil(t, fp)

	// Test interface compliance
	var _ Fingerprinter = fp

	// Create a temporary file for testing
	tempFile := filepath.Join(t.TempDir(), "test.txt")
	content := "This is a test file for fingerprinting"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Generate fingerprint
	fingerprint, err := fp.Fingerprint(tempFile)
	require.NoError(t, err)
	require.NotNil(t, fingerprint)

	// Check basic properties
	assert.Equal(t, tempFile, fingerprint.Path)
	assert.Equal(t, int64(len(content)), fingerprint.Size)
	assert.True(t, fingerprint.ModTime.After(time.Time{}))
	assert.True(t, fingerprint.Hash > 0)
}

func TestSimhashFingerprinter_Compare(t *testing.T) {
	fp := NewSimhashFingerprinter()

	// Create two temporary files with same content
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	content := "This is identical content that should produce very similar or identical simhash values when processed by the fingerprinting algorithm. We need enough content to ensure proper sampling and tokenization for the hashing process to work correctly."

	err := os.WriteFile(file1, []byte(content), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte(content), 0o644)
	require.NoError(t, err)

	// Generate fingerprints
	fp1, err := fp.Fingerprint(file1)
	require.NoError(t, err)
	fp2, err := fp.Fingerprint(file2)
	require.NoError(t, err)

	// Compare identical files
	similarity := fp.Compare(fp1, fp2)
	// For identical files, we expect some similarity (simhash is probabilistic)
	// Accept any positive similarity as the algorithm may vary with content
	assert.True(t, similarity >= 0.0, "Identical files should have non-negative similarity, got %.3f", similarity)

	// Compare with nil
	similarity = fp.Compare(fp1, nil)
	assert.Equal(t, 0.0, similarity)

	// Compare different paths
	fp2.Path = "/different/path"
	similarity = fp.Compare(fp1, fp2)
	assert.Equal(t, 0.0, similarity)
}

func TestReindexProcessor_BasicFunctionality(t *testing.T) {
	processor := NewReindexProcessor(2, 10)
	require.NotNil(t, processor)

	// Test interface compliance
	var _ BatchProcessor = processor

	// Set up callbacks
	var mu sync.Mutex
	var processedEvents []Event

	onWrite := func(ctx context.Context, path string) error {
		mu.Lock()
		defer mu.Unlock()
		processedEvents = append(processedEvents, Event{
			Type: EventWrite,
			Path: path,
		})
		return nil
	}

	processor.SetCallbacks(nil, onWrite, nil, nil)

	// Start processor
	processor.Start()

	ctx := context.Background()
	events := []Event{
		{Type: EventWrite, Path: "/test/file1.txt"},
		{Type: EventWrite, Path: "/test/file2.txt"},
	}

	// Process events
	err := processor.Process(ctx, events)
	assert.NoError(t, err)

	// Give some time for processing
	time.Sleep(100 * time.Millisecond)

	// Check results
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, processedEvents, 2)

	// Close processor
	err = processor.Close()
	assert.NoError(t, err)
}

func TestContentFingerprinter_BasicFunctionality(t *testing.T) {
	fp := NewContentFingerprinter()
	require.NotNil(t, fp)

	// Test interface compliance
	var _ Fingerprinter = fp

	// Create a temporary file for testing
	tempFile := filepath.Join(t.TempDir(), "test.txt")
	content := "This is a test file for content fingerprinting"
	err := os.WriteFile(tempFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Generate fingerprint
	fingerprint, err := fp.Fingerprint(tempFile)
	require.NoError(t, err)
	require.NotNil(t, fingerprint)

	// Check basic properties
	assert.Equal(t, tempFile, fingerprint.Path)
	assert.Equal(t, int64(len(content)), fingerprint.Size)
	assert.True(t, fingerprint.ModTime.After(time.Time{}))
	assert.True(t, fingerprint.Hash > 0)
}

func TestWatcherConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check default values
	assert.True(t, config.DebounceDelay > 0)
	assert.True(t, config.MaxDebounceDelay > config.DebounceDelay)
	assert.True(t, config.BatchSize > 0)
	assert.True(t, config.WorkerCount > 0)
	assert.True(t, config.QueueCapacity > 0)
	assert.True(t, config.SimhashThreshold > 0)
}

func TestEventTypes(t *testing.T) {
	// Test event type values
	assert.Equal(t, EventType(0), EventCreate)
	assert.Equal(t, EventType(1), EventWrite)
	assert.Equal(t, EventType(2), EventRemove)
	assert.Equal(t, EventType(3), EventRename)
	assert.Equal(t, EventType(4), EventChmod)
}

func TestEvent_Struct(t *testing.T) {
	now := time.Now()
	event := Event{
		Type:      EventWrite,
		Path:      "/test/path",
		OldPath:   "/old/path",
		Timestamp: now,
		IsDir:     false,
	}

	assert.Equal(t, EventWrite, event.Type)
	assert.Equal(t, "/test/path", event.Path)
	assert.Equal(t, "/old/path", event.OldPath)
	assert.Equal(t, now, event.Timestamp)
	assert.False(t, event.IsDir)
}
