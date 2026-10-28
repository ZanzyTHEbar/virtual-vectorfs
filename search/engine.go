// Package search combines FFF file discovery with local Bleve text, metadata,
// and vector search. It supports Linux; semantic and hybrid queries require
// caller-supplied vectors.
// Build with -tags=vectors and the native FFF and FAISS libraries installed.
package search

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ZanzyTHEbar/virtual-vectorfs/internal/fffbridge"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

type Engine struct {
	options Options
	index   bleve.Index
	bridge  *fffbridge.Bridge

	lifeMu      sync.RWMutex
	closed      bool
	closeErr    error
	writeMu     sync.Mutex
	expandSlots chan struct{}

	stateMu              sync.RWMutex
	generationID         string
	lastReport           BuildReport
	lastError            string
	previousEvidencePath string

	jobs         chan string
	pendingMu    sync.Mutex
	pending      map[string]bool
	rescan       chan struct{}
	rescanCtx    context.Context
	rescanCancel context.CancelFunc
	pollStop     chan struct{}
	pollDone     chan struct{}
	workersDone  chan struct{}
	rescanDone   chan struct{}
}

func Open(ctx context.Context, options Options) (*Engine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	indexMapping, err := newIndexMapping(options)
	if err != nil {
		return nil, err
	}
	bridgeOptions := fffbridge.Options{
		EnableContentIndexing: options.FFFContentCache,
		EnableMmapCache:       options.FFFMmapCache,
		CacheBudget: fffbridge.CacheBudget{
			MaxFiles:    options.FFFCacheMaxFiles,
			MaxBytes:    options.FFFCacheMaxBytes,
			MaxFileSize: options.FFFCacheMaxFileSize,
		},
	}
	bridge, err := fffbridge.Open(options.Root, bridgeOptions)
	if err != nil {
		return nil, fmt.Errorf("open FFF bridge: %w", err)
	}
	index, err := openIndex(options, indexMapping)
	if err != nil {
		bridge.Close()
		return nil, err
	}
	if err := validateOpenedMapping(index.Mapping(), options); err != nil {
		_ = index.Close()
		bridge.Close()
		return nil, err
	}
	rescanCtx, rescanCancel := context.WithCancel(context.Background())
	engine := &Engine{
		options:      options,
		index:        index,
		bridge:       bridge,
		expandSlots:  make(chan struct{}, min(options.Workers, 4)),
		jobs:         make(chan string, options.QueueCapacity),
		pending:      make(map[string]bool),
		rescan:       make(chan struct{}, 1),
		rescanCtx:    rescanCtx,
		rescanCancel: rescanCancel,
		pollStop:     make(chan struct{}),
		pollDone:     make(chan struct{}),
		workersDone:  make(chan struct{}),
		rescanDone:   make(chan struct{}),
	}
	if err := engine.loadManifest(); err != nil {
		_ = index.Close()
		bridge.Close()
		return nil, err
	}
	if engine.generationID == "" {
		engine.generationID, err = newGenerationID()
		if err != nil {
			_ = index.Close()
			bridge.Close()
			return nil, err
		}
	}
	engine.startWorkers()
	go engine.pollEvents()
	return engine, nil
}

func openIndex(options Options, indexMapping *mapping.IndexMappingImpl) (bleve.Index, error) {
	if options.Storage == StorageMemory {
		index, err := bleve.NewUsing("", indexMapping, bleve.Config.DefaultIndexType, bleve.Config.DefaultMemKVStore, nil)
		if err != nil {
			return nil, fmt.Errorf("create in-memory Bleve index: %w", err)
		}
		return index, nil
	}
	if err := os.MkdirAll(filepath.Dir(options.IndexPath), 0o755); err != nil {
		return nil, fmt.Errorf("create index parent directory: %w", err)
	}
	if _, err := os.Stat(options.IndexPath); err == nil {
		index, err := bleve.Open(options.IndexPath)
		if err != nil {
			return nil, fmt.Errorf("open Bleve index at %q: %w", options.IndexPath, err)
		}
		return index, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat index path: %w", err)
	}
	index, err := bleve.New(options.IndexPath, indexMapping)
	if err != nil {
		return nil, fmt.Errorf("create Bleve index at %q: %w", options.IndexPath, err)
	}
	return index, nil
}

// Upsert indexes documents as one Bleve batch; callers should submit bounded chunks.
func (e *Engine) Upsert(ctx context.Context, documents ...Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return err
	}
	if err := e.validateDocuments(documents); err != nil {
		return err
	}
	if len(documents) == 0 {
		return nil
	}
	prepared := make([]indexedDocument, len(documents))
	for i, document := range documents {
		canonicalHash := contentHashForDocument(document)
		if document.ContentHash != "" && document.ContentHash != canonicalHash {
			return fmt.Errorf("document %q content hash does not match its indexed fields", document.ID)
		}
		document.ContentHash = canonicalHash
		var err error
		prepared[i], err = indexedData(document, documentKind, "", false)
		if err != nil {
			return err
		}
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	batch := e.index.NewBatch()
	for _, document := range prepared {
		if err := batch.Index(document.ID, document); err != nil {
			return fmt.Errorf("prepare document %q: %w", document.ID, err)
		}
	}
	if err := e.index.Batch(batch); err != nil {
		return fmt.Errorf("index documents: %w", err)
	}
	return nil
}

func (e *Engine) validateDocuments(documents []Document) error {
	ids := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		if document.ID == "" {
			return errors.New("document ID must not be empty")
		}
		for _, field := range []struct{ name, value string }{
			{"ID", document.ID}, {"source", document.Source}, {"path", document.Path},
			{"title", document.Title}, {"text", document.Text}, {"context", document.Context},
			{"content hash", document.ContentHash},
		} {
			if strings.IndexByte(field.value, 0) >= 0 {
				return fmt.Errorf("document %q %s must not contain NUL", document.ID, field.name)
			}
		}
		for key, value := range document.Metadata {
			if strings.IndexByte(key, 0) >= 0 || strings.IndexByte(value, 0) >= 0 {
				return fmt.Errorf("document %q metadata keys and values must not contain NUL", document.ID)
			}
		}
		if strings.HasPrefix(document.ID, fileIDPrefix) {
			return fmt.Errorf("document ID %q uses the reserved FFF file ID prefix", document.ID)
		}
		if _, exists := ids[document.ID]; exists {
			return fmt.Errorf("duplicate document ID %q in upsert", document.ID)
		}
		ids[document.ID] = struct{}{}
		if len(document.Vector) == 0 {
			continue
		}
		if len(document.Vector) != e.options.VectorDimensions {
			return fmt.Errorf("document %q vector has dimension %d; want %d", document.ID, len(document.Vector), e.options.VectorDimensions)
		}
		if err := validateVector(document.Vector); err != nil {
			return fmt.Errorf("document %q: %w", document.ID, err)
		}
	}
	return nil
}

func (e *Engine) Delete(ctx context.Context, ids ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return err
	}
	batch := e.index.NewBatch()
	for _, id := range ids {
		if id == "" {
			return errors.New("document ID must not be empty")
		}
		if strings.HasPrefix(id, fileIDPrefix) {
			return fmt.Errorf("document ID %q uses the reserved FFF file ID prefix", id)
		}
		batch.Delete(id)
	}
	if len(ids) == 0 {
		return nil
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := e.index.Batch(batch); err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}
	return nil
}

func (e *Engine) FindFiles(ctx context.Context, request FileQuery) (FilePage, error) {
	if err := ctx.Err(); err != nil {
		return FilePage{}, err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return FilePage{}, err
	}
	if strings.TrimSpace(request.Text) == "" {
		return FilePage{}, errors.New("file search query must not be empty")
	}
	if request.Offset < 0 {
		return FilePage{}, errors.New("file search offset must not be negative")
	}
	limit, err := normalizeSearchLimit(request.Limit)
	if err != nil {
		return FilePage{}, err
	}
	page, err := e.bridge.SearchPaths(request.Text, request.Offset, limit)
	if err != nil {
		return FilePage{}, fmt.Errorf("FFF path search: %w", err)
	}
	matches := make([]FileMatch, 0, len(page.Matches))
	for _, item := range page.Matches {
		if item.PathEncodingWarning {
			continue
		}
		matches = append(matches, FileMatch{
			Path: item.Path, Size: item.Size, Modified: item.Modified,
			Rank: item.Rank, Score: item.Score, MatchByteOffsets: item.MatchByteOffsets,
		})
	}
	return FilePage{
		Matches: matches, TotalMatched: page.TotalMatched, TotalFiles: page.TotalFiles,
		NextOffset: page.NextOffset,
	}, nil
}

func (e *Engine) Grep(ctx context.Context, request GrepQuery) (GrepPage, error) {
	if err := ctx.Err(); err != nil {
		return GrepPage{}, err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return GrepPage{}, err
	}
	if strings.TrimSpace(request.Text) == "" {
		return GrepPage{}, errors.New("grep query must not be empty")
	}
	switch request.Mode {
	case GrepModePlain, GrepModeRegex, GrepModeFuzzy:
	default:
		return GrepPage{}, fmt.Errorf("invalid grep mode %q", request.Mode)
	}
	if request.FileOffset < 0 {
		return GrepPage{}, errors.New("grep file offset must not be negative")
	}
	if request.ContextBefore < 0 || request.ContextAfter < 0 {
		return GrepPage{}, errors.New("grep context must not be negative")
	}
	limit, err := normalizeGrepMatchTarget(request.Limit)
	if err != nil {
		return GrepPage{}, err
	}
	before := min(request.ContextBefore, MaxGrepContextLines)
	after := min(request.ContextAfter, MaxGrepContextLines)
	page, err := e.bridge.Grep(request.Text, string(request.Mode), request.FileOffset, limit, before, after)
	if err != nil {
		return GrepPage{}, fmt.Errorf("FFF grep: %w", err)
	}
	matches := make([]GrepMatch, 0, len(page.Matches))
	for _, item := range page.Matches {
		if item.Path == nil || item.PathEncodingWarning {
			continue
		}
		matches = append(matches, GrepMatch{
			Path:             *item.Path,
			LineNumber:       item.LineNumber,
			Column:           item.Column,
			ByteOffset:       item.ByteOffset,
			LineContent:      item.LineContent,
			MatchByteOffsets: item.MatchByteOffsets,
			ContextBefore:    item.ContextBefore,
			ContextAfter:     item.ContextAfter,
			Truncated:        item.Truncated,
		})
	}
	return GrepPage{
		Matches: matches, TotalFiles: page.TotalFiles, FilesSearched: page.FilesSearched,
		FilteredFileCount: page.FilteredFileCount, NextFileOffset: page.NextFileOffset,
		LiteralFallback: page.LiteralFallback,
	}, nil
}

func (e *Engine) Status() Status {
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	e.stateMu.RLock()
	status := Status{
		Root:                 e.options.Root,
		IndexPath:            e.options.IndexPath,
		Storage:              e.options.Storage,
		GenerationID:         e.generationID,
		LastSyncIndexedFiles: e.lastReport.Indexed,
		Closed:               e.closed,
		LastError:            e.lastError,
	}
	e.stateMu.RUnlock()
	if e.closed {
		return status
	}
	bridgeStatus, err := e.bridge.Status()
	if err != nil {
		status.FFFWatchError = err.Error()
	} else {
		status.FFFReady = bridgeStatus.WatcherReady && bridgeStatus.ScanComplete && bridgeStatus.WatchSubscribed
		status.FFFWatchError = bridgeStatus.WatchError
	}
	return status
}

func (e *Engine) Close() error {
	e.lifeMu.Lock()
	defer e.lifeMu.Unlock()
	if e.closed {
		return e.closeErr
	}
	e.closed = true
	close(e.pollStop)
	<-e.pollDone
	close(e.jobs)
	e.rescanCancel()
	<-e.workersDone
	close(e.rescan)
	<-e.rescanDone
	e.bridge.Close()
	if err := e.index.Close(); err != nil {
		e.closeErr = fmt.Errorf("close Bleve index: %w", err)
	}
	return e.closeErr
}

func (e *Engine) checkOpen() error {
	if e.closed {
		return errors.New("search engine is closed")
	}
	return nil
}

func (e *Engine) startWorkers() {
	var workers sync.WaitGroup
	for range e.options.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for path := range e.jobs {
				for {
					if err := e.reconcilePath(path); err != nil {
						e.setLastError(err)
						e.scheduleRescan()
					}
					e.pendingMu.Lock()
					if e.pending[path] {
						e.pending[path] = false
						e.pendingMu.Unlock()
						continue
					}
					delete(e.pending, path)
					e.pendingMu.Unlock()
					break
				}
			}
		}()
	}
	go func() {
		workers.Wait()
		close(e.workersDone)
	}()
	go func() {
		defer close(e.rescanDone)
		for range e.rescan {
			if _, err := e.syncFiles(e.rescanCtx); err != nil {
				e.setLastError(err)
			}
		}
	}()
}
