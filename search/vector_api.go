package search

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

// MissingVectors returns at most Workers vector inputs after afterID and the next cursor.
func (e *Engine) MissingVectors(ctx context.Context, afterID string, limit int) ([]VectorInput, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return nil, "", err
	}
	limit, err := normalizeSearchLimit(limit)
	if err != nil {
		return nil, "", err
	}
	limit = min(limit, e.options.Workers)
	missing := bleve.NewBoolFieldQuery(false)
	missing.SetField(fieldHasVector)
	notStale := bleve.NewBoolFieldQuery(false)
	notStale.SetField(fieldStale)
	request := bleve.NewSearchRequestOptions(bleve.NewConjunctionQuery(missing, notStale), limit+1, 0, false)
	request.Score = bleve.ScoreNone
	request.SortBy([]string{"_id"})
	if afterID != "" {
		request.SearchAfter = []string{afterID}
	}
	request.Fields = []string{fieldSource, fieldPath, fieldTitle, fieldText, fieldContext, fieldContentHash, fieldRecord, fieldMetadataRecord, fieldStale, fieldHasVector}
	result, err := e.index.SearchInContext(ctx, request)
	if err != nil {
		return nil, "", fmt.Errorf("find vectorless documents: %w", err)
	}
	pageLength := min(len(result.Hits), limit)
	items := make([]VectorInput, 0, pageLength)
	for _, hit := range result.Hits[:pageLength] {
		document, err := storedDocument(hit.Fields[fieldRecord], hit.ID, hit.Fields)
		if err != nil {
			return nil, "", fmt.Errorf("read vector input %q: %w", hit.ID, err)
		}
		items = append(items, VectorInput{
			ID:          document.ID,
			Source:      document.Source,
			Path:        document.Path,
			Title:       document.Title,
			Text:        document.Text,
			Context:     document.Context,
			Metadata:    document.Metadata,
			ContentHash: document.ContentHash,
			HasVector:   boolField(hit.Fields, fieldHasVector),
			Stale:       boolField(hit.Fields, fieldStale),
		})
	}
	if len(result.Hits) <= limit {
		return items, "", nil
	}
	return items, result.Hits[pageLength-1].ID, nil
}

// UpsertVectors replaces vectors only when the indexed content still matches the supplied hash.
// Batch size is capped at Workers so stored document content remains bounded.
func (e *Engine) UpsertVectors(ctx context.Context, updates ...VectorUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	if len(updates) > e.options.Workers {
		return fmt.Errorf("vector update batch exceeds configured worker count %d", e.options.Workers)
	}
	validated := make([]VectorUpdate, len(updates))
	seen := make(map[string]struct{}, len(updates))
	for i, update := range updates {
		if update.ID == "" {
			return errors.New("vector update ID must not be empty")
		}
		if update.ExpectedContentHash == "" {
			return fmt.Errorf("vector update %q requires an expected content hash", update.ID)
		}
		if _, ok := seen[update.ID]; ok {
			return fmt.Errorf("duplicate vector update ID %q", update.ID)
		}
		seen[update.ID] = struct{}{}
		if len(update.Vector) != e.options.VectorDimensions {
			return fmt.Errorf("document %q vector has dimension %d; want %d", update.ID, len(update.Vector), e.options.VectorDimensions)
		}
		if err := validateVector(update.Vector); err != nil {
			return fmt.Errorf("document %q: %w", update.ID, err)
		}
		validated[i] = VectorUpdate{
			ID:                  update.ID,
			ExpectedContentHash: update.ExpectedContentHash,
			Vector:              append([]float32(nil), update.Vector...),
		}
	}

	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	batch := e.index.NewBatch()
	for _, update := range validated {
		if err := ctx.Err(); err != nil {
			return err
		}
		record, found, err := e.documentByIDLocked(update.ID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("vector update document %q not found", update.ID)
		}
		if strings.HasPrefix(update.ID, fileIDPrefix) && (record.Kind != fileKind || record.Document.Source != fileSource) {
			return fmt.Errorf("reserved FFF file ID %q belongs to a non-FFF document", update.ID)
		}
		if record.Stale || record.Document.ContentHash != update.ExpectedContentHash {
			return &StaleContentError{
				ID:       update.ID,
				Expected: update.ExpectedContentHash,
				Actual:   record.Document.ContentHash,
				Stale:    record.Stale,
			}
		}
		if record.Kind == fileKind && record.Document.Source == fileSource {
			data, _, skip, err := e.readIndexedFile(record.Document.Path)
			if err != nil || skip != "" {
				return &StaleContentError{
					ID:       update.ID,
					Expected: update.ExpectedContentHash,
					Actual:   record.Document.ContentHash,
					Stale:    true,
				}
			}
			if actual := contentHash(data); actual != update.ExpectedContentHash {
				return &StaleContentError{ID: update.ID, Expected: update.ExpectedContentHash, Actual: actual}
			}
		}
		document := record.Document
		document.Vector = update.Vector
		prepared, err := indexedData(document, record.Kind, record.Generation, false)
		if err != nil {
			return err
		}
		if err := batch.Index(prepared.ID, prepared); err != nil {
			return fmt.Errorf("prepare vector update %q: %w", update.ID, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.index.Batch(batch); err != nil {
		return fmt.Errorf("index vector updates: %w", err)
	}
	return nil
}
