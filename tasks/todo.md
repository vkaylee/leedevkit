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

- [x] Compose is normal startup orchestrator.
- [x] No direct Podman calls in normal `lifecycle_up`.
- [x] Build freshness, project isolation, and mode mapping tests pass.

## Phase 2 — Compose-owned teardown and identity

- [x] T3: Make teardown Compose-native and volume-safe.
  - [x] Use project-scoped `compose down --remove-orphans --timeout N`.
  - [x] Never pass `-v` or `--volumes` during normal teardown.
  - [x] Preserve attached-project and idempotent cleanup behavior.
  - [x] Remove engine-specific force deletion from current-project teardown.
- [x] T4: Use Compose identity for health and stale-run GC.
  - [x] Resolve services through `compose ps -q`/labels.
  - [x] Remove hardcoded container-name identity as primary path.
  - [x] GC inspects only unlocked LeeDevKit-owned Compose projects; deletion skips when age is unverifiable.
  - [x] Add Compose metadata and lock-safety tests.

## Checkpoint 2 — Lifecycle ownership

- [x] Compose owns normal up/down/health/dependency behavior.
- [x] LeeDevKit owns policy, lock, ownership, build decision, and reports.
- [x] Cleanup cannot delete unrelated containers or volumes.
- [x] Failure paths remain blocked/failed, never falsely passed.

## Phase 3 — Test orchestration and provider compatibility

- [x] T5: Integrate Compose-native lifecycle with `TestHandler` phases.
  - [x] Preserve setup-blocked, test-failed, teardown-always-runs behavior.
  - [x] Preserve JSON/JSONL phase fields, logs, rerun, and partial status.
  - [x] Consolidate redundant dependency startup where Compose owns it.
- [x] T6: Verify provider capability matrix and fallback boundaries.
  - [x] Test Docker default and legacy compatibility fallback.
  - [ ] Test Podman runtime; unavailable on current host.
  - [x] Reject unsupported custom-provider capabilities before mutation.
  - [x] Redact provider diagnostics.

## Checkpoint 3 — End-to-end regression

- [ ] `./leedevkit test all --json` passes or reports runtime limitation truthfully; wrapper timeout remains.
- [ ] `./leedevkit test all --json-stream --quiet` emits parseable JSONL only.
- [ ] Concurrent project isolation works.
- [ ] Shared cache volumes survive teardown.
- [ ] Docker verified; Podman unavailable on current host.
- [x] Direct Python suite: `959 passed, 1 skipped`.

## Go core migration

- [x] G1: Add stdlib-only Go CLI compatibility core for help/version/unknown commands.
  - [x] Add `go.mod`, `cmd/leedevkit/main.go`, and Go unit tests.
  - [x] Preserve current Python launcher as default.
  - [x] Verify inside `golang:1.26.11-bookworm` container because host lacks Go.
- [x] G2: Add Go provider resolution parity core.
  - [x] Preserve provider precedence, argv arrays, capabilities, and redacted diagnostics in `internal/provider`.
  - [x] Add table-style parity tests for defaults, overrides, invalid capability, and secrets.
- [x] G3: Add Go Compose command/lifecycle plan; keep Python lifecycle executor authoritative.
  - [x] Add native wait, project ownership, provider capability, and volume-safe teardown planning in `internal/lifecycle`.
  - [x] Keep actual Compose execution and cleanup in Python.
- [x] G4: Add deterministic Go quality gate planner; keep Python execution/reporting authoritative.
  - [x] Mirror existing Go defaults: gofmt, go-vet, go-test, go-coverage.
  - [x] Add deterministic selection and missing-gate failure tests.
- [x] G5: Add opt-in Go core bridge and safe Python fallback.
  - [x] Add versioned `--plan-gates` stdin/stdout JSON protocol.
  - [x] Keep Python default path; enabled bridge fails closed on missing binary, timeout, non-zero exit, malformed response, or bad schema.
  - [x] Restrict bridge environment to explicit safe allowlist.
- [ ] G6: Cut over safe commands after release acceptance.
  - [x] Extend release payload validation to include `go.mod` and `cmd/leedevkit/main.go`.
  - [ ] Build and ship platform Go binaries; launcher cutover remains disabled.

## Phase 4 — Cleanup and migration readiness

- [x] T7: Remove obsolete lifecycle code and document contract.
  - [x] Delete verified-dead `_compose_up.py` paths and obsolete profile force-kill map.
  - [x] Add ADR with migration and rollback details: `docs/architecture/adr-003-compose-native-lifecycle.md`.
  - [x] Document Compose ownership, provider capabilities, healthchecks, cleanup, and volume policy.
  - [x] Document Go-core boundary: Go supervises policy; Compose executes containers.

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
