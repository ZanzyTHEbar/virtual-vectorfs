# Quick Start

## 1. Build native dependencies

The first build needs Linux, Go 1.25+, Rust stable, Cargo, pkg-config, a C/C++ toolchain, CMake, OpenBLAS/LAPACK, and OpenMP development libraries. On Debian/Ubuntu:

```sh
sudo apt-get install build-essential cmake pkg-config libopenblas-dev libomp-dev liblapack-dev
git clone https://github.com/ZanzyTHEbar/virtual-vectorfs.git
cd virtual-vectorfs
make rust-test
make
make test
```

`make native` builds the pinned FFF bridge and FAISS C API. Build output stays under ignored `build/` directories. The engine makes no network requests at runtime.

## 2. Open and synchronize a file corpus

```go
ctx := context.Background()

engine, err := search.Open(ctx, search.Options{
    Root:         "/home/me/project",
    IndexPath:    "/home/me/.cache/vvfs/project.bleve",
    Storage:      search.StorageDisk,
    VectorDimensions: 384, // example only; must match the embedding model
    FilterFields: []string{"language", "kind"},
})
if err != nil {
    return err
}
defer engine.Close()

report, err := engine.SyncFiles(ctx)
if err != nil {
    return err
}
fmt.Printf("indexed=%d skipped=%d stale=%d errors=%d\n",
    report.Indexed, report.Skipped, report.Stale, report.Errors)
fmt.Printf("per-file evidence: %s\n", report.EvidencePath)
```

`SyncFiles` waits for FFF's initial catalog/watch readiness, reads FFF inventory pages, hashes eligible file contents, and writes bounded Bleve batches. FFF events continue to update the index after the initial sync. `Status()` reports readiness and background errors.

## 3. Search file names, content, and metadata

```go
files, err := engine.FindFiles(ctx, search.FileQuery{
    Text: "watcher",
    Offset: 0,
    Limit: 20,
})
if err != nil {
    return err
}

matches, err := engine.Grep(ctx, search.GrepQuery{
    Text: "overflow",
    Mode: search.GrepModePlain,
    Limit: 20,
    ContextBefore: 1,
    ContextAfter: 1,
})
if err != nil {
    return err
}

hits, err := engine.Search(ctx, search.Query{
    Mode: search.ModeKeyword,
    Text: "recover missed file events",
    Limit: 10,
})
```

`FindFiles` and `Grep` are FFF lanes. Grep accepts a match target up to 80, at most 20 matches per file, a five-line context cap on each side, a 384-byte output cap per line/context, a 16 MiB file cap, and a 500 ms search time budget. Since FFF completes the current file before paging, a page can return up to 99 matches. Use `NextFileOffset` for continuation. Keyword, semantic, and hybrid queries use Bleve. FFF path/grep scores are not mixed with Bleve scores; if a future unified search combines lanes, it should fuse ranks rather than raw score scales.

Hybrid search uses RRF in one Bleve request by default. Set `IncludeRankEvidence: true` only when per-channel ranks are needed; in hybrid mode it adds two extra Bleve searches.

## 4. Supply vectors from the driver

The engine does not contain an embedding model. `VectorDimensions` is required when opening every index and must match the driver's model; Bleve supports up to 4096 dimensions. The default repository build enables vectors. Direct Go builds must include `-tags=vectors` and supply the query vector to semantic/hybrid search.

`MissingVectors` returns bounded pages of current, non-stale documents with exact indexed text and its content hash. The caller can embed each item locally and attach the result. Stale documents caused by unreadable or skipped files are reported in evidence but are not actionable embedding inputs:

```go
cursor := ""
for {
    items, next, err := engine.MissingVectors(ctx, cursor, 32)
    if err != nil {
        return err
    }
    updates := make([]search.VectorUpdate, 0, len(items))
    for _, item := range items {
        vector, err := callerEmbed(ctx, item.Text) // caller-owned model/provider
        if err != nil {
            return err
        }
        updates = append(updates, search.VectorUpdate{
            ID:                  item.ID,
            ExpectedContentHash: item.ContentHash,
            Vector:              vector,
        })
    }
    if err := engine.UpsertVectors(ctx, updates...); err != nil {
        return err
    }
    if next == "" {
        break
    }
    cursor = next
}
```

The expected hash prevents a late vector result from being attached to content that changed while embedding was in progress. A changed file loses its previous vector and becomes visible in `MissingVectors` again.

```go
hits, err := engine.Search(ctx, search.Query{
    Mode: search.ModeHybrid,
    Text: "where is the watcher queue drained?",
    Vector: queryVector, // caller supplied, same dimensions as the index
    Limit: 10,
})
```

## 5. Index caller-owned records

Caller records share the same Bleve corpus. Configure exact-match metadata fields at `Open`; `id`, `source`, and `path` are reserved for top-level filters:

```go
err := engine.Upsert(ctx, search.Document{
    ID:      "manual:watcher-design",
    Source:  "manual",
    Title:   "Watcher recovery design",
    Context: "filesystem indexing / reliability",
    Text:    "On overflow, perform a full reconciliation...",
    Metadata: map[string]string{
        "kind": "design-note",
    },
    Vector: embedding, // optional, supplied by caller
})
```

Caller IDs beginning with `file:` are reserved for FFF-managed files. Use `Delete` to remove caller-owned records.

FFF-managed files do not automatically have `language` or `kind` metadata. To search the caller-owned record above by metadata, use `Filters: map[string]string{"kind": "design-note"}` with a keyword query for `"overflow"`.

## 6. Choose tiers

`StorageDisk` (default) persists the Bleve index locally; `StorageMemory` keeps search state ephemeral. Evidence remains on disk in separate disk/memory evidence directories. FFF mmap/content cache budgets are configured separately.

`VectorQuality` selects Bleve's FAISS-backed `latency`, `recall`, or `memory_efficient` index optimization when the index is created. This changes the physical index and requires a rebuild; it is not a per-query exact/approximate switch. No custom exact vector-scan mode is implemented yet.

## 7. Build in another Go driver

The Go import does not install native libraries. From a checkout of this repository, run these commands at its root to install them into a writable prefix and expose them to cgo/pkg-config:

```sh
PREFIX="$HOME/.local/vvfs-search"
make native NATIVE_PREFIX="$PREFIX"
export PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export CGO_ENABLED=1
export CGO_CFLAGS="-I$PREFIX/include"
export CGO_LDFLAGS="-L$PREFIX/lib -Wl,-rpath,$PREFIX/lib"
export LD_LIBRARY_PATH="$PREFIX/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
```

From the consuming Go module (with the exported environment still set):

```sh
go get github.com/ZanzyTHEbar/virtual-vectorfs/search@latest
go build -tags=vectors ./...
```

The Rust bridge is exposed through a small C ABI. FFF paths currently cross that boundary as UTF-8; lossy or unsupported paths are reported and skipped. Support for arbitrary non-UTF-8 Linux path bytes is not yet implemented.
