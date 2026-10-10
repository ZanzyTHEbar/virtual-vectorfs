// FFF integration tests require Linux and the locally built native bridge.
package search

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFFFFileSearchPaginationAndEvidence(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"target-a.txt", "target-b.txt", "target-c.txt"} {
		writeTestFile(t, filepath.Join(root, name), []byte("content\n"))
	}
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}

	first, err := engine.FindFiles(context.Background(), FileQuery{Text: "target", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Matches) != 1 || first.TotalMatched != 3 || first.TotalFiles != 3 || first.NextOffset != 1 {
		t.Fatalf("first FFF path page = %+v", first)
	}
	if first.Matches[0].Rank != 1 || len(first.Matches[0].MatchByteOffsets) == 0 {
		t.Fatalf("FFF path rank or match evidence missing: %+v", first.Matches[0])
	}

	second, err := engine.FindFiles(context.Background(), FileQuery{Text: "target", Offset: first.NextOffset, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Matches) != 1 || second.Matches[0].Path == first.Matches[0].Path || second.Matches[0].Rank != 2 || second.NextOffset != 2 {
		t.Fatalf("second FFF path page = %+v", second)
	}
	third, err := engine.FindFiles(context.Background(), FileQuery{Text: "target", Offset: second.NextOffset, Limit: 1})
	if err != nil || len(third.Matches) != 1 || third.NextOffset != 0 {
		t.Fatalf("final FFF path page = %+v, %v", third, err)
	}
}

func TestFFFGrepModesPaginationAndContext(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeTestFile(t, filepath.Join(root, name), []byte("before\nneedle target\nafter\n"))
	}
	engine := openTestEngine(t, root, Options{})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}

	first, err := engine.Grep(context.Background(), GrepQuery{
		Text: "needle", Mode: GrepModePlain, Limit: 1, ContextBefore: 1, ContextAfter: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Matches) != 1 || first.TotalFiles != 3 || first.FilesSearched < 1 || first.FilteredFileCount < 1 || first.NextFileOffset == 0 {
		t.Fatalf("first FFF grep page or counters = %+v", first)
	}
	match := first.Matches[0]
	if len(match.ContextBefore) != 1 || match.ContextBefore[0] != "before" || len(match.ContextAfter) != 1 || match.ContextAfter[0] != "after" {
		t.Fatalf("FFF grep context was not propagated: %+v", match)
	}
	if len(match.MatchByteOffsets) != 1 || match.MatchByteOffsets[0] != [2]int{0, 6} {
		t.Fatalf("FFF grep byte ranges = %v, want [[0 6]]", match.MatchByteOffsets)
	}

	second, err := engine.Grep(context.Background(), GrepQuery{
		Text: "needle", Mode: GrepModePlain, FileOffset: first.NextFileOffset, Limit: 1,
	})
	if err != nil || len(second.Matches) != 1 || second.Matches[0].Path == match.Path {
		t.Fatalf("second FFF grep page = %+v, %v", second, err)
	}

	regex, err := engine.Grep(context.Background(), GrepQuery{Text: `needle.*target`, Mode: GrepModeRegex, Limit: 10})
	if err != nil || len(regex.Matches) == 0 {
		t.Fatalf("FFF regex grep = %+v, %v", regex, err)
	}
	fuzzy, err := engine.Grep(context.Background(), GrepQuery{Text: "needle", Mode: GrepModeFuzzy, Limit: 10})
	if err != nil || len(fuzzy.Matches) == 0 {
		t.Fatalf("FFF fuzzy grep = %+v, %v", fuzzy, err)
	}
}

func TestFFFGrepCapsPerFileAndBoundsUTF8Output(t *testing.T) {
	root := t.TempDir()
	longLine := strings.Repeat("é", 220) + "needle" + strings.Repeat("x", 300)
	contextLine := strings.Repeat("ctx", 200)
	var content strings.Builder
	for i := 0; i < 7; i++ {
		content.WriteString(contextLine + "\n")
	}
	for i := 0; i < 30; i++ {
		content.WriteString(longLine + "\n")
	}
	for i := 0; i < 7; i++ {
		content.WriteString(contextLine + "\n")
	}
	writeTestFile(t, filepath.Join(root, "long.txt"), []byte(content.String()))
	engine := openTestEngine(t, root, Options{MaxFileBytes: int64(content.Len()) + 1})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	page, err := engine.Grep(context.Background(), GrepQuery{
		Text: "needle", Mode: GrepModePlain, Limit: MaxGrepMatchTarget,
		ContextBefore: 100, ContextAfter: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Matches) != MaxGrepMatchesPerFile || page.NextFileOffset != 0 {
		t.Fatalf("FFF per-file match/page limits = %d, next=%d", len(page.Matches), page.NextFileOffset)
	}
	for _, match := range page.Matches {
		if !match.Truncated || len(match.LineContent) > MaxGrepLineBytes || !utf8.ValidString(match.LineContent) {
			t.Fatalf("unbounded or invalid matched line: %+v", match)
		}
		if len(match.ContextBefore) > MaxGrepContextLines || len(match.ContextAfter) > MaxGrepContextLines {
			t.Fatalf("context line limit exceeded: before=%d after=%d", len(match.ContextBefore), len(match.ContextAfter))
		}
		for _, line := range append(match.ContextBefore, match.ContextAfter...) {
			if len(line) > MaxGrepLineBytes || !utf8.ValidString(line) {
				t.Fatalf("unbounded or invalid context line: %q", line)
			}
		}
		if len(match.MatchByteOffsets) != 1 {
			t.Fatalf("expected one match range, got %v", match.MatchByteOffsets)
		}
		rangeBytes := match.MatchByteOffsets[0]
		if rangeBytes[0] < 0 || rangeBytes[1] > len(match.LineContent) || match.LineContent[rangeBytes[0]:rangeBytes[1]] != "needle" {
			t.Fatalf("truncated match range is not adjusted: %v in %q", rangeBytes, match.LineContent)
		}
	}
}

func TestFindFilesAndGrepRejectInvalidRequests(t *testing.T) {
	engine := openTestEngine(t, t.TempDir(), Options{})
	ctx := context.Background()
	if _, err := engine.FindFiles(ctx, FileQuery{Text: " \t\n", Limit: MaxSearchLimit}); err == nil {
		t.Fatal("expected empty FFF path query to be rejected")
	}
	if _, err := engine.Grep(ctx, GrepQuery{Text: " \t\n", Mode: GrepModePlain, Limit: MaxSearchLimit}); err == nil {
		t.Fatal("expected empty FFF grep query to be rejected")
	}
	if _, err := engine.Grep(ctx, GrepQuery{Text: "needle", Mode: "invalid", Limit: 1}); err == nil {
		t.Fatal("expected invalid FFF grep mode to be rejected")
	}
	for _, request := range []FileQuery{
		{Text: "target", Offset: -1, Limit: 1},
		{Text: "target", Limit: -1},
	} {
		if _, err := engine.FindFiles(ctx, request); err == nil {
			t.Fatalf("expected invalid FFF file query to be rejected: %+v", request)
		}
	}
	for _, request := range []GrepQuery{
		{Text: "needle", Mode: GrepModePlain, FileOffset: -1, Limit: 1},
		{Text: "needle", Mode: GrepModePlain, Limit: -1},
		{Text: "needle", Mode: GrepModePlain, Limit: 1, ContextBefore: -1},
		{Text: "needle", Mode: GrepModePlain, Limit: 1, ContextAfter: -1},
	} {
		if _, err := engine.Grep(ctx, request); err == nil {
			t.Fatalf("expected invalid FFF grep query to be rejected: %+v", request)
		}
	}
}
