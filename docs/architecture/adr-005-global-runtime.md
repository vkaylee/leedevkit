# ADR-005: Global LeeDevKit Runtime

- Status: accepted
- Date: 2026-11-22
- Owner: LeeDevKit maintainers

## Context

Projects currently install their own runtime and launcher while an optional global command is also available. This duplicates payloads and makes invocation and updates ambiguous. The product accepts differing LeeDevKit versions across machines and CI environments.

## Decision

Install one active LeeDevKit release per user environment and invoke `leedevkit` through PATH. The global installation owns executable and bundled assets; the current working directory selects project configuration and project-owned state. `init` creates project integration without installing a runtime. `update` updates the global installation, not a project. Projects do not pin the runtime version.

`leedevkit.lock` records community skill revisions and remains project-owned. Existing user data, including project skills, configuration, launchers and local runtime files, is not deleted automatically. A migration must preserve or explicitly relocate project-owned assets and warn about obsolete launchers. CI installs the global tool independently and may use a different version from developer machines.

## Alternatives

- Global command plus per-project runtime: rejected; retains duplicate installation and ambiguous version selection.
- Global-only without a skill lockfile: rejected; the skill lock is unrelated to runtime versioning.
- Global runtime version manager: rejected; project-specific version pinning is not required.

## Consequences and security

A global update can change behavior in every project; release notes and recovery instructions must make this visible. Release artifacts retain immutable versions, integrity checks and available provenance evidence. As ADR-001 states, checksums alone do not authenticate a publisher. Downloads and updates must not activate incomplete or integrity-invalid artifacts, and failure must leave the previously active installation usable. Project-specific configuration and skills remain isolated.

## Migration and rollback

Inventory project-local runtime consumers and provide explicit migration instructions. Preserve existing project files, including custom skills and `.venv`, until their ownership and safe migration are verified. Update templates, CI, documentation, and packaging gates alongside the executable. A breaking release communicates the change; do not claim migration of external projects without evidence. On failure, restore the previous global release and project files from backup or version control. Verify rollback on representative isolated projects before release.

## Verification

Exercise fresh install, existing project migration, two projects sharing a global installation, nested working directories, skill ownership, update failure and rollback, offline execution after install, and release acceptance. The configured full regression and packaging gates apply.
