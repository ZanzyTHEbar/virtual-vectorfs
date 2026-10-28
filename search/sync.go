package search

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/ZanzyTHEbar/virtual-vectorfs/internal/fffbridge"
	"github.com/blevesearch/bleve/v2"
)

// Sync retains one inventory page and vector state (at most 16 MiB of vector
// payload at 4096 dimensions), plus Workers*MaxFileBytes + WriteBatchBytes +
// MaxFileBytes of file content (one-document overshoot).
const (
	inventoryPageSize      = 512
	maxWriteBatchDocuments = 256
)

var errInventoryPathDisappeared = fmt.Errorf("file disappeared after FFF inventory: %w", os.ErrNotExist)

func (e *Engine) SyncFiles(ctx context.Context) (BuildReport, error) {
	if err := ctx.Err(); err != nil {
		e.setLastError(err)
		return BuildReport{}, err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		e.setLastError(err)
		return BuildReport{}, err
	}
	report, err := e.syncFiles(ctx)
	if err != nil {
		e.setLastError(err)
	}
	return report, err
}

func (e *Engine) syncFiles(ctx context.Context) (BuildReport, error) {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := e.bridge.WaitReady(ctx); err != nil {
		return BuildReport{}, fmt.Errorf("wait for FFF inventory: %w", err)
	}
	if e.options.NativeReadPolicy == NativeReadRootOnly {
		if err := e.bridge.Rescan(ctx); err != nil {
			return BuildReport{}, fmt.Errorf("complete requested RootOnly native rescan: %w", err)
		}
	}
	generation, err := newGenerationID()
	if err != nil {
		return BuildReport{}, err
	}
	artifactWriter, err := e.newEvidenceWriter(generation)
	if err != nil {
		return BuildReport{}, err
	}
	defer artifactWriter.Abort()

	report := BuildReport{
		GenerationID:        generation,
		Root:                e.options.Root,
		InventoryPageSize:   inventoryPageSize,
		Workers:             e.options.Workers,
		MaxFileBytes:        e.options.MaxFileBytes,
		WriteBatchBytes:     e.options.WriteBatchBytes,
		WriteBatchDocuments: maxWriteBatchDocuments,
		CreatedAt:           time.Now().UTC(),
	}
	reader := newInventoryReader(e, ctx)
	defer reader.Close()
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		page, err := e.bridge.Inventory(offset, inventoryPageSize)
		if err != nil {
			return report, fmt.Errorf("read FFF inventory page at %d: %w", offset, err)
		}
		if !page.Done && page.NextOffset <= offset {
			return report, fmt.Errorf("FFF inventory cursor did not advance from %d to %d", offset, page.NextOffset)
		}
		if page.NextOffset > offset {
			report.InventoryPages++
		}
		report.Scanned += len(page.Items)
		existingVectors, err := e.fileVectorStatesLocked(inventoryFileIDs(page.Items))
		if err != nil {
			return report, err
		}
		batch := newBleveBatchWriter(e.index, &report, e.options.WriteBatchBytes)
		pageErr := reader.ReadPage(page.Items, func(result inventoryRead) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if result.unsupported {
				evidence := FileEvidence{ID: result.id, Path: result.item.Path, Status: "unsupported"}
				if result.id != "" {
					before := batch.batch.Size()
					old, found, err := e.markFileStaleBatch(batch.batch, result.id, generation)
					if err != nil {
						return fmt.Errorf("mark unsupported path %q stale: %w", result.item.Path, err)
					}
					if found {
						evidence.SHA256 = old.ContentHash
						report.Stale++
						evidence.Status = "stale_unsupported"
						if batch.batch.Size() > before {
							if err := batch.AddDocument(int64(len(old.Text))); err != nil {
								return err
							}
						}
					}
				}
				report.Skipped++
				report.SkippedUnsupported++
				return artifactWriter.Write(evidence)
			}

			evidence := FileEvidence{ID: result.id, Path: result.relative, Bytes: result.size}
			if result.err != nil {
				return e.handleInventoryReadError(result, evidence, generation, batch, &report, artifactWriter)
			}

			if result.skip != "" {
				before := batch.batch.Size()
				old, found, err := e.markFileStaleBatch(batch.batch, result.id, generation)
				if err != nil {
					return fmt.Errorf("preserve skipped path %q: %w", result.relative, err)
				}
				report.Skipped++
				if found {
					report.Stale++
					evidence.SHA256 = old.ContentHash
					evidence.Status = "stale_" + result.skip
					if batch.batch.Size() > before {
						if err := batch.AddDocument(int64(len(old.Text))); err != nil {
							return err
						}
					}
				} else {
					evidence.Status = "skipped_" + result.skip
				}
				switch result.skip {
				case "binary":
					report.SkippedBinary++
				case "over_limit":
					report.SkippedOverLimit++
				default:
					report.SkippedUnsupported++
				}
				return artifactWriter.Write(evidence)
			}

			document := Document{
				ID:          result.id,
				Source:      fileSource,
				Path:        result.relative,
				Title:       path.Base(result.relative),
				Text:        string(result.data),
				ContentHash: contentHash(result.data),
			}
			evidence.SHA256 = document.ContentHash
			evidence.Bytes = result.size
			before := batch.batch.Size()
			hasVector, err := e.indexFileBatch(batch.batch, document, generation, existingVectors[result.id])
			if err != nil {
				result.err = err
				return e.handleInventoryReadError(result, evidence, generation, batch, &report, artifactWriter)
			}
			if batch.batch.Size() > before {
				if err := batch.AddDocument(int64(len(result.data))); err != nil {
					return err
				}
			}
			report.Indexed++
			report.Bytes += int64(len(result.data))
			if !hasVector {
				report.MissingVectors++
			}
			evidence.Status = "indexed"
			evidence.HasVector = hasVector
			return artifactWriter.Write(evidence)
		})
		if pageErr != nil {
			return report, pageErr
		}
		if err := batch.Flush(); err != nil {
			return report, fmt.Errorf("index FFF inventory page at %d: %w", offset, err)
		}

		done := page.Done
		offset = page.NextOffset
		page.Items = nil
		if done {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if report.Errors > 0 {
		if err := artifactWriter.Commit(); err != nil {
			return report, err
		}
		report.EvidencePath = artifactWriter.finalPath
		report.EvidenceCount = artifactWriter.count
		return report, fmt.Errorf("file sync completed with %d file errors; previous file records were preserved and pruning was skipped", report.Errors)
	}
	if err := e.pruneStaleFiles(ctx, generation); err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := artifactWriter.Commit(); err != nil {
		return report, err
	}
	report.EvidencePath = artifactWriter.finalPath
	report.EvidenceCount = artifactWriter.count
	if err := ctx.Err(); err != nil {
		_ = os.Remove(artifactWriter.finalPath)
		return report, err
	}
	if err := e.writeManifest(ctx, report); err != nil {
		_ = os.Remove(artifactWriter.finalPath)
		return report, err
	}
	previousEvidencePath := e.previousEvidencePath
	e.previousEvidencePath = report.EvidencePath
	e.stateMu.Lock()
	e.generationID = report.GenerationID
	e.lastReport = report
	e.lastError = ""
	e.stateMu.Unlock()
	if e.options.Storage == StorageDisk && previousEvidencePath != "" && previousEvidencePath != report.EvidencePath && filepath.Dir(previousEvidencePath) == evidenceDirectory(e.options.IndexPath, StorageDisk) {
		_ = os.Remove(previousEvidencePath)
	}
	return report, nil
}

func (e *Engine) handleInventoryReadError(
	result inventoryRead,
	evidence FileEvidence,
	generation string,
	batch *bleveBatchWriter,
	report *BuildReport,
	artifactWriter *evidenceWriter,
) error {
	if errors.Is(result.err, errInventoryPathDisappeared) {
		batch.batch.Delete(result.id)
		if err := batch.AddDocument(0); err != nil {
			return err
		}
		report.Skipped++
		evidence.Status = "deleted_during_sync"
		return artifactWriter.Write(evidence)
	}

	before := batch.batch.Size()
	old, found, err := e.markFileStaleBatch(batch.batch, result.id, generation)
	if err != nil {
		return errors.Join(result.err, fmt.Errorf("preserve stale path %q: %w", result.relative, err))
	}
	report.Errors++
	evidence.Status = "error"
	evidence.Error = result.err.Error()
	if found {
		report.Stale++
		evidence.Status = "stale_error"
		evidence.SHA256 = old.ContentHash
		evidence.Bytes = int64(len(old.Text))
		if batch.batch.Size() > before {
			if err := batch.AddDocument(int64(len(old.Text))); err != nil {
				return err
			}
		}
	}
	return artifactWriter.Write(evidence)
}

type inventoryRead struct {
	item        fffbridge.InventoryItem
	relative    string
	id          string
	data        []byte
	size        int64
	skip        string
	err         error
	unsupported bool
}

type inventoryReadJob struct {
	item    fffbridge.InventoryItem
	results chan<- inventoryRead
}

func inventoryFileIDs(items []fffbridge.InventoryItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if relative, exact := exactInventoryPath(item); exact {
			ids = append(ids, stableFileID(relative))
		}
	}
	return ids
}

func exactInventoryPath(item fffbridge.InventoryItem) (string, bool) {
	relative, err := normalizedRelativePath(item.Path)
	if err != nil || item.PathEncodingWarning || relative == "" {
		return "", false
	}
	return relative, true
}

type inventoryReader struct {
	engine  *Engine
	ctx     context.Context
	jobs    chan inventoryReadJob
	workers sync.WaitGroup
}

type bleveBatchWriter struct {
	index         bleve.Index
	report        *BuildReport
	batch         *bleve.Batch
	byteBudget    int64
	contentByte   int64
	documentCount int
}

func newBleveBatchWriter(index bleve.Index, report *BuildReport, byteBudget int64) *bleveBatchWriter {
	return &bleveBatchWriter{index: index, report: report, batch: index.NewBatch(), byteBudget: byteBudget}
}

func (w *bleveBatchWriter) AddDocument(contentBytes int64) error {
	maxInt64 := int64(^uint64(0) >> 1)
	if contentBytes > 0 && w.contentByte > maxInt64-contentBytes {
		w.contentByte = maxInt64
	} else {
		w.contentByte += contentBytes
	}
	w.documentCount++
	if w.contentByte >= w.byteBudget || w.documentCount >= maxWriteBatchDocuments {
		return w.Flush()
	}
	return nil
}

func (w *bleveBatchWriter) Flush() error {
	if w.batch.Size() == 0 {
		w.contentByte = 0
		w.documentCount = 0
		return nil
	}
	if err := w.index.Batch(w.batch); err != nil {
		return err
	}
	w.batch.Reset()
	w.contentByte = 0
	w.documentCount = 0
	w.report.WriteBatches++
	return nil
}

func newInventoryReader(engine *Engine, ctx context.Context) *inventoryReader {
	reader := &inventoryReader{
		engine: engine,
		ctx:    ctx,
		jobs:   make(chan inventoryReadJob, inventoryPageSize),
	}
	for i := 0; i < engine.options.Workers; i++ {
		reader.workers.Add(1)
		go reader.worker()
	}
	return reader
}

func (r *inventoryReader) ReadPage(items []fffbridge.InventoryItem, consume func(inventoryRead) error) error {
	results := make(chan inventoryRead)
	for _, item := range items {
		r.jobs <- inventoryReadJob{item: item, results: results}
	}
	var consumeErr error
	for range items {
		result := <-results
		if consumeErr == nil {
			consumeErr = consume(result)
		}
	}
	return consumeErr
}

func (r *inventoryReader) worker() {
	defer r.workers.Done()
	for job := range r.jobs {
		result := inventoryRead{item: job.item}
		relative, pathIsExact := exactInventoryPath(job.item)
		if pathIsExact {
			result.id = stableFileID(relative)
		}
		if !pathIsExact || r.engine.isIndexPath(relative) {
			result.unsupported = true
			job.results <- result
			continue
		}
		result.relative = relative
		if err := r.ctx.Err(); err != nil {
			result.err = err
		} else {
			result.data, result.size, result.skip, result.err = r.engine.readIndexedFile(relative)
		}
		job.results <- result
	}
}

func (r *inventoryReader) Close() {
	close(r.jobs)
	r.workers.Wait()
}

func (e *Engine) readIndexedFile(relative string) ([]byte, int64, string, error) {
	clean, err := normalizedRelativePath(relative)
	if err != nil {
		return nil, 0, "unsupported", nil
	}
	root, err := os.OpenRoot(e.options.Root)
	if err != nil {
		return nil, 0, "", fmt.Errorf("open root: %w", err)
	}
	defer root.Close()
	file, err := root.OpenFile(filepath.FromSlash(clean), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, "", fmt.Errorf("%w: %q", errInventoryPathDisappeared, relative)
		}
		return nil, 0, "", fmt.Errorf("open %q: %w", relative, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, "", fmt.Errorf("stat %q: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return nil, info.Size(), "unsupported", nil
	}
	if info.Size() > e.options.MaxFileBytes {
		return nil, info.Size(), "over_limit", nil
	}
	limit := e.options.MaxFileBytes
	if limit < int64(^uint64(0)>>1) {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return nil, info.Size(), "", fmt.Errorf("read %q: %w", relative, err)
	}
	if int64(len(data)) > e.options.MaxFileBytes {
		return nil, int64(len(data)), "over_limit", nil
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, int64(len(data)), "binary", nil
	}
	return data, int64(len(data)), "", nil
}

func (e *Engine) isIndexPath(relative string) bool {
	fullPath := filepath.Join(e.options.Root, filepath.FromSlash(relative))
	for _, excluded := range []string{e.options.IndexPath, evidenceDirectory(e.options.IndexPath, e.options.Storage)} {
		rel, err := filepath.Rel(excluded, fullPath)
		if err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))) {
			return true
		}
	}
	return false
}

func normalizedRelativePath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.ContainsRune(value, '\ufffd') {
		return "", errors.New("path is empty or not representable as UTF-8")
	}
	clean := path.Clean(value)
	if clean == "." || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q is not root-relative", value)
	}
	if clean != value {
		return "", fmt.Errorf("path %q is not normalized", value)
	}
	return clean, nil
}

func stableFileID(relative string) string {
	hash := sha256.Sum256([]byte(relative))
	return fileIDPrefix + hex.EncodeToString(hash[:])
}

func contentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (e *Engine) indexFileLocked(document Document, generation string) (bool, error) {
	batch := e.index.NewBatch()
	existing, found, err := e.fileDocumentLocked(document.ID)
	if err != nil {
		return false, err
	}
	state := fileVectorState{}
	if found {
		state = fileVectorState{ContentHash: existing.ContentHash, Vector: existing.Vector}
	}
	hasVector, err := e.indexFileBatch(batch, document, generation, state)
	if err != nil {
		return false, err
	}
	if batch.Size() > 0 {
		if err := e.index.Batch(batch); err != nil {
			return false, err
		}
	}
	return hasVector, nil
}

func (e *Engine) indexFileBatch(batch *bleve.Batch, document Document, generation string, existing fileVectorState) (bool, error) {
	if existing.ContentHash == document.ContentHash && e.validFileVector(existing.Vector) {
		document.Vector = existing.Vector
	}
	prepared, err := indexedData(document, fileKind, generation, false)
	if err != nil {
		return false, err
	}
	if err := batch.Index(prepared.ID, prepared); err != nil {
		return false, err
	}
	return len(document.Vector) != 0, nil
}

func (e *Engine) validFileVector(vector []float32) bool {
	return len(vector) == e.options.VectorDimensions && validateVector(vector) == nil
}

func (e *Engine) markFileStaleBatch(batch *bleve.Batch, id, generation string) (Document, bool, error) {
	document, found, err := e.fileDocumentLocked(id)
	if err != nil || !found {
		return document, found, err
	}
	document.Vector = nil
	prepared, err := indexedData(document, fileKind, generation, true)
	if err != nil {
		return Document{}, false, err
	}
	if err := batch.Index(prepared.ID, prepared); err != nil {
		return Document{}, false, err
	}
	return document, true, nil
}

func (e *Engine) markFileStaleLocked(id, generation string) (Document, bool, error) {
	batch := e.index.NewBatch()
	document, found, err := e.markFileStaleBatch(batch, id, generation)
	if err != nil || !found {
		return document, found, err
	}
	if err := e.index.Batch(batch); err != nil {
		return Document{}, false, err
	}
	return document, true, nil
}

func (e *Engine) deleteFilePathLocked(relative string) error {
	id := stableFileID(relative)
	_, found, err := e.fileDocumentLocked(id)
	if err != nil || !found {
		return err
	}
	return e.index.Delete(id)
}

func (e *Engine) pruneStaleFiles(ctx context.Context, generation string) error {
	source := bleve.NewTermQuery(fileSource)
	source.SetField(fieldSource)
	kind := bleve.NewTermQuery(fileKind)
	kind.SetField(fieldKind)
	currentGeneration := bleve.NewTermQuery(generation)
	currentGeneration.SetField(fieldGeneration)
	query := bleve.NewBooleanQuery()
	query.AddMust(source, kind)
	query.AddMustNot(currentGeneration)
	pageSize := inventoryPageSize
	var after []string
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		request := bleve.NewSearchRequestOptions(query, pageSize, 0, false)
		request.Score = bleve.ScoreNone
		request.SortBy([]string{"_id"})
		request.SearchAfter = after
		result, err := e.index.Search(request)
		if err != nil {
			return fmt.Errorf("find stale FFF file documents: %w", err)
		}
		if len(result.Hits) == 0 {
			return nil
		}
		after = append(after[:0], result.Hits[len(result.Hits)-1].Sort...)
		if len(after) == 0 {
			return errors.New("Bleve stale-file prune returned no ID cursor")
		}
		batch := e.index.NewBatch()
		for _, hit := range result.Hits {
			if !strings.HasPrefix(hit.ID, fileIDPrefix) {
				return fmt.Errorf("refuse to prune non-FFF document ID %q", hit.ID)
			}
			batch.Delete(hit.ID)
		}
		if err := e.index.Batch(batch); err != nil {
			return fmt.Errorf("prune stale FFF file documents: %w", err)
		}
	}
}
