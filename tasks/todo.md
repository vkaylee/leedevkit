# TODO: Generic configurable Go quality gates

Plan: [plan.md](plan.md). Implementation not started.

## Phase 1 — Config and Go execution

- [x] T1: Define normalized quality-gate schema, validation, compatibility defaults, and config tests.
- [x] T2: Add Go race/integration configured execution, missing-tool status, lifecycle setup/teardown, and runner tests. Core path complete; integration setup/tool-status extensions remain.
- [x] Checkpoint 1: Review schema, Go command evidence, no-task behavior, and Rust/TypeScript regression.

## Phase 2 — Generic dispatch and reporting

- [x] T3: Dispatch configured gates through existing phases, lifecycle, runner, logs, timeout, and required/optional status. Gate command/task evidence now appears in phase reports.
- [x] T4: Add gate-level JSON/JSONL evidence and safe base-ref/new-findings metadata. Base-ref resolution is implemented; per-tool new-findings execution remains limited.
- [x] Checkpoint 2: Run and parse stable JSON plus JSONL; verify no false green.

## Phase 3 — Hooks, validation, docs

- [x] T5: Add generic generated-code freshness hook with check/fix separation. Generic `[quality.generated]` gate and explicit `fix_command` now supported.
- [x] T6: Add expected tool-version validation and fail-closed false-green tests. Seeded workspace verification remains explicitly deferred.
- [x] Checkpoint 3: Adversarial review for injection, secrets, path safety, status truthfulness, and cleanup.
- [x] T7: Update README migration/config notes; full configured regression passed.

Current limitation: seeded workspace verification now copies into isolated temporary workspace, proves seeded fail then clean pass, and cleans up. New-findings metadata records mode/skip reason; environment allowlists pass through Compose argv; integration setup/teardown commands run around Go integration gates.

## Test-impact matrix

- [ ] Config: missing section, old config, wrong type, empty command, bad path, duplicate gate.
- [ ] Execution: success, non-zero, missing binary, timeout, cancellation, signal, large stderr, dangerous input.
- [ ] Status: pass, required fail, optional fail, setup blocked, skipped, no task, mixed result.
- [ ] Go: format, vet, unit, race, staticcheck, golangci-lint, govulncheck, mod verify, integration.
- [ ] Reporting: JSON, JSONL, quiet, logs, rerun, full regression.
- [ ] Lifecycle: build/no-build, cleanup after success/failure, dependency readiness.
- [ ] Compatibility: Rust, TypeScript, existing Go config, existing CLI flags.

## Verification commands

- [ ] `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_devkit_config.py -q`
- [ ] Focused Go/runner/handler/lifecycle pytest suites.
- [ ] `./leedevkit --help`
- [ ] `./leedevkit test --help`
- [ ] `./leedevkit test infra --lint-only`
- [ ] `./leedevkit test infra --unit-only`
- [ ] `./leedevkit test all --json`
- [ ] `./leedevkit test all --json-stream --quiet`
- [ ] `ruff format --check scripts`
- [ ] `ruff check scripts`
- [ ] `mypy scripts`
- [ ] Shell syntax checks for existing wrappers.
- [ ] Release build/acceptance only if packaging/install/bootstrap/release files change.

## Constraints

- [ ] No LeeAttend-specific service/path/tool/database/migration/image/version hardcoding.
- [ ] No host Go/npm/bun/cargo/Docker execution when project wrapper exists.
- [ ] No unsafe shell string from TOML/CLI; preserve exit code/stderr/timeout/signal.
- [ ] No required failed/blocked/skipped/no-task gate reported as passed.
- [ ] No commit until requested.
