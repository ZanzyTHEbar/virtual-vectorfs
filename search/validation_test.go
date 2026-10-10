package search

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestQueryValidation(t *testing.T) {
	tests := []struct {
		name       string
		query      Query
		dimensions int
		wantError  bool
	}{
		{name: "keyword requires text", query: Query{Mode: ModeKeyword}, wantError: true},
		{name: "keyword valid", query: Query{Mode: ModeKeyword, Text: "hello"}},
		{name: "semantic requires vector", query: Query{Mode: ModeSemantic}, dimensions: 2, wantError: true},
		{name: "semantic disabled", query: Query{Mode: ModeSemantic, Vector: []float32{1}}, wantError: true},
		{name: "semantic dimension mismatch", query: Query{Mode: ModeSemantic, Vector: []float32{1}}, dimensions: 2, wantError: true},
		{name: "semantic valid", query: Query{Mode: ModeSemantic, Vector: []float32{1, 0}}, dimensions: 2},
		{name: "hybrid requires both", query: Query{Mode: ModeHybrid, Text: "hello"}, dimensions: 2, wantError: true},
		{name: "hybrid valid", query: Query{Mode: ModeHybrid, Text: "hello", Vector: []float32{1, 0}}, dimensions: 2},
		{name: "mode is explicit", query: Query{Text: "hello"}, wantError: true},
		{name: "non-finite vector", query: Query{Mode: ModeSemantic, Vector: []float32{float32(math.NaN())}}, dimensions: 1, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateQuery(test.query, test.dimensions)
			if (err != nil) != test.wantError {
				t.Fatalf("validateQuery() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestDocumentVectorValidationBeforeIndexWrites(t *testing.T) {
	engine := &Engine{options: Options{VectorDimensions: 2}}
	err := engine.validateDocuments([]Document{
		{ID: "good", Vector: []float32{1, 0}},
		{ID: "bad", Vector: []float32{1}},
	})
	if err == nil {
		t.Fatal("expected the mismatched second vector to be rejected")
	}
}

func TestStableFileIDAndContentHash(t *testing.T) {
	const relative = "src/thing.go"
	if stableFileID(relative) != stableFileID(relative) {
		t.Fatal("file ID is not deterministic")
	}
	if stableFileID(relative) == stableFileID("src/other.go") {
		t.Fatal("different paths produced the same file ID")
	}
	if got := contentHash([]byte("example")); got != contentHash([]byte("example")) {
		t.Fatal("content hash is not deterministic")
	}
}

func TestPreviewUTF8BoundaryAndTruncation(t *testing.T) {
	text := strings.Repeat("a", 511) + "é" + "target"
	preview, truncated := boundedDocumentPreview(text, "")
	if !truncated || !utf8.ValidString(preview) || preview != text[:511] {
		t.Fatalf("preview split a UTF-8 rune or lost truncation state: %q truncated=%t", preview, truncated)
	}
}

func TestMemoryEfficientQualityUsesBleve261OptimizationName(t *testing.T) {
	if got := vectorQualityName(VectorMemoryEfficient); got != "memory-efficient" {
		t.Fatalf("Bleve optimization name = %q, want memory-efficient", got)
	}
}

func TestOpenedIndexRequiresStoredVectorRecord(t *testing.T) {
	options := Options{VectorDimensions: 2, VectorMetric: VectorCosine, VectorQuality: VectorLatency}
	indexMapping, err := newIndexMapping(options)
	if err != nil {
		t.Fatal(err)
	}
	fieldMapping := indexMapping.DefaultMapping.Properties[fieldRecord]
	if fieldMapping == nil || len(fieldMapping.Fields) != 1 {
		t.Fatal("vector-state field mapping not found")
	}
	fieldMapping.Fields[0].Store = false
	if err := validateOpenedMapping(indexMapping, options); err == nil {
		t.Fatal("existing index without stored vector state was accepted")
	}
}

func TestFilterFieldsAreIndexedButNotStoredTwice(t *testing.T) {
	indexMapping, err := newIndexMapping(Options{VectorDimensions: 2, VectorMetric: VectorCosine, VectorQuality: VectorLatency, FilterFields: []string{"kind"}})
	if err != nil {
		t.Fatal(err)
	}
	field := indexMapping.FieldMappingForPath(fieldMetadata + ".kind")
	if field.Analyzer != "keyword" || !field.Index || field.Store || field.IncludeTermVectors || field.IncludeInAll || field.DocValues {
		t.Fatalf("filter field mapping = %+v", field)
	}
}

func TestOpenedIndexRejectsRedundantlyStoredFilterField(t *testing.T) {
	options := Options{VectorDimensions: 2, VectorMetric: VectorCosine, VectorQuality: VectorLatency, FilterFields: []string{"kind"}}
	indexMapping, err := newIndexMapping(options)
	if err != nil {
		t.Fatal(err)
	}
	metadata := indexMapping.DefaultMapping.Properties[fieldMetadata]
	if metadata == nil || metadata.Properties["kind"] == nil || len(metadata.Properties["kind"].Fields) != 1 {
		t.Fatal("filter field mapping not found")
	}
	metadata.Properties["kind"].Fields[0].Store = true
	if err := validateOpenedMapping(indexMapping, options); err == nil {
		t.Fatal("existing index with duplicated stored filter metadata was accepted")
	}
}

func TestOpenedIndexRejectsRemovedFilterField(t *testing.T) {
	options := Options{VectorDimensions: 2, VectorMetric: VectorCosine, VectorQuality: VectorLatency, FilterFields: []string{"kind"}}
	indexMapping, err := newIndexMapping(options)
	if err != nil {
		t.Fatal(err)
	}
	options.FilterFields = nil
	if err := validateOpenedMapping(indexMapping, options); err == nil {
		t.Fatal("existing index with a removed filter field was accepted")
	}
}

func TestTextFieldsSkipUnusedStoredIndexFeatures(t *testing.T) {
	indexMapping, err := newIndexMapping(Options{VectorDimensions: 2, VectorMetric: VectorCosine, VectorQuality: VectorLatency})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fieldTitle, fieldText, fieldContext} {
		field := indexMapping.FieldMappingForPath(name)
		if !field.Store || !field.Index || field.IncludeTermVectors || field.IncludeInAll || field.DocValues {
			t.Errorf("text field %q mapping retains unused features: %+v", name, field)
		}
	}
}

func TestCallerIndexedFieldsRejectNUL(t *testing.T) {
	tests := []Document{
		{ID: "text", Text: "bad\x00text"},
		{ID: "title", Title: "bad\x00title"},
		{ID: "context", Context: "bad\x00context"},
		{ID: "path", Path: "bad\x00path"},
		{ID: "source", Source: "bad\x00source"},
		{ID: "metadata-key", Metadata: map[string]string{"bad\x00key": "value"}},
		{ID: "metadata-value", Metadata: map[string]string{"key": "bad\x00value"}},
	}
	engine := &Engine{}
	for _, document := range tests {
		t.Run(document.ID, func(t *testing.T) {
			if err := engine.validateDocuments([]Document{document}); err == nil {
				t.Fatal("caller document with NUL was accepted")
			}
		})
	}
}

func TestOptionsValidationAndDefaults(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name      string
		options   Options
		wantError bool
	}{
		{name: "dimensions required", options: Options{Root: root}, wantError: true},
		{name: "defaults", options: Options{Root: root, VectorDimensions: 384}},
		{name: "index nested in FFF root", options: Options{Root: root, IndexPath: filepath.Join(root, "index"), VectorDimensions: 384}, wantError: true},
		{name: "index parent traversal remains inside root", options: Options{Root: root, IndexPath: filepath.Join(root, "nested", "..", "index"), VectorDimensions: 384}, wantError: true},
		{name: "dimensions above Bleve limit", options: Options{Root: root, VectorDimensions: 4097}, wantError: true},
		{name: "unknown metric", options: Options{Root: root, VectorDimensions: 384, VectorMetric: "manhattan"}, wantError: true},
		{name: "negative write batch budget", options: Options{Root: root, VectorDimensions: 384, WriteBatchBytes: -1}, wantError: true},
		{name: "duplicate filter", options: Options{Root: root, VectorDimensions: 384, FilterFields: []string{"kind", "kind"}}, wantError: true},
		{name: "invalid filter path", options: Options{Root: root, VectorDimensions: 384, FilterFields: []string{"metadata.kind"}}, wantError: true},
		{name: "reserved metadata filter name", options: Options{Root: root, VectorDimensions: 384, FilterFields: []string{"path"}}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options, err := normalizeOptions(test.options)
			if (err != nil) != test.wantError {
				t.Fatalf("normalizeOptions() error = %v, wantError %t", err, test.wantError)
			}
			if err == nil && (options.Storage != StorageDisk || options.VectorDimensions != test.options.VectorDimensions || options.VectorMetric != VectorCosine || options.VectorQuality != VectorLatency || options.MaxFileBytes <= 0 || options.WriteBatchBytes != 32<<20) {
				t.Fatalf("unexpected defaults: %+v", options)
			}
		})
	}
}

func TestOptionsRejectEvidenceSiblingInsideFFFRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace.evidence")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(parent, "workspace")
	if _, err := normalizeOptions(Options{Root: root, IndexPath: indexPath, VectorDimensions: 384}); err == nil {
		t.Fatal("expected evidence sidecar inside FFF root to be rejected")
	}
}

func TestOptionsResolveIndexParentSymlinksBeforeContainmentCheck(t *testing.T) {
	root := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeOptions(Options{Root: root, IndexPath: filepath.Join(link, "nested", "index"), VectorDimensions: 384}); err == nil {
		t.Fatal("expected a symlinked parent resolving under the FFF root to be rejected")
	}
}

func TestIndexPathDefaultsOutsideRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	options, err := normalizeOptions(Options{Root: root, VectorDimensions: 384})
	if err != nil {
		t.Fatal(err)
	}
	if options.IndexPath == root || options.IndexPath == filepath.Join(root, ".vvfs-search") {
		t.Fatalf("default index path is inside scanned root: %q", options.IndexPath)
	}
}
