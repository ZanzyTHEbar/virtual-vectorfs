# Search improvement requirements and measurement gates

**Local integration update — 2026-10-28:** F2 is integrated locally with fresh
vector-enabled race/build/vet and native checks recorded in [GATES](../GATES.md);
it is not published remotely. The assessment baseline, dirty-file inventory and
handoff observations below are historical as of 2026-10-21, not the current Git
status. S0–S6 remain unimplemented by this integration. Related `familiar`
worktrees now exist according to coordinator metadata; this integration neither
inspected them nor qualifies their F3/F4 status. The old target-selection question
below is not a current blocker established by this document.

**Decision approved: 2026-10-21.** Keep the current FFF/Bleve/FAISS engine and
preserve unpublished F2 expansion work. Transfer concepts from the retired
LibSQL experiment, not its implementation. Implement only changes justified by
measurements, starting with unchanged-write avoidance. This document records
requirements and future gates; it does not claim implementation or performance.

This is the search library's improvement plan. [Architecture](ARCHITECTURE.md)
owns the current product boundary. [Local Agent Plan](LOCAL_AGENT_PLAN.md) and
[Local Agent Policy](LOCAL_AGENT_POLICY.md) remain the sole companion delivery
and authority contracts. Their F/R/W/P IDs are not replaced by the S IDs here.
The [historical findings](HISTORICAL_LIBSQL_FINDINGS.md) preserve everything needed
to evaluate the old ideas without retaining its branch, code, or runtime.

## 1. Baseline and non-negotiable boundaries

Assessment baseline: authoritative `/mnt/common/projects/go-pkgs/virtual-vectorfs`,
HEAD and live GitHub main `fe092dd03d398570b5d93cf0c030f573bc90257b`. That commit
does **not** include the working tree's F2 API. Record both commit and dirty-source
hashes in every experiment; never label working-tree results as pristine main.

The project's Memory Bank still describes the retired LibSQL cutover at this
handoff. It is historical context, not current architecture authority; resolve
conflicts in favor of the current source and these repository documents.

Preserve these pre-existing files unchanged during this documentation handoff:

- Modified: `docs/ARCHITECTURE.md`, `docs/LOCAL_AGENT_PLAN.md`,
  `docs/QUICK_START.md`, `search/engine.go`, `search/types.go`.
- Untracked: `.NOTE.md`, `GATES.md`, `search/expand.go`, `search/expand_test.go`.

F2 is locally validated but unpublished, not an upstream dependency a downstream
consumer can assume exists. Publishing/integrating F2 is a separate action.
Local `.NOTE.md` directions concerning faults-go, dbbootstrap and ports/adapters
are preserved; they do not justify adding a SQL database or doing unrelated
refactors as part of a search optimization.

| Current behavior | Source at assessment |
| --- | --- |
| FFF 0.11.0 owns inventory, ignores, path/grep and watch; no second Go tree walk | `native/fffbridge/src/bridge.rs:218–236,379–427` |
| One Bleve 2.6.0 index; one eligible UTF-8 document/file; no automatic chunks or PDF/Office parser | `go.mod:3–12`; `search/sync.go:164–175` |
| Hash-equal files retain vectors, but full sync still reads/reindexes them | `search/sync.go:431–473,533–544` |
| Bounded event queue, coalescing, rename/delete handling and overflow reconciliation | `search/events.go:37–118` |
| Driver-supplied vectors; hash-checked updates; no core embedding runtime/model | `search/vector_api.go:12–65,68–164` |
| Filtered FAISS ANN; Bleve RRF; `max(limit,100)` KNN candidates; rank evidence adds two queries | `search/vector_enabled.go:22–64` |
| Disk/memory Bleve; stored JSON metadata/vector state; separate JSON manifest/JSONL evidence | `search/index.go:39–60,262–293`; `search/manifest.go:32–62` |
| FFF content/mmap caches are independent budgets, default off; not an embedding cache | `native/fffbridge/src/bridge.rs:37–43`; `search/types.go:56–74` |
| Go 1.25, CGO, Rust cdylib and pinned FAISS shared libraries; Linux amd64 CI | `Makefile:1–12,31–72`; `.github/workflows/ci.yml:10–45` |

These line references describe the assessed tree; symbols/contracts take priority
if later edits move lines. Do not import old LibSQL schemas, migrations, sqlc,
Goose, Rust/C cancellation patches, Starlark/FlatBuffers SDK, or organization stack.
Do not replace FFF/Bleve/FAISS based on unrelated benchmarks. Existing RRF and
vector reuse must not be duplicated by a parallel implementation.

## 2. Ranked work and acceptance gates

All entries are **planned**, not implementation authorization from a completion
checkbox. Record the actual owner, source fingerprint, exact CHECK/EXPECT,
evidence location and result when executing an entry. Keep one focused real
acceptance scenario when practical; reuse existing checks, not mock-heavy suites.

| ID | Dependency | Deliverable and exit gate |
| --- | --- | --- |
| **S0** | — | Actual-engine baseline using section 4. Establish disk and memory behavior, event churn, full-sync writes, judged retrieval and native cancellation limits. No optimization decision from synthetic latency alone. |
| **S1** | S0 demonstrates redundant event writes | Hash-verified unchanged-event write avoidance. Meet section 3 and zero redundant text/vector writes for unchanged healthy records. If churn is immaterial, record that and defer. |
| **S2** | S0, S1 decision; full-sync cost remains material | Design generation-safe unchanged full-sync reuse, then implement only the smallest justified change. Prove unchanged records survive prune, missing files disappear, partial/canceled scans cannot falsely publish success, and stale records recover correctly. |
| **S3** | S0 native-boundary evidence | Fix measured cancellation gaps using current-engine mechanisms. Prioritize pre-effect cancellation checks after lock waits. Verify owner isolation, native unwind and safe reuse/close; explicitly document non-interruptible calls. No LibSQL backport. |
| **S4** | S0 retrieval/storage measurements | Evaluate FAISS quality modes, filters, candidate windows, rank-evidence overhead, FFF caches and stored vector duplication independently. Change one axis per experiment; preserve relevance and bound resources. Rebuild disposable indexes for schema changes. |
| **S5** | Companion R1/R3; observed profile-mixing risk | Driver-owned model/tokenizer/preprocessing provenance and bounded ingestion. Core API changes require evidence that the driver cannot enforce the contract. Model changes require explicit invalidation/reindex, even at the same dimension. |
| **S6** | Judged corpus shows whole-file/context failures | Specify token-aware chunks/extraction and source evidence before implementation. Add only formats/representations justified by failures; meet section 5. No generic strategy framework in anticipation of future needs. |

**Next task:** S0, then S1 if justified. S3 safety findings need not wait for S2.
No companion module, doctor CLI, agent loop or harness scaffolding is created by
this plan. F3 and subsequent companion tasks retain their existing gates.

## 3. Unchanged-write avoidance contract

Start with events, which do not require a new full-sync generation. Reuse is
allowed only after reading eligible content under the configured root and
matching its SHA-256 to the indexed, correctly owned, non-stale file record.
Compare every indexed field affected by the operation, not just content bytes.
Preserve the vector only when the content/representation identity is compatible.
Do not skip a stale-to-current transition because its bytes happen to match.

Required behavior:

1. Unchanged healthy event: no text analysis/document replacement/ANN mutation;
   same ID, hash, fields and vector. Reading/hash verification remains allowed.
2. Changed bytes, including same-size rewrites with restored mtime: new searchable
   content; old vector invalidated; stale vector updates rejected.
3. Rename: delete/add path-derived identity, not inode-based vector transfer.
   Delete-to-empty works; caller records sharing a path remain untouched.
4. Unreadable, oversized, binary or unsupported content: preserve current stale
   policy and invalidate vector availability; retries can recover. Do not claim
   success by hiding an error or dropping prior evidence.
5. Repeated/coalesced events, concurrent vector updates, queue overflow and
   reconciliation converge without overwriting newer state. Preserve writer
   serialization and revalidate observations within its ownership boundary.
6. Memory and disk/reopened indexes obey the same result contract. Index/evidence
   directories remain outside the root; no source-file mutation is introduced.

**Full-sync trap:** `indexFileBatch` stamps the new generation, and
`pruneStaleFiles` deletes records not stamped with it (`search/sync.go:533–544,
588–627`). Simply skipping unchanged `batch.Index` calls would delete live files.
S2 must explicitly separate scan membership from content replacement or provide
an equally safe mechanism. Specify bounded state, interruption/restart behavior,
pruning ownership and generation meaning before changing this code. Do not import
the old catalog's transactional snapshot guarantee: Bleve batches, FFF and the
manifest currently have no shared transaction.

No mtime/size-only no-read shortcut. `/mnt/common` was observed as `fuseblk`.
Any later metadata-based read avoidance requires per-entry mount/identity/change
clock validation, nested-mount handling and hash fallback for unknown/network/
FUSE filesystems. Privileged actors can falsify metadata; ordinary kernel reads
can stall. Neither limitation is solved by a content cache or Go context.

## 4. Experiments, performance goals and evidence

### Reproducible protocol

- Use only newly created scratch indexes and synthetic or explicitly approved
  copied corpora. No user databases, home/vendor/build indexing, cache deletion,
  global module patches, automatic downloads or remote writes. Keep outputs
  outside corpus roots and isolate evidence, cache and process state.
- Record commit plus dirty-source hashes, corpus/query hashes, hardware/CPU,
  filesystem/mount, toolchain, native commits/build flags, thread counts,
  dimensions/metric/quality, batch/worker/cache budgets and model profile if used.
  Measure total process-tree RSS/CPU; Go's memory limit does not bound native or
  separate model processes. Start with two Go workers/threads and a 256 MiB Go
  soft memory limit; explicitly bound native/model threads, time and scratch use.
- First use 1,000 files of 4,096 bytes, plus empty/large/UTF-8/error fixtures.
  Run initial build, unchanged scans, one-file changes, repeated no-op events,
  restored-mtime rewrites, rename, delete and delete-to-empty. Add a representative
  approved corpus before product conclusions; disclose unsupported mounts.
- Use at least five paired update/sync runs per treatment, alternating order,
  with identical starting contents and separately initialized indexes. Separate
  first index construction, process-cold reopen and warm in-process work. Do not
  call a warm OS cache a cold-storage test or evict global caches.
- For retrieval, predeclare queries and relevance judgments; use three excluded
  warmup passes and 1,000 measured requests per mode/configuration. Report raw
  samples and percentile estimator, p50/p95/p99, allocations, CPU and peak RSS.
  Repeat a claimed win independently. Change only one tuning axis at a time.

### Initial engineering targets (not measured results or public SLAs)

| Area | Goal / decision rule |
| --- | --- |
| Correctness/security | Zero lost live records, leaked cross-scope results, accepted stale vectors or false-current citations in the declared acceptance flow. Never trade these for throughput. |
| S1 no-op events | Zero redundant document/ANN writes and zero embedding recomputations after hash verification; unchanged search output and vector state. |
| S2 full sync | Unchanged text/vector payload replacements approach zero; payload work scales with changed eligible documents, not inventory size. Account for scan-membership/evidence metadata separately; no zero-read promise. |
| Optimization benefit | Predeclare at least 20% median improvement in the targeted elapsed-time metric **or** at least 50% reduction in targeted write bytes on the paired fixture. Report both; a microbenchmark win alone is insufficient if representative cost is immaterial. These are initial engineering thresholds, not historical achievements. |
| Regression budget | Investigate/reject a repeatable >5% increase in untargeted query p95 or peak process-tree RSS on identical workloads. If run-to-run variation exceeds 5%, stabilize or increase sampling rather than call it a pass. Record startup/build/storage trade-offs explicitly. |
| Retrieval tuning | Report ANN recall@10 against exact ground truth using the same vectors/metric; initial candidate target >=0.95. No loss of judged NDCG@10 for a performance-only change. If baseline misses the recall target, record the gap; do not lower it after seeing treatment. Small corpora cannot establish large-scale recall. |
| Cancellation | Canceled-before-effect requests produce zero effects; a queued cancellation never interrupts the active owner. Measure cancellation-to-unwind and post-return CPU. Do not return success early while native work continues unowned. Numeric native deadline SLO remains unset until S0 shows supported behavior. |
| Absolute SLOs | Query/update p95/p99, initial index time, index bytes/document and RAM ceilings remain **TBD on named hardware and corpus**. The old LibSQL 300 ms warm-hybrid aspiration is not this engine's SLA. |

Measure file reads/bytes, document and ANN mutations, logical/allocated/physical
write bytes where available, Bleve growth including segment merges, evidence
bytes, FFF cache use, index/reopen time and update-to-search-visible latency.
Counters must distinguish attempted writes from committed ones. Do not mistake
SQLite WAL measurements for Bleve write amplification.

For S4 compare latency/recall/memory-efficient FAISS modes on identical vectors,
multiple filter selectivities and vector-coverage levels. Measure optional rank
evidence separately. Verify filters affect candidate selection rather than merely
hiding a global top-k; authorization still belongs to the host. Inspect text,
JSON vector-state, ANN and cache bytes separately before changing persistence.
Do not change ranking windows, tie behavior or fuse FFF raw scores with Bleve
scores without judged evidence and a documented rank-fusion contract.

Existing entry points: `BenchmarkSearchModes` and `BenchmarkSyncFilesUnchanged`
in `search/performance_test.go` use 2,048 documents, 128-dimensional synthetic
vectors and **memory storage**. They are smoke/regression baselines, not disk,
model, production-relevance or scalability evidence. With native dependencies
already installed, a bounded smoke invocation is:

```sh
# Set native linking environment as documented in README first.
# Set TMPDIR/GOCACHE to an isolated, capacity-checked scratch directory.
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOMAXPROCS=2 GOMEMLIMIT=256MiB \
  timeout 120s go test -mod=readonly -p=2 -tags=vectors -run '^$' \
  -bench 'Benchmark(SearchModes|SyncFilesUnchanged)$' \
  -benchtime=1x -count=1 ./search
```

One iteration is not evidence for the performance gates above. Do not run `make`
as a supposedly read-only check: its native prerequisites can build/download.

## 5. Conditional features and safety requirements

**Provenance/embeddings (S5, companion R1/R3).** Pin model and tokenizer revision,
asset hashes, license, runtime, pooling, normalization, dimensions, input template,
token limit and preprocessing version. Bind the profile to the index/driver;
same dimensions do not mean compatible vectors. Reject incompatible profiles and
non-finite/invalid vectors. No runtime download, remote inference, implicit model
switch or silent truncation. Version/profile changes require controlled reindex.
Keep query embedding outside any future expensive snapshot/reader lifetime.

**Chunks/extraction (S6).** Define parent/source identity, stable unit IDs, raw
UTF-8 byte/page ranges, overlapping token windows, profile invalidation,
deletion/rename, document aggregation and dedup before indexing chunks. Whole
document representations must identify bounded inputs honestly. Preserve raw
source as evidence; summaries/path units are not source quotations. Required
quality evidence includes late-file matches and cross-window boundaries, not
just first-256-token examples. PDF/Office parsing remains conditional on corpus
need and verified page provenance. PageIndex/LEANN are research options, not
committed feature requirements; any new backend needs scoped candidate retrieval,
checksummed publication, source consistency, deletion, compaction and recovery.

**Citations/F2.** Preserve hash+source checks, UTF-8 windows and 16 KiB output cap.
`Hit.ByteOffset` is preview start, not match location. Grep ranges are relative
to a possibly truncated displayed line; locate and verify actual source bytes,
including matches crossing pages. Expand currently hashes the entire eligible
file; caller fields can materialize in full. Do not advertise bounded total
memory or atomic index/filesystem snapshots. Immutable saved tool results and
replay belong to companion R4/R5, not this engine's generation field.

**Scope.** A root or optional metadata filter is not an authorization policy.
The companion must enforce immutable allowed roots/sources on search, path,
grep, expand, offloaded results and replay. Never infer grants from hits or
caller-provided `Path`; caller paths must never become filesystem read authority.
Separate indexes are the initial isolation option; broader aggregation requires
explicit evidence that every lane preserves scope.

**Cancellation (S3).** Audit pre-canceled, queued, active and nearly complete work
across lock waits, Bleve queries/writes, FAISS, FFF, reads and close. Verify
callbacks/owners cannot outlive freed resources or cancel another request.
Current contexts do not prove native interruption; grep's 500 ms budget is
cooperative. Filesystem I/O need not be preemptible, and cancellation after an
already-completed commit is not rollback. If native work cannot interrupt, expose
that limit or use a deliberately designed process boundary; do not fake success
with an abandoned goroutine. Go race testing is not C/Rust sanitizer evidence.

**Packaging.** Preserve native version pins, commercial redistribution notices
and offline runtime. A Go import alone cannot install native dependencies.
Linux arm64/macOS, non-UTF-8 paths and network filesystem guarantees require
their own runtime evidence; cross-compilation alone is not validation.

## 6. Companion scope and remaining decisions

Do not resurrect the old Starlark SDK or mix agent orchestration into search.
The agreed companion is a separate Go product: deterministic router, pinned
local Gemma/EmbeddingGemma/llama.cpp probes, versioned SQLite event log,
effect-free replay, immutable offloaded result references, disposable read-only
Goja worker and parent-mediated depth-one inference. A single broker owns later
approved effects, durable intent/preimages, crash reconciliation and conditional
undo. It does not promise atomic multi-file rollback or arbitrary shell undo.

Policy defaults remain those in `LOCAL_AGENT_POLICY.md`: initially empty app-owned
writable root, explicitly selected external read roots, 0700/0600 private state,
8 MiB maximum write, separate 1 GiB undo and payload budgets without silent
expiry, 8K total context, 1,024 output tokens, four calls/task, one active
generation/embedding job, two minutes/call, ten minutes/task, depth one/fan-out
one, loopback model endpoint and runtime egress denied. No write is enabled by
this document. F1 is a policy decision, not an implemented security feature.

Still unresolved: companion repository location; supported hardware/model fit;
filesystem recovery capability; judged task/relevance and absolute SLO budgets;
actual native cancellation; whether chunks or extraction are necessary; and
the first-party identity of “Muse.” F3 onward remain planned. The F-integration
sentence in the companion plan is a target gate, not evidence that planned F3/F4
are complete. Do not start those tasks as incidental optimization work.

## 7. Verification status at handoff

The preceding read-only assessment passed `git diff --check`, formatting checks
on F2 Go files, and focused existing vector-enabled F2/search/update integration
tests (2.431 s) with installed native libraries and isolated scratch/cache.
This is not fresh full-suite/race/Rust/performance evidence. `GATES.md` records
earlier F2 evidence. S0–S6 have not been completed. No benchmark numbers in the
historical record establish current-engine performance.
