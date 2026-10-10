//go:build vectors

package search

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const (
	benchmarkDocumentCount = 2048
	benchmarkVectorDims    = 128
)

func BenchmarkSearchModes(b *testing.B) {
	engine := benchmarkEngine(b, b.TempDir(), Options{VectorDimensions: benchmarkVectorDims})
	indexBenchmarkDocuments(b, engine, benchmarkDocumentCount)
	vector := benchmarkVector(0)
	queries := []struct {
		name  string
		query Query
	}{
		{name: "keyword", query: Query{Mode: ModeKeyword, Text: "common topic", Limit: 10}},
		{name: "semantic", query: Query{Mode: ModeSemantic, Vector: vector, Limit: 10}},
		{name: "hybrid", query: Query{Mode: ModeHybrid, Text: "common topic", Vector: vector, Limit: 10}},
		{name: "hybrid-rank-evidence", query: Query{Mode: ModeHybrid, Text: "common topic", Vector: vector, Limit: 10, IncludeRankEvidence: true}},
	}
	for _, test := range queries {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := engine.Search(context.Background(), test.query); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSyncFilesUnchanged(b *testing.B) {
	root := b.TempDir()
	for i := range benchmarkDocumentCount {
		path := filepath.Join(root, fmt.Sprintf("file-%04d.txt", i))
		if err := os.WriteFile(path, []byte("unchanged searchable content"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	engine := benchmarkEngine(b, root, Options{VectorDimensions: benchmarkVectorDims})
	if _, err := engine.SyncFiles(context.Background()); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		report, err := engine.SyncFiles(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		for _, path := range []string{report.EvidencePath, manifestPath(engine.options, report.GenerationID)} {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				b.Fatal(err)
			}
		}
		b.StartTimer()
	}
}

func benchmarkEngine(b *testing.B, root string, options Options) *Engine {
	b.Helper()
	options.Root = root
	options.IndexPath = filepath.Join(b.TempDir(), "index")
	options.Storage = StorageMemory
	engine, err := Open(context.Background(), options)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := engine.Close(); err != nil {
			b.Errorf("close benchmark engine: %v", err)
		}
	})
	return engine
}

func indexBenchmarkDocuments(b *testing.B, engine *Engine, count int) {
	b.Helper()
	const batchSize = 128
	for start := 0; start < count; start += batchSize {
		end := min(start+batchSize, count)
		documents := make([]Document, 0, end-start)
		for i := start; i < end; i++ {
			documents = append(documents, Document{
				ID:     fmt.Sprintf("benchmark:%d", i),
				Source: "benchmark",
				Title:  fmt.Sprintf("topic %d", i%64),
				Text:   fmt.Sprintf("common topic %d document %d", i%64, i),
				Vector: benchmarkVector(i),
			})
		}
		if err := engine.Upsert(context.Background(), documents...); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkVector(seed int) []float32 {
	vector := make([]float32, benchmarkVectorDims)
	vector[seed%benchmarkVectorDims] = 1
	vector[(seed*17+1)%benchmarkVectorDims] += 0.25
	return vector
}
