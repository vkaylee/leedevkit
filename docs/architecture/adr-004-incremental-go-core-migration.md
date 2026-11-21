# ADR-004: Incremental Go Core Migration

- Status: superseded
- Date: 2026-11-20
- Updated: 2026-11-21
- Owner: LeeDevKit maintainers

## Context

LeeDevKit completed migration of control-plane ownership to Go. This record documents the superseded shadow-migration design and its rollback evidence; it is not an active runtime contract.

## Decision

Retain this ADR as historical migration evidence. Current runtime behavior is defined by Go implementation, Go tests, and the active release/packaging rules.

The former shadow boundaries covered provider diagnostics, target expansion, and service-mode expansion. Those boundaries are now Go-owned and execute directly through the Go CLI. No shadow flag or alternate control-plane runtime remains.

Go binary discovery uses explicit `binary_path`, `leedevkit-core` on `PATH`, or the platform-selected binary under `DEVKIT_HOME`.

## Alternatives

- Big-bang provider migration: rejected; TOML parser and merge behavior are not yet proven equivalent.
- Silent Go fallback on mismatch: rejected; it hides contract drift.
- Keep an unversioned subprocess interface: rejected; incompatible payloads would fail ambiguously.

## Consequences

Positive:

- Go owns runtime policy, TOML loading, lifecycle execution, and diagnostics.
- One control-plane implementation removes dispatch drift.
- Protocol tests cover malformed input, schema mismatch, validation, and redaction.

Negative:

- Historical parity fixtures remain useful maintenance evidence but no longer provide cross-runtime comparison.

## Security and operational impact

Provider diagnostics redact secret-like keys and URL credentials. Commands remain argv-based; no shell interpolation is added. Unsupported or malformed input fails closed before runtime mutation.

## Migration and rollback

1. Keep Go resolution and execution as source of truth.
2. Retain Go conformance fixtures for repository, host, environment, CLI, default, malformed, and custom-provider cases.
3. Roll back through the prior tagged Go release when release acceptance requires it.

The alternate control-plane bridge and source were removed after Go ownership and release gates passed.

## Completion criteria

- Provider, test-target, and mode protocols are versioned and tested.
- Go diagnostics, target expansion, and mode maps match supported fixtures.
- Go binary discovery works for every release platform.
- Production Go ownership is recorded by this superseding update.
