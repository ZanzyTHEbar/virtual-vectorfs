# Retired LibSQL experiment: transferable findings

**Historical scope note — 2026-10-28:** The earlier preservation/retirement
boundary below records the first handoff, not current retained resources.
[Permanent LibSQL Retirement](LIBSQL_RETIREMENT.md) records the later authorized
deletion and corrects the final arm64 cross-build statement. F2 is now integrated
locally, not published remotely. Historical source/evidence pointers remain
provenance only; no deleted resource was recreated for this integration.

**Retirement decision: 2026-10-21.** This is a self-contained technical handoff,
not a retained implementation, reproducible build recipe, or competing roadmap.
[Search Improvements](SEARCH_IMPROVEMENTS.md) owns current requirements and gates.
No old branch/runtime is required to implement those requirements. Old source
paths and hashes below are provenance only, not links or supported dependencies.
Raw external evidence locations are historical pointers; their availability has
not been revalidated and they are not needed for new experiments.

## Provenance and retirement boundary

- Authoritative GitHub-aligned engine: `fe092dd03d398570b5d93cf0c030f573bc90257b`,
  `/mnt/common/projects/go-pkgs/virtual-vectorfs`; unpublished F2 remains separate.
- Old tools canonical: `/mnt/common/projects/tools/virtual-vectorfs`, HEAD
  `4902ee6247bbc00c8bef4fb1c8ab76b6bdc8d9c3`. Its extensive uncommitted LibSQL
  cutover is preserved, **not integrated and not removed** by branch retirement.
- Retired comparison worktree:
  `/mnt/common/projects/tools/virtual-vectorfs__worktrees/optimize-incremental-local-vectors`;
  local branch `optimize-incremental-local-vectors`, final HEAD
  `c2793c8867c86a62bcd8f9c40f83a21d3c985023`.
- Retirement scope: remove that worktree and local branch after this handoff is
  verified. No tag, bundle, backup branch or source transplant is created. No
  remote ref deletion, object pruning, canonical tools cleanup, external model,
  corpus, database or evidence deletion is authorized by this scope.
- Pre-removal inspection found no tracked modifications or non-ignored untracked
  source in the comparison worktree. Ignored contents were a Workmux task prompt,
  Linux amd64/arm64 native archives/build metadata, generated native-build Go
  fingerprints and Python bytecode. They are task/build artifacts, not needed
  runtime assets for the chosen engine.
- Removal status: **completed 2026-10-21**, after documenting and checking this
  handoff. `git worktree remove` succeeded without force; the named local branch
  was then explicitly deleted. No replacement ref/archive was created. Git may
  retain unreachable objects until its normal housekeeping; no purge is claimed.

Historical implementation milestones:

| Commit | Change |
| --- | --- |
| `75079cef557d1494a906ac59ac7b1a9b272005bf` | Incremental document/chunk reuse |
| `496e4c6d78307198b9f04b0650e54cdec46840fc` | Reuse safety/portability fixes |
| `a0ec5ba671a54383bb71a512e7f4b6d7eefdcbde` | Local MiniLM/native ANN |
| `bb9a96b08d21668da98447d6d988524539acb55f` | Benchmark scope correction |
| `76eceb59a1f2710904b5598f82710ad91c02a0b0` | Lexical/semantic hybrid |
| `19b5e3705b5929323329d43c4bba3fb181b0fb30` | Catalog API/snapshot verification |
| `9e43b591b047d7adbaf23d98b94748a0df9f65eb` | Native SQL/ANN cancellation |
| `c2793c8867c86a62bcd8f9c40f83a21d3c985023` | Cancellation before native VM entry |

## Transfer decisions

| Idea | Classification | Decision |
| --- | --- | --- |
| Hash-checked unchanged vector reuse/invalidation | Already present | Preserve current API and file behavior. |
| Avoid unchanged document/chunk writes | Applicable concept; reimplement | Start with measured no-op events. Full sync needs Bleve-specific pruning design; no current chunks exist. |
| Pinned offline CPU embeddings/profile | Applicable concept; reimplement | Driver/companion concern; MiniLM is historical evidence, not a selected replacement for the companion's model gates. |
| Silent 256-token truncation | Incompatible; must not port | Explicit refusal or separately identified windowing is required. |
| LibSQL migrations, sqlc/Goose, partial ANN DDL/compression | Incompatible; must not port | Current storage is Bleve/FAISS. No SQL catalog or backend swap. |
| Hybrid RRF | Already present | Keep Bleve's implementation; do not transplant old formula/windows. |
| Coherent citations and pre-candidate workspace isolation | Applicable concept; reimplement | Define current-engine scope/version contracts, not SQLite transaction emulation. |
| LibSQL driver/Rust/C cancellation patch | Incompatible; must not port | Transfer boundary/ownership checks only. |
| LEANN/PageIndex | Unknown benefit | Defer until actual-engine workload evidence justifies research. |

## Incremental reuse: algorithm and pitfalls worth preserving

Old authority was one LibSQL catalog with staged scans, source-owner leases and
generation compare-and-swap. Existing document IDs survived same-path updates.
Deleted paths were swept; rename was delete/add. Changed rows, chunk updates,
deletions and generation publication shared one database transaction.

Reuse keys included reader/extractor version, chunker version, model/dimension,
ignore policy, native backend profile and file device/inode/mode/size/mtime/ctime.
Empty or ambiguous identity and extraction errors forced extraction/retry.
Linux identity extraction was isolated; other systems fell back to reads.
The old fingerprint also conservatively disabled reuse when ctime nanoseconds
were zero. This incidental implementation detail is not a new engine requirement.

Mount trust was checked **per entry**, including verification, not inherited
from the source root across nested mounts. Trusted local ext/XFS/Btrfs/ZFS/tmpfs
could avoid reads with identity/change clocks; unknown/network/FUSE paths were
hashed before reuse and reverified before promotion. Privileged metadata
falsification remained outside the guarantee. A nested-mount namespace test was
blocked by unavailable user-namespace mapping; it was not claimed as passed.

Reused documents staged membership without replacing active content. Changed
documents upserted by `(source_id, relative_path)`; chunks keyed by document,
kind and ordinal changed only when text hash, provenance span or embedding
changed. Unchanged chunk embeddings survived metadata-only changes. Old app DML
was sqlc-generated, with dynamic versioned ANN DDL confined to migrations.
These SQL mechanics are **not** the solution for Bleve's generation/prune coupling.

Old source evidence: `internal/app/catalog/catalog.go:625–744`,
`internal/app/catalog/incremental.go:17–57`,
`internal/app/catalog/identity_linux.go:12–34`, and
`internal/adapter/libsql/queries/incremental.sql:1–77` at final HEAD above.

Recorded guarded experiment: 1,000 files, 4,096 bytes/file, 2,000 chunks;
isolated process/cache/database/corpus, Go 1.26.8/Linux amd64/CGO,
`GOMAXPROCS=2`, `GOMEMLIMIT=256MiB`; three unchanged and three one-file scans.
Final follow-up medians: unchanged **3,148 -> 800 ms**, one-file **3,112 -> 838 ms**,
unchanged WAL **7,584,952 -> 799,312 bytes**. Treatment binary SHA-256:
`35ec0706ab28afc660abdc106acba9302ccdd14de0aaf4140d58fccf41801b91`.
Initial builds were not improved. FUSE open tracing counted **1,998 regular
corpus opens for both versions** after deletion: avoided work was row/FTS/WAL
churn, not reads. Small samples and SQLite WAL cannot predict Bleve performance.
Evidence pointers: `/mnt/common/vvfs-reviewfix-evidence-20261021`,
`/mnt/common/vvfs-review-75079cef-sray2jux-evidence`, and
`/mnt/common/vvfs-incremental-evidence-20261021`.

## Embedding identity and runtime observations

Historical profile, preserved for interpretation rather than installation:

```json
{
  "model": "sentence-transformers/all-MiniLM-L6-v2",
  "revision": "1110a243fdf4706b3f48f1d95db1a4f5529b4d41",
  "profile": "all-MiniLM-L6-v2:1110a243:onnx-f32:mean-l2:256:v1",
  "dimension": 384,
  "format": "ONNX float32",
  "license": "Apache-2.0",
  "runtime": {"onnxruntime": "1.23.2", "tokenizers": "0.22.1", "numpy": "2.3.4"},
  "tokenizer_revision": "1110a243fdf4706b3f48f1d95db1a4f5529b4d41",
  "files": {
    "onnx/model.onnx": {"sha256": "6fd5d72fe4589f189f8ebc006442dbb529bb7ce38f8082112682524616046452", "bytes": 90405214},
    "tokenizer.json": {"sha256": "be50c3628f2bf5bb5e3a7f17b1f74611b2561a3a27eeab05e5aa30f411572037", "bytes": 466247},
    "1_Pooling/config.json": {"sha256": "4be450dde3b0273bb9787637cfbd28fe04a7ba6ab9d36ac48e92b11e350ffc23", "bytes": 190},
    "sentence_bert_config.json": {"sha256": "fc1993fde0a95c24ec6c022539d41cf6e2f7c9721e5415d6fb6897472a9cd4b7", "bytes": 53}
  }
}
```

Python 3.12 worker used CPUExecutionProvider, two intra-op/one inter-op threads,
attention-mask mean pooling, L2 normalization, no task prefix and batches <=32.
Assets/runtime were checked; Go checked profile/dimensions/finite values/norms.
Assets were external, not packaged into the binary, and totaled 90,871,704 bytes.
The worker stayed warm across batches, not CLI invocations. Go memory limits did
not bound Python/ONNX arenas. Query-only RSS was roughly 175 MiB, peak 199 MiB;
indexing/recomputation could require about 1 GiB. Treat these as old observations,
not budgets for the chosen models/hardware.

Critical deficiency: `backend.py:45` enabled silent truncation at 256 tokens.
The extractor emitted a path chunk and one bounded content chunk, **not** automatic
token windows or separate document vectors. Model assets, strategy manifests and
model migration remained incomplete. Do not represent this as full-document
semantic coverage. Old source: `internal/adapter/embed/model.lock.json`,
`backend.py:33–65`, `internal/app/catalog/catalog.go:703–711`.

## Native ANN versus experimental LEANN

Matched experiment: 146 approved production-Go source slices <=1,200 characters,
292 actual normalized MiniLM path/content vectors, ten queries (five natural
language/five source snippets). Native workspace-partial DiskANN used 16
neighbors, float8 neighbor vectors and `search_l=200`; authoritative vectors
remained float32. LEANN 0.3.8 HNSW used M=16, efConstruction=200, pruned embeddings
and compact CSR artifacts; real ZMQ callbacks recomputed candidates through ONNX.

| Recorded metric | Compressed LibSQL native | LEANN + fresh inference |
| --- | --- | --- |
| Derived build, vectors already available | 732 ms | 182 ms |
| Derived index bytes | 2,408,448 | 23,059 |
| Retrieval excluding query embedding | 1.34–2.66 ms | 4,003–6,435 ms |
| Retrieval median | 1.55 ms | 4,781 ms |

Both matched exact chunk-ID recall@10=1.0 on those ten queries. LEANN search
complexity was oracle-tuned (16 for most, 64 for two; attempted 16/64/256/1024),
excluding failed tuning attempts from final latency. Only one sample/query;
exact search was itself about 1 ms. This is not document relevance or scale proof.
Shared query embedding was 2.53–11.64 ms. Native total DB was 5,083,136 bytes;
LEANN graph bytes exclude source/catalog/model. Process memory observations were
not backend-only peak comparisons. The derived-index ratio is not total storage
savings, nor any evidence that FAISS should be replaced.

LEANN integration remained immutable/experimental. Missing production work:
workspace/profile/generation manifests, checksums, source/candidate consistency,
atomic publication, base/delta/tombstones, compaction and crash recovery.
The old decision was to keep LibSQL for its mutable workspaces; the **current**
decision is to keep FAISS. PageIndex hierarchy was an optional concept, not a
validated current-engine feature. Uber's 11-byte logical entry comprises a
6-byte sharded hash, 2-byte RAM slot and 3-byte aligned disk offset—not total
storage. No new LEANN/PageIndex/Uber performance claim is made here.

Evidence pointer: `/mnt/common/vvfs-local-minilm/evaluation-reviewed/results.json`;
historical binary `ebd140b4218239f53bd6e56ae2b3b6bb92c660cb2456393a1b7541c2ea66b668`.

## Hybrid, API, scope and citation lessons

Old lexical FTS terms were quoted and AND-combined. Semantic used native ANN;
explicit semantic/hybrid failed without capability rather than falling back.
Hybrid requested `min(1000,max(100,limit*10))` document candidates/route; ANN
expanded based on actual unit counts, then collapsed to best document ranks.
RRF was `1/(60+lexical_rank) + 1/(60+semantic_rank)` for present routes.
Deterministic ties used best rank, path and document ID. Same-hash different
paths remained distinct. Winning snippet text/unit/span/route and raw score
stayed together; path units were not presented as content quotes.

Query embedding preceded the read transaction. Scope, capability checks,
lexical/ANN results and batched citations shared one transaction/generation.
The API used an immutable nonempty host allowlist (<=64 workspaces); each
workspace ANN selected candidates locally. Multiple workspace candidates were
globally ranked before page truncation, not fused separately and concatenated.
Inspect/trace used indexed IDs plus explicit scope, never arbitrary paths.
Separate calls could see different generations; no script-long snapshot existed.
Starlark/FlatBuffers was the old boundary, not the agreed Goja companion design.

Acceptance used real synthetic fixtures and an actual reader barrier: generation
7 remained coherent while a writer committed 8; a new reader saw 8; an aborted
promotion retained prior data with stale warnings. A 24-window fixture was seeded
directly and did **not** prove automatic chunk extraction. Scope, distinct paths,
rename/delete, missing capabilities and selected snippet provenance were checked.

Recorded hybrid latency: 145 documents/290 vectors, four unjudged queries,
80 pooled warm samples/mode after three excluded warmups. Warm p50/p95:
lexical **0.69/1.18 ms**, semantic **5.72/7.19 ms**, hybrid **9.33/11.86 ms**.
Semantic/hybrid included query embedding. New-process CLI p50/p95 on one different
query: **24.94/28.66**, **691.39/746.17**, **709.54/765.24 ms**, respectively.
These were process-cold with warm OS/model caches, not cold storage; differing
workloads prevent subtracting them to derive startup cost. No judged relevance
set or current-engine SLA follows. Four tiny semantic-answer checks are not
general quality evidence.

Old source: `internal/app/catalog/search.go:30–138,163–235`.
Evidence pointers: `/mnt/common/vvfs-wave2-hybrid-20261021/{acceptance-final,benchmark}`
and `/mnt/common/vvfs-wave3-api-20261021/acceptance-committed`.

## Cancellation: methodology, not a portable patch

Old driver revision `60e59c7150f4`, LibSQL 0.9.17 runtime commit
`cda864ed885764b704b418b94676df26f644da70`, Rust 1.85.0; source/patch/header hashes
and generated native ABI were locked. The patch used a Rust-owned connection
token, never an opaque-handle-to-SQLite cast. It polled at VM entry/progress and
DiskANN traversal/mutation/output, propagated native error 9 through prefetched
and lazy rows, and handled interrupted-write rollback/autocommit state.

Important discovered bug: periodic polling alone missed short writes canceled
before VM entry because idle interrupt state reset. The final fix checked the
persistent token **after reset/activation but before the first opcode**.
Transfer the requirement to check cancellation at the real effect boundary,
not SQLite opcodes. Near-complete committed work cannot always be canceled.

Ownership lessons: operation lease through lazy rows; queued cancellation cannot
interrupt the current owner; callbacks disarmed/joined before freeing native
resources, reusing or closing connections. Active model cancellation killed and
drained only its owned worker and failed the session closed; queued cancellation
did not send bytes or kill another request. No stale reply consumption, automatic
restart or semantic fallback. Remote/sync cancellation was explicitly unsupported.

Recorded Linux amd64 native/API evidence: billion-step SQL canceled on a 10 ms
budget in **10.55 ms**; post-return CPU **0.098 ms over 150 ms**; expensive lazy
row **20.20 ms** on a 20 ms budget. Distinct actual MiniLM encodings (4,000,
synthetic correctness corpus) gave **373.50 ms** uncanceled ANN traversal versus
**10.23–10.35 ms** for five active 10 ms cancellations, with native error 9.
Checks covered pre-entry zero writes, queued-owner isolation, 100 late-callback/
reuse cycles, ten active-cancel/reuse cycles, rollback, other connections and
old-index compatibility. Returning early while native work continued was not
accepted as cancellation. Duplicate-vector stress data was discarded as unreliable.

Limits: cooperative, not kernel-I/O preemption or hostile-tenant isolation.
Linux arm64 cross-built before the final VM-entry follow-up; that follow-up
needed recross-build and had no arm64 runtime evidence. Both macOS builds were
blocked by absent licensed SDK/toolchain. Go race was not a native sanitizer pass.
These results do not establish FAISS/FFF interruption.
Evidence pointer: `/mnt/common/vvfs-native-interrupt-20261021`, especially
`prestart-fixed`, `acceptance-v3`, `api-regression` and `hybrid-regression`.

## Source-document checksums

SHA-256 of the six old documents read at final HEAD. Their useful technical
content and limitations are distilled above; these hashes identify provenance,
not a requirement to retain the files or recover dangling Git objects.

```text
7d590dd43504dbe70ce105b1c623a4bbe83b727bb7e90d1450c19482b2de46d0 INCREMENTAL_VERIFICATION.md
d5d3f0458ff66c26adea6aac25a33aec28db14db41139c86aa118342c70a70cc LOCAL_EMBEDDINGS.md
c2cc15841d8ac6314f02c24ae5e7050377b66fdb72f50423762b4b2eab71d1aa VECTOR_EVALUATION.md
64c1d779fe8cf9c725e3ae6ebf472e9960b610eb5e91db851e26940c25ad7d43 LOCAL_HYBRID.md
79edac8136980ee7467161d600bea97a0c4b57ff8627c0c4a4215614552ab2bc API_CONCURRENCY.md
0f64c5498e2ddb8594b9bc616a1999d72e89bde385547a893bedaff2e9bc3f1d NATIVE_CANCELLATION.md
```
