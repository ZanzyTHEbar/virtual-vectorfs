package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DefaultConfig returns a default watcher configuration
func DefaultConfig() WatcherConfig {
	return WatcherConfig{
		DebounceDelay:      100 * time.Millisecond,
		MaxDebounceDelay:   2 * time.Second,
		BatchSize:          100,
		WorkerCount:        4,
		QueueCapacity:      1000,
		FingerprintEnabled: true,
		SimhashThreshold:   0.95,
	}
}

// NewWatcher creates a new watcher based on available system capabilities
func NewWatcher(config WatcherConfig) (Watcher, error) {
	// Try fanotify first on Linux (if available and we have permissions)
	if fanotifyWatcher, err := newFanotifyWatcher(config); err == nil {
		slog.Info("Using fanotify watcher for Linux")
		return fanotifyWatcher, nil
	} else {
		slog.Debug("Fanotify not available, falling back to fsnotify", "error", err)
	}

	// Fallback to fsnotify
	slog.Info("Using fsnotify watcher")
	return NewFSNotifyWatcher(config)
}

// NewWatcherWithProcessor creates a watcher with a batch processor
func NewWatcherWithProcessor(config WatcherConfig, processor BatchProcessor) (Watcher, error) {
	watcher, err := NewWatcher(config)
	if err != nil {
		return nil, err
	}

	// If the watcher supports setting a processor, set it
	if fsWatcher, ok := watcher.(*FSNotifyWatcher); ok {
		fsWatcher.SetProcessor(processor)
	}

	return watcher, nil
}

// WatchPaths is a convenience function for watching paths with default configuration
func WatchPaths(ctx context.Context, paths []string, eventHandler func(Event)) error {
	config := DefaultConfig()
	config.DebounceDelay = 500 * time.Millisecond // Slightly longer debounce for convenience

	watcher, err := NewWatcher(config)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Create simple processor
	processor := NewSimpleProcessor(func(ctx context.Context, event Event) error {
		eventHandler(event)
		return nil
	})

	// Start with processor
	if watcherWithProcessor, ok := watcher.(interface {
		SetProcessor(BatchProcessor)
	}); ok {
		watcherWithProcessor.SetProcessor(processor)
	}

	// Start watching
	if err := watcher.Start(ctx, paths); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to start watching: %w", err)
	}

	// Handle events and errors
	go func() {
		for {
			select {
			case <-ctx.Done():
				watcher.Close()
				return

			case event := <-watcher.Events():
				slog.Debug("Received event", "type", event.Type, "path", event.Path)

			case err := <-watcher.Errors():
				slog.Error("Watcher error", "error", err)
			}
		}
	}()

	return nil
}

// SetProcessor sets the processor for a watcher (if supported)
func (w *FSNotifyWatcher) SetProcessor(processor BatchProcessor) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.processor = processor
}
