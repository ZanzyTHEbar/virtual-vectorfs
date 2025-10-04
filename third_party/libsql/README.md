# LibSQL C Header

## Provenance

This directory contains the vendored C header file for LibSQL's experimental C bindings.

**Source:** [tursodatabase/libsql](https://github.com/tursodatabase/libsql)  
**Version:** v0.9.23 (commit: `7eed898`)  
**File:** `bindings/c/include/libsql.h`  
**License:** MIT (see LibSQL repository)

## Why Vendored?

The `libsql.h` header is vendored here to enable:
- Offline builds without requiring Docker/network access
- Stable API reference for CGO bindings
- Version pinning for reproducible builds

## Updating

To update this header to a newer LibSQL version:

1. Build LibSQL with Docker:
   ```bash
   make build-libsql-amd64-full
   ```

2. Copy the generated header:
   ```bash
   cp build/artifacts/amd64/libsql.h third_party/libsql/include/
   ```

3. Update this README with new version/commit info

4. Commit the changes:
   ```bash
   git add third_party/libsql/
   git commit -m "chore(deps): update libsql.h to <new-version>"
   ```

## Build Integration

This header is referenced in CGO build flags:
```makefile
CGO_CFLAGS="-I$(PWD)/third_party/libsql/include"
```

The actual static library (`.a` files) are built separately via Docker and stored in `lib/`.

