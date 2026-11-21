# ADR-003: Compose-Native Container Lifecycle

- Status: accepted
- Date: 2026-11-19
- Owner: LeeDevKit maintainers

## Context

LeeDevKit lifecycle code previously split ownership between Compose and engine-specific runtime code. Startup used separate dependency calls and health polling; teardown force-removed Docker/Podman resources; a Podman-only scheduler duplicated Compose dependency semantics. This created provider drift, container-name assumptions, and cross-project cleanup risk.

## Decision

Compose/Compose-compatible CLI owns container lifecycle:

- service graph and dependency conditions
- service healthchecks and native readiness where provider declares `wait`
- service create/start/stop/remove
- project networks, labels, orphan cleanup, and Compose identity
- normal `ps`, logs, and exec operations

LeeDevKit remains policy supervisor:

- provider selection and required capability validation
- project naming, lock, and ownership
- profiles, selected services, and configured dependency allowlists
- image freshness, build locks, and build/no-build policy
- phase orchestration, timeout, reports, and rerun evidence
- stale-project eligibility and fail-safe cleanup policy

Normal teardown uses `compose down --remove-orphans --timeout 1` and never removes volumes. Providers without native `wait` retain bounded compatibility health checks. Legacy health targets resolve service IDs via `compose ps -q`; generated container names are fallback only.

The obsolete Podman scheduler (`scripts/_compose_up.py`) and its tests are removed after confirming no production callers remain.

## Alternatives

- Keep engine-specific lifecycle supervisor: rejected; duplicates Compose and blocks provider portability.
- Keep Podman-only dependency scheduler: rejected; creates a second dependency implementation.
- Big-bang Go rewrite: rejected; migration risk is higher than incremental boundary extraction.
- Delete configured dependency support: rejected; existing projects rely on `[test.dependencies]`.

## Consequences

Positive:

- One normal lifecycle implementation across supported providers.
- Compose v2 naming and project metadata work without hardcoded names.
- Shared cache volumes survive normal cleanup.
- Provider capability gaps fail before mutation.
- Go core can call a narrow Compose boundary later.

Negative:

- Compose implementations differ in `wait`, project listing, and structured output.
- Providers without native wait retain compatibility engine health checks.
- Compose project metadata does not expose portable creation age, so stale GC skips deletion when age cannot be verified.
- Docker and Podman runtime parity still requires provider-specific CI evidence.

## Security and operational impact

All provider/config values remain argv-based; no shell interpolation is added. Cleanup is project-scoped and lock-aware. Attached projects are not torn down by process exit. `-v`/`--volumes` is prohibited for normal lifecycle cleanup. Unsupported provider capabilities fail closed. Stale cleanup never performs engine-wide `ps`/`rm`.

## Migration

1. Route explicit `ContainerProviderConfig` through bootstrap and lifecycle commands.
2. Use Compose native `up --wait` where declared; retain bounded compatibility fallback.
3. Use Compose-native teardown and remove duplicate handler startup.
4. Resolve legacy health identity via `compose ps -q`.
5. Remove obsolete Podman scheduler after no-caller verification.
6. Continue Go migration with Go owning policy supervision and Compose executing containers.

## Rollback

Revert commits through `5b50196`, `2bfd075`, `ad2d0f3`, `3118804`, `904f68b`, `4a448fa`, `861205d`, `2a82b84`, and `c1c85a1` in reverse order, or pin the prior LeeDevKit release. Rollback must not use `compose down -v` and must preserve shared cache volumes. If a provider fails native readiness, use the compatibility fallback or prior release while preserving project isolation.

## Verification

- `go test ./...`
- Focused lifecycle/provider package tests
- `git diff --check`
- Runtime Docker/Podman matrix when providers are available
