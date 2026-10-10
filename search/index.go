package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

const (
	fieldID               = "id"
	fieldSource           = "source"
	fieldPath             = "path"
	fieldTitle            = "title"
	fieldText             = "text"
	fieldContext          = "context"
	fieldPreview          = "preview"
	fieldPreviewTruncated = "preview_truncated"
	fieldMetadata         = "metadata"
	fieldMetadataRecord   = "metadata_record"
	fieldContentHash      = "content_hash"
	fieldKind             = "kind"
	fieldRecord           = "record"
	fieldVector           = "vector"
	fieldGeneration       = "generation"
	fieldStale            = "stale"
	fieldHasVector        = "has_vector"
	fileIDPrefix          = "file:"
	fileSource            = "fff-file"
	fileKind              = "file"
	documentKind          = "document"
	maxHitPreviewBytes    = 512
)

type indexedDocument struct {
	ID               string            `json:"id"`
	Source           string            `json:"source"`
	Path             string            `json:"path"`
	Title            string            `json:"title"`
	Text             string            `json:"text"`
	Context          string            `json:"context"`
	Preview          string            `json:"preview"`
	PreviewTruncated bool              `json:"preview_truncated"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ContentHash      string            `json:"content_hash,omitempty"`
	Kind             string            `json:"kind"`
	Generation       string            `json:"generation,omitempty"`
	Stale            bool              `json:"stale"`
	HasVector        bool              `json:"has_vector"`
	Record           string            `json:"record"`
	MetadataRecord   string            `json:"metadata_record"`
	Vector           []float32         `json:"vector,omitempty"`
}

type storedDocumentState struct {
	Vector []float32 `json:"vector,omitempty"`
}

type fileVectorState struct {
	ContentHash string
	Vector      []float32
}

func newIndexMapping(options Options) (*mapping.IndexMappingImpl, error) {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = mapping.NewDocumentStaticMapping()
	for _, field := range []string{fieldID, fieldSource, fieldPath, fieldContentHash, fieldKind, fieldGeneration} {
		indexMapping.DefaultMapping.AddFieldMappingsAt(field, mapping.NewKeywordFieldMapping())
	}
	indexMapping.DefaultMapping.AddFieldMappingsAt(fieldStale, mapping.NewBooleanFieldMapping())
	indexMapping.DefaultMapping.AddFieldMappingsAt(fieldHasVector, mapping.NewBooleanFieldMapping())
	for _, field := range []string{fieldTitle, fieldText, fieldContext} {
		textMapping := mapping.NewTextFieldMapping()
		textMapping.IncludeInAll = false
		textMapping.IncludeTermVectors = false
		textMapping.DocValues = false
		indexMapping.DefaultMapping.AddFieldMappingsAt(field, textMapping)
	}
	for _, field := range []string{fieldPreview, fieldRecord, fieldMetadataRecord} {
		storedText := mapping.NewTextFieldMapping()
		storedText.Index = false
		storedText.IncludeInAll = false
		storedText.IncludeTermVectors = false
		storedText.DocValues = false
		indexMapping.DefaultMapping.AddFieldMappingsAt(field, storedText)
	}
	indexMapping.DefaultMapping.AddFieldMappingsAt(fieldPreviewTruncated, mapping.NewBooleanFieldMapping())

	metadataMapping := mapping.NewDocumentStaticMapping()
	for _, field := range options.FilterFields {
		filterMapping := mapping.NewKeywordFieldMapping()
		filterMapping.Store = false
		filterMapping.IncludeTermVectors = false
		filterMapping.IncludeInAll = false
		filterMapping.DocValues = false
		metadataMapping.AddFieldMappingsAt(field, filterMapping)
	}
	indexMapping.DefaultMapping.AddSubDocumentMapping(fieldMetadata, metadataMapping)
	configureVectorMapping(indexMapping, options)
	if err := indexMapping.Validate(); err != nil {
		return nil, fmt.Errorf("validate Bleve mapping: %w", err)
	}
	return indexMapping, nil
}

func validateOpenedMapping(indexMapping mapping.IndexMapping, options Options) error {
	concrete, ok := indexMapping.(*mapping.IndexMappingImpl)
	if !ok {
		return errors.New("existing index uses an unsupported Bleve mapping implementation")
	}
	if indexMapping.FieldMappingForPath(fieldGeneration).Analyzer != "keyword" || indexMapping.FieldMappingForPath(fieldStale).Type != "boolean" || indexMapping.FieldMappingForPath(fieldHasVector).Type != "boolean" {
		return errors.New("existing index lacks the FFF generation/vector-state fields; rebuild it before opening")
	}
	metadataRecord := indexMapping.FieldMappingForPath(fieldMetadataRecord)
	if metadataRecord.Type != "text" || metadataRecord.Index || !metadataRecord.Store {
		return errors.New("existing index lacks the stored metadata field; rebuild it before opening")
	}
	record := indexMapping.FieldMappingForPath(fieldRecord)
	if record.Type != "text" || record.Index || !record.Store {
		return errors.New("existing index lacks the stored vector state; rebuild it before opening")
	}
	preview := indexMapping.FieldMappingForPath(fieldPreview)
	if preview.Type != "text" || preview.Index || !preview.Store || indexMapping.FieldMappingForPath(fieldPreviewTruncated).Type != "boolean" {
		return errors.New("existing index lacks bounded preview fields; rebuild it before opening")
	}
	vectorField := indexMapping.FieldMappingForPath(fieldVector)
	if vectorField.Dims != options.VectorDimensions {
		return fmt.Errorf("existing index vector dimension does not match configured dimension %d", options.VectorDimensions)
	}
	if vectorField.Similarity != vectorMetricName(options.VectorMetric) || vectorField.VectorIndexOptimizedFor != vectorQualityName(options.VectorQuality) {
		return errors.New("existing index vector metric or quality does not match configured options")
	}
	configuredFilters := make(map[string]struct{}, len(options.FilterFields))
	for _, field := range options.FilterFields {
		configuredFilters[field] = struct{}{}
	}
	if metadata := concrete.DefaultMapping.Properties[fieldMetadata]; metadata != nil {
		for field := range metadata.Properties {
			if _, configured := configuredFilters[field]; !configured {
				return fmt.Errorf("existing index has filter field %q not configured; rebuild it with matching FilterFields", field)
			}
		}
	}
	for _, field := range options.FilterFields {
		filterMapping := indexMapping.FieldMappingForPath(fieldMetadata + "." + field)
		if filterMapping.Analyzer != "keyword" || !filterMapping.Index {
			return fmt.Errorf("existing index does not have configured keyword filter field %q", field)
		}
		if filterMapping.Store || filterMapping.IncludeTermVectors || filterMapping.IncludeInAll || filterMapping.DocValues {
			return fmt.Errorf("existing index stores filter field %q redundantly; rebuild it with the compact mapping", field)
		}
	}
	return nil
}

func vectorMetricName(metric VectorMetric) string {
	switch metric {
	case VectorDot:
		return "dot_product"
	case VectorL2:
		return "l2_norm"
	default:
		return "cosine"
	}
}

func vectorQualityName(quality VectorQuality) string {
	if quality == VectorMemoryEfficient {
		return "memory-efficient"
	}
	return string(quality)
}

func (e *Engine) fileDocumentLocked(id string) (Document, bool, error) {
	record, found, err := e.documentByIDLocked(id)
	if err != nil || !found {
		return Document{}, found, err
	}
	if record.Document.Source != fileSource || record.Kind != fileKind {
		return Document{}, false, fmt.Errorf("reserved file ID %q is occupied by a non-FFF document", id)
	}
	return record.Document, true, nil
}

type indexedRecord struct {
	Document   Document
	Kind       string
	Generation string
	HasVector  bool
	Stale      bool
}

func (e *Engine) documentByIDLocked(id string) (indexedRecord, bool, error) {
	request := bleve.NewSearchRequestOptions(bleve.NewDocIDQuery([]string{id}), 1, 0, false)
	request.Fields = []string{fieldSource, fieldKind, fieldPath, fieldTitle, fieldText, fieldContext, fieldContentHash, fieldRecord, fieldMetadataRecord, fieldGeneration, fieldStale, fieldHasVector}
	result, err := e.index.Search(request)
	if err != nil {
		return indexedRecord{}, false, fmt.Errorf("read indexed document %q: %w", id, err)
	}
	if len(result.Hits) == 0 {
		return indexedRecord{}, false, nil
	}
	hit := result.Hits[0]
	document, err := storedDocument(hit.Fields[fieldRecord], hit.ID, hit.Fields)
	if err != nil {
		return indexedRecord{}, false, err
	}
	hasVector := boolField(hit.Fields, fieldHasVector)
	stale := boolField(hit.Fields, fieldStale)
	if hasVector && len(document.Vector) == 0 {
		return indexedRecord{}, false, fmt.Errorf("indexed document %q marks a missing vector as present", id)
	}
	if !hasVector || stale {
		document.Vector = nil
	}
	return indexedRecord{
		Document:   document,
		Kind:       stringField(hit.Fields, fieldKind),
		Generation: stringField(hit.Fields, fieldGeneration),
		HasVector:  hasVector,
		Stale:      stale,
	}, true, nil
}

func (e *Engine) fileVectorStatesLocked(ids []string) (map[string]fileVectorState, error) {
	states := make(map[string]fileVectorState, len(ids))
	if len(ids) == 0 {
		return states, nil
	}
	request := bleve.NewSearchRequestOptions(bleve.NewDocIDQuery(ids), len(ids), 0, false)
	request.Fields = []string{fieldSource, fieldKind, fieldContentHash, fieldRecord, fieldStale, fieldHasVector}
	result, err := e.index.Search(request)
	if err != nil {
		return nil, fmt.Errorf("read indexed file vectors: %w", err)
	}
	for _, hit := range result.Hits {
		if stringField(hit.Fields, fieldSource) != fileSource || stringField(hit.Fields, fieldKind) != fileKind {
			return nil, fmt.Errorf("reserved file ID %q is occupied by a non-FFF document", hit.ID)
		}
		state, err := decodeStoredDocumentState(hit.Fields[fieldRecord])
		if err != nil {
			return nil, err
		}
		hasVector := boolField(hit.Fields, fieldHasVector)
		stale := boolField(hit.Fields, fieldStale)
		if hasVector && len(state.Vector) == 0 {
			return nil, fmt.Errorf("indexed document %q marks a missing vector as present", hit.ID)
		}
		vector := state.Vector
		if !hasVector || stale {
			vector = nil
		}
		states[hit.ID] = fileVectorState{ContentHash: stringField(hit.Fields, fieldContentHash), Vector: vector}
	}
	return states, nil
}

func indexedData(document Document, kind, generation string, stale bool) (indexedDocument, error) {
	if document.Source == "" {
		document.Source = document.ID
	}
	encoded, err := json.Marshal(storedDocumentState{Vector: document.Vector})
	if err != nil {
		return indexedDocument{}, fmt.Errorf("marshal document %q: %w", document.ID, err)
	}
	metadataEncoded, err := json.Marshal(document.Metadata)
	if err != nil {
		return indexedDocument{}, fmt.Errorf("marshal document metadata %q: %w", document.ID, err)
	}
	preview, previewTruncated := boundedDocumentPreview(document.Text, document.Context)
	return indexedDocument{
		ID:               document.ID,
		Source:           document.Source,
		Path:             document.Path,
		Title:            document.Title,
		Text:             document.Text,
		Context:          document.Context,
		Preview:          preview,
		PreviewTruncated: previewTruncated,
		Metadata:         document.Metadata,
		ContentHash:      document.ContentHash,
		Kind:             kind,
		Generation:       generation,
		Stale:            stale,
		HasVector:        len(document.Vector) > 0,
		Record:           string(encoded),
		MetadataRecord:   string(metadataEncoded),
		Vector:           document.Vector,
	}, nil
}

func boundedDocumentPreview(text, context string) (string, bool) {
	source := text
	if source == "" {
		source = context
	}
	valid := utf8.ValidString(source)
	if !valid {
		source = strings.ToValidUTF8(source, "\uFFFD")
	}
	if len(source) <= maxHitPreviewBytes {
		return source, !valid
	}
	end := maxHitPreviewBytes
	for end > 0 && !utf8.RuneStart(source[end]) {
		end--
	}
	return source[:end], true
}

func contentHashForDocument(document Document) string {
	encoded, _ := json.Marshal(struct {
		Path     string            `json:"path"`
		Title    string            `json:"title"`
		Text     string            `json:"text"`
		Context  string            `json:"context"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{document.Path, document.Title, document.Text, document.Context, document.Metadata})
	return contentHash(encoded)
}

func storedDocument(value any, id string, fields map[string]interface{}) (Document, error) {
	state, err := decodeStoredDocumentState(value)
	if err != nil {
		return Document{}, err
	}
	metadata, err := decodeStoredMetadata(fields[fieldMetadataRecord])
	if err != nil {
		return Document{}, err
	}
	return Document{
		ID:          id,
		Source:      stringField(fields, fieldSource),
		Path:        stringField(fields, fieldPath),
		Title:       stringField(fields, fieldTitle),
		Text:        stringField(fields, fieldText),
		Context:     stringField(fields, fieldContext),
		Metadata:    metadata,
		Vector:      state.Vector,
		ContentHash: stringField(fields, fieldContentHash),
	}, nil
}

func decodeStoredDocumentState(value any) (storedDocumentState, error) {
	encoded, ok := value.(string)
	if !ok {
		return storedDocumentState{}, fmt.Errorf("stored document record has type %T", value)
	}
	var state storedDocumentState
	if err := json.Unmarshal([]byte(encoded), &state); err != nil {
		return storedDocumentState{}, fmt.Errorf("decode stored document record: %w", err)
	}
	return state, nil
}

func decodeStoredMetadata(value any) (map[string]string, error) {
	encoded, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("stored metadata record has type %T", value)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(encoded), &metadata); err != nil {
		return nil, fmt.Errorf("decode stored metadata: %w", err)
	}
	return metadata, nil
}
