# Optional FFF RootOnly source patch

Normal builds use registry `fff-search` 0.11.0 and `NativeReadAmbient`.
`NativeReadRootOnly` is experimental and unavailable in those builds: opening it
returns an error rather than falling back to Ambient.

`fff-search-0.11.0-root-only-v3.patch` targets the published crates.io
`fff-search` 0.11.0 package, not an upstream checkout. To use it, apply the patch
to a separate copy of that package, configure a Cargo `[patch.crates-io]` path
override to the patched copy, and build the bridge with its
`scoped-reads-prototype` feature. The feature alone does not compile against the
unpatched registry package. Use a separate native prefix and configure the Go
linker environment as described in the [README](../../README.md); do not replace
an existing installation unintentionally. The normal Makefile does not select
this patch or feature.

## Behavior and restrictions

- RootOnly disables ignore interpretation, including `.gitignore` and `.ignore`,
  and includes hidden files. `.git` entries and symlinks are excluded from its
  inventory. Explicitly authorize all ordinary in-root content, including ignored
  credentials and private exports, before selecting this policy. Do not reuse an
  Ambient index assuming the same privacy behavior.
- Keep root ancestry, mount identity, and indexed leaves stable during calls.
  Root ancestry is checked for symlinks, but directory identities are not pinned
  and ordinary native opens lack kernel no-follow protection. Replacement races
  can escape the intended scope; hardlink-origin and secret detection are absent.
- This is not an OS sandbox, hostile-writer defense, or process-wide read boundary.
  Index/evidence locations and runtime facilities have separate access. The caller
  remains responsible for authorization. Even caller-record operations start FFF
  on the supplied root.
- RootOnly synchronization requests a native scan ticket and waits for completion
  before reconciling inventory. Reported traversal/entry-metadata failures reject
  the scan before inventory replacement; synchronization fails before pruning
  previous documents/vectors. This is not an atomic filesystem snapshot or a hard
  filesystem-I/O deadline.
- Strict opening requires the matching native provider and extensions. Missing,
  mixed, or feature-disabled providers are rejected rather than downgraded.

These are source-level restrictions, not a security certification or a guarantee
of compatibility with another native installation.

## Source identity and license

- Registry package checksum (also in `native/fffbridge/Cargo.lock`):
  `a80ed5bdfa89cc81cb314374a4460da880d68e4c81d9574a587f28757a9c943e`.
- Patch SHA-256:
  `83be0ad1c4e35853dd478a489992bdc40d12189b7eb21354cfc1e0e738472e9e`.

The upstream MIT copyright and permission notice is retained in
[fff-search.LICENSE](fff-search.LICENSE). Repository modifications are covered
by the root [MIT license](../../LICENSE). Retain both notices when distributing
the patched source.
