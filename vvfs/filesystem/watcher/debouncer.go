package watcher

import (
	"context"
	"sync"
	"time"
)

// EventBatch represents a batch of events for the same path
type EventBatch struct {
	Path      string
	Events    []Event
	LastEvent Event
	Timer     *time.Timer
}

// DebouncerImpl implements the Debouncer interface
type DebouncerImpl struct {
	delay         time.Duration
	maxDelay      time.Duration
	eventChan     chan []Event
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	pendingEvents map[string]*EventBatch
}

// NewDebouncer creates a new debouncer
func NewDebouncer(delay, maxDelay time.Duration, queueCapacity int) *DebouncerImpl {
	ctx, cancel := context.WithCancel(context.Background())

	return &DebouncerImpl{
		delay:         delay,
		maxDelay:      maxDelay,
		eventChan:     make(chan []Event, queueCapacity),
		ctx:           ctx,
		cancel:        cancel,
		pendingEvents: make(map[string]*EventBatch),
	}
}

// Add adds an event to be debounced
func (d *DebouncerImpl) Add(event Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	batch, exists := d.pendingEvents[event.Path]
	if !exists {
		batch = &EventBatch{
			Path:   event.Path,
			Events: make([]Event, 0, 5), // Pre-allocate for common case
		}
		d.pendingEvents[event.Path] = batch
	}

	// Add event to batch
	batch.Events = append(batch.Events, event)
	batch.LastEvent = event

	// Reset or create timer
	if batch.Timer != nil {
		batch.Timer.Stop()
	}

	batch.Timer = time.AfterFunc(d.delay, func() {
		d.processBatch(event.Path)
	})

	// Also set max delay timer to prevent indefinite waiting
	time.AfterFunc(d.maxDelay, func() {
		d.mu.Lock()
		if b, exists := d.pendingEvents[event.Path]; exists && b.Timer != nil {
			b.Timer.Stop()
			d.processBatch(event.Path)
		}
		d.mu.Unlock()
	})
}

// Events returns the debounced events channel
func (d *DebouncerImpl) Events() <-chan []Event {
	return d.eventChan
}

// processBatch processes a batch of events for a path
func (d *DebouncerImpl) processBatch(path string) {
	d.mu.Lock()
	batch, exists := d.pendingEvents[path]
	if !exists {
		d.mu.Unlock()
		return
	}

	delete(d.pendingEvents, path)
	d.mu.Unlock()

	// Send the batch
	select {
	case d.eventChan <- batch.Events:
	case <-d.ctx.Done():
		return
	}
}

// Close stops the debouncer
func (d *DebouncerImpl) Close() {
	d.mu.Lock()
	d.cancel()

	// Stop all timers
	for _, batch := range d.pendingEvents {
		if batch.Timer != nil {
			batch.Timer.Stop()
		}
	}

	d.mu.Unlock()

	// Wait for any pending operations
	d.wg.Wait()

	close(d.eventChan)
}
