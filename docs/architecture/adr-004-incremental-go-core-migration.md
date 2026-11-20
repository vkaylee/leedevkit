# ADR-004: Incremental Go Core Migration

- Status: accepted
- Date: 2026-11-20
- Owner: LeeDevKit maintainers

## Context

LeeDevKit is migrating selected deterministic policy from Python to Go. Python currently remains the control plane and owns TOML loading, orchestration, subprocess execution, and runtime mutation. A direct replacement would risk configuration drift and would duplicate behavior before parity is proven.

## Decision

Use versioned, opt-in shadow protocols before changing production ownership.

The first shadow boundary is normalized container-provider diagnostics:

- Python loads and resolves TOML configuration.
- Python sends normalized provider data to Go through `--resolve-provider`.
- Go validates provider name, argv, capabilities, and redacted diagnostics.
- Python compares Go diagnostics with its own result.
- `LEEDEVKIT_USE_GO_CORE=1` enables the shadow check; default behavior remains Python-only.
- Protocol failures and mismatches fail closed when the opt-in flag is enabled.

Go binary discovery uses explicit `binary_path`, `leedevkit-core` on `PATH`, or the platform-selected binary under `DEVKIT_HOME`.

## Alternatives

- Big-bang provider migration: rejected; TOML parser and merge behavior are not yet proven equivalent.
- Silent Go fallback on mismatch: rejected; it hides contract drift.
- Keep an unversioned subprocess interface: rejected; incompatible payloads would fail ambiguously.

## Consequences

Positive:

- Python remains backward-compatible by default.
- Provider parity can be measured before ownership changes.
- Go receives a narrow contract with deterministic output.
- Protocol tests cover malformed input, schema mismatch, validation, and redaction.

Negative:

- Shadow mode adds one Go process per provider resolution when enabled.
- Provider behavior is temporarily duplicated.
- The Go protocol currently validates normalized input; it does not parse TOML.

## Security and operational impact

Provider diagnostics redact secret-like keys and URL credentials before comparison. Commands remain argv-based; no shell interpolation is added. Shadow mode is disabled by default and fails closed when explicitly enabled, preventing an unverified Go result from silently changing provider behavior.

## Migration and rollback

1. Keep Python resolution as source of truth.
2. Enable shadow mode in CI and parity fixtures.
3. Resolve and fix all normalized-output mismatches.
4. Add Go ownership only after parity evidence covers repository, host, environment, CLI, default, malformed, and custom-provider cases.
5. Roll back by unsetting `LEEDEVKIT_USE_GO_CORE`; no project configuration or runtime state changes are required.

The transitional bridge may be removed after Go owns provider resolution and the Python parity suite is retained as a contract suite for one release cycle.

## Completion criteria

- Provider protocol schema is versioned and tested.
- Go and Python diagnostics match across supported provider fixtures.
- Go binary discovery works for every release platform.
- Production provider ownership switch has an explicit follow-up ADR or update to this ADR.
