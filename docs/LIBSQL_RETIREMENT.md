# Permanent LibSQL retirement

## Authority and review boundary

The user authorized permanent deletion on 2026-10-21 of the obsolete
`/mnt/common/projects/tools/virtual-vectorfs` checkout, including its dirty
source, and identified exclusive LibSQL artifacts. No archive, backup, trash
move, source port, push or merge is authorized or needed.

The authoritative engine remains `/mnt/common/projects/go-pkgs/virtual-vectorfs`
(FFF/Bleve/FAISS). Its dirty README, Search Improvements and Historical LibSQL
Findings were read without editing them. This supplementary document belongs to
`retire-libsql-codebase`; it must be reviewed alongside those existing handoff
documents, which are not present in this branch's `fe092dd` base. It does not
copy or take ownership of another worker's unpublished documentation.

The historical handoff inspected had SHA-256
`4fac219b6e8f77e421766cd9284349a6bd27c5c5920d6287ea5ac1b143ee982d`.
Its narrow earlier authorization (retain tools canonical and external evidence)
is superseded by this explicit permanent-retirement request. Its algorithmic
findings remain useful; old source paths, commit IDs and evidence paths are
provenance identifiers, not recoverable dependencies or rerun instructions.

## Extraction completeness and supplementary findings

Read the dirty tools README, SPEC/cutover and salvage context, filesystem and
catalog implementation, and all six final `c2793c8` verification documents.
Compared the existing handoff with model/native locks, driver operation ownership
and final native/API/hybrid result records. No implementation or raw transcript
is retained here.

The existing Historical LibSQL Findings already preserves incremental identity
and hash guards, staged membership and transactional promotion, measured WAL/read
limits, model assets/preprocessing, native-versus-LEANN methodology and rejected
integration, hybrid ranking/citations/scope, generation barriers and cancellation
ownership. Search Improvements correctly classifies what must be reimplemented
against Bleve/FAISS rather than transplanted. In particular:

- Keep hash/profile invalidation, owner isolation, fail-closed capabilities and
  coherent source evidence as requirements. Existing RRF and vector reuse are
  not missing features. No mtime-only optimization or SQLite snapshot promise
  transfers to the current engine.
- The old canonical cutover was a lexical walking skeleton, not the final
  comparison implementation. It rewrote active source rows and emitted one path
  and one bounded content chunk. README/SPEC promises of full document vectors,
  structural strategies, portable packaging, organization and Git were not proof
  those features worked. The final MiniLM worker silently truncated at 256 tokens;
  directly seeded 24-window fixtures did not validate automatic chunking.
- Old safety work used canonical roots, relative containment, non-following
  traversal/open, descriptor identity checks, bounded UTF-8 reads, repeated
  inventory/hash verification and owner/lease/generation checks. These reduced
  ordinary races; they did not establish an atomic filesystem snapshot or a
  hostile-actor security boundary. FFF owns current inventory/watch; do not add
  the old walker alongside it.
- Salvage policy rejected lexical path-prefix safety, nondeterministic map/rank
  ordering, zero-based RRF, silent queue drops, disabled debounce, placeholder or
  zero-vector fallback, broad mutable trees, fanotify-only behavior, shell-backed
  Git and direct existence-check-then-write mutation. Overflow/ambiguous changes
  require reconciliation. These are design lessons, not reasons to import the
  old service graph, schemas, runtime indexes or organization subsystem.
- Raw spans remain evidence; summaries and path units are not content quotes.
  Model/profile changes require controlled invalidation, even at equal dimension.
  Derived-index compression ratios exclude model/source/catalog bytes. LEANN's
  oracle-tuned ten-query result is neither judged relevance nor scale evidence.
- The Starlark/FlatBuffers executor was bounded in-process, not a per-interpreter
  heap/security sandbox. No generic dispatch/FS/network authority was exposed;
  hostile code required an OS-isolated worker. Current companion Goja/broker
  design remains separate. Old plan/dry-run/journal/conditional recovery ideas
  are not implemented search-library features or authorization to mutate files.

### Final evidence corrects one stale platform statement

Historical LibSQL Findings says the final VM-entry correction still needed an
arm64 recross-build. The later `entry-verification.json` explicitly identifies
`c2793c8867c86a62bcd8f9c40f83a21d3c985023` and records:

| Target | Final native build / Go CLI link | Runtime |
| --- | --- | --- |
| Linux amd64 | PASS | PASS |
| Linux arm64 | PASS / PASS | NOT RUN |
| macOS amd64 and arm64 | BLOCKED: licensed Apple SDK/tools absent | NOT RUN |

The corrected C boundary returned native error 9, inserted zero rows and reused
the same connection. The existing race-enabled
`TestCancellationBeforeNativeEntryDoesNotWrite` passed in 1.017 s. This verifies
checking the persistent token **after idle reset/activation, before first opcode**;
periodic polling alone missed short pre-cancelled writes. These are historical
results read from evidence, not new test executions in this cleanup.

Independent final `vvfs-review-c2793c8-r97hb5p8-*` records additionally report:

- Native recursive SQL: 10.704321 ms; post-return CPU: 0.138 ms over 150 ms;
  lazy-row interruption: 20.318374 ms. Uncancelled real 4,000-vector ANN:
  297.164876 ms; five active cancellations: 10.233849–10.318392 ms, native error 9.
- Before-start/queued isolation, 100 late callbacks, rollback, other connections,
  graph rollback and old compressed-index reuse were VERIFIED. No abandoned
  native query goroutine, kernel-I/O preemption or native sanitizer pass claimed.
- API and CLI hybrid acceptance both VERIFIED. Reader 7 stayed coherent across
  writer 8; aborted promotion kept 8 with stale warnings. Scope, global RRF,
  inspect/trace, citations, worker cancellation and no semantic fallback passed.
  API summary's generic “no hard native-C interruption” caveat is not a hard
  deadline guarantee; the separate native record demonstrates cooperative unwind.
  Four synthetic semantic-answer fixtures remain tiny correctness checks.

### Exact native pin provenance

The final source lock named LibSQL 0.9.17 at
`cda864ed885764b704b418b94676df26f644da70`, Go driver `60e59c7150f4`, Rust 1.85.0
and Zig 0.14.1. SHA-256 identifiers:

| Item | SHA-256 |
| --- | --- |
| Upstream source archive | `7f19a8dedd13751331b880579bbfebbfd091bb6231ff3804a795dee22de0f573` |
| Original Cargo.lock | `5567d9bf4e6d698f1bd21db655ce4ed0f5af369f5b2fa917283f9be01511c535` |
| Final cancellation patch | `f554fb703106089caa69efe182defa96761f6217b895d913d65e27448b325f20` |
| Generated C header | `f6b6f9af7d4c830a7d8e05e812197812860b5b29dfd4f6f5ae5a317fcce2bfb5` |
| Final Linux amd64 archive | `df068b8b0782b5bc4c5b95f1a4e4e3b102d57a308d382c18e730cb8828a19ca3` |
| Final Linux arm64 archive | `6bd7b36753eb5f4d443e2c405d0003ae3c603cd16c05e003858516385aabd083` |
| Zig Linux x86_64 archive | `24aeeec8af16c381934a6cd7d95c807a8cb2cf7df9fa40d359aa884195c4716c` |

Model lock exactly agrees with the historical handoff: MiniLM revision
`1110a243fdf4706b3f48f1d95db1a4f5529b4d41`, 384D ONNX float32, mean/L2/256 v1,
Apache-2.0, Python 3.12, ONNX Runtime 1.23.2, tokenizers 0.22.1, numpy 2.3.4;
the four asset hashes and byte sizes are already preserved there. Runtime licenses
identified LibSQL, SQLite, multiple-ciphers and MIT Go driver. No binary/source is
redistributed by this documentation, and no install/rebuild support is promised.

## Ownership and deletion inventory

Baseline owner explicitly protects the entire authoritative checkout (including
ignored `build/native`, `build/deps/faiss-fff814dea2bdda020363506904979b204ee201aa`,
`build/cargo`, `native/fffbridge`) and `/mnt/common/vvfs-engine-baseline-lwqjeoyd`.
It confirms no dependency on old tools, MiniLM, native-interrupt, prior baselines,
reviews or `/tmp/opencode` scratch. Reviewer confirms idle/no background jobs;
implementation session was previously paused and was notified to stop.

Ownership was checked using commit-named source/build layouts, model/patch locks,
synthetic corpus and results provenance, and read-only SQLite schema inspection
(`workspaces/sources/documents/chunks`, FTS5, or isolated native ANN fixtures).
Validation exports contain the old LibSQL module, not current search sources.
The temporary assessment cache below is deliberately excluded. A `/proc`
cwd/fd/executable/maps check found no active model/native/benchmark consumers of
the deletion inventory; obsolete shells/UI and shared MCP services had cwd-only
references. Shared MCP services are not exclusive deletion targets.

Exact deleted paths and pre-removal bytes (`du -sb` apparent,
`du -s -B1` filesystem-reported allocation; symlink targets not followed):

| Path | Apparent bytes | Allocated bytes |
| --- | ---: | ---: |
| `/mnt/common/vvfs-baseline-y9solvp1` | 17,313,806 | 1,186,816 |
| `/mnt/common/vvfs-current-baseline-3drpoira` | 16,355,342 | 1,162,240 |
| `/mnt/common/vvfs-incremental-evidence-20261021` | 41,912,300 | 3,702,784 |
| `/mnt/common/vvfs-local-minilm` | 570,190,325 | 270,900,224 |
| `/mnt/common/vvfs-milestone1-evidence-20261021` | 32,451,311 | 2,727,936 |
| `/mnt/common/vvfs-milestone1-safe-evidence-20261021` | 32,234,217 | 2,748,416 |
| `/mnt/common/vvfs-native-interrupt-20261021` | 2,893,383,080 | 1,142,007,296 |
| `/mnt/common/vvfs-review-19b5e37-enifzmu6-acceptance` | 817,314 | 224,768 |
| `/mnt/common/vvfs-review-19b5e37-enifzmu6-cli` | 429,003 | 94,208 |
| `/mnt/common/vvfs-review-75079cef-sray2jux-evidence` | 32,234,221 | 2,736,128 |
| `/mnt/common/vvfs-review-76eceb5-xsanfrt4-acceptance` | 430,605 | 98,304 |
| `/mnt/common/vvfs-review-76eceb5-xsanfrt4-benchmark` | 5,478,869 | 3,381,760 |
| `/mnt/common/vvfs-review-9e43b59-84jw6jff-api` | 849,281 | 225,280 |
| `/mnt/common/vvfs-review-9e43b59-84jw6jff-compat` | 699,246 | 96,768 |
| `/mnt/common/vvfs-review-9e43b59-84jw6jff-hybrid` | 430,586 | 98,304 |
| `/mnt/common/vvfs-review-9e43b59-84jw6jff-native` | 123,046,424 | 46,946,816 |
| `/mnt/common/vvfs-review-a0ec5ba-rj5iaahs-reuse` | 32,209,639 | 2,748,416 |
| `/mnt/common/vvfs-review-a0ec5ba-rj5iaahs-vectors` | 8,827,330 | 5,236,224 |
| `/mnt/common/vvfs-review-c2793c8-r97hb5p8-api` | 849,307 | 225,280 |
| `/mnt/common/vvfs-review-c2793c8-r97hb5p8-compat.db` | 208,896 | 33,280 |
| `/mnt/common/vvfs-review-c2793c8-r97hb5p8-hybrid` | 430,802 | 98,304 |
| `/mnt/common/vvfs-review-c2793c8-r97hb5p8-native` | 123,046,426 | 46,819,840 |
| `/mnt/common/vvfs-reviewfix-evidence-20261021` | 32,197,358 | 2,723,840 |
| `/mnt/common/vvfs-semantic-review-2n_gs91y` | 248,870 | 61,952 |
| `/mnt/common/vvfs-wave2-hybrid-20261021` | 50,541,756 | 25,665,536 |
| `/mnt/common/vvfs-wave3-api-20261021` | 145,214,339 | 68,910,592 |
| `/tmp/opencode/vvfs-baseline-3ngriusf` | 752,545,948 | 774,000,640 |
| `/tmp/opencode/vvfs-milestone1-catalog.go` | 30,736 | 32,768 |
| `/tmp/opencode/vvfs-milestone1-incremental.go` | 6,100 | 8,192 |
| `/tmp/opencode/vvfs-milestone1-validation` | 347,832 | 495,616 |
| `/tmp/opencode/vvfs-native-probe.db` | 24,576 | 24,576 |
| `/tmp/opencode/vvfs-native-probe.go` | 692 | 4,096 |
| `/tmp/opencode/vvfs-review-19b5e37-enifzmu6` | 49,228,116 | 49,463,296 |
| `/tmp/opencode/vvfs-review-75079cef-sray2jux` | 22,478,167 | 22,634,496 |
| `/tmp/opencode/vvfs-review-76eceb5-xsanfrt4` | 44,711,059 | 44,920,832 |
| `/tmp/opencode/vvfs-review-9e43b59-84jw6jff` | 149,439,538 | 149,733,376 |
| `/tmp/opencode/vvfs-review-a0ec5ba-rj5iaahs` | 43,416,170 | 43,614,208 |
| `/tmp/opencode/vvfs-review-c2793c8-r97hb5p8` | 149,451,288 | 149,749,760 |
| `/tmp/opencode/vvfs-reviewfix-validation` | 364,122 | 532,480 |
| `/tmp/opencode/reviewfix-incremental.go` | 2,370 | 4,096 |
| `/tmp/opencode/reviewfix-catalog.go` | 30,282 | 32,768 |
| `/mnt/common/projects/tools/virtual-vectorfs` | 25,003,040 | 18,756,608 |

Total: **5,399,110,689 apparent bytes; 2,884,869,120 allocated bytes**.
These are file-tree accounting, not guaranteed physical host-pool reclamation:
FUSE, compression, hardlinks, shared backing extents, open handles and snapshots
can differ. No secure overwrite, snapshot destruction or off-host purge is claimed.

## Protected and uncertain paths

- Keep the authoritative tree, this cleanup worktree/session, companion worktree,
  and the current baseline tree above. No current workers are retirement targets.
- Keep `/tmp/opencode/vvfs-assessment-flZwVn`: inactive but current-engine
  assessment cache, not proven exclusive obsolete LibSQL ownership.
- Keep `/home/ubuntu/go/pkg/mod`, shared Go build caches, Rust toolchains/targets,
  Cargo caches and shared interpreter/model/package-manager caches. No global
  cache deletion follows from a historical LibSQL dependency name.
- Keep `/mnt/common/projects/ai/mcp-memory-libsql-go`,
  `/mnt/common/bin/bin/mcp-memory-libsql-go`, all unrelated projects and backups.
  Matching “libsql” is not ownership proof.
- Shared MCP processes whose cwd is the obsolete checkout are not killed merely
  for that cwd. Their server/index storage outside the identified paths has not
  been authorized by exact path; no broad service or database cleanup is attempted.

## Removal and verification status

Completed 2026-10-21. Extraction and the exact inventory were written before
permanent unlink/removal from this safe, external cleanup worktree. All **42**
listed paths were verified absent, as was the additionally removed empty
`/mnt/common/projects/tools/virtual-vectorfs__worktrees` container. No source
backup/archive or replacement Git ref was created. Deleting the obsolete common
Git directory also removed its local refs, reflogs and unreachable source objects;
remote repositories were not changed.

`workmux close main` in the obsolete project successfully closed
` virtual-vectorfs` before checkout deletion. The old comparison worktree and
branch were already absent on first inspection. Its orphaned session
` optimize-incremental-local-vectors` **remains**: both `workmux close` and
`workmux remove --force` rejected it with “Worktree not found.” No direct tmux
kill or recreation of retired source was used to bypass the lifecycle contract.
The baseline owner acknowledged this limitation for coordinator handoff. Its
remaining idle UI/shell has a deleted cwd, not a retained checkout or worktree.

Remaining relevant Workmux inventory: authoritative `main`, `companion-harness`
and `retire-libsql-codebase`, all retained with their sessions; plus the orphaned
obsolete session above, absent from Git/Workmux worktree inventory. Unrelated
project sessions were not changed.

Post-removal checks:

- All 48 fingerprinted authoritative files (tracked/untracked source and docs,
  plus the three current native shared libraries) remained byte-identical.
  Canonical dirty changes were not staged, committed, reset or overwritten.
- Top-level related-artifact scans leave only protected
  `/mnt/common/vvfs-engine-baseline-lwqjeoyd` and
  `/tmp/opencode/vvfs-assessment-flZwVn`. The temporary hash/inventory metadata
  used for preservation checking is removed after verification; no raw logs or
  source snapshot are retained with this report.
- Immediate available-space deltas: `/mnt/common` **+1,211,105,280 bytes**;
  `/tmp/opencode` tmpfs **+1,235,234,816 bytes**. These are concurrent-filesystem
  observations, not exclusive causal measurements. The 5.40 GB apparent / 2.88 GB
  allocated tree totals above are the inventoried removal amounts. Actual
  physical host-pool bytes reclaimed are **not measurable from this guest**;
  tmpfs release is memory/swap-backed space, not host disk reclamation.
- `git diff --check` passed in the authoritative tree. Documentation-only branch
  validation checks whitespace and inventory/preservation, not compilation or
  reruns of the deleted historical experiments. No new runtime test is claimed.

Memory Bank `virtual-vectorfs/activeContext.md` was updated to identify the
authoritative engine and completed deletion; its other old cutover narratives
are explicitly historical, not implementation authority.
