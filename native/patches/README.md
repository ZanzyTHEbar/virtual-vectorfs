# Experimental FFF scoped-read patch

`fff-search-0.11.0-root-only-v3.patch` is the full reviewed source delta against
the **published crates.io `fff-search` 0.11.0 package**, not an upstream checkout.
It archives a prototype; it does **not** vendor, fork, select a distribution
strategy, change the production pin, or enable RootOnly in normal builds.

## Provenance and license

- Package: `fff-search` 0.11.0 by Dmitriy Kovalenko, declared MIT in its manifest.
- Registry package checksum (also in the bridge's unchanged `Cargo.lock`):
  `a80ed5bdfa89cc81cb314374a4460da880d68e4c81d9574a587f28757a9c943e`.
- Packaged VCS metadata: upstream commit
  `95fd777c2529fc7b4d7572dabff64cc07268f2c5`, path `crates/fff-core` in
  [dmtrKovalenko/fff.nvim](https://github.com/dmtrKovalenko/fff.nvim).
- Upstream copyright/permission notice is reproduced in
  [fff-search.LICENSE](fff-search.LICENSE), inspected at that exact commit's
  [LICENSE](https://github.com/dmtrKovalenko/fff.nvim/blob/95fd777c2529fc7b4d7572dabff64cc07268f2c5/LICENSE).
  Virtual VectorFS modifications are covered by this repository's MIT license;
  retain both notices when distributing the patched source.
- Exact patch SHA-256:
  `83be0ad1c4e35853dd478a489992bdc40d12189b7eb21354cfc1e0e738472e9e`.

The patch adds opt-in RootOnly policy, request-specific native scan tickets,
continuous successor-handoff signaling, and complete-or-error strict traversal.
Independent source reviews resolved provider coherence, handoff and incomplete-
walk/pruning findings. Real native/public acceptance passed. This is not an
upstream release or a guarantee of atomic filesystem snapshots or sandboxing.
Read the [caller contract](../../docs/SCOPED_READ_PROTOTYPE.md) before enabling it.

## Reproduce without changing the production dependency

Prerequisites: an already cached copy of the exact registry package and its
dependencies, Rust toolchain, `patch`, and the repository's normal native build
requirements. These commands deliberately run offline; stop if caches are missing.
Never edit Cargo's shared registry cache or install this over a normal native prefix.

From the repository root, set `FFF_REGISTRY_SOURCE` to that **unmodified cached
package directory** and `SCRATCH` to a new, empty, private writable directory:

```sh
CHECKOUT="$PWD"
test -d "$FFF_REGISTRY_SOURCE"
test -d "$SCRATCH"
test -z "$(ls -A "$SCRATCH")"
sha256sum native/patches/fff-search-0.11.0-root-only-v3.patch
cp -R "$FFF_REGISTRY_SOURCE" "$SCRATCH/fff-search"
chmod -R u+w "$SCRATCH/fff-search"
patch --batch --fuzz=0 -d "$SCRATCH/fff-search" -p1 \
  -i "$CHECKOUT/native/patches/fff-search-0.11.0-root-only-v3.patch"
cp -R native/fffbridge "$SCRATCH/fffbridge"
cat >> "$SCRATCH/fffbridge/Cargo.toml" <<'EOF'

[patch.crates-io]
fff-search = { path = "../fff-search" }
EOF
export CARGO_NET_OFFLINE=true CARGO_BUILD_JOBS=1
export CARGO_PROFILE_DEV_DEBUG=0 CARGO_PROFILE_TEST_DEBUG=0
export CARGO_TARGET_DIR="$SCRATCH/target"
# Only this scratch lock changes for the path override.
cargo build --offline --manifest-path "$SCRATCH/fffbridge/Cargo.toml" \
  --features scoped-reads-prototype
cargo test --offline --locked --manifest-path "$SCRATCH/fffbridge/Cargo.toml" \
  --features scoped-reads-prototype -- --test-threads=1
cargo clippy --offline --locked --manifest-path "$SCRATCH/fffbridge/Cargo.toml" \
  --features scoped-reads-prototype --all-targets -- -D warnings
```

Enabling the Cargo feature against unpatched registry FFF is unsupported and will
not compile. The root repository manifest/lock must stay unchanged. For a private
Go acceptance prefix, copy the built `target/debug/libfffbridge.so`, this checkout's
bridge header/pkg-config file, and existing compatible FAISS headers/libraries into
a **new** prefix's `lib`, `include`, and `lib/pkgconfig` directories. Set
`PKG_CONFIG_PATH`, `CGO_CFLAGS`, `CGO_LDFLAGS`, and `LD_LIBRARY_PATH` to that prefix,
as in the normal README. Never overwrite canonical native artifacts.

The tagged public acceptance can exercise the unreadable-subtree contract with a
fresh synthetic root containing `kept/saved.txt` whose text includes
`retainedneedle`, and state outside the root:

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local CGO_ENABLED=1 \
  go test -p=1 -tags=vectors,scoped_native_acceptance -c \
  -o "$SCRATCH/scoped-acceptance" ./search
SCOPED_ACCEPTANCE_ROOT="$SYNTHETIC_ROOT" \
SCOPED_ACCEPTANCE_STATE="$SYNTHETIC_STATE" \
SCOPED_ACCEPTANCE_MODE=strict-incomplete \
  "$SCRATCH/scoped-acceptance" -test.run='^TestNativeReadPolicyAcceptance$' -test.v
```

Use a non-root user with real permission enforcement and private HOME/XDG/Git
config, bound CPU/memory/time, and fresh fixtures for each run. The scenario indexes
and vectors a file, makes its subtree unreadable, requires native ticket failure
and preserved inventory/documents/vectors, restores permissions, verifies stable
identities and retries. It fails rather than mocking success if chmod does not
cause an actual permission denial. Other tagged modes require their matching
synthetic ignore/Git/symlink/provider fixtures; unset inputs are never inferred
from personal directories. Build outputs, fixtures and evidence are not source
deliverables in this patch directory.
