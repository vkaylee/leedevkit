# Python → Go Control-Plane Migration

## Context

Go currently owns narrow policy/protocol paths; Python still owns TOML loading, orchestration, subprocess execution, and runtime mutation. No `SPEC.md` exists. This plan follows ADR-004 and preserves explicit Python fallback until Go parity is proven.

## Ownership boundary

- Go owns CLI contracts, normalized policy, process supervision, reports, locks, ownership, and cleanup policy.
- Compose owns service graph, dependencies, health, containers, networks, volumes, and service identity.
- Consumer projects own DB schemas/migrations; LeeDevKit builds Compose `run`/`exec` argv only.
- Go errors fail closed. No silent Python fallback in Go mode.

## Dependency graph

```text
Go CLI dispatch + argument contract
  → normalized project/provider boundary
  → argv planner + process supervisor
  → ownership/lock/cleanup
  → explicit Go dispatch
  → JSON/JSONL report contract
  → target/mode + phase scheduler
  → gate execution + lifecycle/build policy
  → full test all

Python TOML/provider resolver (temporary boundary) → run/test lifecycle
TOML conformance corpus → Go provider ownership
Project-state transactions → doctor → init/update/skills
Release/install/bootstrap → Go-first routing → fallback retirement
```

## Phase 1 — Restore lost `run` path

### Task 1: Freeze `run` contract and argv planner

Port Python `RunHandler` policy into pure Go planning for `go`, `cargo`, `npm`, and `diesel`: project mode, service selection, profiles/files, pooler env, `--` stripping, Cargo test rewrite, and sanitizer policy. Planner must emit argv only; no shell interpolation.

Acceptance:
- Exact argv, cwd, env additions, and ownership mode match Python fixtures for tools and project modes.
- Unknown tools, malformed values, unsafe args fail closed before mutation.
- Automatic paths never emit volume removal.

Verify with Go table tests, Python parity fixtures, fake Compose/tool argv checks, and `go test ./...` in configured Go container.

### Task 2: Add process supervisor parity

Port safe child execution needed by `run`: argv-only execution, streams, timeout, process-group termination, signals, exit mapping, bounded logs, and missing executable errors. Match `_safe_run.py`.

Acceptance:
- Exit behavior `0`, `1`, `124`, `127`, `130` matches contract.
- Timeout/signals terminate descendants.
- cwd, streams, and log paths preserved without secret leakage.

### Task 3: Add ownership, locks, and cleanup

Port immutable project identity, lock ownership, external-project protection, startup-failure cleanup, bounded idempotent teardown, stale sweep safety, and no-volume-removal invariant. Reuse `internal/lifecycle.Plan.Down()` for argv; Compose remains executor.

### Task 4: Switch production `run` dispatch

`LEEDEVKIT_CORE=go` executes Go `run`; `LEEDEVKIT_CORE=python` keeps Python path. Existing default remains backward-compatible until parity checkpoint. Go mode fails closed and never invokes `_orchestrator.py`.

### Checkpoint 1 — run parity

Golden argv/cwd/env/output/exit, sanitizer, timeout/signal/descendant cleanup, ownership, launcher routing, Python regression, shell syntax, and `git diff --check` pass.

## Phase 2 — Freeze shared machine contracts

### Task 5: Freeze JSON/JSONL reports

Encode current Python contract: schema version, run ID, status/counts, phase records, timestamps/duration, exit code, logs/rerun diagnostics, `next_action`, `partial`, `full_regression`, and ordered JSONL sequence. Implement Go encode-only model before scheduler.

### Task 6: Port normalized target/mode selection

Use existing Go modes/targets with Python config normalization as temporary boundary. Preserve order, aliases, uncovered services, compatibility defaults, and fail-closed shadow mismatch.

### Checkpoint 2 — contracts

Run/process/cleanup/report fixtures stable. No normal Go path silently invokes Python.

## Phase 3 — Full `test all`

### Task 7: Port phase scheduler and flags

Move target loop, phase selection, skip/only/coverage/fix/pattern/timeout/base-ref/dry-run/build-policy semantics. Scheduler uses fake executors first; no Compose ownership yet.

### Task 8: Port gate execution

Cover timeout, requires/conditions, version checks, optional/required/block policy, env, cwd, fix, new-findings, setup/teardown, coverage. Keep normalized resolution at Python boundary until parity.

### Task 9: Integrate lifecycle/build policy

Use `internal/lifecycle.Plan` plus narrow adapters for image fingerprints, build locks, readiness, dependencies, DB/migration argv, and teardown. Compose remains lifecycle authority.

### Checkpoint 3 — full test parity

`test all --json` and `--json-stream --quiet`, fake Compose, build policy, readiness, two-project isolation, failure cleanup, no-volume removal, and full wrapper regression pass. Focused runs stay partial.

## Phase 4 — Provider ownership and remaining commands

### Task 10: Provider/TOML conformance and ownership

Keep Python provider resolver until cross-language corpus matches. Then use standards-compliant TOML parser; preserve deep merge, CLI > environment > host > repository > Docker precedence, nested/multiline/escaped values, capabilities, malformed errors, and redaction. Approve dependency and supply-chain evidence.

### Task 11+: Separate vertical slices

Migrate `dev`, read-only `doctor`, transactional `doctor --fix`, `init`, `update`, `skills`, `db:*`/`migrate:*`, `debug`, `sync:api`, `fmt:infra`, `verify:infra`, generated, and coverage as separate paths. Reuse established contracts; do not bundle.

### Final checkpoint — packaging and retirement

After two compatibility releases, full regression, release acceptance, static proof normal Go commands do not launch `_orchestrator.py`, and human approval: make release/install/bootstrap Go-first and remove dead bridge/handlers in separate cleanup. Update ADR with ownership, rollback, expiry, and evidence.

## Risks

| Risk | Mitigation |
|---|---|
| Rewrite drops behavior | Vertical slices, golden fixtures, stop at checkpoints |
| Go duplicates Compose | Compose remains executor; Go emits argv/supervises |
| Child leaks | Process-group and descendant tests |
| External cleanup damage | Immutable identity, lock ownership, no-volume invariant |
| TOML drift | Standard parser and cross-language corpus |
| Report breakage | Freeze Python JSON/JSONL fixtures first |
| Shell injection | Existing sanitizer policy and argv-only execution |
| Missing host Go | Use `golang:1.26.11-bookworm` container |

## Verification baseline

```bash
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.26.11-bookworm go test ./...
./leedevkit test infra --lint-only
./leedevkit test infra --unit-only
./leedevkit test all --json
./leedevkit test all --json-stream --quiet
.venv/bin/ruff format --check scripts
.venv/bin/ruff check scripts
.venv/bin/mypy scripts
```

Packaging/runtime changes additionally require release build and acceptance from `.agent/rules/devkit-internals.md`.
