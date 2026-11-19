# Python → Go Control-Plane Migration

## Foundation

- [ ] Define canonical Go command/config/runtime/report contracts.
- [ ] Add explicit `LEEDEVKIT_CORE=go|python` routing; Go errors fail closed.
- [ ] Add golden parity fixtures for argv, streams, exit codes, signals, JSON, and JSONL.
- [x] Checkpoint A: outside-checkout Go binary and runtime precedence pass.
  - [x] Explicit `DEVKIT_HOME` wins over project runtime.
  - [x] Quoted `vX.Y.Z` project pin matches runtime `X.Y.Z`.
  - [x] No-argument help, version aliases, and malformed gate JSON covered.
  - [x] Go tests pass in `golang:1.26.11-bookworm` container.
  - [x] Python Go-core/launcher tests pass.

## Test control plane

- [x] Migrate focused paths `test infra --lint-only` and `test infra --unit-only` to Go.
  - [x] Run runtime `ruff check`, `mypy`, and optional `shellcheck` through argv.
  - [x] Run pytest with production coverage and 80% gate through argv.
  - [x] Support validated `--pattern` filtering without full-coverage claims.
  - [x] Preserve subprocess exit code and stdout/stderr forwarding.
  - [x] Stop on first failed command.
- [ ] Migrate remaining `test infra` and focused test paths to Go.
- [ ] Preserve gate selection, process safety, logs, blocked/failed/skipped/partial semantics.
- [ ] Migrate full `test all` phase state machine and DB/migration Compose command paths.
- [ ] Checkpoint B: focused tests pass in Go mode.
- [ ] Checkpoint C: full JSON/JSONL, Compose integration, isolation, teardown pass.

## Compose command paths

- [ ] Migrate `dev up/down/ps/logs/exec` to Go Compose adapter.
- [ ] Keep `dev clean` explicit and destructive; never reuse for automatic cleanup.
- [ ] Verify no Go code duplicates Compose dependency, health, identity, or volume logic.

## Project state commands

- [ ] Migrate read-only `doctor`.
- [ ] Migrate transactional `doctor --fix` with rollback.
- [ ] Migrate `init` acquisition, extraction, venv, rules, harness, skills, and pinning.
- [ ] Migrate `update` staging, native binary activation, config sync, and rollback.
- [ ] Migrate transactional `skills` operations and harness sync.

## Remaining paths

- [ ] Migrate `run` and safe process execution.
- [ ] Migrate `db:*` and `migrate:*` Compose command builders.
- [ ] Migrate `debug` read-only bounded MCP path.
- [ ] Migrate `sync:api`, `fmt:infra`, `verify:infra`, generated, and coverage paths.
- [ ] Prove no normal Go command launches `_orchestrator.py`.

## Packaging and retirement

- [ ] Make release/install/bootstrap Go-first without requiring `_orchestrator.py`.
- [ ] Keep Python fallback through two tagged compatibility releases.
- [ ] Run full regression, release build/acceptance, local mirror, checksums, SBOM, and public release verification.
- [ ] Human approval before deleting Python handlers, bridge, fallback env, or legacy wrapper.
- [ ] Remove dead compatibility code in separate reviewed cleanup.
