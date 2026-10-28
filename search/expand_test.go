// F2 contract tests require Linux and the locally installed native bridge and FAISS libraries.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type expansionWaitContext struct {
	context.Context
	reached chan struct{}
	once    sync.Once
}

func (c *expansionWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.reached) })
	return c.Context.Done()
}

func expandTestHit(t *testing.T, engine *Engine, query string) Hit {
	t.Helper()
	hits, err := engine.Search(context.Background(), Query{Mode: ModeKeyword, Text: query})
	if err != nil || len(hits) != 1 || hits[0].ContentHash == "" {
		t.Fatalf("search %q = %+v, %v; want one versioned hit", query, hits, err)
	}
	return hits[0]
}

func TestExpandCallerFieldsAndSourceRanges(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	doc := Document{
		ID: "caller", Source: "external", Path: "not-a-live-file.txt",
		Title: "résumé title", Text: "caller café body", Context: "context notes",
		Vector: []float32{1, 0},
	}
	if err := engine.Upsert(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "caller")
	for _, test := range []struct {
		field, source string
		offset, limit int
		want          string
	}{
		{"text", doc.Text, 7, 5, "café"},
		{"context", doc.Context, 8, 5, "notes"},
		{"title", doc.Title, 0, 8, "résumé"},
	} {
		t.Run(test.field, func(t *testing.T) {
			got, err := engine.Expand(context.Background(), ExpandRequest{
				ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: test.field,
				Offset: test.offset, Limit: test.limit,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != doc.ID || got.Source != doc.Source || got.Path != doc.Path || got.Field != test.field || got.ContentHash != hit.ContentHash || got.Origin != OriginIndexedRecord {
				t.Fatalf("excerpt provenance = %+v", got)
			}
			if got.StartByte != int64(test.offset) || got.EndByte != got.StartByte+int64(len(got.Text)) || got.TotalBytes != int64(len(test.source)) || got.Text != test.want || got.Text != test.source[got.StartByte:got.EndByte] {
				t.Fatalf("excerpt range = %+v; source = %q", got, test.source)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"vector", "record"} {
				if _, ok := fields[forbidden]; ok {
					t.Fatalf("excerpt exposed %q: %s", forbidden, encoded)
				}
			}
		})
	}
}

func TestExpandBoundedUTF8Windows(t *testing.T) {
	if MaxExpandBytes != 16<<10 {
		t.Fatalf("MaxExpandBytes = %d, want 16 KiB", MaxExpandBytes)
	}
	engine := openTestEngine(t, t.TempDir(), Options{})
	text := "aéZ" + strings.Repeat("x", MaxExpandBytes+4096)
	if err := engine.Upsert(context.Background(), Document{ID: "window", Title: "windowtoken", Text: text}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "windowtoken")
	base := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text"}
	first, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Offset: 4, Limit: 4096})
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Offset: int(first.EndByte), Limit: 4096})
	if err != nil || first.Text+second.Text != text[4:int(second.EndByte)] {
		t.Fatalf("consecutive pages = %+v then %+v, %v", first, second, err)
	}
	for _, test := range []struct {
		name          string
		offset, limit int
		want          string
	}{
		{"default limit", 4, 0, strings.Repeat("x", 4096)},
		{"maximum limit", 4, MaxExpandBytes, strings.Repeat("x", MaxExpandBytes)},
		{"complete rune", 1, 2, "é"},
		{"trim partial rune", 0, 2, "a"},
		{"end of field", len(text) - 1, 10, "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := base
			request.Offset, request.Limit = test.offset, test.limit
			got, err := engine.Expand(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if got.Text != test.want || got.StartByte != int64(test.offset) || got.EndByte != got.StartByte+int64(len(test.want)) || got.TotalBytes != int64(len(text)) || got.EndByte > int64(len(text)) {
				t.Fatalf("window = %+v; want offset %d text %q", got, test.offset, test.want)
			}
		})
	}
	for _, test := range []struct {
		name          string
		offset, limit int
	}{
		{"negative offset", -1, 1},
		{"mid-rune offset", 2, 2},
		{"no full rune fits", 1, 1},
		{"at EOF", len(text), 1},
		{"past EOF", len(text) + 1, 1},
		{"negative limit", 0, -1},
		{"over maximum", 0, MaxExpandBytes + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := base
			request.Offset, request.Limit = test.offset, test.limit
			if got, err := engine.Expand(context.Background(), request); err == nil {
				t.Fatalf("invalid window returned %+v", got)
			}
		})
	}
}

func TestExpandMatchAcrossPageBoundaryNeedsOverlap(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	text := strings.Repeat("x", 4095) + "needle"
	if err := engine.Upsert(context.Background(), Document{ID: "boundary", Title: "boundarymarker", Text: text}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "boundarymarker")
	request := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 4096}
	first, err := engine.Expand(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Offset = int(first.EndByte)
	second, err := engine.Expand(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(first.Text, "needle") || strings.Contains(second.Text, "needle") {
		t.Fatal("test requires a match split across non-overlapping pages")
	}
	request.Offset = int(first.EndByte) - len("needle") + 1 // ASCII offset is a UTF-8 boundary.
	overlap, err := engine.Expand(context.Background(), request)
	if err != nil || int(overlap.StartByte)+strings.Index(overlap.Text, "needle") != 4095 {
		t.Fatalf("overlap failed to verify the source range: %+v, %v", overlap, err)
	}
}

func TestExpandRequiresValidIdentityVersionAndField(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "original content"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "original")
	if got, err := engine.Expand(context.Background(), ExpandRequest{ID: "unknown", ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text"}); !errors.Is(err, os.ErrNotExist) || got.Text != "" {
		t.Fatalf("missing caller ID = %+v, %v; want os.ErrNotExist", got, err)
	}
	for _, test := range []struct {
		name string
		req  ExpandRequest
	}{
		{"missing ID", ExpandRequest{ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text"}},
		{"unknown ID", ExpandRequest{ID: "unknown", ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text"}},
		{"missing hash", ExpandRequest{ID: hit.ID, ExpectedSource: hit.Source, Field: "text"}},
		{"missing source", ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, Field: "text"}},
		{"missing field", ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source}},
		{"unknown field", ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "metadata"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := engine.Expand(context.Background(), test.req); err == nil {
				t.Fatalf("invalid request returned %+v", got)
			}
		})
	}
	request := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8}
	if got, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: "wrong", ExpectedSource: hit.Source, Field: "text", Limit: 8}); !errors.Is(err, ErrStaleContent) || got.Text != "" {
		t.Fatalf("mismatched caller hash = %+v, %v; want ErrStaleContent without text", got, err)
	}
	// Changing only metadata must invalidate even an unchanged requested text field.
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "original content", Metadata: map[string]string{"revision": "two"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := engine.Expand(context.Background(), request); !errors.Is(err, ErrStaleContent) || got.Text != "" {
		t.Fatalf("superseded caller hash = %+v, %v; want ErrStaleContent without text", got, err)
	}
}

func TestExpandRejectsChangedCallerSourceWithUnchangedContentHash(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Source: "original-source", Text: "unchanged source test"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "unchanged")
	if err := engine.Upsert(context.Background(), Document{ID: "record", Source: "new-source", Text: "unchanged source test"}); err != nil {
		t.Fatal(err)
	}
	updated := expandTestHit(t, engine, "unchanged")
	if updated.ContentHash != hit.ContentHash || updated.Source == hit.Source {
		t.Fatalf("test must change provenance without changing legacy content hash: before=%+v after=%+v", hit, updated)
	}
	request := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8}
	if excerpt, err := engine.Expand(context.Background(), request); !errors.Is(err, ErrStaleContent) || excerpt.Text != "" {
		t.Fatalf("changed caller source was accepted: %+v, %v", excerpt, err)
	}
}

func TestExpandReadsVerifiedLiveFileNotIndexedPreview(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("0123456789", 54) + "012345678 " + "tail café\n"
	writeTestFile(t, filepath.Join(root, "note.txt"), []byte(content))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "tail")
	if !hit.SnippetTruncated || hit.ByteOffset != 0 {
		t.Fatalf("expected preview-only hit, got %+v", hit)
	}
	got, err := engine.Expand(context.Background(), ExpandRequest{
		ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Offset: 550, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != OriginLiveFile || got.ID != hit.ID || got.Source != hit.Source || got.Path != "note.txt" || got.Field != "text" || got.ContentHash != hit.ContentHash || got.StartByte != 550 || got.EndByte != 560 || got.TotalBytes != int64(len(content)) || got.Text != content[550:560] || got.Text != "tail café" {
		t.Fatalf("live source range = %+v", got)
	}
	for _, field := range []string{"title", "context"} {
		if result, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: field, Limit: 10}); err == nil {
			t.Fatalf("file field %q returned %+v; only text is a live-file field", field, result)
		}
	}
}

func TestExpandVerifiesGrepDisplayRangeAgainstLiveFile(t *testing.T) {
	root := t.TempDir()
	content := "prefix\n" + strings.Repeat("é", 220) + " needle " + strings.Repeat("x", 300) + "\n"
	writeTestFile(t, filepath.Join(root, "long.txt"), []byte(content))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "needle")
	page, err := engine.Grep(context.Background(), GrepQuery{Text: "needle", Mode: GrepModePlain, Limit: 1})
	if err != nil || len(page.Matches) != 1 {
		t.Fatalf("grep = %+v, %v", page, err)
	}
	match := page.Matches[0]
	if len(match.MatchByteOffsets) != 1 {
		t.Fatalf("grep display ranges = %+v", match)
	}
	display := match.MatchByteOffsets[0]
	if match.LineContent[display[0]:display[1]] != "needle" {
		t.Fatalf("grep range did not locate needle in display: %+v", match)
	}
	actualStart := strings.Index(content, "needle")
	if display[0] == actualStart {
		t.Fatal("test requires distinct display-relative and file byte offsets")
	}
	excerpt, err := engine.Expand(context.Background(), ExpandRequest{
		ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Offset: int(match.ByteOffset), Limit: 512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if excerpt.Text != content[excerpt.StartByte:excerpt.EndByte] {
		t.Fatalf("expanded content does not match live file range: %+v", excerpt)
	}
	needle := strings.Index(excerpt.Text, "needle")
	if needle < 0 || int(excerpt.StartByte)+needle != actualStart {
		t.Fatalf("grep's display range was mistaken for an absolute file range: %+v, grep=%+v", excerpt, match)
	}
}

func TestExpandPagesPastTruncatedGrepLine(t *testing.T) {
	root := t.TempDir()
	content := "prefix\n" + strings.Repeat("é", 9000) + " needle\n"
	writeTestFile(t, filepath.Join(root, "far.txt"), []byte(content))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "needle")
	page, err := engine.Grep(context.Background(), GrepQuery{Text: "needle", Mode: GrepModePlain, Limit: 1})
	if err != nil || len(page.Matches) != 1 {
		t.Fatalf("grep far match = %+v, %v", page, err)
	}
	match := page.Matches[0]
	if match.ByteOffset != int64(len("prefix\n")) {
		t.Fatalf("grep source line location = %+v", match)
	}
	if len(match.MatchByteOffsets) != 0 || strings.Contains(match.LineContent, "needle") {
		t.Fatalf("far match should be absent from truncated grep display: %+v", match)
	}
	request := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Offset: int(match.ByteOffset), Limit: MaxExpandBytes}
	first, err := engine.Expand(context.Background(), request)
	if err != nil || strings.Contains(first.Text, "needle") {
		t.Fatalf("first page should not falsely claim distant match: %+v, %v", first, err)
	}
	request.Offset = int(first.EndByte)
	second, err := engine.Expand(context.Background(), request)
	if err != nil || !strings.Contains(second.Text, "needle") || second.Text != content[second.StartByte:second.EndByte] {
		t.Fatalf("next page did not verify distant match: %+v, %v", second, err)
	}
}

func TestExpandRejectsChangedStaleAndUnavailableFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, path, root string)
	}{
		{"changed content", func(t *testing.T, path, _ string) {
			writeTestFile(t, path, []byte("newer content with the same path"))
		}},
		{"missing", func(t *testing.T, path, _ string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
		{"renamed", func(t *testing.T, path, root string) {
			if err := os.Rename(path, filepath.Join(root, "renamed.txt")); err != nil {
				t.Fatal(err)
			}
		}},
		{"over file-size limit", func(t *testing.T, path, _ string) {
			writeTestFile(t, path, []byte(strings.Repeat("x", 128)))
		}},
		{"nonregular", func(t *testing.T, path, _ string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink escape", func(t *testing.T, path, _ string) {
			outside := filepath.Join(t.TempDir(), "secret.txt")
			// Matching bytes make a hash-only guard insufficient to prevent escape.
			writeTestFile(t, outside, []byte("original indexed content"))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "note.txt")
			writeTestFile(t, path, []byte("original indexed content"))
			engine := openTestEngine(t, root, Options{MaxFileBytes: 64})
			if _, err := engine.SyncFiles(context.Background()); err != nil {
				t.Fatal(err)
			}
			hit := expandTestHit(t, engine, "original")
			test.mutate(t, path, root)
			got, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 32})
			if err == nil || got.Text != "" {
				t.Fatalf("unavailable or changed live file returned %+v, %v; want no indexed-text fallback", got, err)
			}
			if test.name == "changed content" && !errors.Is(err, ErrStaleContent) {
				t.Fatalf("changed file error = %v, want ErrStaleContent", err)
			}
			if test.name == "missing" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("removed file error = %v, want os.ErrNotExist", err)
			}
		})
	}
	// An explicitly stale index record cannot be cited even if the file's bytes have not changed.
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "stale.txt"), []byte("stale indexed content"))
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "stale")
	engine.writeMu.Lock()
	_, found, err := engine.markFileStaleLocked(hit.ID, engine.currentGeneration())
	engine.writeMu.Unlock()
	if err != nil || !found {
		t.Fatalf("mark stale: found=%t err=%v", found, err)
	}
	if got, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 5}); !errors.Is(err, ErrStaleContent) || got.Text != "" {
		t.Fatalf("stale indexed file returned %+v, %v; want ErrStaleContent without text", got, err)
	}
}

func TestExpandCallerFieldsAfterDiskReopen(t *testing.T) {
	options := Options{Root: t.TempDir(), IndexPath: filepath.Join(t.TempDir(), "index"), Storage: StorageDisk, VectorDimensions: 2}
	engine, err := Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Upsert(context.Background(), Document{ID: "persisted", Source: "caller", Text: "persisted body", Title: "persisted title"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "persisted")
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	excerpt, err := engine.Expand(context.Background(), ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "title", Limit: 4096})
	if err != nil || excerpt.Text != "persisted title" || excerpt.Origin != OriginIndexedRecord {
		t.Fatalf("persisted title excerpt = %+v, %v", excerpt, err)
	}
}

func TestExpandCancelledAndClosed(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "cancelled test"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "cancelled")
	request := ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := engine.Expand(ctx, request); !errors.Is(err, context.Canceled) || got.Text != "" {
		t.Fatalf("cancelled expand = %+v, %v", got, err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := engine.Expand(context.Background(), request); err == nil || got.Text != "" {
		t.Fatalf("closed expand = %+v, %v", got, err)
	}
}

func TestExpandReadCapacityHonorsCancellation(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "bounded reads"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "bounded")
	for i := 0; i < cap(engine.expandSlots); i++ {
		engine.expandSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(engine.expandSlots); i++ {
			<-engine.expandSlots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := engine.Expand(ctx, ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expand waiting for read capacity returned %v, want context deadline", err)
	}
}

func TestExpandPendingReadDoesNotBlockClose(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "close while pending"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "pending")
	for i := 0; i < cap(engine.expandSlots); i++ {
		engine.expandSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(engine.expandSlots); i++ {
			<-engine.expandSlots
		}
	}()
	parent, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx := &expansionWaitContext{Context: parent, reached: make(chan struct{})}
	readDone := make(chan error, 1)
	go func() {
		_, err := engine.Expand(ctx, ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8})
		readDone <- err
	}()
	select {
	case <-ctx.reached: // Expand reached its full-slot select before Close starts.
	case err := <-readDone:
		t.Fatalf("Expand returned before waiting for a slot: %v", err)
	case <-parent.Done():
		t.Fatalf("timed out waiting for Expand to enter the slot select: %v", parent.Err())
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- engine.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked behind an expansion waiting for read capacity")
	}
	cancel()
	if err := <-readDone; err == nil {
		t.Fatal("pending expansion returned an excerpt after Close")
	}
}

func TestExpandDoesNotWaitOnIndexWriterLock(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	if err := engine.Upsert(context.Background(), Document{ID: "record", Text: "independent expansion"}); err != nil {
		t.Fatal(err)
	}
	hit := expandTestHit(t, engine, "independent")
	engine.writeMu.Lock()
	defer engine.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := engine.Expand(ctx, ExpandRequest{ID: hit.ID, ExpectedContentHash: hit.ContentHash, ExpectedSource: hit.Source, Field: "text", Limit: 8})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read-only expansion waited for writer lock: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("read-only expansion blocked behind writer lock")
	}
}
