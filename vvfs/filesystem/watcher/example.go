package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Example usage of the file system watcher
func Example() {
	// Create a context for the watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary directory for demonstration
	tempDir := "/tmp/watcher_example"
	os.MkdirAll(tempDir, 0o755)
	defer os.RemoveAll(tempDir)

	// Create watcher configuration
	config := WatcherConfig{
		DebounceDelay:      200 * time.Millisecond,
		MaxDebounceDelay:   1 * time.Second,
		BatchSize:          50,
		WorkerCount:        2,
		QueueCapacity:      100,
		FingerprintEnabled: true,
		SimhashThreshold:   0.9,
	}

	// Create a batch processor for handling events
	processor := NewReindexProcessor(2, 50)

	// Set up event handlers
	processor.SetCallbacks(
		// onCreate
		func(ctx context.Context, path string) error {
			fmt.Printf("File created: %s\n", path)
			return nil
		},
		// onWrite
		func(ctx context.Context, path string) error {
			fmt.Printf("File modified: %s\n", path)
			return nil
		},
		// onRemove
		func(ctx context.Context, path string) error {
			fmt.Printf("File removed: %s\n", path)
			return nil
		},
		// onRename
		func(ctx context.Context, oldPath, newPath string) error {
			fmt.Printf("File renamed: %s -> %s\n", oldPath, newPath)
			return nil
		},
	)

	// Start the processor
	processor.Start()

	// Create watcher with processor
	watcher, err := NewWatcherWithProcessor(config, processor)
	if err != nil {
		slog.Error("Failed to create watcher", "error", err)
		return
	}

	// Start watching
	if err := watcher.Start(ctx, []string{tempDir}); err != nil {
		slog.Error("Failed to start watcher", "error", err)
		return
	}

	fmt.Printf("Watcher started, monitoring: %s\n", tempDir)

	// Create some test files to trigger events
	go func() {
		time.Sleep(500 * time.Millisecond)

		// Create a file
		file1 := filepath.Join(tempDir, "test1.txt")
		if err := os.WriteFile(file1, []byte("Hello, World!"), 0o644); err != nil {
			slog.Error("Failed to create test file", "error", err)
			return
		}
		fmt.Printf("Created test file: %s\n", file1)

		time.Sleep(500 * time.Millisecond)

		// Modify the file
		if err := os.WriteFile(file1, []byte("Hello, Updated World!"), 0o644); err != nil {
			slog.Error("Failed to modify test file", "error", err)
			return
		}
		fmt.Printf("Modified test file: %s\n", file1)

		time.Sleep(500 * time.Millisecond)

		// Create another file
		file2 := filepath.Join(tempDir, "test2.txt")
		if err := os.WriteFile(file2, []byte("Another file"), 0o644); err != nil {
			slog.Error("Failed to create second test file", "error", err)
			return
		}
		fmt.Printf("Created second test file: %s\n", file2)

		time.Sleep(500 * time.Millisecond)

		// Rename file
		file2Renamed := filepath.Join(tempDir, "renamed.txt")
		if err := os.Rename(file2, file2Renamed); err != nil {
			slog.Error("Failed to rename test file", "error", err)
			return
		}
		fmt.Printf("Renamed file: %s -> %s\n", file2, file2Renamed)

		time.Sleep(500 * time.Millisecond)

		// Remove file
		if err := os.Remove(file1); err != nil {
			slog.Error("Failed to remove test file", "error", err)
			return
		}
		fmt.Printf("Removed test file: %s\n", file1)

		time.Sleep(500 * time.Millisecond)

		// Stop the watcher
		cancel()
	}()

	// Handle events and errors
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Watcher example completed")
			watcher.Close()
			processor.Close()
			return

		case event := <-watcher.Events():
			fmt.Printf("Received event: %v for %s\n", event.Type, event.Path)

		case err := <-watcher.Errors():
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

// Example of using the convenience function
func ExampleConvenience() {
	// Create a temporary directory
	tempDir := "/tmp/watcher_convenience"
	os.MkdirAll(tempDir, 0o755)
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the convenience function
	err := WatchPaths(ctx, []string{tempDir}, func(event Event) {
		fmt.Printf("Event: %v, Path: %s, IsDir: %v\n", event.Type, event.Path, event.IsDir)
	})
	if err != nil {
		slog.Error("WatchPaths failed", "error", err)
	}
}

// Example of using fingerprinting for change detection
func ExampleFingerprinting() {
	fp := NewSimhashFingerprinter()

	// Create a test file
	tempFile := "/tmp/fingerprint_test.txt"
	content1 := "This is the original content"
	err := os.WriteFile(tempFile, []byte(content1), 0o644)
	if err != nil {
		slog.Error("Failed to create test file", "error", err)
		return
	}
	defer os.Remove(tempFile)

	// Generate initial fingerprint
	fp1, err := fp.Fingerprint(tempFile)
	if err != nil {
		slog.Error("Failed to generate fingerprint", "error", err)
		return
	}

	fmt.Printf("Initial fingerprint: Size=%d, Hash=%d\n", fp1.Size, fp1.Hash)

	// Modify the file
	content2 := "This is the modified content with some changes"
	err = os.WriteFile(tempFile, []byte(content2), 0o644)
	if err != nil {
		slog.Error("Failed to modify test file", "error", err)
		return
	}

	// Generate new fingerprint
	fp2, err := fp.Fingerprint(tempFile)
	if err != nil {
		slog.Error("Failed to generate second fingerprint", "error", err)
		return
	}

	fmt.Printf("Modified fingerprint: Size=%d, Hash=%d\n", fp2.Size, fp2.Hash)

	// Compare fingerprints
	similarity := fp.Compare(fp1, fp2)
	fmt.Printf("Similarity: %.2f\n", similarity)

	if similarity < 0.8 {
		fmt.Println("Files are significantly different")
	} else {
		fmt.Println("Files are very similar")
	}
}
