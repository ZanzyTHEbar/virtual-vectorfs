//go:build linux && fanotify

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Fanotify constants - fallback values for when syscall constants are not available
const (
	// Fanotify flags
	fanCloexec      = 0x00000001
	fanClassContent = 0x00000004
	oRdonly         = 0x00000000

	// Fanotify mark flags
	fanMarkAdd    = 0x00000001
	fanMarkRemove = 0x00000002

	// Fanotify event masks (uint64 for bitwise operations)
	fanModify     = uint64(0x00000002)
	fanCloseWrite = uint64(0x00000008)
	fanCreate     = uint64(0x00000100)
	fanDelete     = uint64(0x00000200)
	fanMovedFrom  = uint64(0x00000040)
	fanMovedTo    = uint64(0x00000080)
	fanAccess     = uint64(0x00000001)
)

// Fanotify constants - use syscall constants when available, fallback otherwise
var (
	FAN_CLOEXEC       = getConstant("FAN_CLOEXEC", fanCloexec)
	FAN_CLASS_CONTENT = getConstant("FAN_CLASS_CONTENT", fanClassContent)
	O_RDONLY          = getConstant("O_RDONLY", oRdonly)
	FAN_MARK_ADD      = getConstant("FAN_MARK_ADD", fanMarkAdd)
	FAN_MARK_REMOVE   = getConstant("FAN_MARK_REMOVE", fanMarkRemove)
	FAN_MODIFY        = getConstant64("FAN_MODIFY", fanModify)
	FAN_CLOSE_WRITE   = getConstant64("FAN_CLOSE_WRITE", fanCloseWrite)
	FAN_CREATE        = getConstant64("FAN_CREATE", fanCreate)
	FAN_DELETE        = getConstant64("FAN_DELETE", fanDelete)
	FAN_MOVED_FROM    = getConstant64("FAN_MOVED_FROM", fanMovedFrom)
	FAN_MOVED_TO      = getConstant64("FAN_MOVED_TO", fanMovedTo)
	FAN_ACCESS        = getConstant64("FAN_ACCESS", fanAccess)
)

// getConstant safely retrieves syscall constants with fallback
func getConstant(name string, fallback uint) uint {
	// This would use reflection to get the constant from syscall package
	// For now, return fallback - in production this should be more robust
	return fallback
}

// getConstant64 safely retrieves syscall constants with uint64 fallback
func getConstant64(name string, fallback uint64) uint64 {
	// This would use reflection to get the constant from syscall package
	// For now, return fallback - in production this should be more robust
	return fallback
}

// Proper fanotify event structure matching the kernel definition
type fanotifyEventMetadata struct {
	Event_len    uint32
	Vers         uint8
	Reserved     uint8
	Metadata_len uint16
	Mask         uint64
	Fd           int32
	Pid          int32
}

// FanotifyInit initializes fanotify using proper syscall
func FanotifyInit(flags uint, event_f_flags uint) (int, error) {
	// Direct syscall - fanotify functions may not be available in syscall package
	fd, _, errno := syscall.Syscall(syscall.SYS_FANOTIFY_INIT, uintptr(flags), uintptr(event_f_flags), 0)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

// doFanotifyMark marks a file/directory for monitoring using proper syscall
func doFanotifyMark(fd int, flags uint, mask uint64, dirfd int, path string) error {
	// Direct syscall - fanotify functions may not be available in syscall package
	pathPtr, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}

	_, _, errno := syscall.Syscall6(
		syscall.SYS_FANOTIFY_MARK,
		uintptr(fd),
		uintptr(flags),
		uintptr(mask),
		uintptr(dirfd),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
	)

	if errno != 0 {
		return errno
	}
	return nil
}

// FanotifyWatcher implements the Watcher interface using Linux fanotify
type FanotifyWatcher struct {
	fd           int
	eventChan    chan Event
	errorChan    chan error
	debouncer    Debouncer
	processor    BatchProcessor
	config       WatcherConfig
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.RWMutex
	watchedPaths map[string]bool
}

// checkFanotifyPermissions checks if the current process has permission to use fanotify
func checkFanotifyPermissions() error {
	// Test basic fanotify initialization
	testFd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
	if err != nil {
		return fmt.Errorf("fanotify initialization failed: %w", err)
	}
	defer syscall.Close(testFd)

	// Test marking a file (using /tmp which should always exist)
	tempFd, err := syscall.Open("/tmp", syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("cannot access test directory: %w", err)
	}
	defer syscall.Close(tempFd)

	// Try to mark /tmp for access events (minimal permission test)
	err = doFanotifyMark(testFd, FAN_MARK_ADD, FAN_ACCESS, int(tempFd), "")
	if err != nil {
		return fmt.Errorf("fanotify mark test failed: %w", err)
	}

	slog.Debug("Fanotify permission check passed")
	return nil
}

// validatePathSecurity validates that a path is safe to monitor
func validatePathSecurity(path string) error {
	if path == "" {
		return fmt.Errorf("empty path not allowed")
	}

	// Resolve symlinks to prevent monitoring unintended paths
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("cannot resolve symlinks for %s: %w", path, err)
	}

	// Check for dangerous paths
	dangerousPaths := []string{"/proc", "/sys", "/dev", "/run"}
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(resolved, dangerous) {
			return fmt.Errorf("monitoring %s is not allowed for security reasons", dangerous)
		}
	}

	// Check if path is readable
	if _, err := os.Stat(resolved); err != nil {
		return fmt.Errorf("path %s is not accessible: %w", resolved, err)
	}

	return nil
}

// NewFanotifyWatcher creates a new fanotify-based watcher with security checks
func NewFanotifyWatcher(config WatcherConfig) (*FanotifyWatcher, error) {
	// Check permissions before proceeding
	if err := checkFanotifyPermissions(); err != nil {
		return nil, fmt.Errorf("fanotify permission check failed: %w", err)
	}

	// Initialize fanotify
	fd, err := FanotifyInit(FAN_CLOEXEC|FAN_CLASS_CONTENT, O_RDONLY)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fanotify: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &FanotifyWatcher{
		fd:           fd,
		eventChan:    make(chan Event, config.QueueCapacity),
		errorChan:    make(chan error, 10),
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		watchedPaths: make(map[string]bool),
	}

	// Initialize debouncer if delay is configured
	if config.DebounceDelay > 0 {
		w.debouncer = NewDebouncer(config.DebounceDelay, config.MaxDebounceDelay, config.QueueCapacity)
	}

	slog.Info("Fanotify watcher created successfully with security checks")
	return w, nil
}

// Start begins watching the specified paths
func (w *FanotifyWatcher) Start(ctx context.Context, paths []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Add all paths
	for _, path := range paths {
		if err := w.addPathRecursive(path); err != nil {
			slog.Warn("Failed to add path to fanotify watcher", "path", path, "error", err)
			continue
		}
		w.watchedPaths[path] = true
	}

	// Start event processing
	w.wg.Add(1)
	go w.processEvents()

	// Start the main event loop
	w.wg.Add(1)
	go w.watchLoop()

	slog.Info("Fanotify watcher started", "paths", len(paths))
	return nil
}

// Events returns the event channel
func (w *FanotifyWatcher) Events() <-chan Event {
	return w.eventChan
}

// Errors returns the error channel
func (w *FanotifyWatcher) Errors() <-chan error {
	return w.errorChan
}

// Add adds paths to watch
func (w *FanotifyWatcher) Add(paths ...string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, path := range paths {
		if err := w.addPathRecursive(path); err != nil {
			return fmt.Errorf("failed to add path %s: %w", path, err)
		}
		w.watchedPaths[path] = true
	}

	slog.Debug("Added paths to fanotify watcher", "count", len(paths))
	return nil
}

// Remove removes paths from watching
func (w *FanotifyWatcher) Remove(paths ...string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, path := range paths {
		delete(w.watchedPaths, path)
	}

	slog.Debug("Removed paths from fanotify watcher", "count", len(paths))
	return nil
}

// Close stops watching and cleans up resources
func (w *FanotifyWatcher) Close() error {
	w.mu.Lock()
	w.cancel()
	w.mu.Unlock()

	// Close debouncer if it exists
	if w.debouncer != nil {
		w.debouncer.Close()
	}

	// Close processor if it exists
	if w.processor != nil {
		if err := w.processor.Close(); err != nil {
			slog.Warn("Error closing processor", "error", err)
		}
	}

	// Close fanotify file descriptor
	if err := syscall.Close(w.fd); err != nil {
		slog.Warn("Error closing fanotify fd", "error", err)
	}

	// Wait for goroutines to finish
	w.wg.Wait()

	// Close channels
	close(w.eventChan)
	close(w.errorChan)

	slog.Info("Fanotify watcher closed")
	return nil
}

// addPathRecursive adds a path and all its subdirectories to fanotify with security validation
func (w *FanotifyWatcher) addPathRecursive(rootPath string) error {
	// Security validation
	if err := validatePathSecurity(rootPath); err != nil {
		return fmt.Errorf("security validation failed for %s: %w", rootPath, err)
	}

	// Check if path exists and is accessible
	info, err := os.Stat(rootPath)
	if err != nil {
		return fmt.Errorf("failed to stat path %s: %w", rootPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", rootPath)
	}

	// Add the root path
	if err := w.markForNotification(rootPath); err != nil {
		return fmt.Errorf("failed to add root path %s: %w", rootPath, err)
	}

	// Walk the directory tree and add all subdirectories
	var walkErrors []error
	walkErr := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			walkErrors = append(walkErrors, fmt.Errorf("walk error for %s: %w", path, err))
			return nil // Continue walking despite errors
		}

		if info.IsDir() {
			if err := w.markForNotification(path); err != nil {
				walkErrors = append(walkErrors, fmt.Errorf("failed to add subdirectory %s: %w", path, err))
				// Don't return error, continue with other directories
			}
		}

		return nil
	})

	// If walk itself failed, that's a critical error
	if walkErr != nil {
		return fmt.Errorf("failed to walk directory tree: %w", walkErr)
	}

	// Log any non-critical errors
	if len(walkErrors) > 0 {
		slog.Warn("Some paths failed to be added to fanotify watcher", "errors", len(walkErrors))
		for _, err := range walkErrors {
			slog.Debug("Fanotify path addition error", "error", err)
		}
	}

	return nil
}

// markForNotification marks a path for fanotify notification with proper error handling
func (w *FanotifyWatcher) markForNotification(path string) error {
	if path == "" {
		return fmt.Errorf("empty path provided")
	}

	// Open the directory/file with proper error handling
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to open path %s: %w", path, err)
	}
	defer func() {
		if closeErr := syscall.Close(fd); closeErr != nil {
			slog.Warn("Failed to close file descriptor", "fd", fd, "error", closeErr)
		}
	}()

	// Mark for notification with comprehensive event mask
	// FAN_MODIFY | FAN_CLOSE_WRITE | FAN_CREATE | FAN_DELETE | FAN_MOVED_FROM | FAN_MOVED_TO
	eventMask := uint64(FAN_MODIFY | FAN_CLOSE_WRITE | FAN_CREATE | FAN_DELETE | FAN_MOVED_FROM | FAN_MOVED_TO)

	err = doFanotifyMark(w.fd, FAN_MARK_ADD, eventMask, int(fd), "")
	if err != nil {
		return fmt.Errorf("fanotify mark failed for path %s: %w", path, err)
	}

	slog.Debug("Successfully marked path for fanotify notification", "path", path)
	return nil
}

// watchLoop is the main event processing loop with comprehensive error handling
func (w *FanotifyWatcher) watchLoop() {
	defer w.wg.Done()

	eventBuf := make([]byte, 4096)
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0

	slog.Debug("Starting fanotify watch loop")

	for {
		select {
		case <-w.ctx.Done():
			slog.Debug("Watch loop context cancelled, exiting")
			return

		default:
			// Read fanotify events with error handling
			n, err := syscall.Read(w.fd, eventBuf)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					slog.Error("Too many consecutive read errors, stopping watch loop", "errors", consecutiveErrors)
					select {
					case w.errorChan <- fmt.Errorf("fanotify read failed after %d attempts: %w", maxConsecutiveErrors, err):
					default:
						slog.Error("Failed to send error to channel", "error", err)
					}
					return
				}

				// Handle specific error types
				if errno, ok := err.(syscall.Errno); ok {
					switch errno {
					case syscall.EBADF:
						slog.Error("Fanotify file descriptor invalid, stopping watch loop")
						return
					case syscall.EINTR:
						// Interrupted, continue
						continue
					case syscall.EAGAIN:
						// No data available, small delay before retry
						time.Sleep(10 * time.Millisecond)
						continue
					}
				}

				select {
				case w.errorChan <- fmt.Errorf("fanotify read error: %w", err):
				case <-w.ctx.Done():
					return
				default:
					slog.Warn("Error channel full, dropping error", "error", err)
				}
				continue
			}

			// Reset error counter on successful read
			consecutiveErrors = 0

			if n == 0 {
				// EOF or no data
				continue
			}

			if n > len(eventBuf) {
				slog.Warn("Event buffer overflow, truncating", "read", n, "buffer", len(eventBuf))
				n = len(eventBuf)
			}

			// Parse events with error handling
			events, parseErr := w.parseEventsSafe(eventBuf[:n])
			if parseErr != nil {
				slog.Warn("Failed to parse events", "error", parseErr)
				continue
			}

			// Process parsed events
			for _, event := range events {
				if w.debouncer != nil {
					w.debouncer.Add(event)
				} else {
					select {
					case w.eventChan <- event:
					case <-w.ctx.Done():
						return
					default:
						slog.Warn("Event channel full, dropping event", "path", event.Path)
					}
				}
			}
		}
	}
}

// parseEventsSafe safely parses events with error handling
func (w *FanotifyWatcher) parseEventsSafe(buf []byte) ([]Event, error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in event parsing", "panic", r)
		}
	}()

	return w.parseEvents(buf), nil
}

// parseEvents parses raw fanotify events using proper structure
func (w *FanotifyWatcher) parseEvents(buf []byte) []Event {
	var events []Event

	for len(buf) >= int(unsafe.Sizeof(fanotifyEventMetadata{})) {
		event := (*fanotifyEventMetadata)(unsafe.Pointer(&buf[0]))

		watcherEvent := w.convertFanotifyEvent(event)
		if watcherEvent != nil {
			events = append(events, *watcherEvent)
		}

		// Move to next event using the event length
		eventLen := event.Event_len
		if eventLen == 0 || eventLen > uint32(len(buf)) {
			// Fallback to struct size if event_len is invalid
			eventLen = uint32(unsafe.Sizeof(fanotifyEventMetadata{}))
		}
		buf = buf[eventLen:]
	}

	return events
}

// convertFanotifyEvent converts a fanotify event to a watcher.Event
func (w *FanotifyWatcher) convertFanotifyEvent(event *fanotifyEventMetadata) *Event {
	if event == nil {
		return nil
	}

	// Get the path from the file descriptor
	path, err := w.getPathFromFd(int(event.Fd))
	if err != nil {
		slog.Warn("Failed to get path from fd", "fd", event.Fd, "error", err)
		return nil
	}

	// Close the file descriptor after getting the path
	defer func() {
		if event.Fd >= 0 {
			syscall.Close(int(event.Fd))
		}
	}()

	var eventType EventType

	switch {
	case event.Mask&FAN_CREATE != 0:
		eventType = EventCreate
	case event.Mask&FAN_MODIFY != 0:
		eventType = EventWrite
	case event.Mask&FAN_CLOSE_WRITE != 0:
		eventType = EventWrite
	case event.Mask&FAN_DELETE != 0:
		eventType = EventRemove
	case event.Mask&(FAN_MOVED_FROM|FAN_MOVED_TO) != 0:
		eventType = EventRename
	default:
		return nil // Ignore unknown events
	}

	// Check if path matches watched paths
	for watchedPath := range w.watchedPaths {
		if strings.HasPrefix(path, watchedPath) {
			return &Event{
				Type:      eventType,
				Path:      path,
				Timestamp: time.Now(),
				IsDir:     false, // fanotify doesn't provide this info directly
			}
		}
	}

	return nil // Path not in watched paths
}

// getPathFromFd gets the path from a file descriptor using /proc/self/fd
func (w *FanotifyWatcher) getPathFromFd(fd int) (string, error) {
	if fd < 0 {
		return "", fmt.Errorf("invalid file descriptor: %d", fd)
	}

	linkPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	path, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("failed to readlink %s: %w", linkPath, err)
	}

	// Clean up path (remove " (deleted)" suffix if present)
	cleanPath := strings.TrimSuffix(path, " (deleted)")
	return cleanPath, nil
}

// processEvents handles debounced events
func (w *FanotifyWatcher) processEvents() {
	defer w.wg.Done()

	if w.debouncer == nil {
		return
	}

	for {
		select {
		case <-w.ctx.Done():
			return

		case events, ok := <-w.debouncer.Events():
			if !ok {
				return
			}

			// Process events in batch
			if w.processor != nil {
				if err := w.processor.Process(w.ctx, events); err != nil {
					slog.Error("Error processing events", "error", err)
				}
			} else {
				// Send individual events
				for _, event := range events {
					select {
					case w.eventChan <- event:
					case <-w.ctx.Done():
						return
					default:
						slog.Warn("Event channel full, dropping event", "path", event.Path)
					}
				}
			}
		}
	}
}
