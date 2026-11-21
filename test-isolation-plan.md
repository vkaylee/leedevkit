# Test Isolation Plan

Status: Completed for Go-only runtime

## Invariants

- Tests that initialize or synchronize projects use temporary roots.
- Tests never mutate tracked source checkout files.
- Runtime caches and project context reset between fixture cases.
- Generated files contain no temporary absolute paths.
- Customer database lifecycle remains outside LeeDevKit ownership.

## Verification

- Go package tests cover runtime, lifecycle, provider, process, report, and release behavior.
- Shell syntax checks cover launchers, installer/bootstrap, and release scripts.
- Release acceptance covers extracted offline commands, manifest, SBOM, checksum, local mirror bootstrap, and rollback.
