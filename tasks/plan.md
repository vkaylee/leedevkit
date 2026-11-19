# Python → Go Control-Plane Migration

## Context

Go currently owns only help/version/runtime lookup and delegates operational commands to `scripts/_orchestrator.py`. Goal: move complete user-visible command paths to Go, one vertical slice at a time. Python remains explicit compatibility fallback during migration; no silent fallback from Go errors.

## Ownership boundary

- Go owns CLI/config/provider policy, project identity, locks/ownership, build policy, phase orchestration, process supervision, timeout/signal handling, reports, and cleanup policy.
- Docker Compose owns config validation, service graph, dependency ordering, healthchecks, create/start/stop/remove, networks, labels, service identity, logs, exec, and volumes.
- Go must not recreate Compose lifecycle logic.
- Consumer projects own DB schemas/migrations; LeeDevKit only builds Compose `run`/`exec` commands.

## Dependency graph

```text
Go CLI/runtime → config/provider policy → Compose argv boundary
                         ├→ dev commands
                         ├→ test lifecycle + phase runner → JSON/JSONL reports
                         └→ DB/migration Compose commands
Go project-state transactions → doctor → init → update → skills
release/install/bootstrap → artifact acceptance → fallback retirement
```

## Vertical slices

### 0. Contract and dispatch foundation

Define command model, runtime precedence, explicit `LEEDEVKIT_CORE=go|python` routing, schema-versioned reports, process supervision, and fail-closed Go errors. Add parity fixtures. Preserve offline help/version and Python escape hatch.

**Implemented narrow slice:** runtime precedence and protocol/CLI regression foundation. Explicit `DEVKIT_HOME` now wins over project runtime; quoted `vX.Y.Z` pins normalize before compatibility comparison; empty Python dispatch fails closed; no-argument help, version aliases, and malformed gate JSON have regression tests. Full command dispatch, report schema, and process supervisor remain future slices.

**Checkpoint A:** Go binary works outside checkout; runtime selection matches Python; stdin/stdout/stderr, exit codes, signals, and version pin behavior pass.

**Status:** partial — runtime/CLI contract portion complete; full command model, reports, and process supervision not started.

### 1. Focused tests: `test infra`

Move parser flags, gate resolution, argv execution, logs, status/partial semantics, and JSON/JSONL output. Compose remains execution boundary. Switch production dispatch only after golden parity.

**Implemented increments:** `test infra --lint-only` and `test infra --unit-only` now have Go-native argv runners. Lint runs runtime `ruff check`, `mypy`, and optional `shellcheck`; unit runs pytest with production coverage and the 80% gate. Unit supports validated `--pattern` filtering without claiming full coverage. All preserve project working directory and first-failure exit propagation. Gate resolution, reports, JSON/JSONL, and other focused modes remain Python until later increments.

**Checkpoint B:** `test infra --lint-only`, `--unit-only`, pattern, dry-run, blocked/failed behavior pass in Go and Python regression remains green.

### 2. Full `test all`

Move target expansion, phase state machine, build policy, lock/ownership, timeout/signal cleanup, DB setup/migration command construction, and reports. Compose remains container lifecycle authority. Automatic teardown never removes volumes.

**Checkpoint C:** full JSON and JSONL, concurrent project isolation, shared cache safety, Docker integration, and teardown-on-failure pass.

### 3. `dev`

Move `up/down/ps/logs/exec`; keep explicit destructive `clean` separate. Use resolved Compose argv only.

### 4. `doctor`

Move read-only diagnostics first. Move `--fix` only with staged writes, atomic replace, rollback, and failure-injection tests.

### 5. `init`

Move config creation, release/local acquisition, safe extraction, version matching, `.venv`, rules/overrides/harness projection, skills initialization, pinning, and project-state ownership.

### 6. `update`

Move bounded download, archive/integrity validation, staged runtime/binary/config swap, post-sync, and byte-for-byte rollback of runtime, entrypoint, config, `.venv`, and `skills.d`.

### 7. `skills`

Move catalog/TOML/URL validation, bounded git operations, lock/pin updates, staged clone/update/remove, harness sync, and rollback.

### 8. Remaining paths

Move `run`, `db:*`, `migrate:*`, `debug`, `sync:api`, `fmt:infra`, `verify:infra`, generated and coverage paths. Migrate complete command paths, not helper layers.

### 9. Packaging and retirement

Make release/install/bootstrap Go-first without requiring `_orchestrator.py`. Keep Python fallback through two tagged compatibility releases. Remove dead handlers, bridge, and legacy wrapper only after static proof, full regression, release acceptance, and human approval.

## Rollback and removal gates

- `LEEDEVKIT_CORE=python` remains known-good escape hatch.
- Explicit Go mode fails closed; no silent Python fallback.
- Stop at failed checkpoint; do not stack slices on broken behavior.
- Remove fallback only after two tagged releases, full JSON/JSONL regression, init/update rollback, two-project isolation, release acceptance, and static proof that no Go command launches Python.

## Required verification

```bash
go test ./...
./leedevkit test infra --lint-only
./leedevkit test infra --unit-only
./leedevkit test all --json
./leedevkit test all --json-stream --quiet
ruff format --check scripts
ruff check scripts
mypy scripts
git diff --check
.venv/bin/python3 scripts/_release_build.py --repo-root . --output /tmp/dist
.venv/bin/python3 scripts/_release_acceptance.py --repo-root . --artifact /tmp/dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```

Packaging/runtime changes require release build and acceptance per `.agent/rules/devkit-internals.md`.
