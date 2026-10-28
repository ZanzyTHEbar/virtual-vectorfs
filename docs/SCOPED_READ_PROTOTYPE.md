# Scoped native reads — local prototype, not dependency adoption

**Reviewed experimental source:** local real-native validation and independent
source re-review resolved provider ownership, successor handoff and incomplete-
walk/pruning findings. The exact [FFF patch and reproduction instructions](../native/patches/README.md)
are archived as source, not adopted as a production dependency. No Familiar R2
clearance, distribution strategy or consumer privacy acceptance is implied.

`Options.NativeReadPolicy = NativeReadRootOnly` explicitly selects rooted FFF
inventory/grep behavior. Zero/default (`NativeReadAmbient`) preserves existing
Git discovery/status and ignore behavior. No existing grant is silently changed.

RootOnly is **not available in normal native builds**. The production dependency
still pins registry `fff-search = 0.11.0`, which lacks this policy. A local native
build must explicitly enable `scoped-reads-prototype` and use a scratch-only Cargo
override to the independently reviewed local FFF source. No fork/vendor strategy
or production pin change has been selected. Public patch archival does not select
dependency adoption. Default builds use only
the registry API and explicitly reject RootOnly. Enabling the feature against the
unpatched registry dependency is unsupported and will not compile.

## Caller contract

- ALL ignore interpretation is OFF, including root-local `.ignore/.gitignore`;
  hidden files are included and ignore files are ordinary searchable content.
  `.git` entries and symlinks are excluded from FFF inventory.
- Explicitly authorize all ordinary in-root content, including ignored secrets,
  `.env` files and private exports. Familiar must acknowledge these semantics
  before any integration; this prototype does not clear R2. Use a fresh scoped
  index and do not silently reuse Ambient grants/privacy expectations.
- Keep ancestry, mount identity AND indexed leaves stable during calls. RootOnly
  root normalization checks for symlink ancestors before native startup; direct
  C RootOnly opens also check top-down before canonicalization. Neither path
  pins directory identities or stops replacement races. Ordinary native
  opens lack kernel no-follow; replacing an indexed leaf can escape the policy
  absent stability. Hardlink-origin/secret detection is not provided.
- This is not an OS sandbox, hostile-same-UID guarantee, or capability boundary
  against the Rust/Go caller's public mutation/API authority. Index/evidence
  locations, runtime libraries/proc/sys and separately opened facilities retain
  their own authority. Zero synthetic sentinel reads is not zero process-wide
  outside-root reads. No network-denial or mmap-use/warmup proof is implied.
- Caller-record operations still start FFF on the supplied root. An empty granted
  synthetic root can be used, but this is not a record-only/no-native mode.

## ABI and lifecycle

The original `fffb_open` and handle ABI remain Ambient. Before constructing a
strict handle, Go's C dispatch identifies the actual bound legacy-open provider,
verifies that all bound handle methods/free/destructor share it, pins that DSO,
and resolves `fffb_open_root_only_v1`, `fffb_request_rescan_v2` and
`fffb_wait_rescan_v2` ONLY from that provider. Extension addresses must belong to
the same DSO. Validated pointers are retained through error-string disposal and
handle destruction; no independent global lookup is performed on later calls.
Missing/mixed providers or a feature-disabled implementation fail explicitly,
never falling back to Ambient. Default callers keep the old linked ABI without
new hard undefined symbols. Symbol/provider checks are not binary authentication
or protection against a malicious in-process caller. Positional Go Options
literals require updating for the additive field; keyed/default callers retain
their behavior.

Policy is set before FFF scan startup and retained by the reviewed FFF patch
through scans/watch updates. RootOnly `SyncFiles` first waits for startup, requests
a native FFF rescan ticket, waits for that request's successful completion, then
reconciles the paged inventory to Bleve. Generic idle flags and Go generation IDs
do not fence an explicit request. The isolated FFF fix serializes scan admission
for native watcher and explicit requests, coalesces pending requests into a
successor with a request cutoff, and keeps busy continuously through the handoff.
A predecessor cannot clear its successor's signal. Failure/cancel/replacement
does not become successful ticket completion. The retained v1 rescan ABI now
returns an explicit unsupported error rather than an unfenced success.
Ambient `SyncFiles` retains its existing inventory-only reconciliation.

RootOnly ripgrep traversal is complete-or-error: any reported traversal or
entry-metadata error rejects that scan before inventory replacement. Its native
ticket fails, so `SyncFiles` returns before reconciliation/pruning and retains
previous documents/vectors. Restoring access permits a new request/retry.
Ambient retains best-effort walking. This is not an atomic filesystem snapshot,
nor a guarantee that every indexed file remains readable after the walk.
Neither is an atomic snapshot or a preemptible filesystem-I/O guarantee. Close
retains the existing Go/cgo/Rust exclusion, subscription and cancellation lifecycle.

## Reproducibility boundary

Validation uses a new private copy of bridge sources with a scratch manifest
override, isolated Cargo target and native prefix. The Go package is built from
the reviewed worktree sources using that prefix, offline module resolution and real
FFF/Bleve/FAISS. The ordinary/default-feature bridge is also built separately
against the untouched registry dependency; an old native prefix provides the
missing-symbol fail-closed/legacy compatibility control. No Make download/install
prerequisites or canonical native overwrites are needed.

`SCOPED_READ_GATES.md` defines this wave's gates. Private records bind exact source,
override/lock, binary identities and command exits; they do not turn a local
override into an adopted dependency. Next decisions remain distribution/maintenance
ownership and explicit consumer privacy acceptance, followed by integration review.
