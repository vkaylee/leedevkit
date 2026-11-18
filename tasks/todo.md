# TODO: Compose-Native Lifecycle Migration

Plan: [plan.md](plan.md)

## Phase 1 — Provider-neutral Compose boundary

- [x] T1: Define Compose lifecycle command boundary.
  - [x] Wire resolved `ContainerProviderConfig` into bootstrap/lifecycle command construction.
  - [x] Preserve project, file, profile, and argv ordering.
  - [x] Add provider capability validation and exact-argv tests.
- [x] T2: Move dependency ordering and startup readiness into Compose for providers declaring `wait`.
  - [x] Use Compose-native `up --wait --wait-timeout 120` and one startup call.
  - [x] Preserve bounded compatibility fallback for providers without native wait.
  - [x] Preserve build freshness, `skip_build`, and phase-build semantics.
  - [x] Remove normal direct Podman dev startup branch.

## Checkpoint 1 — Startup contract

- [ ] Compose is normal startup orchestrator.
- [ ] No direct Podman calls in normal `lifecycle_up`.
- [ ] Build freshness, project isolation, and mode mapping tests pass.

## Phase 2 — Compose-owned teardown and identity

- [x] T3: Make teardown Compose-native and volume-safe.
  - [x] Use project-scoped `compose down --remove-orphans --timeout N`.
  - [x] Never pass `-v` or `--volumes` during normal teardown.
  - [x] Preserve attached-project and idempotent cleanup behavior.
  - [x] Remove engine-specific force deletion from current-project teardown.
- [ ] T4: Use Compose identity for health and stale-run GC.
  - [ ] Resolve services through `compose ps -q`/labels.
  - [ ] Remove hardcoded container-name identity as primary path.
  - [ ] GC only stale, unlocked, LeeDevKit-owned projects.
  - [ ] Add cross-project and shared-volume safety tests.

## Checkpoint 2 — Lifecycle ownership

- [ ] Compose owns normal up/down/health/dependency behavior.
- [ ] LeeDevKit owns policy, lock, ownership, build decision, and reports.
- [ ] Cleanup cannot delete unrelated containers or volumes.
- [ ] Failure paths remain blocked/failed, never falsely passed.

## Phase 3 — Test orchestration and provider compatibility

- [ ] T5: Integrate Compose-native lifecycle with `TestHandler` phases.
  - [ ] Preserve setup-blocked, test-failed, teardown-always-runs behavior.
  - [ ] Preserve JSON/JSONL phase fields, logs, rerun, and partial status.
  - [ ] Consolidate redundant dependency startup where Compose owns it.
- [ ] T6: Verify provider capability matrix and fallback boundaries.
  - [ ] Test Docker default.
  - [ ] Test Podman without normal Podman-only lifecycle code.
  - [ ] Reject unsupported custom-provider capabilities before mutation.
  - [ ] Redact provider diagnostics.

## Checkpoint 3 — End-to-end regression

- [ ] `./leedevkit test all --json` passes or reports runtime limitation truthfully.
- [ ] `./leedevkit test all --json-stream --quiet` emits parseable JSONL only.
- [ ] Concurrent project isolation works.
- [ ] Shared cache volumes survive teardown.
- [ ] Docker and Podman behavior verified separately where available.

## Phase 4 — Cleanup and migration readiness

- [ ] T7: Remove obsolete lifecycle code and document contract.
  - [ ] Delete verified-dead `_compose_up.py` paths and hardcoded-name fallbacks only after parity tests pass.
  - [ ] Add/update ADR with migration and rollback details.
  - [ ] Document Compose ownership, provider capabilities, healthchecks, cleanup, and volume policy.
  - [ ] Document Go-core boundary: Go supervises policy; Compose executes containers.

## Final verification

- [ ] `./leedevkit --help`
- [ ] `./leedevkit test --help`
- [ ] `./leedevkit test infra --lint-only`
- [ ] `./leedevkit test infra --unit-only`
- [ ] `./leedevkit test all --json`
- [ ] `./leedevkit test all --json-stream --quiet`
- [ ] `ruff format --check scripts`
- [ ] `ruff check scripts`
- [ ] `mypy scripts`
- [ ] `git diff --check`
- [ ] Release build/acceptance if packaging/runtime files change.

## Constraints

- [ ] No direct engine command in generic lifecycle path.
- [ ] No `compose down -v`/`--volumes` for normal cleanup.
- [ ] No shell interpolation for provider/config values.
- [ ] No teardown of attached/external projects.
- [ ] No broad stale-container deletion without ownership, lock, project, and age checks.
- [ ] No required failed/blocked phase reported as passed.
- [ ] No commit until explicitly requested.
