# Virtual VectorFS Search

A Linux-only local search library for files and caller-supplied records. FFF provides the file catalog, fuzzy path search, grep, and watch events. Bleve provides persisted full-text, metadata, and vector search.

The Go search engine does not create embeddings or call remote services. Drivers supply document and query vectors. Runtime indexing and queries stay local.

## Build and test

Requirements: Go 1.25+, Rust/Cargo, pkg-config, a C/C++ toolchain, CMake, OpenBLAS/LAPACK, and OpenMP development libraries. On Debian/Ubuntu:

```sh
sudo apt-get install build-essential cmake pkg-config libopenblas-dev libomp-dev liblapack-dev
git clone https://github.com/ZanzyTHEbar/virtual-vectorfs.git
cd virtual-vectorfs
make rust-test
make
make test-fast
make test
make vet
```

`make` is the default vector-enabled build. It supplies Bleve's required `vectors` build tag and builds FFF `fff-search` 0.11.0 plus Bleve 2.6.0's compatible FAISS C API into `build/native`. Native sources are downloaded on the first build; runtime network access is not used. `make test-fast` runs the same vector-enabled suite without the race detector. Native output stays under ignored `build/` paths.

For another Go module, first build the native libraries from a checkout of this repository; the Go import alone cannot install them. From the repository root, install into a writable prefix:

```sh
PREFIX="$HOME/.local/vvfs-search"
make native NATIVE_PREFIX="$PREFIX"
export PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export CGO_ENABLED=1
export CGO_CFLAGS="-I$PREFIX/include"
export CGO_LDFLAGS="-L$PREFIX/lib -Wl,-rpath,$PREFIX/lib"
export LD_LIBRARY_PATH="$PREFIX/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
```

Then, from the consuming Go module:

```sh
go get github.com/ZanzyTHEbar/virtual-vectorfs/search@latest
go build -tags=vectors ./...
```

Every index requires its embedding model's `VectorDimensions` (1–4096); there is no keyword-only index mode or built-in embedding model. The repository's default `make` build enables vectors. Direct Go builds, including downstream builds, must include `-tags=vectors` as required by upstream Bleve.

## Driver API

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ZanzyTHEbar/virtual-vectorfs/search"
)

func main() {
	ctx := context.Background()
	engine, err := search.Open(ctx, search.Options{
		Root:             "/home/me/project",
		IndexPath:        "/home/me/.cache/vvfs/project.bleve",
		Storage:          search.StorageDisk,
		VectorDimensions: 384, // example only; must match the driver's embedding model
		FilterFields:     []string{"kind", "language"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer engine.Close()

	report, err := engine.SyncFiles(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("indexed %d of %d files; evidence: %s\n",
		report.Indexed, report.Scanned, report.EvidencePath)

	files, err := engine.FindFiles(ctx, search.FileQuery{Text: "watcher", Limit: 10})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("FFF path hits: %d\n", len(files.Matches))

	hits, err := engine.Search(ctx, search.Query{
		Mode:  search.ModeKeyword,
		Text:  "watch event overflow",
		Limit: 10,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Bleve keyword hits: %d\n", len(hits))
}
```

See [Quick Start](docs/QUICK_START.md) for vector provision, grep modes, records, and storage options. [Architecture](docs/ARCHITECTURE.md) documents component boundaries, update recovery, tiers, evidence, and current limits. The separate local-agent product has an [approved delivery plan](docs/LOCAL_AGENT_PLAN.md); it is not implemented in this library.

[Search Improvements](docs/SEARCH_IMPROVEMENTS.md) records the approved FFF/Bleve/FAISS direction, measurement-gated work, performance goals, and safety requirements. [Historical LibSQL Findings](docs/HISTORICAL_LIBSQL_FINDINGS.md) preserves transferable lessons and evidence limits from the retired experiment; its implementation is not the architecture of this library.

## Search lanes

- `FindFiles`: FFF fuzzy file/path search with scores and pagination.
- `Grep`: FFF plain, regex, or fuzzy content search with line/context evidence and pagination.
- `Search(ModeKeyword)`: Bleve full-text search across title, text, and context fields.
- `Search(ModeSemantic)`: Bleve approximate vector search.
- `Search(ModeHybrid)`: Bleve full-text + vector retrieval with RRF.
- `MissingVectors` / `UpsertVectors`: let a driver create offline vectors and attach them only to matching content hashes.

Mode requirements are explicit: keyword requires text; semantic requires a query vector; hybrid requires both. The engine never silently invokes a model or changes modes.

Hybrid search uses Bleve's RRF fusion in one search request by default. Set `Query.IncludeRankEvidence` only when individual keyword/vector ranks are needed; in hybrid mode, this adds two searches. Otherwise hits omit `Ranks`.

## Data and locality

`FilterFields` and vector schema are fixed per index; changing them requires rebuilding the index. `SyncFiles` reads FFF's inventory; the engine does not run a second filesystem traversal. FFF owns its watch and file catalog. Go reads eligible file content into bounded batches for Bleve. Create/modify/delete/rename events update the local index asynchronously; overflow triggers a full reconciliation.

Bleve data is persisted locally in disk mode or kept ephemeral in memory mode. Disk evidence is written to `<IndexPath>.evidence/`; memory-index evidence is isolated in `<IndexPath>.memory.evidence/`. Each contains a summary manifest and per-file JSONL with IDs, paths, SHA-256 hashes, byte sizes, vector availability, and indexing status—not source text. Memory-mode evidence retains every successful sync generation without automatic expiry; remove that sidecar directory when its audit history is no longer needed. The index and evidence directories must be outside the FFF root.

`Status.LastSyncIndexedFiles` reports the last successful full sync; watcher-driven file updates are asynchronous and do not change this historical count. `Upsert` accepts a variadic batch as one Bleve write, so callers should submit bounded chunks for caller-owned records.

FFF-managed files have no inferred `language`, `kind`, or other metadata. `FilterFields` only makes caller-supplied metadata searchable; use `FindFiles` for path/name search.

The current file policy indexes one UTF-8 text document per file, rejects binary/NUL/invalid UTF-8 content, and skips files over `MaxFileBytes`. FFF's C/Rust boundary is UTF-8; unsupported or lossy paths are reported rather than indexed under a corrupted path. FFF grep is bounded to 16 MiB files, 20 matches per file, five context lines, and 384 bytes per returned line/context; pages use a match target and may overshoot by at most the per-file cap. File chunking, document parsing, and offline embedding are not included in this initial engine.

## Product boundary

This is a search library, not an AI Agent application. It contains no LLM, embedding model, prompt/harness code, agent-memory graph, SQL database server, organization/move workflow, or FUSE mount. Offline embedding can be added later as a separate opt-in component that uses the vector task/update API.

## License and dependencies

This repository is [MIT licensed](LICENSE). It builds against [FFF (MIT)](https://github.com/dmtrKovalenko/fff/blob/main/LICENSE), [FAISS (MIT)](https://github.com/blevesearch/faiss/blob/fff814dea2bdda020363506904979b204ee201aa/LICENSE), and [Bleve (Apache-2.0)](https://github.com/blevesearch/bleve/blob/v2.6.0/LICENSE). If you distribute compiled binaries, include the applicable upstream licenses and notices; this source repository does not ship prebuilt native libraries.
