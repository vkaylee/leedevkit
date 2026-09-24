# v0.7.14 — Verification, Acquisition, and Supply-Chain Hardening

## Highlights

- Fixed verifier and checklist applicability semantics for CLI projects.
- Added truthful skip/failure handling, including `--no-e2e` and zero-check protection.
- Fixed infra phase selection and split production/test-source coverage reporting.
- Added bounded downloads, safe archive validation, version checks, and transactional rollback for install, bootstrap, update, and project init.
- Added transactional skill installation/update with exact lock-pin enforcement and rollback.
- Added timeout process-tree ownership and descendant cleanup.
- Added pinned runtime dependencies, lazy Playwright installation, CycloneDX SBOM generation, supply-chain evidence, and artifact checksums.
- Added artifact authenticity/provenance ADR. Current release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

## Verification

- `./leedevkit test infra`: 831 passed, 1 skipped; 83.87% production coverage.
- `python3 scripts/_release_acceptance.py --repo-root .`: passed.
- `python3 .agent/scripts/checklist.py .`: passed; inapplicable checks reported as skips.
- `python3 .agent/scripts/verify_all.py . --no-e2e`: passed; inapplicable checks reported as skips.
- Shell syntax and `git diff --check`: passed.

## Upgrade

```bash
./leedevkit update --version v0.7.14
```

## Rollback

```bash
./leedevkit update --version v0.7.13
```

## Known limitations

- One existing lifecycle test remains skipped.
- Release artifacts are not publisher-signed. See `docs/architecture/adr-001-artifact-authenticity.md` before adopting a trust anchor.
