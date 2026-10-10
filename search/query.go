package search

import (
	"context"
	"fmt"
	"sort"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

var hitFields = []string{
	fieldSource, fieldPath, fieldTitle, fieldPreview, fieldPreviewTruncated,
	fieldContentHash, fieldStale, fieldGeneration, fieldHasVector, fieldMetadataRecord,
}

func (e *Engine) Search(ctx context.Context, request Query) ([]Hit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.lifeMu.RLock()
	defer e.lifeMu.RUnlock()
	if err := e.checkOpen(); err != nil {
		return nil, err
	}
	limit, err := validateQuery(request, e.options.VectorDimensions)
	if err != nil {
		return nil, err
	}
	if request.Mode != ModeKeyword {
		return e.vectorSearch(ctx, request, limit)
	}
	return e.keywordSearch(ctx, request, limit)
}

func (e *Engine) keywordSearch(ctx context.Context, request Query, limit int) ([]Hit, error) {
	filter, err := e.filterQuery(request.Filters)
	if err != nil {
		return nil, err
	}
	searchQuery := e.keywordQuery(request.Text)
	if filter != nil {
		searchQuery = bleve.NewConjunctionQuery(searchQuery, filter)
	}
	searchRequest := bleve.NewSearchRequestOptions(searchQuery, limit, 0, false)
	searchRequest.Fields = hitFields
	result, err := e.index.SearchInContext(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("Bleve keyword search: %w", err)
	}
	return e.hits(result, request, nil, nil)
}

func (e *Engine) keywordQuery(text string) query.Query {
	fields := []struct {
		name  string
		boost float64
	}{{fieldTitle, 3}, {fieldText, 1}, {fieldContext, 0.75}}
	clauses := make([]query.Query, 0, len(fields))
	for _, field := range fields {
		match := bleve.NewMatchQuery(text)
		match.SetField(field.name)
		match.SetBoost(field.boost)
		clauses = append(clauses, match)
	}
	return bleve.NewDisjunctionQuery(clauses...)
}

func (e *Engine) filterQuery(filters map[string]string) (query.Query, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	configured := make(map[string]struct{}, len(e.options.FilterFields))
	for _, field := range e.options.FilterFields {
		configured[field] = struct{}{}
	}
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	clauses := make([]query.Query, 0, len(keys))
	for _, key := range keys {
		field := key
		switch key {
		case fieldID, fieldSource, fieldPath:
		default:
			if _, ok := configured[key]; !ok {
				return nil, fmt.Errorf("filter field %q is not configured", key)
			}
			field = fieldMetadata + "." + key
		}
		term := bleve.NewTermQuery(filters[key])
		term.SetField(field)
		clauses = append(clauses, term)
	}
	return bleve.NewConjunctionQuery(clauses...), nil
}

func (e *Engine) channelRanks(ctx context.Context, textQuery query.Query, filters query.Query, limit int) (map[string]int, error) {
	searchQuery := textQuery
	if searchQuery == nil {
		searchQuery = bleve.NewMatchAllQuery()
	}
	if filters != nil {
		searchQuery = bleve.NewConjunctionQuery(searchQuery, filters)
	}
	request := bleve.NewSearchRequestOptions(searchQuery, limit, 0, false)
	request.Fields = []string{fieldID}
	result, err := e.index.SearchInContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Bleve channel ranking: %w", err)
	}
	ranks := make(map[string]int, len(result.Hits))
	for i, hit := range result.Hits {
		ranks[hit.ID] = i + 1
	}
	return ranks, nil
}

func (e *Engine) hits(result *bleve.SearchResult, request Query, keywordRanks, vectorRanks map[string]int) ([]Hit, error) {
	hits := make([]Hit, 0, len(result.Hits))
	e.stateMu.RLock()
	generation := e.generationID
	e.stateMu.RUnlock()
	for i, match := range result.Hits {
		metadata, err := e.hitMetadata(match.Fields[fieldMetadataRecord])
		if err != nil {
			return nil, fmt.Errorf("decode hit metadata for %q: %w", match.ID, err)
		}
		path := stringField(match.Fields, fieldPath)
		snippet := stringField(match.Fields, fieldPreview)
		locatorKind := "preview=0"
		if snippet == "" {
			snippet = stringField(match.Fields, fieldTitle)
			locatorKind = "title"
		}
		locator := path
		if locator == "" {
			locator = match.ID
		}
		locator = locator + "#" + locatorKind
		var evidence *RankEvidence
		if request.IncludeRankEvidence {
			ranks := RankEvidence{KeywordRank: keywordRanks[match.ID], VectorRank: vectorRanks[match.ID]}
			if request.Mode == ModeKeyword {
				ranks.KeywordRank = i + 1
			}
			if request.Mode == ModeSemantic {
				ranks.VectorRank = i + 1
			}
			evidence = &ranks
		}
		documentGeneration := stringField(match.Fields, fieldGeneration)
		if documentGeneration == "" {
			documentGeneration = generation
		}
		hits = append(hits, Hit{
			ID:               match.ID,
			Source:           stringField(match.Fields, fieldSource),
			Path:             path,
			Title:            stringField(match.Fields, fieldTitle),
			Snippet:          snippet,
			SnippetTruncated: boolField(match.Fields, fieldPreviewTruncated),
			Metadata:         metadata,
			ContentHash:      stringField(match.Fields, fieldContentHash),
			Score:            match.Score,
			ByteOffset:       0,
			Locator:          locator,
			Ranks:            evidence,
			Generation:       documentGeneration,
			Stale:            boolField(match.Fields, fieldStale),
			HasVector:        boolField(match.Fields, fieldHasVector),
		})
	}
	return hits, nil
}

func (e *Engine) hitMetadata(value any) (map[string]string, error) {
	if len(e.options.FilterFields) == 0 {
		return nil, nil
	}
	all, err := decodeStoredMetadata(value)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	configured := make(map[string]string, len(e.options.FilterFields))
	for _, field := range e.options.FilterFields {
		if value, ok := all[field]; ok {
			configured[field] = value
		}
	}
	if len(configured) == 0 {
		return nil, nil
	}
	return configured, nil
}

func boolField(fields map[string]interface{}, name string) bool {
	if fields == nil {
		return false
	}
	switch value := fields[name].(type) {
	case bool:
		return value
	case []interface{}:
		if len(value) > 0 {
			result, _ := value[0].(bool)
			return result
		}
	}
	return false
}

func stringField(fields map[string]interface{}, name string) string {
	if fields == nil {
		return ""
	}
	switch value := fields[name].(type) {
	case string:
		return value
	case []interface{}:
		if len(value) > 0 {
			text, _ := value[0].(string)
			return text
		}
	}
	return ""
}
