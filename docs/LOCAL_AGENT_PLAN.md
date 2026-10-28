# Local Personal Agent — Delivery Plan

**Status:** Work sequence approved on 2026-10-11; the companion agent has not been built.
**Authority:** This file is the source of truth for deliverables, dependencies, and acceptance gates until the companion has its own repository and this plan is deliberately transferred there. Keep issue trackers and execution logs linked to these IDs, not as competing plans. Update this file when a decision changes scope or a gate; do not mark a task complete because code was written or a test was intended to run.

This is a plan for a **separate Go product** that imports `github.com/ZanzyTHEbar/virtual-vectorfs/search`. This repository remains a search-only library. F0 published the public-readiness cleanup, this plan, and its README link together; do not blend that work into agent implementation.

## Delivery contract

- One ID is one reviewable causal change, including its tests and documentation. Complete it only after the stated gate passes and its dependencies are verified. Put specific `CHECK`/`EXPECT` commands into the leaf's gates file **before implementation**; rerun them at the parent/integration boundary. A plan entry is not test evidence.
- Runtime stays offline. A first-time build or model download may need network access, but the running product must work with outbound networking disabled. The Go harness owns one logical agent, tool authority, resource budgets, and event recording. A disposable Goja worker may compute and propose; it cannot mutate the workspace. Recursive inference is parent-mediated and limited to depth one.
- All **supported agent-initiated user-workspace writes** go through one write broker. Supported actions must have durable intent, recoverable preimages where needed, approval, and conditional undo. The immutable event log itself is never rolled back; memory corrections are versioned. Arbitrary shell commands, other applications' writes, non-cooperating editors, and unsupported file types are outside any unconditional rollback claim. Refuse a write if its backup or recovery conditions cannot be met.
- Search hits are leads, not exact citations or write authority. `Hit.ByteOffset` is the preview start, not a match offset; file IDs derive from paths and change on rename. Expansion must verify the expected content hash and source range. FFF `Grep` provides a file byte offset, but its `MatchByteOffsets` are relative to the *returned display line* and adjusted on truncation; they are not directly file offsets. Caller-owned records require their own verified range evidence. Vector inputs remain driver-supplied; the index is derived state, not the authoritative file.
- Begin with deterministic routing. A local decision model must beat a fixed offline baseline on predeclared quality, latency, and memory budgets before it replaces that route. No model selection changes authorization policy.

**F1 decisions:** the user delegated conservative defaults, recorded in the [scope and recovery policy](LOCAL_AGENT_POLICY.md). The default writable root is app-owned and initially empty; personal directories remain read-only unless separately granted. Runtime filesystem and model/hardware checks still precede any capability. If a guarantee depends on exclusive control or filesystem snapshots, record that assumption rather than silently promising atomic multi-file rollback.

## Work set F — foundations and existing fixes

| ID | Needs | Atomic deliverable | Observable gate | State |
| --- | --- | --- | --- | --- |
| **F0** | — | Review the existing public-readiness cleanup, this plan, and its README link together; publish without agent implementation and without rewriting unrelated work. | Search library's vector-enabled build, race tests, vet, Rust checks, and final diff review pass; published README link resolves to the published plan; publishing requires an explicit user request. | Complete — published in `8c03f5c`; remote plan link verified |
| **F1** | — | Agree on the companion's write-scope, threat, retention, hardware, privacy, and undo contract; record supported and explicitly refused operations. | Every supported write has preconditions, durable recovery, undo, and conflict behavior; user accepts assumptions before W1. | Complete as a policy decision — delegated defaults reviewed; runtime gates pending |
| **F2** | F0 | Add a bounded version-aware read/expand API for indexed documents; keep previews and exact match evidence distinct. | Tests cover caller records, changed/stale/missing files, renames, size limits, and verified source ranges. Grep's file offset and display-relative match ranges are interpreted correctly even for truncated lines; caller-record ranges are independently validated. | Integrated locally — F2 gates revalidated 2026-10-28; not published remotely |
| **F3** | F0 | Create a separate companion Go module and read-only `doctor` CLI for Linux, native libraries, search import, and local model endpoints. No agent loop or write tools. | Fresh downstream `go build -tags=vectors ./...` works with installed native dependencies; missing dependencies fail clearly and corpus files remain unchanged. | Planned |
| **F4** | F1, F3 | Version a small representative corpus and judged tasks: exact lookup, sourced answer, cross-document reasoning, duplicates, and organization proposals, with negative controls. | Offline fixture loader verifies inputs and gold evidence; baseline retrieval measurements and predeclared task-success, latency, and RAM/VRAM criteria are reproducible. Measure actual end-to-end task success only after R6 exists. | Planned |

**F integration:** F0–F4 verified; the search package still has no agent/model/mutation dependency.

## Work set R — read-only vertical product

| ID | Needs | Atomic deliverable | Observable gate | State |
| --- | --- | --- | --- | --- |
| **R1** | F3, F4 | Pin and probe local Gemma, EmbeddingGemma, and llama.cpp artifacts; record model hashes, template, context, embedding dimensions, memory, and latency. | Generation and embeddings work with networking disabled; incompatible artifacts and over-context inputs fail explicitly. | Planned |
| **R2** | F2, F3 | Read-only search adapter: candidate search, scoped/version-checked expansion, bounded excerpts, and exact citations. | A changed or missing file cannot be cited as current; tests distinguish previews from verified source ranges. | Planned |
| **R3** | R1, R2 | Bounded embedding ingestion through `MissingVectors`/`UpsertVectors`, with model/dimension provenance and hash checks. | A change invalidates its vector; input beyond the model's context is refused or separately handled, never silently truncated. | Planned |
| **R4** | F3 | Versioned SQLite event log for tasks, routes, model requests/responses, tool calls/results, and errors; add read-only transcript replay. | Restart recovers ordered committed events; malformed records fail safely; replay never calls a live tool or writes to the corpus. | Planned |
| **R5** | R4 | Bounded tool-result offloading with immutable local references, page/range reads, permissions, and retention. | Large results stay out of model context but their recorded bytes remain retrievable even if the source later changes. | Planned |
| **R6** | R1–R5 | One read-only CLI task loop with deterministic routing, explicit completion, budgets, cancellation, errors, and citations. | Fixed tasks complete offline; path-only lookup needs no model; timeout/failure/restart are observable and logged. | Planned |
| **R7** | R6 | Disposable Goja worker with task-lifetime variables and read-only typed bindings. Pin a compatible Goja/toolchain version in the companion. | Timeout, memory exhaustion, malformed requests, and worker death are contained; scripts have no direct filesystem, process, environment, or network authority. | Planned |
| **R8** | R7 | Parent-mediated `llm.ask()` with depth one, bounded fan-out and context, and no inherited write privilege. | A child cannot spawn grandchildren or execute effects; exceeded budgets fail with recorded errors. | Planned |

**R integration:** a cited multi-document answer and a REPL-driven filtering task work offline; recorded replay produces the same observations without reinvoking models, tools, or effects. Model output regenerated on another run is **not** assumed deterministic.

## Work set W — write authority and conditional rollback

No model or REPL path receives a mutation capability until W2 and the corresponding broker operation pass their gates. SQLite transactions do not make filesystem effects atomic; startup must reconcile unfinished intents.

| ID | Needs | Atomic deliverable | Observable gate | State |
| --- | --- | --- | --- | --- |
| **W1** | F1, R4 | Specify broker states and build bounded fault-injection/model-check fixtures for intent, effect, crash, recovery, undo, and conflict. | Check: no acknowledged effect without durable recovery data; detected version changes block undo; duplicate IDs cannot apply twice; replay has zero effects. Non-cooperating writers racing during an effect are outside the guarantee. | Planned |
| **W2** | W1 | Root-scoped path validation and plan/preview/approval bound to exact paths, source versions, action, and expiry; still no mutation. | Traversal, symlink escape, special files, changed plans, and replayed approvals are rejected. | Planned |
| **W3** | W2 | Create-new regular file via the broker, with durable intent, no-replace publication, startup reconciliation, and conditional undo. | Kill at each journal/filesystem boundary: retries cannot create twice; changes detected before undo are conflicts. No guarantee against non-cooperating concurrent writers. | Planned |
| **W3a** | W3 | Brokered create-new directory under an approved existing workspace directory; undo only after checking it is the same, empty directory. | Crash/retry/collision tests never replace an existing path; a detectable change or nonempty directory produces a conflict without removing its contents. Races during removal remain outside the guarantee. | Planned |
| **W4** | W3, R6, R7 | Wire the agent's proposal and user approval flow to the broker; the worker/child may propose but cannot commit directly. | Read → propose → preview → approve → create → restart → conditional undo passes; attempted authority bypasses fail. First writable CLI slice. | Planned |
| **W5** | W3 | Brokered replacement with durable preimage and conditional restoration; refuse files exceeding backup capacity. | Disk-full/crash tests recover the old bytes; external edits detected before effect or undo block it. Concurrent non-cooperating writers are outside the guarantee. | Planned |
| **W6** | W5 | Brokered delete with recoverable storage and destination-collision handling. | Restart and undo restore supported bytes/metadata or report detectable conflicts; no promise against a writer racing during removal/restoration. | Planned |
| **W7** | W5 | Brokered same-filesystem move/rename with source-version and destination-collision checks. | Move and reverse move pass; a changed source or occupied target detected before effect or undo blocks it. Concurrent non-cooperating writers are outside the guarantee. | Planned |
| **W8** | W7 and a separately approved dual-filesystem grant | Optional brokered cross-device move: stage destination copy, flush/verify, publish, then remove source. Disabled by F1 defaults. | Faults at each phase reconcile both paths; uncertain outcomes stop instead of deleting/repeating blindly. | Blocked pending explicit scope change |
| **W9** | W3a, W5–W7 | Same-filesystem multi-operation plans with reverse-order compensating undo and per-operation outcomes. | Interrupted organize/dedupe fixture reports exactly which actions applied, were undone, or conflict; it never claims cross-file atomicity. | Planned |

**W integration:** supported roots/filesystems pass traversal, symlink, collision, edits detected before an effect, disk-full, restart, and recovery tests. A conflict is an acceptable safe outcome; silent data loss under the supported quiescent-effect assumption is not. Concurrent non-cooperating writers during effects remain outside the guarantee. Anything outside the supported operations stays denied.

## Work set P — memory, routing, extraction, interface

| ID | Needs | Atomic deliverable | Observable gate | State |
| --- | --- | --- | --- | --- |
| **P1** | R4–R6, W4 | Versioned, provenance-linked facts and user-approved corrections in SQLite, distinct from the raw event transcript. | Recall cites a current source; undo/correction changes the current memory view without erasing history or bypassing write policy. | Planned |
| **P2** | F4, R6, R8 | Evaluate a pinned small **local decision model** against the deterministic router; log routes and actual physical models. | Same offline corpus/hardware and predeclared quality/p95/RAM limits decide enablement; fallback is bounded and policy is unchanged. | Planned |
| **P3** | F4, R2 | Add local parsing/extraction only for formats proven necessary by the corpus; currently the library indexes UTF-8 text, not invoice PDFs. | Extraction preserves verifiable page/range provenance; unsupported formats stay explicit. | Planned if F4 requires non-text files |
| **P4** | W9, P1, P2 decision; P3 if required | TUI over the existing task, evidence, preview, approval, recovery, and undo APIs. No TUI-only authority path. | CLI and TUI reach identical broker decisions and surface applied, blocked, recovery-required, and conflict states. | Planned |

**P integration:** fixed real tasks and adversarial cases run with outbound networking disabled; record grounded task success, retrieval quality, p95, peak RAM/VRAM, backup usage, and recovery/undo outcomes. Only describe an operation as reversible when its conditional-undo gate passes.

## Execution and handoff rules

- F0 is verified; F2 and F3 may proceed independently. R4 can proceed alongside R1/R2. Safety design W1 may begin after R4 while read-only R5–R8 continue. Serialize changes to the broker protocol and operation implementations; later routing/UI work must reuse its existing authority boundary.
- For each ID, keep its actual owner, branch/PR, runnable gate definition, validation evidence, blocker, and next action with the execution record. Mark this table `Complete` only after gate re-verification and integration review, not on implementation intent. An abandoned gate is a handoff, never success.
- The companion repository should take over this file as its canonical plan once it exists; retain a link here, and do not maintain two independently edited copies. Memory Bank (if available) is for durable architecture decisions, not this backlog; execution tracking is not a second source of truth.
- F0 was published and its README link verified on `main`. Native installation on other machines and model/hardware behavior are not verified by the existence of this document; each later gate still requires its own evidence.

## Source checks behind the plan

- [Search-library boundary](ARCHITECTURE.md) and current [`Hit` contract](../search/types.go): preview offset versus match location, path-derived file identity, eventual indexing.
- [Gemma 4 12B model card](https://huggingface.co/google/gemma-4-12B-it) and [EmbeddingGemma 2 model card](https://huggingface.co/google/embeddinggemma-2): model availability and embedding dimensions; actual local inference is still a gate.
- [Goja](https://github.com/dop251/goja): runtime interruption and single-goroutine constraint are useful, but it is not an OS security boundary.
- [Pi compaction](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/compaction.md), [Letta MemFS](https://docs.letta.com/concepts/memfs), and [Manus context engineering](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus): recoverable results, lean context, and stable tool contracts; none establishes this product's performance without F4/P2 measurements. “Muse” remains unidentified pending a first-party reference.
