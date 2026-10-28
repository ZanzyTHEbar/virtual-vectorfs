package search

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2"
)

// Expand returns a bounded field range from a current indexed record or verified live file.
func (e *Engine) Expand(ctx context.Context, request ExpandRequest) (Excerpt, error) {
	if err := ctx.Err(); err != nil {
		return Excerpt{}, err
	}
	select {
	case e.expandSlots <- struct{}{}:
		defer func() { <-e.expandSlots }()
	case <-ctx.Done():
		return Excerpt{}, ctx.Err()
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return Excerpt{}, err
	}
	if request.ID == "" {
		return Excerpt{}, fmt.Errorf("expand document ID: %w", os.ErrNotExist)
	}
	if request.ExpectedContentHash == "" {
		return Excerpt{}, errors.New("expand requires an expected content hash")
	}
	if request.ExpectedSource == "" {
		return Excerpt{}, errors.New("expand requires an expected source")
	}
	switch request.Field {
	case fieldText, fieldContext, fieldTitle:
	default:
		return Excerpt{}, fmt.Errorf("invalid expand field %q", request.Field)
	}
	if request.Offset < 0 {
		return Excerpt{}, errors.New("expand offset must not be negative")
	}
	limit := request.Limit
	if limit == 0 {
		limit = 4096
	}
	if limit < 0 || limit > MaxExpandBytes {
		return Excerpt{}, fmt.Errorf("expand limit must be between 0 and %d", MaxExpandBytes)
	}
	if err := ctx.Err(); err != nil {
		return Excerpt{}, err
	}

	lookup := bleve.NewSearchRequestOptions(bleve.NewDocIDQuery([]string{request.ID}), 1, 0, false)
	metadataFields := []string{fieldSource, fieldKind, fieldPath, fieldContentHash, fieldStale}
	lookup.Fields = metadataFields
	first, err := e.index.SearchInContext(ctx, lookup)
	if err != nil {
		return Excerpt{}, fmt.Errorf("find expand document %q: %w", request.ID, err)
	}
	if err := ctx.Err(); err != nil {
		return Excerpt{}, err
	}
	if len(first.Hits) == 0 {
		return Excerpt{}, fmt.Errorf("expand document %q: %w", request.ID, os.ErrNotExist)
	}
	fields := first.Hits[0].Fields
	source := stringField(fields, fieldSource)
	kind := stringField(fields, fieldKind)
	path := stringField(fields, fieldPath)
	hash := stringField(fields, fieldContentHash)
	if hash == "" || hash != request.ExpectedContentHash || source != request.ExpectedSource || boolField(fields, fieldStale) {
		return Excerpt{}, fmt.Errorf("expand document %q: %w", request.ID, ErrStaleContent)
	}

	var text, origin string
	if strings.HasPrefix(request.ID, fileIDPrefix) {
		if kind != fileKind || source != fileSource {
			return Excerpt{}, fmt.Errorf("reserved file ID %q is not an FFF file", request.ID)
		}
		if request.Field != fieldText {
			return Excerpt{}, fmt.Errorf("field %q is unavailable for file %q", request.Field, request.ID)
		}
		clean, err := normalizedRelativePath(path)
		if err != nil || !utf8.ValidString(path) || stableFileID(clean) != request.ID || e.isIndexPath(clean) {
			return Excerpt{}, fmt.Errorf("invalid indexed file path %q for %q", path, request.ID)
		}
		if err := ctx.Err(); err != nil {
			return Excerpt{}, err
		}
		data, _, skip, err := e.readIndexedFile(clean)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Excerpt{}, ctxErr
		}
		if err != nil {
			return Excerpt{}, fmt.Errorf("read expand file %q: %w", path, err)
		}
		if skip != "" {
			return Excerpt{}, fmt.Errorf("expand file %q is unavailable: %s", path, skip)
		}
		if contentHash(data) != hash {
			return Excerpt{}, fmt.Errorf("expand file %q: %w", path, ErrStaleContent)
		}
		text, origin = string(data), OriginLiveFile
	} else {
		if kind != documentKind {
			return Excerpt{}, fmt.Errorf("indexed document %q is not a caller record", request.ID)
		}
		// Request only the selected text field; never materialize the stored vector record.
		lookup.Fields = []string{request.Field, fieldSource, fieldKind, fieldPath, fieldContentHash, fieldStale}
		second, err := e.index.SearchInContext(ctx, lookup)
		if err != nil {
			return Excerpt{}, fmt.Errorf("read expand field %q for %q: %w", request.Field, request.ID, err)
		}
		if err := ctx.Err(); err != nil {
			return Excerpt{}, err
		}
		if len(second.Hits) == 0 {
			return Excerpt{}, fmt.Errorf("expand document %q: %w", request.ID, os.ErrNotExist)
		}
		stored := second.Hits[0].Fields
		if stringField(stored, fieldContentHash) != hash || boolField(stored, fieldStale) || stringField(stored, fieldKind) != kind || stringField(stored, fieldSource) != source || stringField(stored, fieldPath) != path {
			return Excerpt{}, fmt.Errorf("expand document %q changed: %w", request.ID, ErrStaleContent)
		}
		if _, ok := stored[request.Field]; !ok {
			return Excerpt{}, fmt.Errorf("field %q is unavailable for document %q", request.Field, request.ID)
		}
		text, origin = stringField(stored, request.Field), OriginIndexedRecord
	}

	if !utf8.ValidString(text) || request.Offset >= len(text) || !utf8.RuneStart(text[request.Offset]) {
		return Excerpt{}, fmt.Errorf("invalid UTF-8 expand offset %d for field %q", request.Offset, request.Field)
	}
	end := request.Offset + min(limit, len(text)-request.Offset)
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	if end == request.Offset {
		return Excerpt{}, fmt.Errorf("expand limit %d cannot fit a full rune at offset %d", limit, request.Offset)
	}
	// Recheck after I/O; like Search, this does not freeze later index updates.
	if err := ctx.Err(); err != nil {
		return Excerpt{}, err
	}
	lookup.Fields = metadataFields
	current, err := e.index.SearchInContext(ctx, lookup)
	if err != nil {
		return Excerpt{}, fmt.Errorf("recheck expand document %q: %w", request.ID, err)
	}
	if err := ctx.Err(); err != nil {
		return Excerpt{}, err
	}
	if len(current.Hits) == 0 {
		return Excerpt{}, fmt.Errorf("expand document %q: %w", request.ID, os.ErrNotExist)
	}
	latest := current.Hits[0].Fields
	if stringField(latest, fieldSource) != source || stringField(latest, fieldKind) != kind || stringField(latest, fieldPath) != path || stringField(latest, fieldContentHash) != hash || boolField(latest, fieldStale) {
		return Excerpt{}, fmt.Errorf("expand document %q changed: %w", request.ID, ErrStaleContent)
	}
	return Excerpt{
		ID: request.ID, Source: source, Path: path, Field: request.Field,
		ContentHash: hash, Origin: origin, StartByte: int64(request.Offset),
		EndByte: int64(end), TotalBytes: int64(len(text)), Text: strings.Clone(text[request.Offset:end]),
	}, nil
}
