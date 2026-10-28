# Gates: F2 verified bounded expansion

OWNS: search/expand.go, search/expand_test.go, search/engine.go, search/types.go, docs/ARCHITECTURE.md, docs/QUICK_START.md, docs/LOCAL_AGENT_PLAN.md, GATES.md

Scope: Publish one hash-checked, size-bounded read of indexed file or caller-record fields without implying query-match provenance.

## Historical validation

The evidence below predates local integration. Its `/tmp/opencode/vvfs-native-prefix`
is no longer present; use the current validation environment below instead.

- [x] G1: F2 regressions verify live files, caller fields, stale/missing versions, UTF-8 windows, and actual source ranges
  CHECK: env PKG_CONFIG_PATH=/tmp/opencode/vvfs-native-prefix/lib/pkgconfig CGO_ENABLED=1 CGO_CFLAGS=-I/tmp/opencode/vvfs-native-prefix/include CGO_LDFLAGS='-L/tmp/opencode/vvfs-native-prefix/lib -Wl,-rpath,/tmp/opencode/vvfs-native-prefix/lib' LD_LIBRARY_PATH=/tmp/opencode/vvfs-native-prefix/lib go test -tags=vectors -count=1 -run '^TestExpand' ./search && printf 'F2_EXPAND_PASS\n'
  EXPECT: F2_EXPAND_PASS
  EVIDENCE: automatic-evidence=v1; definition-sha256=715368a0fb09341b4aacea2227533e34413c7942f6bcb988656d804b84078496; exit=0; EXPECT=matched; output-sha256=96fd41819616b13e1b7d80b4ff97b34f00f1f928af6eea8b8752aad8e8f510b1; output-bytes=74; shell=/bin/sh; cwd=/mnt/common/projects/go-pkgs/virtual-vectorfs; path=92e0b4eee05c/29 entries

- [x] G2: full vector-enabled Go race tests and vet succeed without introducing native changes
  CHECK: make test NATIVE_PREFIX=/tmp/opencode/vvfs-native-prefix && make vet NATIVE_PREFIX=/tmp/opencode/vvfs-native-prefix && printf 'F2_GO_PASS\n'
  EXPECT: F2_GO_PASS
  EVIDENCE: automatic-evidence=v1; definition-sha256=e0824219fbe98df52fec582072e3d44d8a7069f3884f81b4a80c15f74d37d1ff; exit=0; EXPECT=matched; output-sha256=c8aa884a65a52bf1c8e09b182b6ced174a2bb910f2c47f844c6efe41085093f9; output-bytes=36848; shell=/bin/sh; cwd=/mnt/common/projects/go-pkgs/virtual-vectorfs; path=92e0b4eee05c/29 entries

- [x] G3: changed Go files are formatted and tracked diff has no whitespace errors
  CHECK: git diff --check && test -z "$(gofmt -l search/expand.go search/expand_test.go search/engine.go search/types.go)" && printf 'F2_FORMAT_PASS\n'
  EXPECT: F2_FORMAT_PASS
  EVIDENCE: automatic-evidence=v1; definition-sha256=77afc1bf0919e55b1f05d312638b6e0f2255e1b4acdc96f5e519a6fd232d1300; exit=0; EXPECT=matched; output-sha256=ed1087e691d2fa881a12701fc55d1cc58f19857c31f96054af09e4c1727cc765; output-bytes=15; shell=/bin/sh; cwd=/mnt/common/projects/go-pkgs/virtual-vectorfs; path=92e0b4eee05c/29 entries

- [x] G4: public contract and changes receive correctness, security, and resource-bound review
  EVIDENCE: review-worktree ses_ed3727835ffew01HHXTgP15wYe inspected the final F2 worktree on 2026-10-11; no findings; non-atomic freshness and whole-field caller materialization are documented residual limits.

## Local integration validation — 2026-10-28

From the repository root, using existing native libraries without running Make's
build/install/download prerequisites:

```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly
export CARGO_NET_OFFLINE=true
export PKG_CONFIG_PATH="$PWD/build/native/lib/pkgconfig"
export CGO_ENABLED=1 CGO_CFLAGS="-I$PWD/build/native/include"
export CGO_LDFLAGS="-L$PWD/build/native/lib -Wl,-rpath,$PWD/build/native/lib"
export LD_LIBRARY_PATH="$PWD/build/native/lib"
export GOMAXPROCS=4 OPENBLAS_NUM_THREADS=1 OMP_NUM_THREADS=1
timeout 180s go test -tags=vectors -count=1 -run '^TestExpand' ./search
timeout 660s go test -tags=vectors -race -count=1 -timeout=10m ./search/... ./internal/...
timeout 180s go vet -tags=vectors ./search/... ./internal/...
timeout 180s go build -tags=vectors ./search/... ./internal/...
timeout 60s cargo fmt --manifest-path native/fffbridge/Cargo.toml -- --check
timeout 300s env CARGO_TARGET_DIR="$PWD/build/cargo" cargo test --offline --locked --manifest-path native/fffbridge/Cargo.toml
timeout 300s env CARGO_TARGET_DIR="$PWD/build/cargo" cargo clippy --offline --locked --manifest-path native/fffbridge/Cargo.toml --all-targets -- -D warnings
git diff --check
test -z "$(gofmt -l search/*.go internal/fffbridge/*.go)"
```

All commands exited 0. Focused tests: 1.884 s; race suite: search 24.369 s,
bridge 2.045 s; Rust: 10 passed, 0 failed. Go 1.26.8, Cargo 1.98.1,
Linux amd64. Review found no blocking correctness/security issue; non-atomic
freshness, non-preemptible I/O, and whole-field caller materialization remain
limits, not stronger guarantees. No production code or test assertions were
changed during integration. This is local validation, not remote publication,
native sanitizer evidence, or downstream companion qualification.
