# Companion harness: owner handoff and implementation gates

**2026-10-21 — plan only; no companion implementation.** Owner: dedicated
`companion-harness` Workmux agent. Retain this branch/session; no merge, push,
cleanup, dependency transfer, installation, download or inference is authorized
by this handoff. This execution record supplements, not replaces,
[LOCAL_AGENT_PLAN.md](LOCAL_AGENT_PLAN.md) (F/R/W/P delivery IDs) and
[LOCAL_AGENT_POLICY.md](LOCAL_AGENT_POLICY.md) (authority and recovery).
The canonical working-tree versions take precedence over this HEAD-based branch.

## Decision required before code

The approved product is a **separate Go module** importing
`github.com/ZanzyTHEbar/virtual-vectorfs/search`, not a command or internal package
in the search repository. No agreed repository/module/product name was found in
the canonical plan, policy or search-improvement requirements.

Shallow environment inventory found `/mnt/common/projects/tools/familiar`, empty
including hidden entries. It is a candidate, **not an approved target**; nothing
was created there. `/mnt/common/projects/ai/executive-assistant` is an existing
Cloudflare OS product, not evidence of this Go companion's location. No broader
project-content or personal-directory inventory was performed.

**Question sent to coordinator via `workmux send main`:** Is the existing empty
`familiar` directory the intended target? Select/provision it with the Go module
and executable name, or supply the approved absolute repository path and names.
Recommendation: confirm that candidate before creating another product location.
Until answered, only this design document may be committed in this worktree.

## Search dependency boundary

Authoritative source: `/mnt/common/projects/go-pkgs/virtual-vectorfs`.
Assessment HEAD: `fe092dd03d398570b5d93cf0c030f573bc90257b`; the isolated owner
branch starts at the same commit. That commit **does not contain F2**.

Canonical uncommitted `search/expand.go`, `search/expand_test.go`, changes in
`search/engine.go` / `search/types.go`, and their documentation are user work.
`docs/SEARCH_IMPROVEMENTS.md` is also uncommitted and authoritative. None was
copied, moved, staged, reverted or edited here. The baseline owner acknowledges
byte preservation, no API/source transfer this wave, and no published F2 revision.
Baseline measurement drivers may compile against the canonical dirty tree; that
is not permission for this product to acquire an implicit dirty dependency.

Read-only SHA-256 fingerprints, identical before and after this milestone:

```text
docs/SEARCH_IMPROVEMENTS.md  452255632fea16dda55f2a4eaccd537eecb83dad1118344eafdc5a531a591fe6
docs/LOCAL_AGENT_PLAN.md     545384033d60af3a92bbb450305b692c69aec6df617f20c1c75cd16f88c3e6db
docs/LOCAL_AGENT_POLICY.md   c3f89e6f6659b56efb00c8193704d9aee8d113a446920f863a427261176988bc
search/engine.go            bad1a98bcd75c889e75b721adba4ded21d177dd0325e618e721fee73469b33f1
search/types.go             beaced5b0ea93e3b028104a229a2c3f55d9fc3a90be63c519541dc6de8381608
search/expand.go            8a037c3fe67084231f7b80bc262dc1c760baab55d380fc01f2e258cbf11ecc25
search/expand_test.go       78ef18406c55c5ee8ce95e083ee917a0303bc4611d83b4595fadaa9c26571e64
```

| Consumer | Required contract | Integration gate |
| --- | --- | --- |
| F3 doctor | Existing exported search import; Linux/CGO/vector build; installed FFF/FAISS | May use an explicitly pinned existing revision independently of F2 after target selection; no claim that R2 is available |
| R2 evidence adapter | `Engine.Expand`, `ExpandRequest`, `Excerpt`, `MaxExpandBytes`, origins, `ErrStaleContent`; internal `expandSlots` is library-owned | Coordinator-approved F2 integration revision or explicitly approved local dependency strategy; no copying source or silent `replace` |
| R3 embeddings | `MissingVectors`, `UpsertVectors`, expected hash, fixed index vector schema | Driver pins model/tokenizer/preprocessing profile; same dimension is not compatibility |
| All retrieval lanes | `Search`, `FindFiles`, `Grep`, freshness/status, explicit roots/sources | Parent-owned immutable grants on every lane, expansion, result read and replay; start with separate scoped indexes |

R2 must retain these exact semantics:

- Search hits are leads. `Hit.ByteOffset` is preview start, not match offset;
  generation is not an immutable filesystem snapshot. Rename changes file ID.
- Expansion requires ID, expected hash, expected source and an explicit field.
  It returns a verified UTF-8 half-open byte window, origin, total field size and
  hash; default 4 KiB output, maximum 16 KiB. Stale/missing content fails closed.
- File expansion verifies live bytes; caller records use indexed field evidence.
  A caller's `Path` is never filesystem authority. Record citations must identify
  the record field/version, not pretend to quote a live file.
- Grep match ranges refer to the returned, possibly truncated display line.
  Locate and verify source bytes before issuing exact citations, including matches
  crossing excerpt pages. Never add display offsets blindly to file offsets.
- Expansion can hash the whole eligible file and materialize a caller field.
  An output cap is not a total-memory bound or atomic index/filesystem snapshot.
  Bound admission/process resources separately; show verification time/version.

Dependency notice was sent through Workmux to `main` for the baseline owner;
baseline acknowledgment received. No search architecture changes requested.
Do not use the retiring tools/virtual-vectorfs repository or its artifacts.
Historical findings contribute lessons only: explicit scope, source provenance,
honest cancellation, reproducible measurements; no LibSQL/SDK/schema resurrection.
Memory Bank still describes that retired architecture and is not current authority.
Codebase-memory coverage for the canonical Go repository includes F2; local source
was inspected to verify the contract rather than relying on historical memory.

## Observed environment, not runtime qualification

- Linux `7.0.0-38-generic`, x86_64 KVM, 8 exposed Ryzen 9 5950X CPU cores;
  Go `1.26.8`, Rust `1.98.1`; cc, CMake, pkg-config and sqlite3 on PATH.
- 31 GiB RAM total, approximately 3.1 GiB available at inspection, no swap.
  AMDGPU PCI `1002:6985`: 4 GiB VRAM total, about 2.42 GiB used at inspection.
  Availability is transient; no Gemma/EmbeddingGemma fit or latency claim follows.
- `llama-server`, `llama-cli`, `nvidia-smi`, `rocm-smi` were absent from PATH.
  This is not an exhaustive model/artifact inventory. No endpoint was contacted.
- `/mnt/common` currently reports **virtiofs**, not the earlier assessment's
  fuseblk. Do not qualify either as a supported broker filesystem from its name.
  Scratch capacity: approximately 11 GiB free on /tmp, 1.6 TiB on /mnt/common.
- Native shared libraries exist in canonical `build/native/lib` (ignored build
  output). Default pkg-config cannot find `fffbridge`; explicitly setting
  `PKG_CONFIG_PATH` to its `lib/pkgconfig` reports bridge package `0.1.0`.
  Earlier `/tmp/opencode/vvfs-native-prefix/lib` is absent. No linking/ABI/runtime
  verification was performed. A Go import does not install native dependencies.
- Cgroup CPU/memory ceilings were not established; root `memory.max`/`cpu.max`
  were absent. Worker isolation/resource enforcement remains an acceptance gate.

## Ordered executable slices

Implement required semantics in their owning slice; do not enable capabilities
early behind a pre-alpha waiver. Put exact CHECK/EXPECT commands in the selected
companion's gates file **before** each slice. Commands below are proposed gate
contracts, not commands already run or a claim that these interfaces exist.

1. **F3: first production slice, only after target selection.** Create the separate
   module and one read-only `doctor` CLI. Import the pinned search API; report
   platform, native linkage and configured local model readiness without starting
   an agent, scanning personal files, creating an index or downloading anything.
   Missing dependencies/model configuration must be explicit, not a fake healthy
   result. Model probing is opt-in, loopback-only and bounded; no inference here.
   Select native prefix and cached dependencies explicitly; if unavailable, stop
   and report rather than invoke `make`/install/network fetching.

   After provisioning, bind `COMPANION`, `CLI`, `SCRATCH` and native environment
   to approved paths/names. Use a fresh empty scratch workspace and state directory
   outside it. Proposed CHECK:

   ```sh
   # Run from COMPANION; SCRATCH holds only this gate's build/empty fixtures.
   GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local CGO_ENABLED=1 GOMAXPROCS=2 \
     GOMEMLIMIT=256MiB timeout 120s go build -mod=readonly -p=2 \
     -tags=vectors -o "$SCRATCH/bin/$CLI" ./cmd/"$CLI"
   "$SCRATCH/bin/$CLI" doctor --json --offline \
     --workspace "$SCRATCH/workspace" --state "$SCRATCH/state"
   ```

   EXPECT: build exit 0 with installed compatible native dependencies; doctor
   gives versioned machine-readable ready/blocked reasons and nonzero exit when
   a required capability is absent. Compare pre/post directory manifests and
   bytes: no workspace/state changes, including implicit index/watch setup.
   Run a missing-native launch in an isolated loader environment: a clear loader
   error is expected before main if dynamically required libraries are absent;
   do not promise doctor itself can run without its linked libraries.

2. **F4, then R1:** synthetic versioned corpus/judged evidence and quality/p95/
   process-tree RAM/VRAM thresholds before measuring. R1 requires separately
   authorized artifact acquisition if artifacts are absent. Pin hashes, licenses,
   runtime, template, tokenizer, context, pooling/normalization and dimensions.
   Offline generation/embedding, explicit context refusal and measured fit gate
   enablement. No downloads, installation or inference authorized this milestone.

3. **R2 after approved F2 handling; R3 after R1/R2:** scoped adapter and verified
   citations, then bounded profile-bound ingestion. One real scratch acceptance
   flow must reject a changed/missing file as current and preserve caller-field
   provenance. Invalidated/stale vectors and incompatible profiles are refused.
   Do not add chunking, parsers or another traversal/index architecture speculatively.

4. **R4 after F3; R5 after R4:** parent-owned append-only, schema-versioned SQLite
   events with ordered committed recovery, task/call identities and bounded
   admission. Commit observable results before acknowledging them. Replay opens
   state read-only and reconstructs recorded observations with no model/tool/
   corpus effects. A real record → restart → replay scenario must work while live
   tool/model endpoints are unavailable. Corruption fails explicitly. R5 stores
   immutable, hash-identified result bytes outside readable/indexed roots, with
   task/grant-bound references and bounded page/range reads. Reject forged,
   cross-task, out-of-scope and over-budget references; source edits cannot change
   saved bytes. Explicit pruning preserves audit history and marks unavailable
   replay/undo evidence. Same-UID tampering is not prevented by SQLite/permissions.

5. **R6 → R7 → R8:** deterministic read-only CLI loop first, then disposable Goja
   subprocess, then parent-mediated `llm.ask`. Parent owns immutable grants,
   budgets, cancellation, typed RPC validation and all authority. A Goja runtime
   alone is not a sandbox: enforce process resource limits and OS restrictions
   denying direct filesystem/network/process/environment access before exposing
   the worker. Worker timeout/OOM/death must not kill the parent or leave unowned
   inference/native work running. Gate containment with the actual restricted
   subprocess, not mocks. Native non-interruptibility must be exposed and owned,
   not hidden by abandoned goroutines. Child inference depth <=1 and fan-out <=1;
   children share task budgets and receive no write authority or recursive spawn.

6. **W1 onward only after dependencies:** specify/review broker states against
   durable-intent, crash, recovery, idempotency and conditional-undo invariants.
   W2 approves exact plans without effects. W3 adds create-new regular files only
   on a locally qualified filesystem; W3a gates directories separately. W4 is the
   first writable CLI. Replace/delete/move remain denied until W5–W7; W8 remains
   disabled. W9 is reverse-order compensation, never cross-file atomic rollback.
   P-stage memory/routing decisions and TUI reuse these same boundaries.

## Safety acceptance checklist (mandatory, not yet satisfied)

- No inferred HOME/read/write grants; initially empty app-owned workspace on
  adoption. Check absolute path, trusted ownership, safe nonsymlink ancestry and
  recorded identity every start. State/index/results/backups stay outside roots;
  private directories/files use 0700/0600. Refuse privileged broker execution.
- Runtime egress denied; only explicitly configured loopback inference transport.
  No implicit model switch, silent truncation or authorization changes by models.
  One active generation and embedding job; 8K total context, 1,024 output tokens,
  four calls/task, two minutes/call, ten minutes/task; shared child accounting.
- Write approval single-use, expiring and bound to exact action/root/paths/versions.
  Reject traversal, symlinks, mount crossings, special files and collisions.
  Durable intent/recovery data precede effects; uncertain flush/effect results
  require reconciliation, not blind retries. Conditional undo checks identity
  and postimage; conflicts leave files untouched. Fault-inject actual journal/
  filesystem boundaries before enabling each operation. No guarantee against a
  non-cooperating same-UID writer racing an effect or atomic multi-file rollback.
- Write maximum 8 MiB; separate 1 GiB undo and 1 GiB payload caps; audit metadata
  admission-controlled too. Disk-full/capacity failure refuses work, never silently
  evicts evidence. No automatic expiry. Preserve promised bytes/permissions/mtime
  or refuse; do not promise inode/ACL/xattr restoration without a separate gate.
- Use empty scratch roots/synthetic fixtures only. Existing focused checks first;
  one high-signal real acceptance flow per capability where practical, no helper
  test matrices or fake collaborators. No personal indexing or user databases.

## Milestone result and next action

Plan-only checks: `git diff --check` passed before editing. Read-only environment,
source and dependency inspections above ran; historical F2 gate results were read,
not rerun. No product build, model test, filesystem recovery qualification or
companion acceptance gate has passed this milestone. Post-edit diff checks and
the exact plan commit are reported in the owner handoff message.

**Blocked:** target path/module/executable selection; R2 additionally needs approved
F2 handling. Native ABI/linkage and hardware/model fit remain unverified runtime
gates, not reasons to weaken required semantics. **Next concrete slice:** F3's
separate module + read-only doctor after coordinator provisioning; no agent loop,
Goja, writes or TUI. Ownership remains with `companion-harness` until explicit
handoff; baseline and retirement workers retain their own code/artifacts.
