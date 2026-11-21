# Python → Go Control-Plane Migration

## Phase 1 — Restore `run`

- [ ] Task 1: Freeze `run` golden contract and implement pure Go argv planner.
  - [x] Add versioned `--plan-run` Go protocol.
  - [x] Cover `go`, `cargo`, `npm`, `diesel` planner branches.
  - [x] Cover project, running/non-running service, pooler, Cargo rewrite, sanitizer policy.
  - [x] Add Python opt-in shadow comparison before execution.
  - [ ] Expand exact golden fixtures for every tool/project combination.
- [ ] Task 2: Port safe process supervisor.
  - [x] Preserve argv-only execution, streams, cwd, timeout, signals, exit mapping.
  - [x] Kill process groups on timeout/cancellation.
  - [x] Add Go run execution adapter using process supervisor.
  - [x] Wire owned/external cleanup behavior into execution adapter tests.
  - [x] Integrate bounded project log writer into Go run execution adapter.
- [ ] Task 3: Port ownership, locks, and cleanup safety.
  - [x] Add immutable project identity and lock ownership session.
  - [x] External sessions never auto-teardown.
  - [x] Owned close is idempotent and releases lock after teardown.
  - [x] Add bounded Compose teardown, retry, fallback, and no-volume invariant.
  - [x] Add stale sweep policy: test prefix, age, lock, unknown-age safety.
  - [ ] Add startup-failure integration.
- [ ] Task 4: Switch production `run` dispatch.
  - [x] Add Go CLI parser and normalized project/tool context.
  - [x] Wire Go `run` execution into `cmd/leedevkit`.
  - [x] `LEEDEVKIT_CORE=go` launcher routes all currently supported Go binary commands, including `run`.
  - [x] `LEEDEVKIT_CORE=python` preserves Python path.
  - [x] Go mode fails closed when binary is missing; launcher does not start `_orchestrator.py`.

### Checkpoint 1

- [ ] Golden argv/cwd/env/output/exit parity passes.
- [ ] Timeout/signal/descendant and ownership tests pass.
- [ ] Go and Python focused tests pass.
- [ ] Shell syntax and `git diff --check` pass.

## Phase 2 — Contracts

- [x] Task 5: Freeze Go JSON/JSONL report encoder against Python fixtures.
  - [x] Stable summary schema fields and status/count contract.
  - [x] Ordered JSONL stream with schema version, run ID, sequence, timestamp.
- [x] Task 6: Port normalized target/mode command path with Python config boundary.
  - [x] Add Go testplan adapter over existing targets/modes policy.
  - [x] Preserve alias order, uncovered services, and compatibility modes.

### Checkpoint 2

- [ ] Report/schema/sequence fixtures stable.
- [ ] No normal Go path silently invokes Python.

## Phase 3 — Full `test all`

- [x] Task 7: Port phase scheduler and all test flags.
  - [x] Preserve Python phase order and selected/skipped statuses.
  - [x] Handle lint/unit/e2e/coverage/skip and infra conflicts.
  - [x] Add fake executor run integration with report summary/JSONL.
  - [x] Add versioned `--plan-test` protocol and fail-closed validation.
  - [x] Wire scheduler into Go `test` CLI with default gate executor.
  - [x] Add Compose gate runner that builds `compose exec -T` argv.
  - [x] Wire Go test scheduler gates through Compose argv boundary.
  - [x] Add lifecycle phase wrapper with startup, gate execution, teardown, and blocked startup failures.
  - [x] Add argv-only lifecycle executor with guaranteed teardown.
  - [x] Add Compose executor for lifecycle argv with process exit propagation.
  - [x] Verify lifecycle Plan.Up/Down argv integration.
  - [x] Add Compose phase executor combining lifecycle and gate execution.
  - [x] Wire lifecycle phase executor into Go `test` CLI path.
  - [x] Provide lifecycle Plan.Up/Down args from normalized provider/project context.
  - [x] Add readiness polling adapter with timeout and context cancellation.
  - [x] Add build policy (`--build`/`--no-build`/`skip-build`) to lifecycle Plan.
  - [x] Add normalized provider boundary and capability validation.
  - [x] Add provider precedence/default/custom/unknown conformance fixtures.
  - [x] Move provider TOML loading to pinned `github.com/pelletier/go-toml/v2` parser.
  - [x] Preserve provider precedence and fail-closed malformed/unknown config behavior.
  - [x] Connect Go test lifecycle to Go provider resolver for repository/env precedence.
  - [x] Auto-load host config path when no explicit host file is provided.
  - [x] Add nested host provider override conformance fixture.
  - [x] Add nested host provider override conformance fixture.
  - [x] Add arrays, escaped commas, inline hashes, and multiline TOML fixtures.
  - [x] Remove obsolete line-based provider parser and compatibility array parser.
  - [x] Correct host provider source diagnostics for auto-loaded host config.
  - [x] Add Go project config loader for services, targets, and build policy.
- [x] Add Go project config loader for services, targets, and build policy.
- [x] Connect config loading to Go test command validation boundary.
- [x] Select Compose service/workdir from configured Go service instead of hardcoded values.
- [x] Make config-derived service/target mapping deterministic.
- [x] Complete deep-merge/host override parity and switch all config ownership.
- [x] Task 8: Port quality-gate execution and failure policy.
  - [x] Add argv-only Go gate executor for normalized phase gates.
  - [x] Preserve required failure stop and optional gate handling.
  - [x] Adapt gate executor into test scheduler phases.
  - [x] Add version/setup/teardown/requires/new-findings semantics.
- [ ] Task 9: Integrate lifecycle/build policy, DB/migration argv, and phase cleanup.
  - [x] Load configured dependencies and pass build/skip-build policy into lifecycle Plan.
  - [x] Add readiness polling to actual lifecycle phase execution.
  - [x] Add build/image freshness policy primitives.
  - [x] Add deterministic build fingerprint and metadata persistence primitives.
  - [x] Add build/image verification decision to lifecycle Plan via metadata fingerprint.
  - [x] Add per-fingerprint build lock primitive.
  - [x] Add locked build runner with post-success metadata publication.
  - [x] Add Compose image label fingerprint verification primitive.
  - [x] Add image fingerprint verification helper with mismatch failure.
  - [x] Wire image label verification into locked build runner before metadata publication.
  - [x] Add image verification across all built images.
  - [x] Add lifecycle Plan.BuildArgs using same provider/profile/service boundary.
  - [x] Add BuildFlow using Plan.BuildArgs → build lock → image verify → metadata publish.
  - [x] Resolve configured service image field in project config.
  - [x] Add build input loader and fingerprint source boundary.
  - [x] Add file-based BuildFlow fingerprint + metadata publisher.
  - [x] Load configured Dockerfile, Compose file, and image paths from service config.
  - [x] Fingerprint directory build context deterministically while excluding runtime/cache dirs.
  - [x] Wire build input/fingerprint validation into lifecycle CLI path.
  - [x] Execute BuildFlow and publish verified metadata in lifecycle CLI path.

### Checkpoint 3

- [ ] `test all --json` parity.
- [ ] `test all --json-stream --quiet` parity.
- [ ] Compose, readiness, build, isolation, teardown-on-failure, no-volume tests pass.
- [ ] Full wrapper regression passes.

## Phase 4 — Ownership and remaining paths

- [ ] Task 10: TOML/provider conformance corpus and Go ownership switch.
- [ ] Migrate `dev` as separate vertical slice.
  - [x] Migrate `dev up` and `dev down` to Go; retain Python for `ps`, `logs`, `exec`, and `clean`.
- [ ] Migrate read-only `doctor`.
  - [x] Migrate read-only checks to Go.
- [ ] Migrate transactional `doctor --fix`.
  - [x] Move local AI rule repair to Go; runtime download/integrity/venv/harness transaction remains.
- [x] Migrate `init` to Go-only runtime transaction.
- [x] Migrate `update` to Go-only runtime transaction.
- [ ] Migrate `skills`.
  - [x] Migrate read-only `skills list`; retain Python for transactional skill changes.
- [x] `db:setup` removed — customer repositories own DB schema, migrations, seeds, and test lifecycle.
- [ ] Migrate remaining `db:*` and `migrate:*` paths.
- [ ] Migrate `debug`.
- [ ] Migrate `sync:api`, `fmt:infra`, `verify:infra`, generated, coverage.

## Packaging and retirement

- [ ] Make release/install/bootstrap Go-first.
- [ ] Keep explicit Python fallback through two compatibility releases.
- [ ] Run release build, acceptance, checksums, SBOM, and public release verification.
  - [x] Add standalone Go Debug MCP platform binary build and release payload validation.
- [ ] Human approval before deleting bridge, handlers, fallback env, or wrapper.
- [ ] Remove dead compatibility code in separate reviewed cleanup.

## Existing separate fix

- [ ] Preserve broken `.agent/rules` symlink repair in `scripts/_init_handler.py` and its regression test.
