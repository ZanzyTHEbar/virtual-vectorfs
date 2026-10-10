// These integration tests require Linux, the locally installed native/fffbridge library,
// and FAISS headers plus libfaiss_c available to cgo/runtime linking.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestKeywordVectorAndHybridSearchContract(t *testing.T) {
	root := t.TempDir()
	engine := openTestEngine(t, root, Options{VectorDimensions: 2, FilterFields: []string{"kind"}})
	if err := engine.Upsert(context.Background(),
		Document{ID: "would-be-written", Vector: []float32{1, 0}},
		Document{ID: "invalid", Vector: []float32{1}},
	); err == nil {
		t.Fatal("expected invalid batch vector to be rejected")
	}
	documents := []Document{
		{ID: "doc-a", Source: "source-a", Path: "a.txt", Title: "amber", Text: "amber fox", Metadata: map[string]string{"kind": "animal", "private": "hidden"}, Vector: []float32{1, 0}},
		{ID: "doc-b", Source: "source-b", Path: "b.txt", Title: "blue", Text: "blue whale", Metadata: map[string]string{"kind": "marine", "private": "hidden"}, Vector: []float32{0, 1}},
	}
	if err := engine.Upsert(context.Background(), documents...); err != nil {
		t.Fatal(err)
	}

	keyword, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "amber", Filters: map[string]string{"kind": "animal"}, IncludeRankEvidence: true})
	if err != nil || len(keyword) != 1 || keyword[0].ID != "doc-a" || keyword[0].Ranks.KeywordRank != 1 || keyword[0].Metadata["kind"] != "animal" || len(keyword[0].Metadata) != 1 {
		t.Fatalf("keyword search = %#v, %v", keyword, err)
	}
	encodedHit, err := json.Marshal(keyword[0])
	if err != nil {
		t.Fatal(err)
	}
	var hitFields map[string]json.RawMessage
	if err := json.Unmarshal(encodedHit, &hitFields); err != nil {
		t.Fatal(err)
	}
	if _, ok := hitFields["record"]; ok {
		t.Fatal("Hit exposed the internal record blob")
	}
	if _, ok := hitFields["vector"]; ok {
		t.Fatal("Hit exposed the caller vector")
	}
	semantic, err := engine.Search(context.Background(), Query{Mode: ModeSemantic, Vector: []float32{0.9, 0.1}, IncludeRankEvidence: true})
	if err != nil || len(semantic) == 0 || semantic[0].ID != "doc-a" || semantic[0].Ranks.VectorRank != 1 || semantic[0].Metadata["kind"] != "animal" {
		t.Fatalf("semantic search = %#v, %v", semantic, err)
	}
	hybridFast, err := engine.Search(context.Background(), Query{Mode: ModeHybrid, Text: "whale", Vector: []float32{1, 0}, Limit: 2})
	if err != nil || len(hybridFast) == 0 {
		t.Fatalf("default hybrid search = %#v, %v", hybridFast, err)
	}
	for _, hit := range hybridFast {
		if hit.Ranks != nil {
			t.Fatalf("default hybrid search requested rank evidence: %+v", hit.Ranks)
		}
	}
	hybrid, err := engine.Search(context.Background(), Query{Mode: ModeHybrid, Text: "whale", Vector: []float32{1, 0}, Limit: 2, IncludeRankEvidence: true})
	if err != nil || len(hybrid) == 0 {
		t.Fatalf("hybrid search = %#v, %v", hybrid, err)
	}
	var hasKeywordRank, hasVectorRank bool
	for _, hit := range hybrid {
		hasKeywordRank = hasKeywordRank || hit.Ranks.KeywordRank > 0
		hasVectorRank = hasVectorRank || hit.Ranks.VectorRank > 0
		if hit.Metadata["kind"] == "" || hit.Metadata["private"] != "" {
			t.Fatalf("hybrid result did not expose only configured metadata: %+v", hit.Metadata)
		}
		if hit.Generation == "" {
			t.Fatalf("hybrid result lacks generation evidence: %+v", hit)
		}
	}
	if !hasKeywordRank || !hasVectorRank {
		t.Fatalf("hybrid results lack channel rank evidence: %+v", hybrid)
	}
}

func TestMemoryEfficientUsesBleve261Value(t *testing.T) {
	if got := vectorQualityName(VectorMemoryEfficient); got != "memory-efficient" {
		t.Fatalf("Bleve vector optimization = %q, want supported v2.6.1 value memory-efficient", got)
	}
	indexMapping, err := newIndexMapping(Options{VectorDimensions: 2, VectorQuality: VectorMemoryEfficient})
	if err != nil {
		t.Fatal(err)
	}
	if err := indexMapping.Validate(); err != nil {
		t.Fatalf("Bleve v2.6.1 rejected memory-efficient mapping: %v", err)
	}
}

func TestPersistentIndexRejectsVectorSchemaChanges(t *testing.T) {
	root := t.TempDir()
	options := Options{
		Root:             root,
		IndexPath:        filepath.Join(t.TempDir(), "index"),
		Storage:          StorageDisk,
		VectorDimensions: 2,
		VectorMetric:     VectorCosine,
		VectorQuality:    VectorLatency,
	}
	engine, err := Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		change func(*Options)
	}{
		{name: "dimensions", change: func(options *Options) { options.VectorDimensions = 3 }},
		{name: "metric", change: func(options *Options) { options.VectorMetric = VectorDot }},
		{name: "quality", change: func(options *Options) { options.VectorQuality = VectorRecall }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := options
			test.change(&changed)
			if engine, err := Open(context.Background(), changed); err == nil {
				_ = engine.Close()
				t.Fatal("opened an index with a different vector schema")
			}
		})
	}
}

func TestUnchangedFileSyncPreservesVectorAndChangedContentInvalidatesIt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched.txt")
	if err := os.WriteFile(path, []byte("before change"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := openTestEngine(t, root, Options{VectorDimensions: 2})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	fileID := stableFileID("watched.txt")
	setFileVector(t, engine, "watched.txt", []float32{1, 0})
	unchangedReport, err := engine.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if unchangedReport.MissingVectors != 0 {
		t.Fatalf("unchanged file embedding was counted missing: %+v", unchangedReport)
	}
	unchanged := readFileDocument(t, engine, "watched.txt")
	if len(unchanged.Vector) != 2 || unchanged.Vector[0] != 1 || unchanged.Vector[1] != 0 {
		t.Fatalf("unchanged file vector was not preserved: %v", unchanged.Vector)
	}
	if err := os.WriteFile(path, []byte("after change"), 0o600); err != nil {
		t.Fatal(err)
	}
	changedReport, err := engine.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if changedReport.MissingVectors != 1 {
		t.Fatalf("changed file without a fresh embedding was not counted missing: %+v", changedReport)
	}
	changed := readFileDocument(t, engine, "watched.txt")
	engine.writeMu.Lock()
	changedRecord, found, recordErr := engine.documentByIDLocked(fileID)
	engine.writeMu.Unlock()
	if recordErr != nil || !found {
		t.Fatalf("read changed indexed file state: found=%t err=%v", found, recordErr)
	}
	if changed.ContentHash == unchanged.ContentHash || len(changed.Vector) != 0 || changedRecord.Stale {
		t.Fatalf("changed file retained stale vector or status: %+v", changed)
	}
	if fileID != changed.ID {
		t.Fatalf("file ID changed with content: %q != %q", fileID, changed.ID)
	}
	semantic, err := engine.Search(context.Background(), Query{Mode: ModeSemantic, Vector: []float32{1, 0}})
	if err != nil || len(semantic) != 0 {
		t.Fatalf("changed file remained semantic-searchable: %#v, %v", semantic, err)
	}
}

func TestFileVectorProvisionAndContentHashGuard(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "vector.txt")
	if err := os.WriteFile(filePath, []byte("first indexed content"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := openTestEngine(t, root, Options{VectorDimensions: 2})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	inputs, cursor, err := engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(inputs) != 1 || cursor != "" || inputs[0].Path != "vector.txt" || inputs[0].Text != "first indexed content" || inputs[0].ContentHash == "" {
		t.Fatalf("file missing-vector input = %#v cursor=%q err=%v", inputs, cursor, err)
	}
	fileInput := inputs[0]
	if err := engine.UpsertVectors(context.Background(), VectorUpdate{
		ID: fileInput.ID, ExpectedContentHash: fileInput.ContentHash, Vector: []float32{1, 0},
	}); err != nil {
		t.Fatal(err)
	}
	missing, _, err := engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(missing) != 0 {
		t.Fatalf("provided file vector remained missing: %#v, %v", missing, err)
	}
	semantic, err := engine.Search(context.Background(), Query{Mode: ModeSemantic, Vector: []float32{1, 0}})
	if err != nil || len(semantic) != 1 || semantic[0].ID != fileInput.ID || !semantic[0].HasVector {
		t.Fatalf("provided file vector not searchable: %#v, %v", semantic, err)
	}

	if err := os.WriteFile(filePath, []byte("second indexed content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := engine.UpsertVectors(context.Background(), VectorUpdate{
		ID: fileInput.ID, ExpectedContentHash: fileInput.ContentHash, Vector: []float32{0, 1},
	}); !errors.Is(err, ErrStaleContent) {
		t.Fatalf("disk change racing with vector update error = %v, want ErrStaleContent", err)
	}
	missing, _, err = engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(missing) != 0 {
		t.Fatalf("rejected vector update mutated the indexed file: %#v, %v", missing, err)
	}
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.UpsertVectors(context.Background(), VectorUpdate{
		ID: fileInput.ID, ExpectedContentHash: fileInput.ContentHash, Vector: []float32{0, 1},
	}); !errors.Is(err, ErrStaleContent) {
		t.Fatalf("old hash update error = %v, want ErrStaleContent", err)
	}
	missing, _, err = engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(missing) != 1 || missing[0].ContentHash == fileInput.ContentHash || missing[0].HasVector {
		t.Fatalf("changed file was not exposed as vectorless: %#v, %v", missing, err)
	}
}

func TestCallerRecordVectorUpdateAndHashMismatch(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{VectorDimensions: 2, FilterFields: []string{"kind"}})
	if err := engine.Upsert(context.Background(), Document{
		ID: "caller-record", Source: "caller-source", Path: "shared.txt", Text: "caller record version one", Metadata: map[string]string{"kind": "animal"},
	}); err != nil {
		t.Fatal(err)
	}
	inputs, _, err := engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(inputs) != 1 || inputs[0].ID != "caller-record" || inputs[0].ContentHash == "" || inputs[0].Metadata["kind"] != "animal" {
		t.Fatalf("caller missing-vector input = %#v, %v", inputs, err)
	}
	oldHash := inputs[0].ContentHash
	if err := engine.UpsertVectors(context.Background(), VectorUpdate{
		ID: "caller-record", ExpectedContentHash: oldHash, Vector: []float32{0, 1},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeSemantic, Vector: []float32{0, 1}})
	if err != nil || len(hits) != 1 || hits[0].ID != "caller-record" || !hits[0].HasVector {
		t.Fatalf("caller vector update not searchable: %#v, %v", hits, err)
	}
	if err := engine.Upsert(context.Background(), Document{
		ID: "caller-record", Source: "caller-source", Path: "shared.txt", Text: "caller record version two",
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.UpsertVectors(context.Background(), VectorUpdate{
		ID: "caller-record", ExpectedContentHash: oldHash, Vector: []float32{0, 1},
	}); !errors.Is(err, ErrStaleContent) {
		t.Fatalf("changed caller record vector update error = %v, want ErrStaleContent", err)
	}
	inputs, _, err = engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(inputs) != 1 || inputs[0].ContentHash == oldHash || inputs[0].HasVector {
		t.Fatalf("updated caller record was not re-enumerated: %#v, %v", inputs, err)
	}
}

func TestStaleFileIsLexicalButNotSemanticSearchable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "stale.txt"), []byte("stale lexical evidence"))
	engine := openTestEngine(t, root, Options{VectorDimensions: 2})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	setFileVector(t, engine, "stale.txt", []float32{1, 0})
	engine.writeMu.Lock()
	_, found, err := engine.markFileStaleLocked(stableFileID("stale.txt"), engine.currentGeneration())
	if err != nil || !found {
		engine.writeMu.Unlock()
		t.Fatalf("mark file stale: found=%t err=%v", found, err)
	}
	keyword, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "stale lexical"})
	if err != nil || len(keyword) != 1 || !keyword[0].Stale {
		engine.writeMu.Unlock()
		t.Fatalf("stale file did not remain lexical-searchable with evidence: %#v, %v", keyword, err)
	}
	semantic, err := engine.Search(context.Background(), Query{Mode: ModeSemantic, Vector: []float32{1, 0}})
	engine.writeMu.Unlock()
	if err != nil || len(semantic) != 0 {
		t.Fatalf("stale file remained semantic-searchable: %#v, %v", semantic, err)
	}
}

func TestFileEventsPreserveCallerDocumentsSharingPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched.txt")
	if err := os.WriteFile(path, []byte("file text"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := openTestEngine(t, root, Options{VectorDimensions: 2})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.Upsert(context.Background(), Document{
		ID: "caller", Path: "watched.txt", Text: "caller-owned annotation", Vector: []float32{0, 1},
	}); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(root, "renamed.txt")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	waitForHit(t, engine, "file text", "renamed.txt")
	assertCallerAnnotation(t, engine)
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "file text"})
		return err == nil && len(hits) == 0
	})
	assertCallerAnnotation(t, engine)
}

func setFileVector(t *testing.T, engine *Engine, relative string, vector []float32) {
	t.Helper()
	engine.writeMu.Lock()
	defer engine.writeMu.Unlock()
	document := readFileDocumentLocked(t, engine, relative)
	document.Vector = vector
	prepared, err := indexedData(document, fileKind, engine.currentGeneration(), false)
	if err != nil {
		t.Fatal(err)
	}
	batch := engine.index.NewBatch()
	if err := batch.Index(document.ID, prepared); err != nil {
		t.Fatal(err)
	}
	if err := engine.index.Batch(batch); err != nil {
		t.Fatal(err)
	}
}

func readFileDocument(t *testing.T, engine *Engine, relative string) Document {
	t.Helper()
	engine.writeMu.Lock()
	defer engine.writeMu.Unlock()
	return readFileDocumentLocked(t, engine, relative)
}

func readFileDocumentLocked(t *testing.T, engine *Engine, relative string) Document {
	t.Helper()
	document, found, err := engine.fileDocumentLocked(stableFileID(relative))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("file document for %q not found", relative)
	}
	return document
}

func assertCallerAnnotation(t *testing.T, engine *Engine) {
	t.Helper()
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "caller-owned annotation"})
	if err != nil || len(hits) != 1 || hits[0].ID != "caller" || hits[0].Path != "watched.txt" {
		t.Fatalf("file event changed caller document: %#v, %v", hits, err)
	}
}
