package fffbridge

/*
#cgo linux pkg-config: fffbridge
#include <stdlib.h>
#include "fffbridge.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"
)

type Options struct {
	EnableContentIndexing bool        `json:"enable_content_indexing"`
	EnableMmapCache       bool        `json:"enable_mmap_cache"`
	CacheBudget           CacheBudget `json:"cache_budget"`
}

type CacheBudget struct {
	MaxFiles    int    `json:"max_files,omitempty"`
	MaxBytes    uint64 `json:"max_bytes,omitempty"`
	MaxFileSize uint64 `json:"max_file_size,omitempty"`
}

type Bridge struct {
	mu     sync.RWMutex // FFF calls share the handle; Close excludes all calls.
	handle *C.FffbHandle
}

var errHandleClosed = errors.New("FFF bridge is closed")

const (
	maxGrepMatchTarget  = 80
	maxGrepContextLines = 5
)

type InventoryPage struct {
	Total int `json:"total"` // Live-file count; it is not the raw-slice cursor bound.
	// NextOffset indexes FFF's raw path-sorted inventory, including tombstones.
	NextOffset int `json:"next_offset"`
	// Done reports whether NextOffset reached the end of the raw slice.
	Done  bool            `json:"done"`
	Items []InventoryItem `json:"items"`
}

type InventoryItem struct {
	Path                string `json:"path"`
	PathEncodingWarning bool   `json:"path_encoding_warning"`
	Size                int64  `json:"size"`
	Modified            int64  `json:"modified"`
}

type FileMatch struct {
	Path                string   `json:"path"`
	PathEncodingWarning bool     `json:"path_encoding_warning"`
	Size                int64    `json:"size"`
	Modified            int64    `json:"modified"`
	Rank                int      `json:"rank"`
	Score               int      `json:"score"`
	MatchByteOffsets    [][2]int `json:"match_byte_offsets"`
}

type FilePage struct {
	Matches      []FileMatch `json:"matches"`
	TotalMatched int         `json:"total_matched"`
	TotalFiles   int         `json:"total_files"`
	NextOffset   int         `json:"next_offset"`
}

type GrepMatch struct {
	Path                *string  `json:"path"`
	PathEncodingWarning bool     `json:"path_encoding_warning"`
	LineNumber          int      `json:"line_number"`
	Column              int      `json:"column"`
	ByteOffset          int64    `json:"byte_offset"`
	LineContent         string   `json:"line_content"`
	MatchByteOffsets    [][2]int `json:"match_byte_offsets"`
	ContextBefore       []string `json:"context_before"`
	ContextAfter        []string `json:"context_after"`
	Truncated           bool     `json:"truncated"`
}

type GrepPage struct {
	Matches           []GrepMatch `json:"matches"`
	TotalFiles        int         `json:"total_files"`
	FilesSearched     int         `json:"total_files_searched"`
	FilteredFileCount int         `json:"filtered_file_count"`
	NextFileOffset    int         `json:"next_file_offset"`
	LiteralFallback   string      `json:"literal_fallback"`
}

type Event struct {
	Kind           string  `json:"kind"`
	Path           *string `json:"path"`
	From           *string `json:"from"`
	Reason         string  `json:"reason"`
	RescanRequired bool    `json:"rescan_required"`
}

type Status struct {
	ScanActive      bool   `json:"scan_active"`
	ScanComplete    bool   `json:"scan_complete"`
	WatcherReady    bool   `json:"watcher_ready"`
	WatchSubscribed bool   `json:"watch_subscribed"`
	IndexedFiles    int    `json:"indexed_files"`
	WatchError      string `json:"watch_error"`
}

func Open(root string, options Options) (*Bridge, error) {
	if err := rejectNUL("root", root); err != nil {
		return nil, err
	}
	rootCString := C.CString(root)
	defer C.free(unsafe.Pointer(rootCString))
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("marshal FFF options: %w", err)
	}
	optionsCString := C.CString(string(optionsJSON))
	defer C.free(unsafe.Pointer(optionsCString))

	var cErr *C.char
	handle := C.fffb_open(rootCString, optionsCString, &cErr)
	if cErr != nil {
		message := C.GoString(cErr)
		C.fffb_free_string(cErr)
		if handle != nil {
			C.fffb_close(handle)
		}
		return nil, errors.New(message)
	}
	if handle == nil {
		return nil, errors.New("FFF bridge returned a nil handle")
	}
	return &Bridge{handle: handle}, nil
}

func (b *Bridge) WaitReady(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.mu.RLock()
		handle, err := b.liveHandle()
		if err == nil {
			result := C.fffb_wait_ready(handle, 250)
			b.mu.RUnlock()
			switch result {
			case 1:
				return nil
			case 0:
				continue
			case -2:
				status, statusErr := b.Status()
				if statusErr != nil {
					return fmt.Errorf("FFF watch subscription failed: %w", statusErr)
				}
				if status.WatchError != "" {
					return fmt.Errorf("FFF watch subscription failed: %s", status.WatchError)
				}
				return errors.New("FFF watch subscription worker stopped before subscribing")
			default:
				return errors.New("FFF bridge failed while waiting for readiness")
			}
		}
		b.mu.RUnlock()
		return err
	}
}

func (b *Bridge) Inventory(offset, limit int) (InventoryPage, error) {
	var page InventoryPage
	err := b.decode(func(handle *C.FffbHandle) *C.char {
		return C.fffb_inventory(handle, C.uint64_t(offset), C.uint32_t(limit))
	}, &page)
	return page, err
}

func (b *Bridge) SearchPaths(query string, offset, limit int) (FilePage, error) {
	if err := rejectNUL("path search query", query); err != nil {
		return FilePage{}, err
	}
	queryCString := C.CString(query)
	defer C.free(unsafe.Pointer(queryCString))
	var page FilePage
	err := b.decode(func(handle *C.FffbHandle) *C.char {
		return C.fffb_search_paths(handle, queryCString, C.uint64_t(offset), C.uint32_t(limit))
	}, &page)
	return page, err
}

func (b *Bridge) Grep(query, mode string, fileOffset, limit, contextBefore, contextAfter int) (GrepPage, error) {
	if err := rejectNUL("grep query", query); err != nil {
		return GrepPage{}, err
	}
	if err := rejectNUL("grep mode", mode); err != nil {
		return GrepPage{}, err
	}
	if fileOffset < 0 || contextBefore < 0 || contextAfter < 0 || limit < 0 {
		return GrepPage{}, errors.New("grep offset, match target, and context must not be negative")
	}
	if limit == 0 {
		limit = 10
	}
	limit = min(limit, maxGrepMatchTarget)
	contextBefore = min(contextBefore, maxGrepContextLines)
	contextAfter = min(contextAfter, maxGrepContextLines)
	queryCString := C.CString(query)
	defer C.free(unsafe.Pointer(queryCString))
	modeCString := C.CString(mode)
	defer C.free(unsafe.Pointer(modeCString))
	var page GrepPage
	err := b.decode(func(handle *C.FffbHandle) *C.char {
		return C.fffb_live_grep(handle, queryCString, modeCString, C.uint64_t(fileOffset), C.uint32_t(min(limit, maxGrepMatchTarget)), C.uint64_t(min(contextBefore, maxGrepContextLines)), C.uint64_t(min(contextAfter, maxGrepContextLines)))
	}, &page)
	return page, err
}

func rejectNUL(field, value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s must not contain NUL", field)
	}
	return nil
}

func (b *Bridge) PollEvents(timeout time.Duration, limit int) ([]Event, error) {
	milliseconds := timeout.Milliseconds()
	if milliseconds < 0 {
		milliseconds = 0
	}
	if milliseconds > int64(^uint32(0)) {
		milliseconds = int64(^uint32(0))
	}
	var events []Event
	err := b.decode(func(handle *C.FffbHandle) *C.char {
		return C.fffb_poll_events(handle, C.uint32_t(milliseconds), C.uint32_t(limit))
	}, &events)
	return events, err
}

func (b *Bridge) Status() (Status, error) {
	var status Status
	err := b.decode(func(handle *C.FffbHandle) *C.char {
		return C.fffb_status(handle)
	}, &status)
	return status, err
}

func (b *Bridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.handle != nil {
		C.fffb_close(b.handle)
		b.handle = nil
	}
}

func (b *Bridge) decode(call func(*C.FffbHandle) *C.char, out any) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	handle, err := b.liveHandle()
	if err != nil {
		return err
	}
	result := call(handle)
	if result == nil {
		return errors.New("FFF bridge returned a null JSON string")
	}
	text := C.GoString(result)
	C.fffb_free_string(result)
	if strings.HasPrefix(text, "{") {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal([]byte(text), &envelope); err != nil {
			return fmt.Errorf("decode FFF JSON: %w", err)
		}
		if raw := envelope["error"]; raw != nil {
			var message string
			if err := json.Unmarshal(raw, &message); err != nil {
				return fmt.Errorf("decode FFF error field: %w", err)
			}
			if message != "" {
				return errors.New(message)
			}
		}
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("decode FFF response: %w", err)
	}
	return nil
}

func (b *Bridge) liveHandle() (*C.FffbHandle, error) {
	if b.handle == nil {
		return nil, errHandleClosed
	}
	return b.handle, nil
}
