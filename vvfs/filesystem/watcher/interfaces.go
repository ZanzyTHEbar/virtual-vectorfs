package watcher

import (
	"context"
	"time"
)

// EventType represents the type of file system event
type EventType int

const (
	// EventCreate represents file/directory creation
	EventCreate EventType = iota
	// EventWrite represents file modification
	EventWrite
	// EventRemove represents file/directory removal
	EventRemove
	// EventRename represents file/directory rename
	EventRename
	// EventChmod represents permission changes
	EventChmod
)

// Event represents a file system event
type Event struct {
	Type      EventType
	Path      string
	OldPath   string // For rename events
	Timestamp time.Time
	IsDir     bool
}

// Watcher defines the interface for file system watching
type Watcher interface {
	// Start begins watching the specified paths
	Start(ctx context.Context, paths []string) error

	// Events returns a channel of file system events
	Events() <-chan Event

	// Errors returns a channel of errors encountered during watching
	Errors() <-chan error

	// Close stops watching and cleans up resources
	Close() error

	// Add adds paths to watch
	Add(paths ...string) error

	// Remove removes paths from watching
	Remove(paths ...string) error
}

// WatcherConfig holds configuration for the watcher
type WatcherConfig struct {
	// DebounceDelay is the time to wait before processing events
	DebounceDelay time.Duration

	// MaxDebounceDelay is the maximum time to wait before processing events
	MaxDebounceDelay time.Duration

	// BatchSize is the maximum number of events to process in a batch
	BatchSize int

	// WorkerCount is the number of workers for processing events
	WorkerCount int

	// QueueCapacity is the capacity of the event processing queue
	QueueCapacity int

	// FingerprintEnabled enables file fingerprinting for change detection
	FingerprintEnabled bool

	// SimhashThreshold is the similarity threshold for simhash comparison
	SimhashThreshold float64
}

// Fingerprint represents a file's fingerprint for change detection
type Fingerprint struct {
	Path    string
	Size    int64
	ModTime time.Time
	Hash    uint64 // Simhash of file content
}

// Debouncer handles event debouncing
type Debouncer interface {
	// Add adds an event to be debounced
	Add(event Event)

	// Events returns debounced events
	Events() <-chan []Event

	// Close stops the debouncer
	Close()
}

// Fingerprinter generates fingerprints for files
type Fingerprinter interface {
	// Fingerprint generates a fingerprint for the given path
	Fingerprint(path string) (*Fingerprint, error)

	// Compare compares two fingerprints and returns similarity score
	Compare(a, b *Fingerprint) float64
}

// BatchProcessor processes events in batches
type BatchProcessor interface {
	// Process processes a batch of events
	Process(ctx context.Context, events []Event) error

	// Close stops the processor
	Close() error
}
