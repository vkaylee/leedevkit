# Implementation Plan: Generic configurable Go quality gates

## Context

LeeDevKit has fixed phase orchestration, Compose lifecycle, timeout/process cleanup, logs, and JSON/JSONL reports. Go support currently covers fixed `gofmt`, `go vet`, `go test`, and coverage paths; `int-go` maps to lifecycle but schedules no Go integration task. Config validates only build policy and lifecycle dependencies. Downstream LeeAttend currently carries project-specific Go gates that should become generic LeeDevKit capabilities without LeeAttend names, paths, credentials, migration layout, images, or tool versions.

Planning inspected `CLAUDE.md`, `templates/CLAUDE.base.md`, relevant `.agent/rules/*`, CLI/config/orchestration/lifecycle/runner/reporting code, Go tests, lifecycle tests, and packaging rules. Existing tasks files described older `--pattern` work; this plan replaces them for requested quality-gate work.

## Existing architecture to reuse

- `scripts/_cli_parser.py:49-142`: argparse test flags, timeout, fix, JSON/JSONL, quiet.
- `scripts/_orchestrator.py:40-235`: target dispatch, isolated Compose project, lock, signal/atexit cleanup.
- `scripts/_devkit_config.py:139-276,350-465`: TOML loading/cascade, deep merge, lifecycle dependencies, target expansion, mode map.
- `scripts/_test_handler.py:69-160,196-380,457-632`: phase selection, status/report generation, JSONL events, lifecycle setup/teardown.
- `scripts/_test_utils.py:165-257,286-490`: Compose argv construction, detached stdin, timeout/process cleanup, logs, command and exit metadata.
- `scripts/_test_modules.py:77-177,218-343,346-405,407-539`: language detection, Go service/workdir resolution, current Go command generation.
- `scripts/_bootstrap.py:96-183,196-277` and `scripts/_lifecycle.py:179-432,566-623`: profiles, configured dependency services, health checks, build policy, teardown.
- Tests: `scripts/tests/test_devkit_config.py`, `test_test_modules.py`, `test_test_utils.py`, `test_handlers.py`, `test_lifecycle.py`, `test_bootstrap.py`, `test_usage.py`.

## Dependency graph

```text
Config normalization/validation + default gate contract
    ├── Go gate command selection and integration path
    └── Generic configured-gate dispatch
            └── Gate-level evidence/report contract
                    ├── base-ref/new-findings support
                    ├── generated freshness hook
                    └── tool-version validation / false-green checks
                            └── docs, migration, final verification
```

Existing command runner and lifecycle are shared foundations. Rust/TypeScript paths stay compatibility branches and must not depend on Go changes.

## Architecture decisions

1. Extend existing TOML config with a small normalized gate model. Do not create plugin framework or second executor.
2. Validate command values at config/CLI trust boundaries. Preserve argv semantics; reuse existing runner and Compose lifecycle. Add tests for shell/option injection.
3. Normalize stable name, phase, command, working directory, service, timeout, required, condition, failure policy, optional environment allowlist, base-ref capability, expected version, and setup/fix metadata only where needed.
4. Missing quality config preserves current behavior. Optional Go tools remain opt-in; configured missing tools are `blocked` or `failed`, never passed. No runtime downloads or floating `latest`.
5. Add gate-level evidence while preserving phase-level report fields and existing Rust/TypeScript behavior.
6. Full seeded verification may be deferred if it requires a large new framework. Fail-closed no-op/malformed/missing-evidence tests are mandatory; deferred scope must be explicit.

## Vertical task slices

### Phase 1: Config and Go execution

#### Task 1: Define and validate quality-gate config

**Description:** Add normalized quality-gate resolution at the config boundary. Preserve TOML cascade, build policy, lifecycle dependencies, targets, and legacy configs. Add generic schema plus Go default declarations without project-specific values.

**Acceptance criteria:**
- [ ] Missing `[quality]` preserves current Rust/TypeScript and existing Go behavior.
- [ ] Valid gates normalize name, phase, argv command, workdir, service, timeout, required, condition, failure policy, and optional metadata.
- [ ] Wrong section/type, empty command, invalid path/timeout/phase/policy, duplicate name, unsafe env key, and unsupported version syntax fail before lifecycle startup.
- [ ] Go defaults cover format, vet, unit; race/staticcheck/golangci-lint/govulncheck/mod verify/integration/generated/version remain opt-in unless configured.
- [ ] No LeeAttend service, path, image, credential, migration path, or tool version enters defaults.

**Dependencies:** None.

**Likely files:** `scripts/_devkit_config.py`, `templates/leedevkit.default.toml` or `templates/leedevkit.go.toml`, `scripts/tests/test_devkit_config.py`.

**Verification:**
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_devkit_config.py -q`
- Add tests for missing section, old config, wrong type, empty command, bad path, duplicate name, deterministic ordering, Go defaults, Rust/TypeScript compatibility.

#### Task 2: Complete Go vertical path

**Description:** Use normalized gates and existing runner/lifecycle to execute Go lint/unit/race/integration/coverage. Add configured setup/migration support without DB credentials, migration paths, or destructive defaults. Make `int-go` execute configured integration work and always tear down.

**Acceptance criteria:**
- [ ] Default `go test ./...` remains unit behavior; configured race gate runs only when selected/configured.
- [ ] Integration dependencies use existing lifecycle config; setup success precedes test; setup/readiness failure is `blocked`; test non-zero is `failed`; teardown runs after all outcomes.
- [ ] Missing tool binary reports `blocked`/`failed` with remediation evidence, never green.
- [ ] Existing timeout, cancellation, signal, detached stdin, stderr/log capture, exit code, and rerun behavior remains intact.
- [ ] Rust/TypeScript task generation is unchanged.

**Dependencies:** Task 1.

**Likely files:** `scripts/_test_modules.py`, `scripts/_test_handler.py`, `scripts/_test_utils.py`, `scripts/_bootstrap.py` only if profile mapping requires it, Go/handler tests.

**Verification:**
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_test_modules.py scripts/tests/test_test_utils.py scripts/tests/test_handlers.py -q`
- Mock pass/non-zero/missing binary/timeout/cancel/signal/large stderr/dangerous input.
- Use temporary Go fixture for unit/race/integration/malformed evidence when runtime exists; report unavailable runtime as blocked.
- `./bin/leedevkit test go --unit-only --json` and `./bin/leedevkit test go --lint-only --json` when configured runtime exists.

### Checkpoint 1: Config and Go contract

- [ ] Normalized schema remains small and reusable.
- [ ] Config errors happen before Compose startup.
- [ ] Go integration cannot silently become green with no task.
- [ ] Rust/TypeScript regression tests pass.

### Phase 2: Generic dispatch and reporting

#### Task 3: Generic configured-gate dispatch

**Description:** Run normalized gates by phase through existing `TestHandler.run_phase`, `build_compose_exec`, `run_parallel_ordered`, lifecycle profiles, and dependency resolution. Keep fixed Rust/TypeScript paths intact.

**Acceptance criteria:**
- [ ] One custom configured gate runs end-to-end in declared phase/service/workdir.
- [ ] Ordering, timeout, condition, required flag, and failure policy are deterministic.
- [ ] Gate log is phase/gate-specific and report includes command, target, service/container, workdir, status, exit code, duration, log, and rerun.
- [ ] Required failure fails overall; optional failure follows policy and remains visible; blocked/skipped stay distinct.
- [ ] Required configured gate or phase cannot silently become empty passed task.

**Dependencies:** Tasks 1–2.

**Likely files:** `scripts/_test_handler.py`, `scripts/_test_modules.py`, `scripts/_lifecycle.py`, `scripts/_devkit_config.py`, `scripts/tests/test_handlers.py`, `scripts/tests/test_lifecycle.py`.

**Verification:**
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_handlers.py scripts/tests/test_test_modules.py scripts/tests/test_lifecycle.py -q`
- Dry-run/mocked lifecycle tests for exact argv, injection safety, no-task, required/optional, setup blocked, mixed status, teardown.

#### Task 4: Truthful gate-level reporting and base-ref/new-findings

**Description:** Extend JSON summary and JSONL events with gate evidence and optional base-ref metadata. Add `--base-ref` only if parser scope stays small; otherwise support config first. Resolve refs with safe argument-list Git calls. Activate new-findings mode only for gates declaring support.

**Acceptance criteria:**
- [ ] Each gate result has phase, target, gate, command, status, required, exit code, duration, log, rerun, service/container, and workdir.
- [ ] JSON and JSONL remain parseable; JSONL sequence remains monotonic; human logs stay off machine stdout.
- [ ] Missing/invalid base ref, failed resolution, shallow/unavailable diff, or malformed evidence is explicit `blocked`/`failed`, never pass.
- [ ] Report records base ref, resolved commit, changed paths, whether new-findings ran, and skip reason when unsupported.
- [ ] Unsupported gate does not receive `--new-from-rev` blindly; it runs full or skips with reason.
- [ ] Focused/skipped/no-task runs are `partial` with `full_regression=false`; required failures never become passed/exit 0.

**Dependencies:** Task 3.

**Likely files:** `scripts/_cli_parser.py`, `scripts/_test_handler.py`, `scripts/_test_utils.py`, `scripts/tests/test_handlers.py`, `scripts/tests/test_usage.py`.

**Verification:**
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_handlers.py scripts/tests/test_usage.py -q`
- Direct `json.loads()` tests for every JSON summary/event.
- Cases for JSONL, quiet, log/rerun, invalid/missing base ref, supported/unsupported gate, empty paths, mixed status.

### Checkpoint 2: End-to-end reporting

- [ ] `./bin/leedevkit test all --json` report parses as JSON.
- [ ] `./bin/leedevkit test all --json-stream --quiet` emits parseable JSONL only.
- [ ] Failed/blocked/skipped required gates cannot produce green status.
- [ ] Existing Rust/TypeScript report consumers retain required fields.

### Phase 3: Hooks, validation, docs

#### Task 5: Generated-code freshness hook

**Description:** Add generic configured generated freshness and optional explicitly declared fix command. Run in declared service/container/workdir. Check mode never overwrites generated files.

**Acceptance criteria:**
- [ ] Configured hook runs in declared container/service.
- [ ] Diff/non-zero and missing command fail/block; check mode leaves source unchanged.
- [ ] `--fix` exists only when project declares fix command.
- [ ] Changed generated paths are recorded when safely obtainable.
- [ ] No SQL/OpenAPI tool/path appears in LeeDevKit defaults.

**Dependencies:** Task 4.

**Likely files:** config, gate dispatch, focused tests, generic template/docs if needed.

**Verification:** Temporary workspace clean/changed/missing-tool/error/fix cases; assert check mode preserves working tree.

#### Task 6: Tool versions and false-green contract

**Description:** Validate optional expected tool versions and fail closed on empty/no-op/malformed evidence. Implement seeded isolated-workspace verification only if existing architecture supports it without a large framework; otherwise document exact deferral.

**Acceptance criteria:**
- [ ] Actual/expected versions appear in report.
- [ ] Mismatch, missing tool, or malformed version output is blocked/failed, never passed.
- [ ] No runtime Internet download or floating `latest`.
- [ ] Seeded verification, if implemented, uses temporary workspace, proves seeded fail then clean pass, and cleans up all paths.
- [ ] If deferred, false-green tests cover empty/no-op/malformed/missing evidence and plan records limitation.

**Dependencies:** Task 4.

**Likely files:** runner/handler/config plus `scripts/tests/test_test_utils.py` and `test_handlers.py`.

**Verification:** Mock version match/mismatch/missing/malformed; runner exit 0 without evidence, malformed output, non-zero, timeout, signal.

### Checkpoint 3: Adversarial review

- [ ] Injection, option injection, path traversal, environment leakage, and secret log exposure reviewed.
- [ ] False-green matrix covers no task, missing binary, setup blocked, timeout, cancellation, signal, malformed evidence, optional/required.
- [ ] Teardown runs after every failure path.

#### Task 7: Documentation, migration, final verification

**Description:** Document minimal schema, Go defaults, integration dependency/setup contract, base-ref semantics, generated hooks, versions, statuses, downstream migration, and limitations. Use generic placeholders only.

**Acceptance criteria:**
- [ ] README/templates document valid config and compatibility defaults.
- [ ] Migration note explains replacing downstream adapter gates while retaining project-owned DB/migration/tool policy.
- [ ] CLI help documents supported flags or states config-driven-only behavior.
- [ ] Implemented, deferred, blocked, and skipped capabilities are distinct.
- [ ] No unrelated dependency or packaging changes.

**Dependencies:** Tasks 1–6.

**Likely files:** `README.md`, generic template, `CHANGELOG.md`, `tasks/plan.md`, `tasks/todo.md`; CI only if required by report/base-ref contract.

**Verification:**
- `./leedevkit --help`
- `./leedevkit test --help`
- `./leedevkit test infra --lint-only`
- `./leedevkit test infra --unit-only`
- `./leedevkit test all --json`
- `./leedevkit test all --json-stream --quiet`
- `ruff format --check scripts`
- `ruff check scripts`
- `mypy scripts`
- `bash -n scripts/_ensure-venv.sh install.sh bootstrap.sh bin/leedevkit` for existing paths
- If packaging/install/bootstrap/release files change, run release build and acceptance commands from `.agent/rules/devkit-internals.md`.

## Mandatory test-impact matrix

| Area | Scenarios | Planned coverage |
|---|---|---|
| Config | missing section, old config, wrong type, empty command, bad path, duplicate gate | Task 1 |
| Execution | zero/non-zero, missing binary, timeout, cancellation, signal, large stderr, dangerous input | Tasks 2, 3, 6 |
| Phase state | pass, required fail, optional fail, blocked setup, skipped, no task, mixed | Tasks 3, 4 |
| Go | gofmt, vet, unit, race, staticcheck, govulncheck, mod verify, integration | Tasks 1, 2 |
| Reporting | JSON, JSONL, quiet, logs, rerun, full regression | Task 4 |
| Lifecycle | build/no-build, cleanup success/failure, readiness | Tasks 2, 3 |
| Compatibility | Rust, TypeScript, current Go config, current flags | Tasks 1–4, 7 |

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Schema becomes plugin framework | High | Keep normalized fields minimal; reuse runner/lifecycle. |
| Existing `bash -c` command boundary | High | Validate/quote dynamic data, add injection tests, avoid shell interpolation for untrusted values. |
| Integration setup touches real DB | High | Require disposable/test lifecycle config; no destructive defaults; teardown in finally. |
| Report consumers break | High | Preserve phase fields and statuses; add gate records compatibly; test old fields. |
| Base ref unavailable in shallow CI | Medium | Block explicitly; never default to `origin/main`. |
| Tool versions vary | Medium | Project declares expected versions; validate only; never install. |
| Seeded verification too large | Medium | Fail-closed evidence tests now; explicit deferral and follow-up. |
| Default template filename mismatch | Medium | Verify runtime path and add focused regression before changing template loading. |

## Open decisions for implementation review

- Make config-driven behavior primary; add `--base-ref` only if parser/report scope remains small.
- Keep optional Go tools opt-in; configured missing tools are blocked/failed.
- Defer full seeded verification only with fail-closed tests and explicit limitation.
