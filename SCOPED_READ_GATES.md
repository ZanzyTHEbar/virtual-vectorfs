# Scoped native reads — experimental source gates

This is a reviewed source prototype, not production dependency adoption. Public
patch archival is authorized; distribution/maintenance and consumer privacy
acceptance remain separate decisions. See [prototype contract](docs/SCOPED_READ_PROTOTYPE.md)
and [patch reproduction](native/patches/README.md).

1. Keep registry FFF 0.11.0 pin and production lock unchanged. Default native
   builds use registry APIs; zero/default callers remain Ambient. RootOnly needs
   an explicit scratch path override AND the opt-in native prototype feature.
2. Strict provider dispatch must pin/check one coherent native provider before
   opening a handle, retaining its methods, free and destructor through close.
   Missing/mixed/disabled providers fail explicitly; never downgrade to Ambient.
3. Wait for a real request-specific native rescan ticket before reconciliation.
   Native watcher/explicit admissions share the scheduler; a predecessor cannot
   clear its reserved successor's busy flag or falsely complete its request.
4. Strict reported walk/entry/metadata errors fail before inventory replacement.
   Preserve prior inventory and stop reconciliation/pruning; restoring access
   permits retry. Ambient best effort stays compatible. No atomic-snapshot claim.
5. All ignores OFF includes hidden/sensitive ordinary in-root files. Require
   explicit authorization and stable ancestry AND indexed leaves. Reject symlink
   root ancestry before canonicalization. This is not a sandbox, hostile-writer,
   caller-authority, universal outside-root-read or mmap-warmup guarantee.
6. Verify real mixed providers, native watcher/request/successor barriers, public
   strict and default/old-native controls, direct-C root refusal and a real
   unreadable previously indexed subtree retaining documents AND vectors.
   No mock filesystem/scanner or random Go-generation completion oracle.
7. Run Rust fmt/tests/Clippy and vector Go focused/race/vet offline and serially
   in isolated output. Use bounded CPU/jobs/memory/time; no cache patch, canonical
   install, model/dependency download, changed assertions or hidden failures.
8. Preserve exact source/patch hashes and failed/successful evidence privately.
   Independently review actual fixes; sentinel-negative traces are not sandbox
   proof. Revalidate integrated main against normal installed native libraries.

Local validation and independent source review resolved provider, handoff and
incomplete-walk findings. This does not qualify Familiar R2 or choose adoption.
