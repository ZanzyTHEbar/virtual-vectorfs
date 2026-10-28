//go:build scoped_native_acceptance

package search_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/search"
)

// One manually selected real Go/cgo/native journey; fixtures and trace checking
// live outside the corpus. No fixtures are inferred from personal directories.
func TestNativeReadPolicyAcceptance(t *testing.T) {
	root, state, mode := os.Getenv("SCOPED_ACCEPTANCE_ROOT"), os.Getenv("SCOPED_ACCEPTANCE_STATE"), os.Getenv("SCOPED_ACCEPTANCE_MODE")
	if root == "" || state == "" || mode == "" {
		t.Fatal("explicit synthetic root/state/mode required")
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGURG); err != nil {
		t.Fatal(err)
	}
	policy := search.NativeReadRootOnly
	switch mode {
	case "strict", "strict-incomplete", "old-reject", "disabled-reject", "mixed-reject":
	case "ambient", "ambient-old":
		policy = "" // Exercise default callers, not an explicit Ambient override.
	default:
		t.Fatalf("unknown acceptance mode %q", mode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	engine, err := search.Open(ctx, search.Options{
		Root: root, IndexPath: filepath.Join(state, "index"), Storage: search.StorageMemory,
		VectorDimensions: 2, Workers: 1, NativeReadPolicy: policy,
		FFFContentCache: true, FFFMmapCache: true, FFFCacheMaxFiles: 16,
	})
	if mode == "mixed-reject" {
		maps, mapErr := os.ReadFile("/proc/self/maps")
		if mapErr != nil {
			t.Fatal(mapErr)
		}
		for _, key := range []string{"SCOPED_MIXED_PROVIDER_A", "SCOPED_MIXED_PROVIDER_B"} {
			provider := os.Getenv(key)
			if provider == "" || !strings.Contains(string(maps), provider) {
				t.Fatalf("real mixed-provider control did not map %s=%q", key, provider)
			}
		}
	}
	if mode == "old-reject" || mode == "disabled-reject" || mode == "mixed-reject" {
		if engine != nil {
			_ = engine.Close()
			t.Fatal("strict unexpectedly returned a live engine")
		}
		expected := "bound native provider lacks the scoped-read v2 ABI"
		if mode == "disabled-reject" {
			expected = "requires the local scoped-reads-prototype native build"
		}
		if err == nil || !strings.Contains(err.Error(), expected) {
			t.Fatalf("fail-closed error = %v; want %q", err, expected)
		}
		t.Logf("PASS %s: %v", mode, err)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	initial, err := engine.SyncFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "strict-incomplete" {
		subtree := filepath.Join(root, "kept")
		leaf := filepath.Join(subtree, "saved.txt")
		before, err := os.Stat(leaf)
		if err != nil {
			t.Fatal(err)
		}
		directory, err := os.Stat(subtree)
		if err != nil {
			t.Fatal(err)
		}
		find := func() search.Hit {
			t.Helper()
			hits, err := engine.Search(ctx, search.Query{Mode: search.ModeKeyword, Text: "retainedneedle"})
			if err != nil || len(hits) != 1 || hits[0].Path != "kept/saved.txt" {
				t.Fatalf("previously indexed document lost: %+v, %v", hits, err)
			}
			return hits[0]
		}
		hit := find()
		if err := engine.UpsertVectors(ctx, search.VectorUpdate{ID: hit.ID, ExpectedContentHash: hit.ContentHash, Vector: []float32{1, 0}}); err != nil {
			t.Fatal(err)
		}
		preserved := func() {
			t.Helper()
			current := find()
			if current.ID != hit.ID || current.ContentHash != hit.ContentHash {
				t.Fatalf("prior document identity changed: %+v", current)
			}
			hits, err := engine.Search(ctx, search.Query{Mode: search.ModeSemantic, Vector: []float32{1, 0}})
			if err != nil || len(hits) != 1 || hits[0].ID != hit.ID || !hits[0].HasVector {
				t.Fatalf("prior vector lost: %+v, %v", hits, err)
			}
			paths, err := engine.FindFiles(ctx, search.FileQuery{Text: "saved.txt"})
			if err != nil || len(paths.Matches) != 1 || paths.Matches[0].Path != "kept/saved.txt" {
				t.Fatalf("native inventory was replaced by partial scan: %+v, %v", paths, err)
			}
		}
		preserved()
		if err := os.Chmod(subtree, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(subtree, 0o700)
		if _, err := os.ReadDir(subtree); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("real unreadable subtree required (uid=%d): %v", os.Geteuid(), err)
		}
		failed, err := engine.SyncFiles(ctx)
		if err == nil || !strings.Contains(err.Error(), "requested native FFF rescan failed") || failed.Scanned != 0 {
			documents, docErr := engine.Search(ctx, search.Query{Mode: search.ModeKeyword, Text: "retainedneedle"})
			vectors, vectorErr := engine.Search(ctx, search.Query{Mode: search.ModeSemantic, Vector: []float32{1, 0}})
			t.Logf("failure witness: documents=%+v (%v), vectors=%+v (%v)", documents, docErr, vectors, vectorErr)
			t.Fatalf("incomplete native ticket must fail before reconciliation: %+v, %v", failed, err)
		}
		preserved()
		if err := os.Chmod(subtree, 0o700); err != nil {
			t.Fatal(err)
		}
		after, err := os.Stat(leaf)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("source leaf identity changed: %v", err)
		}
		afterDir, err := os.Stat(subtree)
		if err != nil || !os.SameFile(directory, afterDir) {
			t.Fatalf("source ancestry identity changed: %v", err)
		}
		if _, err := engine.SyncFiles(ctx); err != nil {
			t.Fatalf("restored retry: %v", err)
		}
		preserved()
		if err := engine.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log("PASS strict-incomplete: real native ticket failed; inventory, document and vector preserved; same-inode restoration/retry recovered")
		return
	}
	paths, err := engine.FindFiles(ctx, search.FileQuery{Text: "ignored", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool)
	for _, item := range paths.Matches {
		present[item.Path] = true
	}
	for _, name := range []string{"ambient-ignored.txt", "ancestor-ignored.txt", "local-ignored.txt", "git-local-ignored.txt", "ancestor-git-ignored.txt"} {
		if present[name] != (mode == "strict") {
			t.Fatalf("mode=%s native inventory %q present=%t", mode, name, present[name])
		}
	}
	if mode == "strict" {
		hidden, err := engine.FindFiles(ctx, search.FileQuery{Text: ".hidden.txt"})
		if err != nil || len(hidden.Matches) != 1 {
			t.Fatalf("strict hidden inventory = %+v, %v", hidden, err)
		}
		links, err := engine.FindFiles(ctx, search.FileQuery{Text: "escape"})
		if err != nil || len(links.Matches) != 0 {
			t.Fatalf("strict symlink inventory = %+v, %v", links, err)
		}
	}
	grep, err := engine.Grep(ctx, search.GrepQuery{Text: "briefneedle", Mode: search.GrepModePlain})
	if err != nil || len(grep.Matches) != 1 || grep.Matches[0].Path != "brief.txt" {
		t.Fatalf("real native grep = %+v, %v", grep, err)
	}
	hits, err := engine.Search(ctx, search.Query{Mode: search.ModeKeyword, Text: "briefneedle"})
	if err != nil || len(hits) != 1 {
		t.Fatalf("real indexed search = %+v, %v", hits, err)
	}
	hit := hits[0]
	excerpt, err := engine.Expand(ctx, search.ExpandRequest{ID: hit.ID, ExpectedSource: hit.Source, ExpectedContentHash: hit.ContentHash, Field: "text", Limit: 256})
	if err != nil || excerpt.Origin != search.OriginLiveFile || excerpt.Text != "briefneedle granted content\n" {
		t.Fatalf("verified live expansion = %+v, %v", excerpt, err)
	}

	// Poll actual indexed user-visible state; no SyncFiles call masks watcher delivery.
	waitFor := func(text string, count int) {
		t.Helper()
		for {
			results, err := engine.Search(ctx, search.Query{Mode: search.ModeKeyword, Text: text})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) == count {
				return
			}
			if ctx.Err() != nil {
				t.Fatalf("watch query %q: %v", text, ctx.Err())
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	watched := filepath.Join(root, "watched.txt")
	if err := os.WriteFile(watched, []byte("watchcreated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor("watchcreated", 1)
	if err := os.WriteFile(watched, []byte("watchupdated new bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor("watchupdated", 1)
	nested := filepath.Join(root, "new", "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if mode == "strict" {
		if err := os.Symlink(filepath.Join(filepath.Dir(root), "outside"), filepath.Join(root, "dynamic-escape")); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(filepath.Dir(os.Getenv("HOME")), "linked-ignore")
		if err := os.Symlink(target, filepath.Join(nested, ".ignore")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "late.txt"), []byte("newdirneedle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor("newdirneedle", 1)
	rescanned, err := engine.SyncFiles(ctx) // RootOnly waits for its native v2 completion ticket.
	// The Go generation assertion below checks reconciliation only. Native
	// request/successor interleaving is checked by the separate real FFF journey.
	if err != nil || rescanned.GenerationID == initial.GenerationID {
		t.Fatalf("rescan/reconciliation = %+v, %v", rescanned, err)
	}
	if err := os.Remove(watched); err != nil {
		t.Fatal(err)
	}
	waitFor("watchupdated", 0)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	if !engine.Status().Closed {
		t.Fatal("engine did not close")
	}
	t.Logf("PASS %s: native inventory/grep, indexed search/expand, watcher, sync/rescan, close", mode)
}
