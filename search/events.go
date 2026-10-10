package search

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/internal/fffbridge"
)

func (e *Engine) pollEvents() {
	defer close(e.pollDone)
	for {
		select {
		case <-e.pollStop:
			return
		default:
		}
		events, err := e.bridge.PollEvents(200*time.Millisecond, 128)
		if err != nil {
			e.setLastError(fmt.Errorf("poll FFF events: %w", err))
			e.scheduleRescan()
			select {
			case <-e.pollStop:
				return
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}
		for _, event := range events {
			e.handleEvent(event)
		}
	}
}

func (e *Engine) handleEvent(event fffbridge.Event) {
	if event.RescanRequired && event.Reason == "watch_subscription_gap" {
		return
	}
	if event.RescanRequired || event.Kind == "rescan" || event.Path == nil {
		e.scheduleRescan()
		return
	}
	if event.From != nil {
		e.enqueuePath(*event.From)
	}
	e.enqueuePath(*event.Path)
}

func (e *Engine) enqueuePath(relative string) {
	clean, err := normalizedRelativePath(relative)
	if err != nil {
		e.scheduleRescan()
		return
	}
	e.pendingMu.Lock()
	if _, exists := e.pending[clean]; exists {
		e.pending[clean] = true
		e.pendingMu.Unlock()
		return
	}
	e.pending[clean] = false
	select {
	case e.jobs <- clean:
		e.pendingMu.Unlock()
	default:
		delete(e.pending, clean)
		e.pendingMu.Unlock()
		e.scheduleRescan()
	}
}

func (e *Engine) scheduleRescan() {
	select {
	case e.rescan <- struct{}{}:
	default:
	}
}

func (e *Engine) reconcilePath(relative string) error {
	clean, err := normalizedRelativePath(relative)
	if err != nil {
		return err
	}
	if e.isIndexPath(clean) {
		return nil
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	data, size, skip, err := e.readIndexedFile(clean)
	if err != nil {
		if errors.Is(err, errInventoryPathDisappeared) {
			return e.deleteFilePathLocked(clean)
		}
		generation := e.currentGeneration()
		if _, _, staleErr := e.markFileStaleLocked(stableFileID(clean), generation); staleErr != nil {
			return fmt.Errorf("mark unreadable file %q stale: %w", clean, staleErr)
		}
		return err
	}
	if skip != "" {
		_, _, err := e.markFileStaleLocked(stableFileID(clean), e.currentGeneration())
		return err
	}
	hash := contentHash(data)
	document := Document{
		ID:          stableFileID(clean),
		Source:      fileSource,
		Path:        clean,
		Title:       path.Base(clean),
		Text:        string(data),
		ContentHash: hash,
	}
	generation := e.currentGeneration()
	if _, err := e.indexFileLocked(document, generation); err != nil {
		_, _, staleErr := e.markFileStaleLocked(document.ID, generation)
		return errors.Join(fmt.Errorf("index changed file %q (%d bytes): %w", clean, size, err), staleErr)
	}
	return nil
}

func (e *Engine) currentGeneration() string {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.generationID
}

func (e *Engine) setLastError(err error) {
	e.stateMu.Lock()
	e.lastError = err.Error()
	e.stateMu.Unlock()
}
