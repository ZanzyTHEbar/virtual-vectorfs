# Product checkpoint — 2026-10-28

## Goal

Build a local personal-agent product, Familiar, on a reusable search library.
Virtual VectorFS owns search; Familiar owns orchestration, model calls, authority,
event records, citations and any future write/recovery workflow. A frontend should
call library APIs, not reimplement orchestration. This checkpoint covers only
Virtual VectorFS; Familiar's owner supplies its separate publication/status.

## Where we are

- **Search foundation integrated:** FFF inventory/path/grep/watch plus Bleve
  keyword/vector/hybrid search. Drivers provide embeddings; the library does not
  run models. Local indexing is derived state, not authoritative source content.
- **F2 integrated:** bounded, version-checked expansion distinguishes previews
  from verified source windows and rejects stale/missing content. Vector updates
  require matching content hashes. Fresh integrated-main checks are a publication
  gate; native installation on another machine remains that machine's prerequisite.
- **Scoped native prototype reviewed:** opt-in RootOnly source and additive ABI,
  coherent provider lifetime, request-specific completion and incomplete-walk
  failure before pruning. Real mixed-provider, handoff and permission-denial
  acceptance passed. The full MIT-attributed [FFF patch](../native/patches/README.md)
  is archived publicly, with no production override or dependency adoption.
- **History consolidated:** previous companion handoff and LibSQL-retirement
  branches are already integrated. Their historical records are not proof of
  current Familiar behavior; LibSQL is not the search architecture.

## Remaining gates

Normal native builds still use registry FFF 0.11.0 and Ambient policy. RootOnly
requires an explicitly patched scratch build; enabling the feature alone against
the registry does not work. Distribution/upstreaming/maintenance ownership and
explicit all-ignores-OFF/hidden-file privacy acceptance are undecided. Familiar
R2 is **not cleared** by this source publication.

RootOnly is not an OS sandbox or atomic filesystem snapshot. Stable ancestry AND
indexed leaves are required; caller authority and separately opened state remain
outside it. Zero classified sentinel accesses is not universal read confinement,
hostile-replacement safety or mmap-warmup proof.

The preserved [repository directives](../.NOTE.md) require faults-go error
integration, dbbootstrap database integration, and ports/adapters at core I/O
boundaries. These are requirements for separately scoped follow-up, **not claims
that those migrations were implemented during prototype publication**.

## Best next step

The coordinator combines this search checkpoint with Familiar's own verified
checkpoint, then chooses the FFF distribution/maintenance and consumer privacy
contract before any RootOnly adoption or R2 qualification. No model, companion
completion, write authority or runtime-safety milestone is inferred here.
