# LeeDevKit — Technical Debt Remediation Register

Status: Superseded by Go-only migration

## Current state

LeeDevKit control-plane ownership is Go-only. Release acquisition, safe archive extraction, transactional activation, skills, manifest verification, SBOM generation, checksums, and artifact acceptance use Go and shell paths.

Customer repositories own database schemas, migrations, seed data, and test database lifecycle. LeeDevKit does not create or seed customer databases; `db:setup` remains removed.

## Completed controls

- Go tests cover process, lifecycle, provider, gates, reports, runtime transactions, skills, debug, and release manifest behavior.
- Download and archive paths enforce bounded operations, safe extraction, version validation, staging, activation, and rollback.
- Skill updates use bounded Git operations, atomic lock updates, and rollback on failure.
- Release output includes deterministic manifest, CycloneDX SBOM, checksum, and platform binaries.
- Unsupported commands fail closed; no alternate control-plane runtime remains.

## Verification

Run configured Go tests, shell syntax checks, `git diff --check`, release build, release acceptance, archive inspection, and local-mirror bootstrap rollback checks before packaging changes are released.

## Follow-up

Add only new Go behavior or project-owned checks. Do not restore retired runtime tooling or dependency management.
