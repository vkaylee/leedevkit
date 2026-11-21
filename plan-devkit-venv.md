# Runtime packaging history

Status: Superseded by Go-only runtime migration

This document records retired interpreter-based packaging decisions. Current LeeDevKit runtime uses Go binaries and does not create or manage interpreter environments.

## Retained invariants

- Project runtime remains pinned and transactionally activated.
- Runtime state preserves user-owned skills, configuration, and rollback data.
- Release artifacts include manifest, SBOM, checksum, and platform binaries.
- Runtime and test lifecycle remain isolated from customer project state.

## Current verification

Use Go package tests, shell syntax checks, release build, release acceptance, archive inspection, and local-mirror bootstrap rollback checks. Do not restore retired interpreter tooling.
