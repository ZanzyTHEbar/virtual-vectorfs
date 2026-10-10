// FFF integration tests require Linux and the locally installed native/fffbridge library.
package search

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func openTestEngine(t *testing.T, root string, options Options) *Engine {
	t.Helper()
	if options.VectorDimensions == 0 {
		options.VectorDimensions = 2
	}
	options.Root = root
	options.IndexPath = filepath.Join(t.TempDir(), "index")
	options.Storage = StorageMemory
	engine, err := Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})
	return engine
}

func TestSyncInventoryManifestAndSkipCounts(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "notes.txt"), []byte("needle text\n"))
	writeTestFile(t, filepath.Join(root, "binary.dat"), []byte{0, 1, 2})
	writeTestFile(t, filepath.Join(root, "large.txt"), []byte(strings.Repeat("x", 40)))
	engine := openTestEngine(t, root, Options{MaxFileBytes: 20, Workers: 1})
	report, err := engine.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 3 || report.InventoryPages != 1 || report.InventoryPageSize != inventoryPageSize || report.Indexed != 1 || report.SkippedBinary != 1 || report.SkippedOverLimit != 1 {
		t.Fatalf("unexpected build report: %+v", report)
	}
	if report.EvidenceCount != report.Scanned || filepath.Dir(report.EvidencePath) != evidenceDirectory(engine.options.IndexPath, engine.options.Storage) || filepath.Dir(report.EvidencePath) == engine.options.IndexPath {
		t.Fatalf("unexpected evidence artifact summary: %+v", report)
	}
	manifestBytes, err := os.ReadFile(manifestPath(engine.options, report.GenerationID))
	if err != nil {
		t.Fatal(err)
	}
	var manifest BuildReport
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.GenerationID != report.GenerationID || manifest.EvidenceCount != report.Scanned || strings.Contains(string(manifestBytes), "needle text") {
		t.Fatalf("manifest evidence mismatch or content leak: %s", manifestBytes)
	}
	var manifestFields map[string]json.RawMessage
	if err := json.Unmarshal(manifestBytes, &manifestFields); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifestFields["files"]; ok {
		t.Fatal("manifest retained a corpus-sized file list")
	}
	var notes FileEvidence
	evidenceCount := 0
	artifact, err := os.Open(report.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bufio.NewReader(artifact))
	for {
		var evidence FileEvidence
		if err := decoder.Decode(&evidence); err != nil {
			if err == io.EOF {
				break
			}
			_ = artifact.Close()
			t.Fatal(err)
		}
		evidenceCount++
		if evidence.Path == "notes.txt" {
			notes = evidence
		}
	}
	if err := artifact.Close(); err != nil {
		t.Fatal(err)
	}
	artifactBytes, err := os.ReadFile(report.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if notes.SHA256 == "" || evidenceCount != report.EvidenceCount || strings.Contains(string(artifactBytes), "needle text") {
		t.Fatal("expected streamed per-file SHA-256 evidence")
	}
	files, err := engine.FindFiles(context.Background(), FileQuery{Text: "notes.txt", Limit: 10})
	if err != nil || len(files.Matches) == 0 || files.Matches[0].Path != "notes.txt" {
		t.Fatalf("FFF path search = %#v, %v", files, err)
	}
	grep, err := engine.Grep(context.Background(), GrepQuery{Text: "needle", Mode: GrepModePlain, Limit: 10})
	if err != nil || len(grep.Matches) == 0 || grep.Matches[0].Path != "notes.txt" {
		t.Fatalf("FFF grep = %#v, %v", grep, err)
	}
}

func TestBackslashFilenameDoesNotAliasSlashPath(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, `a\b`), []byte("backslash-only marker"))
	writeTestFile(t, filepath.Join(root, "a", "b"), []byte("slash-only marker"))
	engine := openTestEngine(t, root, Options{Workers: 1})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		text string
		path string
	}{{"backslash-only marker", `a\b`}, {"slash-only marker", "a/b"}} {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: test.text})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, hit := range hits {
			if hit.Path == test.path && strings.Contains(hit.Snippet, test.text) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("search %q = %#v; want matching hit at path %q", test.text, hits, test.path)
		}
	}
}

func TestSyncFilesFailureUpdatesLastError(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.SyncFiles(ctx)
	if err == nil {
		t.Fatal("SyncFiles with a canceled context succeeded")
	}
	if got := engine.Status().LastError; got != err.Error() {
		t.Fatalf("Status.LastError = %q, want %q", got, err.Error())
	}
}

func TestSyncUsesPagedInventoryAndByteBoundedBatches(t *testing.T) {
	root := t.TempDir()
	const (
		fileCount     = inventoryPageSize*2 + 1
		liveFileCount = inventoryPageSize + 1
	)
	content := []byte(strings.Repeat("b ", 32))
	expectedPaths := make(map[string]struct{}, liveFileCount)
	for i := 0; i < fileCount; i++ {
		name := fmt.Sprintf("file-%04d.txt", i)
		if i >= inventoryPageSize {
			expectedPaths[name] = struct{}{}
		}
		writeTestFile(t, filepath.Join(root, name), content)
	}
	const writeBatchBytes = 8 * 1024
	engine := openTestEngine(t, root, Options{Workers: 4, MaxFileBytes: 128, WriteBatchBytes: writeBatchBytes})
	if err := engine.bridge.WaitReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < inventoryPageSize; i++ {
		name := fmt.Sprintf("file-%04d.txt", i)
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	waitUntil(t, func() bool {
		status, err := engine.bridge.Status()
		return err == nil && status.IndexedFiles == liveFileCount
	})
	report, err := engine.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expectedPages := (fileCount + inventoryPageSize - 1) / inventoryPageSize
	if report.Scanned != liveFileCount || report.InventoryPages != expectedPages || report.InventoryPageSize != inventoryPageSize || report.Indexed != liveFileCount || report.EvidenceCount != liveFileCount || report.Bytes != int64(liveFileCount*len(content)) {
		t.Fatalf("unexpected FFF page processing report: %+v", report)
	}
	if report.Workers != 4 || report.MaxFileBytes != 128 || report.WriteBatchBytes != writeBatchBytes || report.WriteBatchDocuments != maxWriteBatchDocuments {
		t.Fatalf("resource bounds were not reported: %+v", report)
	}
	if report.WriteBatches != 5 {
		t.Fatalf("byte budget produced %d Bleve batches, want 5", report.WriteBatches)
	}
	artifact, err := os.Open(report.EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close()
	seenPaths := make(map[string]struct{}, liveFileCount)
	decoder := json.NewDecoder(artifact)
	for {
		var evidence FileEvidence
		if err := decoder.Decode(&evidence); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if _, expected := expectedPaths[evidence.Path]; !expected {
			t.Fatalf("inventory evidence contains unexpected path %q", evidence.Path)
		}
		if _, duplicate := seenPaths[evidence.Path]; duplicate {
			t.Fatalf("inventory indexed %q more than once", evidence.Path)
		}
		seenPaths[evidence.Path] = struct{}{}
	}
	if len(seenPaths) != liveFileCount {
		t.Fatalf("inventory evidence contains %d unique paths, want %d", len(seenPaths), liveFileCount)
	}
	for name := range expectedPaths {
		if _, found := seenPaths[name]; !found {
			t.Errorf("inventory evidence is missing %q", name)
		}
	}
}

func TestFileEventsCreateModifyRenameAndDelete(t *testing.T) {
	root := t.TempDir()
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "watched.txt")
	writeTestFile(t, filePath, []byte("created marker"))
	waitForHit(t, engine, "created marker", "watched.txt")
	if err := engine.Upsert(context.Background(), Document{
		ID: "annotation", Source: "caller", Path: "watched.txt", Text: "caller annotation survives rename and delete",
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filePath, []byte("modified marker"))
	waitForHit(t, engine, "modified marker", "watched.txt")
	renamedPath := filepath.Join(root, "renamed.txt")
	if err := os.Rename(filePath, renamedPath); err != nil {
		t.Fatal(err)
	}
	waitForHit(t, engine, "modified marker", "renamed.txt")
	if err := os.Remove(renamedPath); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "modified marker"})
		return err == nil && len(hits) == 0
	})
	caller, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "caller annotation"})
	if err != nil || len(caller) != 1 || caller[0].ID != "annotation" || caller[0].Path != "watched.txt" {
		t.Fatalf("file removal changed caller document: %#v, %v", caller, err)
	}
}

func TestFailedFileReadPreservesOldSearchableData(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file read permission checks")
	}
	root := t.TempDir()
	filePath := filepath.Join(root, "private.txt")
	writeTestFile(t, filePath, []byte("previous searchable content"))
	engine := openTestEngine(t, root, Options{})
	initial, err := engine.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filePath, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filePath, 0o600)
	report, err := engine.SyncFiles(context.Background())
	if err == nil {
		t.Fatal("expected failed file read to fail the sync")
	}
	if got := engine.Status().LastError; got != err.Error() {
		t.Fatalf("Status.LastError = %q, want %q", got, err.Error())
	}
	if report.Errors != 1 || report.Stale != 1 {
		t.Fatalf("failed read was not reported stale: %+v", report)
	}
	manifestBytes, err := os.ReadFile(manifestPath(engine.options, initial.GenerationID))
	if err != nil {
		t.Fatal(err)
	}
	var manifest BuildReport
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.GenerationID != initial.GenerationID || report.GenerationID == manifest.GenerationID {
		t.Fatalf("failed sync published a new manifest: report=%q manifest=%q", report.GenerationID, manifest.GenerationID)
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "previous searchable"})
	if err != nil || len(hits) != 1 || !hits[0].Stale {
		t.Fatalf("failed read removed or failed to mark prior content stale: %#v, %v", hits, err)
	}
}

func TestInventoryPathDeletedBeforeReadIsNormalDrift(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "gone.txt")
	writeTestFile(t, filePath, []byte("content removed before read"))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	_, _, _, readErr := engine.readIndexedFile("gone.txt")
	if !errors.Is(readErr, errInventoryPathDisappeared) {
		t.Fatalf("read missing inventoried path error = %v, want disappearance sentinel", readErr)
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("missing path error = %v, want os.ErrNotExist", readErr)
	}

	generation, err := newGenerationID()
	if err != nil {
		t.Fatal(err)
	}
	artifactWriter, err := engine.newEvidenceWriter(generation)
	if err != nil {
		t.Fatal(err)
	}
	defer artifactWriter.Abort()
	report := BuildReport{}
	batch := newBleveBatchWriter(engine.index, &report, engine.options.WriteBatchBytes)
	result := inventoryRead{
		relative: "gone.txt",
		id:       stableFileID("gone.txt"),
		err:      readErr,
	}
	evidence := FileEvidence{ID: result.id, Path: result.relative}
	if err := engine.handleInventoryReadError(result, evidence, generation, batch, &report, artifactWriter); err != nil {
		t.Fatal(err)
	}
	if err := batch.Flush(); err != nil {
		t.Fatal(err)
	}
	if report.Errors != 0 || report.Skipped != 1 || report.Stale != 0 {
		t.Fatalf("deleted file was not treated as normal drift: %+v", report)
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "content removed before read"})
	if err != nil || len(hits) != 0 {
		t.Fatalf("deleted file remained searchable: %#v, %v", hits, err)
	}
	if err := engine.reconcilePath("gone.txt"); err != nil {
		t.Fatalf("watcher reconciliation of missing path: %v", err)
	}
}

func TestReadIndexedFileDoesNotBlockOnFIFO(t *testing.T) {
	root := t.TempDir()
	pipe := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	engine := openTestEngine(t, root, Options{})
	result := make(chan struct {
		skip string
		err  error
	}, 1)
	go func() {
		_, _, skip, err := engine.readIndexedFile("pipe")
		result <- struct {
			skip string
			err  error
		}{skip: skip, err: err}
	}()
	select {
	case got := <-result:
		if got.err != nil || got.skip != "unsupported" {
			t.Fatalf("FIFO read = skip %q, error %v; want unsupported", got.skip, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("opening an inventoried FIFO blocked")
	}
}

func TestCallerCannotUseReservedFileIDs(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: fileIDPrefix + "caller"}); err == nil {
		t.Fatal("expected caller file-ID namespace rejection")
	}
	if err := engine.Delete(context.Background(), fileIDPrefix+"caller"); err == nil {
		t.Fatal("expected caller delete namespace rejection")
	}
}

func TestMissingVectorsPagesCallerRecordsWithContentHashes(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{Workers: 1})
	if err := engine.Upsert(context.Background(),
		Document{ID: "a", Source: "source-a", Path: "a.txt", Text: "alpha"},
		Document{ID: "b", Source: "source-b", Path: "b.txt", Text: "bravo"},
		Document{ID: "c", Source: "source-c", Path: "c.txt", Text: "charlie"},
	); err != nil {
		t.Fatal(err)
	}
	var cursor string
	seen := make(map[string]VectorInput)
	for page := 0; page < 3; page++ {
		items, next, err := engine.MissingVectors(context.Background(), cursor, MaxSearchLimit)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("page %d returned %d vector inputs, want one", page, len(items))
		}
		if items[0].ContentHash == "" || items[0].HasVector || items[0].Stale {
			t.Fatalf("unexpected vector input: %+v", items[0])
		}
		seen[items[0].ID] = items[0]
		cursor = next
	}
	if cursor != "" || len(seen) != 3 || seen["b"].Text != "bravo" || seen["b"].Source != "source-b" {
		t.Fatalf("missing-vector pagination lost or duplicated records: cursor=%q seen=%+v", cursor, seen)
	}
}

func TestStaleFilePruneIsPagedAndLeavesCallerDocuments(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{Workers: 1})
	if err := engine.Upsert(context.Background(), Document{
		ID: "caller", Source: fileSource, Path: "shared.txt", Text: "caller shared content",
	}); err != nil {
		t.Fatal(err)
	}
	engine.writeMu.Lock()
	batch := engine.index.NewBatch()
	for _, relative := range []string{"a.txt", "b.txt", "c.txt"} {
		document := Document{ID: stableFileID(relative), Source: fileSource, Path: relative, Text: "stale inventory content", ContentHash: relative}
		prepared, err := indexedData(document, fileKind, "old-generation", false)
		if err != nil {
			engine.writeMu.Unlock()
			t.Fatal(err)
		}
		if err := batch.Index(document.ID, prepared); err != nil {
			engine.writeMu.Unlock()
			t.Fatal(err)
		}
	}
	if err := engine.index.Batch(batch); err != nil {
		engine.writeMu.Unlock()
		t.Fatal(err)
	}
	if err := engine.pruneStaleFiles(context.Background(), "new-generation"); err != nil {
		engine.writeMu.Unlock()
		t.Fatal(err)
	}
	engine.writeMu.Unlock()
	files, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "stale inventory"})
	if err != nil || len(files) != 0 {
		t.Fatalf("stale file docs remained after paged prune: %#v, %v", files, err)
	}
	caller, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "caller shared"})
	if err != nil || len(caller) != 1 || caller[0].ID != "caller" {
		t.Fatalf("prune removed caller document: %#v, %v", caller, err)
	}
}

func TestConfiguredMetadataFiltersAreExact(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{FilterFields: []string{"kind"}})
	if err := engine.Upsert(context.Background(), Document{
		ID: "doc", Source: "source", Title: "filter target", Text: "filter target", Metadata: map[string]string{"kind": "animal"},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "filter", Filters: map[string]string{"kind": "animal"}})
	if err != nil || len(hits) != 1 || hits[0].ID != "doc" || hits[0].Metadata["kind"] != "animal" {
		t.Fatalf("exact metadata filter = %#v, %v", hits, err)
	}
	if _, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "filter", Filters: map[string]string{"unknown": "value"}}); err == nil {
		t.Fatal("expected unconfigured filter field error")
	}
}

func TestCallerHashCannotBeReusedAfterIndexedContentChanges(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	original := Document{
		ID: "caller-hash", Path: "before.txt", Title: "before title", Text: "before text", Context: "before context",
		Metadata: map[string]string{"kind": "before"},
	}
	if err := engine.Upsert(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	inputs, _, err := engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(inputs) != 1 {
		t.Fatalf("initial missing-vector input = %#v, %v", inputs, err)
	}
	originalHash := inputs[0].ContentHash
	changed := original
	changed.Path = "after.txt"
	changed.Title = "after title"
	changed.Text = "after text"
	changed.Context = "after context"
	changed.Metadata = map[string]string{"kind": "after"}
	changed.ContentHash = originalHash
	if err := engine.Upsert(context.Background(), changed); err == nil {
		t.Fatal("Upsert accepted a caller-supplied hash for different indexed content")
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "before text"})
	if err != nil || len(hits) != 1 || hits[0].Path != "before.txt" || hits[0].ContentHash != originalHash {
		t.Fatalf("rejected stale-hash update mutated caller record: %#v, %v", hits, err)
	}
}

func TestSearchLimitsHaveDocumentedMaximum(t *testing.T) {
	for _, limit := range []int{0, 1, MaxSearchLimit, MaxSearchLimit + 1} {
		got, err := normalizeSearchLimit(limit)
		if err != nil {
			t.Fatalf("normalizeSearchLimit(%d): %v", limit, err)
		}
		want := limit
		if want == 0 {
			want = 10
		}
		if want > MaxSearchLimit {
			want = MaxSearchLimit
		}
		if got != want {
			t.Errorf("normalizeSearchLimit(%d) = %d, want %d", limit, got, want)
		}
	}
	if _, err := normalizeSearchLimit(-1); err == nil {
		t.Fatal("negative search limit was accepted")
	}
	if got, err := validateQuery(Query{Mode: ModeKeyword, Text: "query", Limit: MaxSearchLimit + 1}, 0); err != nil || got != MaxSearchLimit {
		t.Fatalf("Search query limit = %d, err=%v; want cap %d", got, err, MaxSearchLimit)
	}
}

func TestKeywordDoesNotSearchPathButExactPathFilterWorks(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{
		ID: "path-only", Path: "notes/unique-path-phrase.txt", Title: "unrelated", Text: "ordinary body",
	}); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"unique-path-phrase", "notes/unique-path-phrase.txt"} {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: text})
		if err != nil || len(hits) != 0 {
			t.Fatalf("keyword search matched path text %q: %#v, %v", text, hits, err)
		}
	}
	hits, err := engine.Search(context.Background(), Query{
		Mode: ModeKeyword, Text: "ordinary", Filters: map[string]string{"path": "notes/unique-path-phrase.txt"},
	})
	if err != nil || len(hits) != 1 || hits[0].ID != "path-only" {
		t.Fatalf("exact path filter failed: %#v, %v", hits, err)
	}
}

func TestSearchHitPayloadUsesBoundedPreviewForLargeDocuments(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{FilterFields: []string{"kind"}})
	largeText := "preview-bound-token " + strings.Repeat("x", 256<<10)
	for i := 0; i < 32; i++ {
		if err := engine.Upsert(context.Background(), Document{
			ID: fmt.Sprintf("large-%02d", i), Text: largeText, Metadata: map[string]string{"kind": "stress"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := engine.Search(context.Background(), Query{
		Mode: ModeKeyword, Text: "preview-bound-token", Filters: map[string]string{"kind": "stress"}, Limit: 64,
	})
	if err != nil || len(hits) != 32 {
		t.Fatalf("large-document search returned %d hits: %v", len(hits), err)
	}
	for _, hit := range hits {
		if len(hit.Snippet) > maxHitPreviewBytes || !hit.SnippetTruncated || hit.ByteOffset != 0 || !strings.HasSuffix(hit.Locator, "#preview=0") {
			t.Fatalf("large-document hit has invalid preview locator: %+v", hit)
		}
		if hit.Metadata["kind"] != "stress" {
			t.Fatalf("hit metadata missing: %+v", hit.Metadata)
		}
		payload, err := json.Marshal(hit)
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) > 2048 {
			t.Fatalf("hit serialized to %d bytes despite bounded preview", len(payload))
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(payload, &fields); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"text", "context", "vector", "record"} {
			if _, ok := fields[forbidden]; ok {
				t.Fatalf("hit payload exposed %q: %s", forbidden, payload)
			}
		}
	}
}

func TestStaleFileHitRemainsLexicalAndExposesStaleState(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "stale.txt"), []byte("stale lexical evidence"))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	engine.writeMu.Lock()
	_, found, err := engine.markFileStaleLocked(stableFileID("stale.txt"), engine.currentGeneration())
	if err != nil || !found {
		engine.writeMu.Unlock()
		t.Fatalf("mark stale: found=%t err=%v", found, err)
	}
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "stale lexical"})
	engine.writeMu.Unlock()
	if err != nil || len(hits) != 1 || !hits[0].Stale {
		t.Fatalf("stale file hit did not expose stale state: %#v, %v", hits, err)
	}
	inputs, _, err := engine.MissingVectors(context.Background(), "", 10)
	if err != nil || len(inputs) != 0 {
		t.Fatalf("stale file was offered for vectorization: %#v, %v", inputs, err)
	}
}

func TestDiskIndexReopens(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(t.TempDir(), "disk-index")
	options := Options{Root: root, IndexPath: indexPath, VectorDimensions: 2}
	engine, err := Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Upsert(context.Background(), Document{ID: "persistent", Text: "persisted value"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "persisted"})
	if err != nil || len(hits) != 1 || hits[0].ID != "persistent" {
		t.Fatalf("reopened index search = %#v, %v", hits, err)
	}
}

func TestDiskAndMemoryEvidenceUseSeparateNamespaces(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "shared.txt"), []byte("shared evidence"))
	indexPath := filepath.Join(t.TempDir(), "index")
	diskOptions := Options{Root: root, IndexPath: indexPath, Storage: StorageDisk, VectorDimensions: 2}
	disk, err := Open(context.Background(), diskOptions)
	if err != nil {
		t.Fatal(err)
	}
	diskReport, err := disk.SyncFiles(context.Background())
	if err != nil {
		_ = disk.Close()
		t.Fatal(err)
	}
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	diskManifestPath := manifestPath(diskOptions, diskReport.GenerationID)
	diskManifest, err := os.ReadFile(diskManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var stableDiskReport BuildReport
	if err := json.Unmarshal(diskManifest, &stableDiskReport); err != nil {
		t.Fatal(err)
	}

	memoryOptions := diskOptions
	memoryOptions.Storage = StorageMemory
	memory, err := Open(context.Background(), memoryOptions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memory.Close() })
	memoryReport, err := memory.SyncFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if evidenceDirectory(diskOptions.IndexPath, diskOptions.Storage) == evidenceDirectory(memoryOptions.IndexPath, memoryOptions.Storage) {
		t.Fatal("disk and memory evidence share a directory")
	}
	if filepath.Base(manifestPath(memoryOptions, memoryReport.GenerationID)) != "manifest-"+memoryReport.GenerationID+".json" {
		t.Fatalf("memory manifest is not generation-specific: %q", manifestPath(memoryOptions, memoryReport.GenerationID))
	}
	afterMemorySync, err := os.ReadFile(diskManifestPath)
	if err != nil || !bytes.Equal(diskManifest, afterMemorySync) {
		t.Fatalf("memory sync modified disk manifest: %v", err)
	}
	if _, err := os.Stat(stableDiskReport.EvidencePath); err != nil {
		t.Fatalf("memory sync removed disk evidence: %v", err)
	}
	if _, err := os.Stat(memoryReport.EvidencePath); err != nil {
		t.Fatalf("memory evidence artifact missing: %v", err)
	}
}

func TestQueueOverflowSchedulesInventoryReconciliation(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "existing.txt"), []byte("existing"))
	engine := openTestEngine(t, root, Options{Workers: 1, QueueCapacity: 1})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		engine.pendingMu.Lock()
		idle := len(engine.pending) == 0
		engine.pendingMu.Unlock()
		return idle && len(engine.jobs) == 0
	})
	engine.writeMu.Lock()
	locked := true
	defer func() {
		if locked {
			engine.writeMu.Unlock()
		}
	}()
	engine.enqueuePath("blocked-one.txt")
	waitUntil(t, func() bool {
		engine.pendingMu.Lock()
		_, queued := engine.pending["blocked-one.txt"]
		engine.pendingMu.Unlock()
		return queued && len(engine.jobs) == 0
	})
	engine.enqueuePath("blocked-two.txt")
	writeTestFile(t, filepath.Join(root, "overflow.txt"), []byte("reconciled token"))
	engine.enqueuePath("overflow.txt")
	engine.writeMu.Unlock()
	locked = false
	waitUntil(t, func() bool {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: "reconciled"})
		return err == nil && len(hits) == 1
	})
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForHit(t *testing.T, engine *Engine, text, relativePath string) {
	t.Helper()
	waitUntil(t, func() bool {
		hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: text})
		return err == nil && len(hits) > 0 && hits[0].Path == relativePath
	})
}

func waitUntil(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition did not become true before timeout")
}
