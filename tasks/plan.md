# Implementation Plan: Compose-Native Lifecycle Migration

## Context

LeeDevKit currently splits container lifecycle ownership between Compose and engine-specific Python code. `scripts/_lifecycle.py` drives Compose `up`, but manually polls engine health, removes containers before `compose down`, and scans stale containers. `scripts/_compose_up.py` independently topologically sorts dependencies and calls `podman inspect`/`podman rm`. `scripts/_orchestrator.py` special-cases Podman. This duplicates Compose behavior, breaks provider portability, and complicates planned Go core migration.

Goal: make Compose/Compose-compatible CLI own service lifecycle while LeeDevKit remains policy supervisor. Preserve test isolation, ownership locks, build-freshness policy, test phases, reports, cleanup safety, provider selection, and shared-volume protection.

## Architecture decision

- Compose owns service create/start/stop/remove, dependency conditions, healthcheck definitions, network, project labels, volumes, `ps`, `logs`, and `exec`.
- LeeDevKit owns project-name generation, lock/ownership, build/no-build decision, lifecycle profile/dependency policy, phase orchestration, timeout/reporting, stale-run policy, and provider capability selection.
- Resolve provider command once; lifecycle code invokes only resolved Compose argv. No direct `docker`, `podman`, or `kiri` calls in generic lifecycle paths.
- Resolve service identity through `compose ps -q` or Compose labels. Hardcoded `<project>_<service>_1` is fallback only.
- Never pass `-v`/`--volumes` during normal teardown; shared Go/Bun cache volumes must survive.
- Prefer Compose-native readiness (`up --wait` where supported). Keep bounded health fallback only for providers without wait capability, with explicit diagnostics.

## Dependency graph

```text
provider resolution + Compose capability model
    ├── Compose command/project/profile construction
    │       ├── Compose-native up/readiness
    │       ├── Compose-native down/ps/identity
    │       └── provider fallback behavior
    └── lifecycle policy (build freshness, ownership, lock)
            └── TestHandler phase setup/teardown
                    └── orchestrator cleanup and stale-run GC
                            └── release/CI acceptance
```

## Vertical task slices

### Phase 1: Provider-neutral Compose boundary

#### Task 1: Define Compose lifecycle command boundary

**Description:** Add a small internal Compose client/helper around existing `bootstrap_env`, resolved provider command, project, Compose files, profiles, and safe runner. Do not change observable lifecycle behavior yet.

**Likely files:** `scripts/_bootstrap.py`, `scripts/_devkit_config.py` if capability lookup is required, focused tests.

**Acceptance criteria:**
- [ ] Generated commands use resolved provider Compose argv, project name, files, and profiles in deterministic order.
- [ ] Commands remain argv lists; no shell interpolation.
- [ ] Docker, Podman, and declared custom providers work without direct engine-name branching.
- [ ] Missing/unsupported capability fails before mutation.
- [ ] Existing config precedence and project isolation remain unchanged.

**Verification:**
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_bootstrap.py scripts/tests/test_devkit_config.py -q`
- Exact-argv tests for Docker, Podman, and custom provider.

**Dependencies:** None.

#### Task 2: Move dependency ordering and startup readiness into Compose

**Description:** Make `lifecycle_up` invoke Compose-native `up` with configured profiles/services and readiness. Retire custom Podman topological startup as default.

**Likely files:** `scripts/_lifecycle.py`, `scripts/_compose_up.py`, `scripts/_bootstrap.py`, Compose templates, `scripts/tests/test_lifecycle.py`, `scripts/tests/test_compose_up.py`.

**Acceptance criteria:**
- [ ] Normal startup uses one Compose lifecycle path for Docker, Podman, and custom providers.
- [ ] Dependency order and `service_healthy`/`service_completed_successfully` come from Compose config, not Python graph traversal.
- [ ] Build, `skip_build`, and phase build policies retain current semantics.
- [ ] Readiness timeout/startup failure/unhealthy dependency become blocked/false with diagnostics.
- [ ] Custom Podman path is removed or reduced to documented capability-gated fallback.

**Verification:**
- Mock exact-argv/readiness/timeout/build/failure tests.
- Temporary Compose fixtures for healthy dependency, failed job, cycle/config failure, and missing service.
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_lifecycle.py scripts/tests/test_compose_up.py scripts/tests/test_bootstrap.py -q`

**Dependencies:** Task 1.

### Checkpoint 1: Startup contract

- [ ] Compose is sole normal startup orchestrator.
- [ ] No direct Podman calls occur in normal `lifecycle_up`.
- [ ] Build freshness and project isolation tests pass.
- [ ] Rust, web, and Go mode mappings remain compatible.
- [ ] Human review before teardown/GC changes.

### Phase 2: Compose-owned teardown and identity

#### Task 3: Make teardown Compose-native and volume-safe

**Description:** Replace preemptive engine container removal in `lifecycle_down` with Compose `down --remove-orphans --timeout N`; retain only bounded, capability-gated fallback. Preserve ownership and attached-project behavior.

**Likely files:** `scripts/_lifecycle.py`, `scripts/_orchestrator.py`, `scripts/tests/test_lifecycle.py`, `scripts/tests/test_orchestrator.py`.

**Acceptance criteria:**
- [ ] Owned teardown calls Compose with correct project/files and `--remove-orphans`.
- [ ] Normal teardown never passes `-v` or `--volumes`.
- [ ] External/attached projects are never torn down on process exit.
- [ ] Cleanup is idempotent and runs after success, failure, timeout, and signal paths.
- [ ] Fallback is project-scoped and cannot delete unrelated containers.
- [ ] Stale sweep is not broad container-name deletion.

**Verification:**
- Mock command-order, volume-safety, ownership, idempotency, and failure-log tests.
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_lifecycle.py scripts/tests/test_orchestrator.py -q`

**Dependencies:** Tasks 1–2.

#### Task 4: Use Compose identity for health and stale-run GC

**Description:** Replace hardcoded container names and broad engine scans with Compose `ps`, project labels, lock state, age metadata, and bounded diagnostics.

**Likely files:** `scripts/_lifecycle.py`, `scripts/_test_utils.py` if helpers are reused, lifecycle/test utility tests.

**Acceptance criteria:**
- [ ] Service identity resolves through `compose ps -q` or Compose labels; hardcoded names are fallback only.
- [ ] Health checks cannot cross project boundaries.
- [ ] GC removes only stale LeeDevKit-owned, unlocked projects with matching labels/age.
- [ ] Active, external, different-provider, and malformed metadata are preserved or reported safely.
- [ ] Diagnostics include project, service, status, and rerun command without secrets.

**Verification:**
- Fixture tests for active lock, stale unlocked project, external project, missing labels, multiple services, and shared volumes.
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_lifecycle.py scripts/tests/test_test_utils.py -q`

**Dependencies:** Task 3.

### Checkpoint 2: Lifecycle ownership

- [ ] Compose owns normal up/down/health/dependency behavior.
- [ ] LeeDevKit owns policy, lock, ownership, build decision, and reports.
- [ ] Normal cleanup/GC cannot delete unrelated containers or volumes.
- [ ] Failure paths preserve truthful blocked/failed status.

### Phase 3: Test orchestration and provider compatibility

#### Task 5: Integrate Compose-native lifecycle with test phases

**Description:** Update `TestHandler` setup/teardown to consume the new lifecycle contract without changing phase/report semantics. Remove special dependency startup where Compose declarations cover it.

**Likely files:** `scripts/_test_handler.py`, `scripts/_orchestrator.py`, `scripts/_lifecycle.py`, `scripts/tests/test_handlers.py`, `scripts/tests/test_orchestrator.py`.

**Acceptance criteria:**
- [ ] Lint/unit/integration/e2e phases start only declared services and dependencies.
- [ ] Setup failure blocks phase; test failure remains failed; teardown always executes.
- [ ] JSON and JSONL retain machine-readable phase fields, logs, and rerun evidence.
- [ ] Rust/TypeScript/Go behavior outside lifecycle remains unchanged.
- [ ] `--skip-build`, focused flags, and `build_policy = "phase"` retain semantics.

**Verification:**
- Mock tests for pass, setup failure, timeout, signal, test failure, teardown failure, JSON, and JSONL.
- `PYTHONPATH=scripts .venv/bin/pytest scripts/tests/test_handlers.py scripts/tests/test_orchestrator.py scripts/tests/test_lifecycle.py -q`

**Dependencies:** Tasks 2–4.

#### Task 6: Verify provider capability matrix and fallback boundaries

**Description:** Define lifecycle capabilities (`config`, `up`, `down`, `ps`, health/wait, project cleanup, optional removal fallback). Fail closed for unsupported operations; never infer compatibility from provider name.

**Likely files:** `scripts/_devkit_config.py`, `scripts/_lifecycle.py`, `scripts/_orchestrator.py`, provider/config tests, docs/templates if contract changes.

**Acceptance criteria:**
- [ ] Docker default path passes.
- [ ] Podman path passes without normal Podman-only lifecycle code.
- [ ] Custom provider is accepted only for declared supported capabilities.
- [ ] Unsupported readiness/cleanup is explicit and bounded, never silently green.
- [ ] Diagnostics redact secrets and expose source/capability state.

**Verification:**
- Provider matrix tests with mocked command paths/capabilities.
- `./leedevkit test infra --lint-only`
- `./leedevkit test infra --unit-only`
- Integration lifecycle check when runtime exists; otherwise report blocked with exact reason.

**Dependencies:** Tasks 1–5.

### Checkpoint 3: End-to-end regression

- [ ] `./leedevkit test all --json` passes or reports runtime limitation truthfully.
- [ ] `./leedevkit test all --json-stream --quiet` emits parseable JSONL only.
- [ ] Compose project isolation works under concurrent test invocations.
- [ ] Shared cache volumes survive teardown.
- [ ] Docker and Podman behavior are verified separately where available.

### Phase 4: Cleanup, docs, and migration readiness

#### Task 7: Remove obsolete lifecycle code and document contract

**Description:** Delete verified-dead custom dependency/container deletion paths, update docs/templates, and record migration/rollback notes. Keep compatibility fallback until capability tests and release acceptance prove it unused.

**Likely files:** `scripts/_compose_up.py`, `scripts/_lifecycle.py`, `README.md`, `templates/leedevkit.default.toml`, `docs/architecture/`, tests, `CHANGELOG.md` if required by convention.

**Acceptance criteria:**
- [ ] Obsolete direct engine lifecycle code is removed or has documented fallback purpose.
- [ ] Compose-native ownership, healthchecks, provider capabilities, cleanup, and volume policy are documented.
- [ ] Rollback to prior release is executable without destroying project data/shared volumes.
- [ ] ADR contains context, decision, alternatives, consequences, security/operations impact, migration, rollback, owner, and status.
- [ ] Docs do not claim unsupported provider/runtime behavior.

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
- `git diff --check`
- If packaging/runtime files change, run release build and acceptance commands from `.agent/rules/devkit-internals.md`.

**Dependencies:** Tasks 1–6.

### Final checkpoint

- [ ] All task acceptance criteria pass.
- [ ] Full regression and packaging evidence reported with exact commands.
- [ ] Remaining provider/runtime limitations are explicit.
- [ ] Human approves Go migration using this Compose boundary.

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Compose implementations differ in `--wait` and dependency semantics | High | Capability-gate wait; bounded fallback; provider matrix tests. |
| Teardown removes another run's containers | High | Explicit project name, ownership lock, Compose labels, project-scoped cleanup tests. |
| Shared cache volume deletion | High | Never use `-v`/`--volumes`; assert in tests. |
| Health status becomes falsely green | High | Service-specific readiness and explicit blocked status on timeout/failure. |
| Custom provider is treated as Docker-compatible incorrectly | High | Explicit capabilities and fail-closed validation. |
| Existing phase/report consumers break | Medium | Preserve phase schema; test JSON and JSONL contracts. |
| Python compatibility path becomes permanent | Medium | Record owner/removal condition; remove after acceptance evidence. |

## Open questions

- Which providers are release-supported: Docker Compose, Podman Compose, Kiri, or only explicitly tested subsets?
- Should `compose up --wait` be mandatory where declared, with polling fallback only for legacy providers?
- Should the Compose-native lifecycle ADR be added before implementation or after Task 1 validates the boundary?
