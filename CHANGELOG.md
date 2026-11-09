# Changelog

All notable changes to LeeDevKit are recorded here. This changelog follows the Keep a Changelog format.

## [0.8.5] - 2026-11-09

### Fixes
- Start only active test services with `compose up -d --build`, preserving Compose/BuildKit cache reuse and invalidation.
- Remove global test prebuild and forced `build --pull` execution.
- Make `--skip-build` use `--no-build`, fail clearly when images are missing, and report partial verification.
- Preserve frontend `bun_init` startup synchronization and profile-specific lifecycle behavior.
- Preserve release evidence when GitHub Actions handles hidden `.ci-artifacts` paths.

#### Compatibility
- No project `.leedevkit/` changes required.
- Existing test profiles, dependency configuration, health checks, cleanup, and concurrent-agent isolation remain unchanged.

#### Rollback
```bash
./leedevkit update --version v0.8.4
```

Rollback restores previous LeeDevKit runtime.

#### Verification
- Focused lifecycle, handler, and orchestrator tests: 243 passed, 1 skipped.
- Full configured regression: 858 passed, 1 skipped; production coverage 82.93%.
- Ruff format/check and mypy passed.
- Release artifact build and acceptance passed for `leedevkit-0.8.5.tar.gz`.

## [0.8.4] - 2026-11-08

### Fixes
- Prevent cached DevKit runtime resolution from leaking across project boundaries after a working-directory change.
- Keep generated AI harness context project-relative when runtime files are staged outside the project.
- Protect source checkout files during filesystem-mutating tests and document fixture isolation rules.

#### Compatibility
- No CLI command or project configuration migration required.
- Existing source-checkout and installed-runtime context paths remain unchanged.

#### Rollback
```bash
./leedevkit update --version v0.8.3
```

Rollback restores the previous LeeDevKit runtime.

#### Verification
- Focused isolation tests passed.
- Full regression passed: 861 passed, 1 skipped; production coverage 82.95%.
- Ruff format/check and mypy passed.
- Release build and acceptance passed.

## [0.8.6] - 2026-11-10

### Fixes
- Build each Docker/Podman image once per `leedevkit test` invocation by default, with configurable per-phase builds through `[test].build_policy = "phase"`.
- Keep API, web, Go, and infrastructure image families independent; failed builds remain retryable in later phases.
- Resolve Rust-only and custom lifecycle services from project configuration while preserving legacy service fallbacks.
- Keep `--skip-build` on `--no-build` and report focused or incomplete verification as partial.

#### Compatibility
- Existing projects default to `build_policy = "once"`; set `[test].build_policy = "phase"` to preserve per-phase build requests.
- No project `.leedevkit/` changes required.

#### Rollback
```bash
./leedevkit update --version v0.8.5
```

Rollback restores previous LeeDevKit runtime.

#### Verification
- Full infrastructure verification: 869 passed, 1 skipped.
- Ruff format/check and mypy passed.
- Release artifact build and acceptance passed.

## [Unreleased]

## [0.8.2] - 2026-11-08

### Fixes
- Reload post-update sync code after replacing the runtime so Claude integrations use the updated handler implementation.
- Keep the doctor repair test independent of GitHub availability.

#### Compatibility
- No CLI command or configuration migration required.

#### Rollback
```bash
./leedevkit update --version v0.8.1
```

Rollback restores the previous LeeDevKit runtime.

#### Verification
- Full regression passed: 857 passed, 1 skipped; production coverage 82.93%.


## [0.8.1]

### v0.8.1 — Leaner AI Context and Project Python Commands

#### Highlights

- Reduce base AI context and load applicable project rules only when needed, improving compatibility with modern agent harnesses.
- Use each project's `.venv/bin/python3` in skill instructions and examples so commands run with project-installed dependencies.

#### Compatibility

- No CLI command or configuration migration required.
- Python skill commands now use the project-local virtual environment; create it with `python3 -m venv .venv` when absent.

#### Rollback

```bash
./leedevkit update --version v0.8.0
```

Rollback restores the previous LeeDevKit runtime. Python commands documented in skills may need the global `python3` form if the project virtual environment is absent.

#### Verification

- Full regression passed: 856 passed, 1 skipped; production coverage 83.28%.
- Release artifact build and acceptance passed for `leedevkit-0.8.1.tar.gz`.

## [0.8.0]

### v0.8.0 — Replace Manage CLI Namespace

#### Highlights

- Replace `leedevkit manage up dev` with `leedevkit dev up`.
- Replace `leedevkit manage down dev` with `leedevkit dev down`.
- Remove the `manage` CLI namespace.
- Promote `init`, `doctor`, and `skills` to top-level commands. Administrative database/migration handlers remain internal; the public launcher does not expose them.
- Update CLI help, wrappers, documentation, and tests.

#### Breaking changes

Scripts invoking `leedevkit manage ...` MUST migrate to the new command layout. Development lifecycle commands use:

```bash
leedevkit dev up
leedevkit dev down
```

Supported commands previously nested under `manage` are now top-level. For example:

```bash
leedevkit init
leedevkit doctor
leedevkit skills list
```

The old `manage` namespace is no longer accepted.

#### AI context and harness compatibility

- Reduce base context and load domain guidance only when its technology and risk apply.
- Remove blanket PTY/shell-syntax bans and mandatory log redirection; use available harness process controls and existing project isolation wrappers.
- Separate reusable safety invariants from project-owned auth, database, license, retention, and recovery policy.
- Resolve generated context and Claude hook/MCP paths for both source checkouts and installed consumer projects; preserve user-owned managed-file surroundings.
- Consolidate behavior verification and test execution rules. Dry-run success is not test evidence.

#### Rollback

```bash
./leedevkit update --version v0.7.30
```

Rollback restores the previous CLI contract. Existing scripts using `manage` require no changes after rollback.

#### Verification

- Focused CLI and orchestrator tests passed.
- Release acceptance tests passed.
- CLI parser smoke test passed for `dev up`, `dev down`, and top-level commands.
- `bash -n` passed for CLI wrappers.
- `git diff --check` passed.

## [0.7.30]

### v0.7.30 — Fix Project Launcher Update Lock

#### Highlights

- Initialize the project launcher lock path before stale-runtime bootstrap.
- Allow `./leedevkit update` to recover projects whose runtime version is outdated.

#### Verification

- Full infrastructure source-quality and regression gate passed: `858 passed, 1 skipped`.
- Release artifact acceptance gate passed.
- Project update smoke test passed from `0.7.28` to `0.7.29`.
- `git diff --check` passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.30
```

#### Rollback

```bash
./leedevkit update --version v0.7.29
```

The previous runtime remains available for rollback if an update or extraction fails.

## [0.7.29]

### v0.7.29 — Deduplicate Resolved Test Targets

#### Highlights

- Avoid running a service twice when `all` includes a service already covered by a configured target alias.
- Preserve service-only projects and deterministic target ordering.

#### Verification

- Full infrastructure source-quality and regression gate passed.
- Release artifact acceptance gate is required before publication.
- `git diff --check` passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.29
```

#### Rollback

```bash
./leedevkit update --version v0.7.28
```

The previous runtime remains available for rollback if an update or extraction fails.

## [0.7.28]

### v0.7.28 — Isolate Concurrent Go Test Runs

#### Highlights

- Reuse stable Go module and build cache volumes across test invocations.
- Keep per-run Compose projects while isolating temporary Go build files in a container tmpfs.
- Scope test and cleanup logs by Compose project to prevent concurrent runs overwriting each other.

#### Verification

- Full source-quality gate passed.
- Full infrastructure regression suite passed.
- Release artifact acceptance gate passed.
- `git diff --check` passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.28
```

#### Rollback

```bash
./leedevkit update --version v0.7.27
```

The previous runtime remains available for rollback if an update or extraction fails.

## [0.7.27]

### v0.7.27 — Prefer Project Runtime During Self-Update

#### Highlights

- Make `./leedevkit update` update the project-local `.leedevkit/` runtime when run from a LeeDevKit checkout/project.
- Prevent repeated `leedevkit.bak` chains caused by treating the project root as the installed runtime.
- Make the committed launcher prefer `.leedevkit/` over source-checkout files when a project runtime exists.
- Keep update downloads on manifest-verified release artifacts.

#### Verification

- Runtime selection and release artifact URL regression tests passed.
- Full infrastructure suite and release acceptance gate passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.27
```

#### Rollback

```bash
./leedevkit update --version v0.7.26
```

## [0.7.26]

### v0.7.26 — Download Manifest-Verified Release Artifacts in Update

#### Highlights

- Fix `./leedevkit update` to download official release tarballs containing `devkit.manifest.json` instead of GitHub source archive snapshots.
- Ensure updated installations pass integrity verification on subsequent `doctor` runs without reporting `manifest missing`.

#### Verification

- Update URL verification test passed.
- Full infrastructure suite: 851 passed, 1 skipped; 83.46% production coverage.
- Release artifact acceptance gate passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.26
```

#### Rollback

```bash
./leedevkit update --version v0.7.25
```

## [0.7.25]

### v0.7.25 — Fix Project Doctor Wrapper Dispatch

#### Highlights

- Fix project launcher `./leedevkit doctor` passing the `doctor` command name twice to the Python doctor parser.
- Preserve `doctor --fix` behavior and runtime manifest repair from v0.7.24.

#### Verification

- Wrapper doctor acceptance regression test passed.
- Ruff format, lint, and Mypy passed.
- Full infrastructure suite and release acceptance gate passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.25
```

#### Rollback

```bash
./leedevkit update --version v0.7.24
```

## [0.7.24]

### v0.7.24 — Repair Missing Runtime Manifests

#### Highlights

- Make `doctor --fix` restore a standalone per-project runtime when `devkit.manifest.json` is missing.
- Preserve `.venv/` and other mutable runtime state during manifest recovery.
- Keep shared worktree-linked runtimes read-only during repair.

#### Verification

- Doctor regression suite: 21 passed.
- Ruff format, lint, and Mypy passed.
- Full infrastructure suite and release acceptance gate passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.24
```

Existing projects can repair a missing manifest with:

```bash
./leedevkit doctor --fix
```

#### Rollback

```bash
./leedevkit update --version v0.7.23
```

## [0.7.23]

### v0.7.23 — Runtime Integrity Reconciliation

#### Highlights

- Report DevKit runtime manifest drift from `leedevkit doctor` without changing files.
- Repair modified or missing runtime files with `doctor --fix` while preserving `.venv/`, `skills.d/`, and `dev-state.json`.
- Refresh AI harness projections after runtime repair without overwriting user content outside managed blocks.
- Resolve generated built-in skill and agent references through the installed `.leedevkit/` runtime.

#### Verification

- Ruff format, lint, and Mypy passed.
- Full infrastructure suite: 848 passed, 1 skipped; 83.10% production coverage and 99% test coverage.
- Targeted doctor and harness tests: 28 passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.23
```

Existing projects can also run:

```bash
./leedevkit doctor --fix
```

#### Rollback

```bash
./leedevkit update --version v0.7.22
```

Mutable runtime state remains preserved during repair; failed replacement restores the previous runtime.

## [0.7.22]

### v0.7.22 — Per-Project Worktree Bootstrap

#### Highlights

- Commit a root `leedevkit` launcher while keeping `.leedevkit/` runtime data ignored.
- Bootstrap missing runtimes from the matching main Git worktree or `DEVKIT_HOME` only when versions match.
- Atomically install pinned releases with concurrent-bootstrap locking and rollback-safe replacement.
- Make `doctor` read-only by default; `doctor --fix` repairs safe DevKit runtime, virtualenv, and missing AI rulebook state.
- Migrate generated `.gitignore` files to ignore only runtime and bootstrap staging artifacts, never the committed launcher.

#### Verification

- Full wrapper suite: 843 passed, 1 skipped; 83.12% coverage.
- Ruff format and lint passed.
- Mypy passed for 68 source files.
- Release artifact acceptance passed.
- Fresh Git worktree smoke test passed for read-only `doctor` and runtime repair.

#### Upgrade

```bash
./leedevkit update --version v0.7.22
```

Existing projects can also run:

```bash
./leedevkit doctor --fix
```

#### Rollback

```bash
./leedevkit update --version v0.7.21
```

The previous runtime remains preserved when a bootstrap download or extraction fails.

## [0.7.21]

### v0.7.21 — Ignore Generated Skill Asset Copies

#### Highlights

- Stop skill discovery warnings caused by duplicate copies under plugin `cli/assets/skills/` directories.
- Preserve duplicate warnings for genuine skill source collisions.

#### Verification

- Skill discovery regression tests: passed.
- Full regression suite and release acceptance gate run before publishing.

#### Upgrade

```bash
./leedevkit update --version v0.7.21
```

#### Rollback

```bash
./leedevkit update --version v0.7.20
```

## [0.7.20]

### v0.7.20 — Correct Per-Project Virtualenv Doctor Check

#### Highlights

- Fix `leedevkit doctor` reporting the virtual environment as missing for per-project installs.
- Check `.leedevkit/.venv/`, the actual runtime location, instead of only checking a project-root `.venv/`.

#### Verification

- Doctor regression tests: 16 passed.
- Full regression suite and release acceptance gate run before publishing.

#### Upgrade

```bash
./leedevkit update --version v0.7.20
```

#### Rollback

```bash
./leedevkit update --version v0.7.19
```

## [0.7.19]

### v0.7.19 — Worktree-Safe Per-Project Install

#### Highlights

- Keep the project-root `leedevkit` launcher tracked by Git.
- Make the launcher self-bootstrapping for fresh clones and missing runtimes.
- Reuse a matching `.leedevkit/` runtime from the main checkout in Git worktrees.
- Add `leedevkit doctor --fix` for runtime links, virtualenvs, and missing AI rulebooks.
- Detect main-repository runtime from `git rev-parse --git-common-dir`.
- Add local-source support to the release builder.

#### Verification

- Full regression suite: 839 passed, 1 skipped.
- Bootstrap, release, usage, and acceptance tests: 76 passed.
- Worktree launcher smoke check passed with a clean worktree.

#### Upgrade

```bash
./leedevkit update --version v0.7.19
```

#### Rollback

```bash
./leedevkit update --version v0.7.18
```

#### Known limitations

- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

## [0.7.18]

### v0.7.18 — Docker Layer Cache in Test Builds

#### Highlights

- Allow Docker layer caching during test image prebuilds by removing forced `--no-cache`.
- Keep `--pull` so updated base images are still fetched when available.
- Update prebuild regression test to verify cached build invocation.

#### Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Prebuild and handler regression tests: passed; 79 passed.
- Full regression suite and release acceptance gate: passing before release.

#### Upgrade

```bash
./leedevkit update --version v0.7.18
```

#### Rollback

```bash
./leedevkit update --version v0.7.17
```

#### Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

## [0.7.17]

### v0.7.17 — Preserve Compose Lookup Environment

#### Highlights

- Preserve the parent process environment when executing commands with custom variables.
- Keep `PATH` available to `_safe_run.py`, including user-installed `podman-compose`.
- Add regression coverage for custom environment merging.

#### Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Bootstrap, lifecycle, and orchestrator tests: passed; 206 passed, 1 skipped.
- Full CI quality and release acceptance gates: required before publication.

#### Upgrade

```bash
./leedevkit update --version v0.7.17
```

#### Rollback

```bash
./leedevkit update --version v0.7.16
```

#### Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

## [0.7.16]

### v0.7.16 — Compose Provider Detection

#### Highlights

- Prefer native `podman compose` when Podman is installed without standalone `podman-compose`.
- Preserve standalone `podman-compose` and Docker Compose detection.
- Replace the unsafe missing-tool fallback with an explicit dependency error.
- Add regression coverage for native Podman Compose and missing Compose implementations.

#### Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Lifecycle and bootstrap tests: passed; 71 passed, 1 skipped.
- Full CI quality and release acceptance gates: required before publication.

#### Upgrade

```bash
./leedevkit update --version v0.7.16
```

#### Rollback

```bash
./leedevkit update --version v0.7.15
```

#### Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

## [0.7.15]

### v0.7.15 — Runtime Guard and Dead-Code Cleanup

#### Highlights

- Enforced project-local `.venv/bin/python3` for direct LeeDevKit Python entrypoints.
- Preserved host-Python execution only for explicit bootstrap paths.
- Migrated CI, release, and documentation examples to project-local Python.
- Removed unused `_docker_ops` Compose wrapper and its obsolete test suite.
- Kept legacy install, global-resolution, and configuration compatibility paths because repository evidence still shows active migration support.

#### Verification

- `./leedevkit test infra --lint-only`: passed.
- `./leedevkit test infra --unit-only`: passed.
- `./leedevkit test all`: passed; 849 passed, 1 skipped.
- `./leedevkit test all --json`: passed; `full_regression=true`, exit code `0`.
- Release build and black-box acceptance: passed.
- Host interpreter smoke: rejected with exit code `125`.
- Project-local interpreter smoke: passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.15
```

#### Rollback

```bash
./leedevkit update --version v0.7.14
```

#### Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
- Legacy install and compatibility paths remain until an explicit deprecation window and migration plan retire them.

## [0.7.14]

### v0.7.14 — Verification, Acquisition, and Supply-Chain Hardening

#### Highlights

- Fixed verifier and checklist applicability semantics for CLI projects.
- Added truthful skip/failure handling, including `--no-e2e` and zero-check protection.
- Fixed infra phase selection and split production/test-source coverage reporting.
- Added bounded downloads, safe archive validation, version checks, and transactional rollback for install, bootstrap, update, and project init.
- Added transactional skill installation/update with exact lock-pin enforcement and rollback.
- Added timeout process-tree ownership and descendant cleanup.
- Added pinned runtime dependencies, lazy Playwright installation, CycloneDX SBOM generation, supply-chain evidence, and artifact checksums.
- Added artifact authenticity/provenance ADR. Current release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.

#### Verification

- `./leedevkit test infra`: 831 passed, 1 skipped; 83.87% production coverage.
- `.venv/bin/python3 scripts/_release_acceptance.py --repo-root .`: passed.
- `python3 .agent/scripts/checklist.py .`: passed; inapplicable checks reported as skips.
- `python3 .agent/scripts/verify_all.py . --no-e2e`: passed; inapplicable checks reported as skips.
- Shell syntax and `git diff --check`: passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.14
```

#### Rollback

```bash
./leedevkit update --version v0.7.13
```

#### Known limitations

- One existing lifecycle test remains skipped.
- Release artifacts are not publisher-signed. See `docs/architecture/adr-001-artifact-authenticity.md` before adopting a trust anchor.

## [0.7.13]

### v0.7.13 — Multi-Harness AI Adapters & Superpowers Methodology

#### Highlights

- Added harness adapter engine (`scripts/_harness_engine.py`) projecting single-source `.agent/` resources into:
  - **Claude Code**: `.claude/skills/`, `.claude/agents/`, and managed `CLAUDE.md`.
  - **Codex / Copilot / Aider / Cline**: `AGENTS.md` with marker blocks.
  - **Cursor**: `.cursor/rules/leedevkit.mdc` (`alwaysApply: true`).
  - **Gemini CLI**: `GEMINI.md` with inlined context and skill index.
  - **Pi**: `.pi/skills/` and `AGENTS.md`.
  - **Oh My Pi**: `.omp/skills/`, `.omp/agents/`, and `.omp/AGENTS.md`.
- Normalized all built-in skills to action-oriented portable guidance with harness capability fallbacks.
- Recursive skill discovery exposing 49 total skills (including nested domain skills such as `game-development/*`).
- Added Superpowers v6.4.1 methodology entry to community skills catalog (`.agent/skills-catalog.toml`).
- Decoupled `leedevkit skills list` and `leedevkit init` from hardcoded `.claude/` assumption.

#### Verification

- Full test suite: 814 passed, 1 skipped.
- Harness engine tests: 7/7 passed (including Pi, OMP, and alias resolution).
- Real binary end-to-end: verified on `omp` v18.2.5 (`omp read skill://api-patterns`, `omp read .omp/AGENTS.md`).
- Release packaging acceptance gate: passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.13
```

#### Rollback

```bash
./leedevkit update --version v0.7.12
```

## [0.7.12]

### v0.7.12 — Nested Go Toolchain Configuration

#### Highlights

- Inject `[services.go].go_version` even when the Go module is nested below the project root.
- Preserve configured nested Go workdir support from v0.7.10.
- Keep the Go container build fix from v0.7.11.

#### Verification

- Nested Go version regression: 38 passed.
- Go 1.24 image build and smoke command passed.
- Release artifact acceptance must pass before publication.

#### Upgrade

```bash
./leedevkit update --version v0.7.12
```

#### Rollback

```bash
./leedevkit update --version v0.7.11
```

## [0.7.11]

### v0.7.11 — Go Container Build Fix

#### Highlights

- Fix the built-in Go test image so its Dockerfile validation command exits successfully.
- Keep nested Go module workdir support from v0.7.10.

#### Verification

- Go 1.24 container image build passed.
- Go image smoke command passed.
- Release artifact acceptance must pass before publication.

#### Upgrade

```bash
./leedevkit update --version v0.7.11
```

#### Rollback

```bash
./leedevkit update --version v0.7.10
```

## [0.7.10]

### v0.7.10 — Nested Go Module Workdir Support

#### Highlights

- Honor `[services.go].workdir` for Go formatting, vetting, unit tests, and coverage.
- Preserve `/workspace` as the fallback when the configured path is absent or does not contain `go.mod`.
- Remove the invalid GitHub Actions pip-cache assumption for repositories without `requirements.txt` or `pyproject.toml`.

#### Verification

- Focused source compilation passed.
- `scripts/tests/test_test_modules.py`: 40 passed.
- Focused Ruff formatting check passed for the changed Python files.
- Release artifact acceptance must pass before publication.

#### Upgrade

```bash
./leedevkit update --version v0.7.10
```

#### Rollback

```bash
./leedevkit update --version v0.7.9
```

## [0.7.9]

### v0.7.9 — Reliability, Verification & Transactional Updates

#### Highlights

- Fix project-local and dogfood launchers so they execute the tracked/local CLI instead of recursively invoking themselves.
- Make `infra --unit-only` execute only infrastructure tests and reject unsupported E2E selection explicitly.
- Report runtime coverage without counting `scripts/tests` as production code; add the missing `types-PyYAML` verification dependency.
- Make global installation and per-project bootstrap transactional, with staged validation, archive link rejection, and rollback preservation for existing installs and project files.
- Make validation runners fail closed when required checkers are missing or no checks execute.
- Terminate owned process trees on task timeout while preserving the `124` timeout contract and unrelated processes.
- Enforce exact Git commit pins for community skills without silently rewriting lock state after checkout failure.
- Add focused regression coverage and a GitHub Actions quality/release acceptance workflow.

#### Verification

- Local source CLI version/help paths: passed.
- `./leedevkit test infra --lint-only`: passed.
- `./leedevkit test infra --unit-only`: passed.
- `./leedevkit test infra`: passed.
- Full test suite: 806 passed, 1 skipped.
- Runtime coverage: 86.18% (threshold 80%).
- Ruff format and lint: clean.
- Mypy: clean across 65 source files.
- Shell syntax checks: passed.
- Release acceptance gate: passed for `leedevkit-0.7.9.tar.gz`.

#### Upgrade

```bash
./leedevkit update --version v0.7.9
```

#### Rollback

```bash
./leedevkit update --version v0.7.8
```

#### Known Risks

- Artifact authenticity still relies on the configured release channel; signed release metadata or an externally trusted digest is not included in this patch.
- GitHub Actions workflow validation occurs after publication in the repository CI environment; the local source and black-box gates above passed before release.

## [0.7.8]

### v0.7.8 — Compose Dependency Sequencing & Dev Project Runs

#### Highlights

- Sequence Podman Compose services according to `depends_on` conditions, including `service_completed_successfully` and `service_healthy`.
- Detect dependency cycles, unknown services, invalid conditions, stale dependency containers, and failed health checks before downstream startup.
- Add `leedevkit run --project dev` support for attaching commands to the long-lived development Compose project without tearing it down.
- Add explicit project selection for `run`, including safe cleanup ownership and service-aware `exec` versus `run` behavior.
- Preserve isolated ephemeral projects for default `test` and `run` invocations.

#### Verification

- Focused orchestration and Compose tests: passed.
- Release acceptance gate: pending.

#### Upgrade

```bash
./leedevkit update --version v0.7.8
```

#### Rollback

```bash
./leedevkit update --version v0.7.7
```

#### Known Risks

- `run --project dev` requires the development Compose files and a running target service when using `exec` paths.
- Podman Compose startup now rejects invalid dependency graphs rather than starting services opportunistically.

## [0.7.7]

### v0.7.7 — Claude Base Context & Release Hygiene

#### Highlights

- Add the **Subagent model selection** section to the `CLAUDE.base.md` template: agents should call `mcp__leedevkit-task-assessor__assess_task` before delegating ambiguous or non-trivial work.
- Ensure every project `CLAUDE.md` references the LeeDevKit base context during `init` and `update`. The check is idempotent and preserves existing user content; it only appends the missing block.
- Fix a mypy type error in the task-assessor MCP entry point (`resp` annotation).
- Reformat `_model_router.py`, `_claude_config.py`, and `_task_assessor_mcp.py` to be Ruff-clean so `leedevkit test all` passes its gate.

#### Verification

- Model routing and base-context tests: passed.
- Full test suite: 779 passed, 1 skipped.
- Coverage: 95.71% (threshold 80%).
- Ruff format + lint: clean. Mypy: clean.
- Release acceptance gate: passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.7
```

#### Rollback

```bash
./leedevkit update --version v0.7.6
```

#### Known Risks

- Existing projects only gain the base-context block on their next `init` or `update`; projects that never run either keep their `CLAUDE.md` untouched.
- The base-context block is appended, not prepended, so it does not reorder existing project instructions.

## [0.7.6]

### v0.7.6 — Claude Code Model Routing

#### Highlights

- Add opt-in project-local model routing for unset Claude Code `Agent` and `Task` models.
- Add the `assess_task` MCP tool for deterministic task-complexity assessment.
- Automatically merge the routing hook into `.claude/settings.json` and the assessor server into `.mcp.json` during init/update.
- Run both integrations with the project-local `.leedevkit/.venv/bin/python3` executable.
- Preserve explicit model selections, existing Claude settings, and user MCP servers.
- Register the community `claude-code-model-router` skill.

#### Configuration

```toml
[ai.model_routing]
enabled = true
default_model = "sonnet"
```

`enabled = false` keeps the installed hook dormant. Explicit `model` values always take precedence.

#### Verification

- Model routing tests: 8 passed.
- Routing and lifecycle tests: 64 passed.
- Full test suite: 778 passed, 1 skipped.
- Ruff: clean.
- Release acceptance gate: passed.

#### Upgrade

```bash
./leedevkit update --version v0.7.6
```

#### Rollback

```bash
./leedevkit update --version v0.7.5
```

#### Known Risks

- Routing uses deterministic keyword and scope heuristics; review explicit model assignments for critical work.
- The feature is disabled by default and requires `enabled = true` to modify unset subagent inputs.

## [0.7.5]

### v0.7.5 — Claude Skill and Agent Optimization

#### Highlights

- Reduce redundant prompt choreography across Claude-facing skills, agents, and orchestration workflows.
- Align agent and skill tool declarations with supported Claude Code tools.
- Repair documented skill script paths so validation commands resolve from the repository root.
- Make community skill installation fail closed on clone, fetch, checkout, and pull failures.
- Remove partial failed clones and preserve existing repositories during failed updates.
- Warn when duplicate skill IDs would otherwise be silently shadowed.
- Add compatibility, path, duplicate-ID, and Git failure-path tests.

#### Verification

- Full test suite: 769 passed, 1 skipped.
- Release acceptance gate: passed.
- Security scan: no real secrets detected; scanner self-patterns and isolated worktree copies produce false positives.
- Lint runner: no linters configured for this project type.

#### Upgrade

```bash
./leedevkit update --version v0.7.5
```

Existing community skill repositories remain in `.leedevkit/skills.d/` during updates.

#### Rollback

If an update fails, the previous installation is preserved. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.4
```

#### Known Risks

- Community repositories must expose a root `SKILL.md` or Claude Code skills under `.claude/skills/`.
- Existing repositories with failed `git pull --ff-only` remain unchanged and are reported as failed.

## [0.7.4]

### v0.7.4 — Safe Source Archive Updates

#### Highlights

- Allow safe relative symlinks inside downloaded archives.
- Continue rejecting absolute links and links escaping the archive root.
- Stop publishing the generated repository-root `leedevkit` symlink in source archives.
- Restore compatibility for upgrades from v0.7.3 and earlier.

#### Upgrade

```bash
./leedevkit update --version v0.7.4
```

This release specifically fixes the rollback seen when updating from v0.7.2 or v0.7.3 through GitHub’s source archive.

#### Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.3
```

#### Security

Archive paths remain traversal-protected. Symlinks are accepted only when their normalized target remains under an archive top-level directory; absolute and escaping targets remain rejected.

## [0.7.3]

### v0.7.3 — Community Plugin Skill Discovery

#### Highlights

- Bridge community skills from Claude Code plugin repositories into `.claude/skills/`.
- Support repositories containing multiple nested skills under `.claude/skills/<skill-id>/`.
- Make `skills list` reflect the same runtime skill registry Claude Code loads.
- Remove all runtime bridges when a community plugin is removed.
- Remove undeclared `tomli_w` usage from tests; test fixtures now use stdlib-compatible TOML.

#### Upgrade

```bash
./leedevkit update --version v0.7.3
```

Existing community skill repositories remain in `.leedevkit/skills.d/`; the update syncs valid Claude Code skill bridges automatically.

#### Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.2
```

#### Known Risks

- Community repositories must expose a root `SKILL.md` or Claude Code skills under `.claude/skills/`.
- User-owned entries in `.claude/skills/` are preserved and never overwritten.

## [0.7.2]

### v0.7.2 — Safe DevKit Updates

#### Highlights

- Preserve installed community skills across `leedevkit update` and `init` replacement flows.
- Preserve `skills.d` directory symlinks and Claude skill bridges.
- Reject unsafe release archive paths, symlinks, and special files before extraction.
- Validate downloaded `VERSION` before replacing the active installation.
- Restore the complete previous installation, including skills, when update fails.

#### Upgrade

```bash
./leedevkit update --version v0.7.2
```

#### Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.1
```

## [0.7.1]

### v0.7.1 — Release Tarball Symlink Protection

#### Highlights

- Remove committed runtime `.claude` symlinks that caused `AbsoluteLinkError` during GitHub archive extraction on Python 3.14 (`tarfile` security filters).
- Add `.claude/` to `.gitignore` so generated bridge artifacts remain project-local.

#### Upgrade

```bash
./leedevkit update --version v0.7.1
```

## [0.7.0]

### v0.7.0 — Claude Code Agent & Skills Standardization

#### Highlights

- Standardize all LeeDevKit specialist agent definitions with official YAML frontmatter (`name`, `description`, `tools`, `model`).
- Add official `api-designer` agent definition and normalize core persona metadata across the catalog.
- Restrict agent execution tooling to supported Claude Code primitives (`Read`, `Write`, `Edit`, `Bash`, `Agent`, `Workflow`, `Grep`, `Glob`).
- Remove legacy Antigravity/Gemini residue and convert routing and skill invocations to standard `Agent(...)` and `Skill(...)` protocols.
- Upgrade resource bridge to discover and symlink community add-on skills from `skills.d/` into `.claude/skills/`.
- Add comprehensive referential-integrity and metadata test coverage.

#### Upgrade

```bash
./leedevkit update --version v0.7.0
```

## [0.6.0]

### v0.6.0 — Claude Code Resource Bridge

#### Highlights

- Bridge LeeDevKit specialist agents (`.agent/agents/*.md`) into `.claude/agents/` automatically on `init` and `update`.
- Bridge built-in skills (`.agent/skills/*/SKILL.md`) into `.claude/skills/` automatically on `init` and `update`.
- Preserve user-authored files, agents, and custom skills in `.claude/` without overwrite or deletion.
- Prune only stale devkit-managed symlinks when devkit resources are removed or renamed.
- Keep `.claude/settings.local.json` untracked for local permissions, document project settings standard in `.claude/settings.json`.

#### Upgrade

```bash
./leedevkit update --version v0.6.0
```

## [0.5.0]

### v0.5.0 — Configurable lifecycle dependencies

#### Highlights

- Add project-configured Compose lifecycle dependencies through `[test.dependencies]`.
- Start the lifecycle mode's primary service and configured dependencies in one isolated Compose project.
- Wait for healthchecks of primary and dependency services.
- Support Docker Compose and Podman Compose.
- Validate malformed configuration and unknown Compose services with clear diagnostics.
- Preserve existing unit/lint behavior, Compose networking, and dynamic project isolation.

#### Configuration

```toml
[test.dependencies]
"int-go" = ["postgres"]
```

The key is LeeDevKit lifecycle mode. Values are Compose service names. For a
multi-service chain, list each required service explicitly:

```toml
[test.dependencies]
"int-go" = ["A", "B", "C", "D"]
```

#### Verification

- `740 passed, 1 skipped`
- `./leedevkit test infra --unit-only` passed
- Docker Compose lifecycle integration test passed

#### Upgrade

```bash
./leedevkit update --version v0.5.0
```

## [0.4.0]

### v0.4.0 — First-class Go support

#### Highlights

leedevkit now supports Go modules as a built-in project type, alongside Rust and web projects.

#### Features

- Auto-detects `go.mod` during `init` and generates Go project configuration.
- Adds built-in Go Dockerfile and Compose profile with dependency and build caches.
- Runs `gofmt`, `go vet`, `go test ./...`, and Go coverage through project-local containers.
- Supports Go test pattern filtering through shell-safe `--pattern` handling.
- Adds `run go ...` with Go-specific service, profile, and compose selection.
- Supports `GO_VERSION` environment overrides and `[services.<name>] go_version` configuration.
- Selects language-specific built-in Compose files without guessing in mixed Rust/Go projects.
- Adds Go-only and mixed-language pipeline regression coverage.
- Documents Go setup, commands, container overrides, and mixed-project requirements.

#### Compatibility

Existing Rust, TypeScript, JavaScript, and Python workflows retain prior behavior. Mixed Rust/Go or Rust/Go/Web projects must provide `.compose/docker-compose.test.yml` defining required services.

#### Upgrade

```bash
./leedevkit update --version v0.4.0
```

#### Rollback

If needed, pin previous release:

```bash
./leedevkit update --version v0.3.18
```

#### Verification

- Python modules compile successfully.
- Go-focused regression tests pass.
- Targeted suite passes: 326 tests.
- Release artifact integrity verification runs through existing release pipeline.

## [0.3.18]

### v0.3.18 — Show complete failure logs

#### Highlights

This patch release improves failure diagnostics in the parallel test runner.

#### Bug fixes

- Displays the complete task log when a parallel test, lint, integration, or coverage task fails instead of truncating output to the last 30 lines.
- Preserves every diagnostic line in the existing `.test_logs/` artifact and adds regression coverage for long failure logs.

#### Upgrade

```bash
./leedevkit update --version v0.3.18
```

#### Rollback

If needed, pin the previous release:

```bash
./leedevkit update --version v0.3.17
```

## [0.3.16]

### v0.3.16 — Enterprise AI governance and development quality gates

#### Highlights

This release upgrades LeeDevKit's shared AI rule system with deterministic governance, enterprise control domains, and a mandatory development–testing workflow designed to prevent agents from stopping at happy-path implementation.

#### Development and testing

- Adds a primary development workflow that treats implementation and testing as one indivisible task.
- Requires a pre-implementation Test Impact Matrix covering success, boundaries, malformed input, negative paths, state transitions, dependency failures, timeouts, retries, compatibility, security, and resource limits.
- Requires bug fixes to include regression tests that fail without the fix when reproducible.
- Strengthens edge-case, negative-side-effect, skipped-test, and flaky-test controls.
- Requires completion reports to identify scenarios, commands, pass/fail/skip results, unverified checks, and remaining risks.

#### AI governance

- Adds rule priority, applicability, severity levels, exception handling, verification discovery, and a shared Definition of Done.
- Prevents Boy Scout cleanup from broadening user authorization.
- Replaces universal over-engineering requirements with risk-based production readiness.
- Removes forced clarification questions and unconditional output redirection from the generated AI context.
- Makes project wrappers conditional on actual repository configuration.

#### Enterprise rulebooks

Adds enterprise governance for:

- Secure development lifecycle and threat modeling
- Vulnerability intake, remediation targets, risk acceptance, and closure
- Architecture decisions, boundaries, review, and evolution
- API and event contracts, compatibility, idempotency, and verification
- Reliability objectives, dependency resilience, capacity, and operational readiness
- Safe migrations, backfills, rollback, roll-forward, and reconciliation
- Reproducible releases, provenance, rollout, and post-release verification

#### Rule quality and portability

- Replaces product-specific project-layout assumptions with repository discovery rules.
- Removes stale references and unverifiable command assumptions.
- Standardizes technical rule content in English.
- Adds explicit dead-code prevention, verification, deprecation, and time-bound exception rules.

#### Upgrade

```bash
./leedevkit update --version v0.3.16
```

## [0.3.5]

### v0.3.5 — Project-agnostic custom linters

#### Correctness fixes

##### Custom linters skip gracefully outside the apiserver layout

The two custom Python linters previously treated a missing target directory as a
fatal error (`exit 1`), which broke `leedevkit test` lint gates in any project
that doesn't use the multi-tenant Rust apiserver layout (e.g. libraries):

- `lint_tenant_isolation.py` required `apiserver/src/repositories`
- `lint_clean_code.py` required `apiserver/src/domain`

Both linters now **skip gracefully** (`exit 0`) with an explanatory message when
their target directory is absent, since these checks are specific to the
apiserver layout and do not apply to other projects.

#### Upgrade

```bash
./leedevkit update --version v0.3.5
```

## [0.3.4]

### v0.3.4 — Black-Box Release Acceptance Gate

#### What's New

##### End-to-end release acceptance

LeeDevKit now builds and verifies the real release artifact before packaging is considered complete:

```bash
python3 scripts/_release_build.py --repo-root . --output /tmp/dist
python3 scripts/_release_acceptance.py \
  --repo-root . \
  --artifact /tmp/dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```

The gate proves:

- the tarball contains the staged allowlist and a fresh integrity manifest
- extracted `bin/leedevkit version|help` works offline without creating `.venv`
- `bootstrap.sh` can install from a local release mirror
- failed bootstrap does not destroy an existing `.leedevkit`

##### Safer packaging and install paths

- Release builds stage payload first, generate `devkit.manifest.json` from that payload, verify it, then archive
- Runtime-generated paths (`.venv/`, `skills.d/`, `dev-state.json`) are ignored by integrity checks
- `bootstrap.sh` validates artifact structure/version/integrity before promoting into `.leedevkit`
- Existing installs are preserved when bootstrap fails
- `LEEDEVKIT_RELEASE_BASE_URL` supports deterministic local bootstrap tests

##### Offline CLI identity paths

`leedevkit version`, `--help`, and unknown top-level commands no longer initialize a virtualenv first. Python-backed commands still bootstrap lazily.

##### Non-mutating verification

`./leedevkit test infra` now runs `ruff format --check` instead of rewriting source during verification.

##### AI agent guidance

Shipped Claude/base context and rulebooks now require the release packaging gate for packaging, bootstrap, install, update, versioning, and release-layout changes. Source-only unit tests are no longer treated as sufficient for those paths.

##### Correctness fixes

- Devkit discovery no longer mistakes the project-local `leedevkit` CLI symlink for a bundled install root
- Manifest version/structure mismatches fail integrity verification

#### Upgrade

```bash
./leedevkit update --version v0.3.4
```

## [0.3.3]

### v0.3.3 — Mandatory Post-Implementation Test Gate

#### What's New

##### Explicit test-impact decision after every implementation

AI agents must now inspect affected tests after every code or behavior change and report one explicit outcome:

- Tests were added or updated, including the covered scenarios; or
- No test change is needed, with a concrete behavior-based justification.

The decision stays with the agent instead of being deferred to the user.

##### Mandatory regression protection

Tests are now explicitly required for new or changed behavior, bug fixes, business logic, validation, authorization, persistence, API contracts, concurrency, and error paths. Bug-fix tests must fail when the fix is removed.

Documentation-only, comment-only, formatting-only, and generated-artifact changes may omit test changes, but the agent must still document why behavior cannot change.

##### Consistent hermetic verification

Test guidance across rulebooks, workflows, specialist-agent context, and the base AI prompt now consistently uses the project-local wrapper:

```bash
./leedevkit test <target>
```

This removes conflicting `./test.sh` and direct `npm test` instructions from the shipped AI context.

##### Regression tests

Added release tests that guard the mandatory test gate, its justified exceptions, and the canonical LeeDevKit test command in shipped templates and rules.

#### Upgrade

```bash
./leedevkit update --version v0.3.3
```

## [0.3.1]

### v0.3.1 — Architecture Refactor & Technical Debt Cleanup

#### What's New

##### 4-round Technical Debt Resolution

This release completes 4 consecutive refactoring rounds targeting the most serious
structural issues in the codebase — all while maintaining 97.68% test coverage.

###### Round 1 — God Class Decomposition
- Extracted `SkillsManager` for community skill lifecycle (install/list/update/remove)
- Extracted `_lock_manager.py` for OS-level file locking
- Decomposed monolithic checker modules into focused single-responsibility files

###### Round 2 — SRP & DRY Cleaning
- Extracted `_logging.py` — shared logging module (eliminated copy-pasted logging functions)
- Extracted `DbHandler`, `TestHandler`, `RunHandler`, `InitHandler` from Orchestrator
- DRY checker patterns via `CheckerBase` shared class across UX and mobile auditors

###### Round 3 — DRY psql & Exception Hardening
- `_exec_psql()` single canonical psql execution in `DbHandler`
- Replaced bare `except Exception` with specific exception types
- Added release build tests (`test_release_build.py`)

###### Round 4 — Final SRP & Dependency Cleanup
- Extracted `HandlerBase` — eliminated ~73 lines of duplicated forwarding boilerplate
  across all 4 handler classes
- Extracted `_doctor.py` (system health check) and `_update_handler.py` (self-update)
  from Orchestrator — reduced from 560 to 368 lines (-34%)
- Fixed wrong import direction: `_skills_manager.py` now imports logging from `_logging`
  instead of `_orchestrator`
- `build_mode_map()` and `inject_rust_version_env()` moved to `_devkit_config.py`
  as shared config utilities

##### New Test Coverage
- `test_doctor.py` — 16 tests for system health check (100% coverage)
- `test_update_handler.py` — 6 tests for self-update flow (97% coverage)
- `test_handlers.py` — 32 tests for all handler classes
- `test_skills_manager.py` — full community skill lifecycle tests

##### Bug Fix
- Venv localized inside DevKit root (not CWD)
- Lazy Chromium import in Playwright runner
- Deduplicated bootstrap path resolution

#### Stats

```
610 tests passed, 0 failures
97.68% code coverage (>80% threshold)
71 files changed, +9827 / -3208 lines
```

#### Upgrade

```bash
./leedevkit update        # auto-update to v0.3.1
```

## [0.3.0]

### v0.3.0 — Rust Container Mode

#### What's New

##### Zero-setup Rust projects

`leedevkit init` now auto-detects `Cargo.toml` and scaffolds the right config.
No Dockerfile, no docker-compose.yml, no database — `./leedevkit test all` just works.

```bash
cd my-rust-crate && git init
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
./leedevkit test all   # cargo fmt → clippy → nextest → llvm-cov
```

##### Built-in container infrastructure

The devkit ships a default Docker image and compose file for Rust projects at
`.leedevkit/container/rust/`. Includes `rust:1.85`, `clippy`, `rustfmt`,
`cargo-nextest`, and `cargo-llvm-cov`. Nothing to install, nothing to configure.

**Language-specific layout** — `container/rust/`, `container/go/` (future), etc.
Adding a new language is a directory; no code changes needed.

##### Configurable Rust version

Two ways to override (higher priority first):

```bash
RUST_VERSION=1.83 ./leedevkit test all         # env var (ad-hoc, CI)
```

```toml
[services.rust]
rust_version = "1.83"                           # leedevkit.toml (committed)
```

Default: `1.85`.

##### Compose file priority

```
1. .compose/docker-compose.test.yml    (your override)
2. .leedevkit/container/<lang>/        (devkit default, auto-discovered)
```

##### Dynamic service detection

The pipeline reads `leedevkit.toml` and auto-detects the Rust service name
instead of hardcoding `"apiserver"`. Backward compatible — old projects
with `[services.apiserver]` work unchanged.

##### Tests

- 36 new tests covering service auto-detection, init Cargo.toml detection,
  compose resolution priority, Rust version injection, and container
  subdirectory auto-discovery.
- 403 total tests passing (up from 367).

## [0.2.0]

### v0.2.0 — Self-Update (`leedevkit update`)

#### What's New

##### `leedevkit update` — self-update from GitHub Releases

DevKit can now upgrade itself in place. No more re-running `init --force` by hand when a new version ships.

```bash
leedevkit update                 # Update to the latest release
leedevkit update --version v0.3.0   # Pin to a specific release
```

Under the hood:

- Queries the GitHub `releases/latest` API to discover the newest tag.
- Compares against the installed `VERSION`; if already current, it's a no-op.
- Backs up the current install to `<root>.bak/` before overwriting, so a manual rollback is always possible.
- Downloads the release tarball and overlays it onto the install.
- On any download or extract failure, automatically rolls back to the previous version and reports the error.

##### Tests

- 13 new tests covering the update parser, GitHub version lookup, and the full update flow (already-latest, success, rollback, latest-release resolution).
- 3 subprocess smoke tests verifying `update --help`, the already-latest path, and `bin/leedevkit` routing.

## [0.1.0]

### v0.1.0 — Per-Project DevKit Install

#### Breaking Change

DevKit no longer requires global installation. Each project now self-contains its own devkit copy at `.leedevkit/`.

#### What's New

##### Per-project install (no sharing)

Running `leedevkit init` downloads the devkit release into your project:

```
my-project/
├── .leedevkit/           # Self-contained devkit (gitignored)
│   ├── scripts/, templates/, bin/, .agent/
│   ├── .venv/            # Project-local Python env
│   ├── skills.d/         # Community skills
│   └── VERSION
├── .agent/rules/         # Project rules (real files, not symlinks)
└── leedevkit → .leedevkit/bin/leedevkit
```

##### AI Agent friendly

All directories an AI Agent needs to write (`.venv/`, `skills.d/`, `.agent/`) now live **inside the project root** — no permission issues with global install.

##### Legacy symlink detection

`init` detects old symlinks from global install and warns with cleanup instructions (no auto-delete).

##### Release tarball builder

`scripts/_release_build.py` builds release artifacts for GitHub releases.

#### Migration

##### New projects

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
cd my-project && git init
leedevkit init
```

##### Existing projects (with old global symlinks)

```bash
cd my-project
rm -rf .agent                      # Remove legacy symlinks
leedevkit init                      # Re-init clean
```

`init` will warn if legacy symlinks are detected.

#### Important: Use `./leedevkit` not `leedevkit`

After `init`, always use `./leedevkit` (project-local wrapper). Using plain `leedevkit`
runs the global bootstrap binary at `~/.leedevkit/`, which is outside your project.

#### Files Changed

- `scripts/_orchestrator.py` — Rewrite `handle_init()`, per-project install flow
- `scripts/_devkit_config.py` — Per-project root resolution priority
- `scripts/_bootstrap.py` — Add `DEVKIT_ROOT` constant
- `scripts/_ensure-venv.sh` — Venv path → `.leedevkit/.venv/`
- `scripts/_release_build.py` — New: release tarball builder
- `bin/leedevkit` — Self-resolving wrapper
- `templates/leedevkit.default.toml` — Add `install` + `source` config
- `.gitignore` — Ignore `.leedevkit/` and `leedevkit` wrapper
- `README.md` — Updated Quick Start + migration guide

#### Tests

- 372 tests pass (358 existing + 14 new)
- New tests cover: per-project init, tarball download, legacy symlink detection, devkit root resolution priority

#### Install Strategies (in order)

1. `DEVKIT_LOCAL_PATH` env — for development
2. GitHub release tarball — for production
3. Copy from current source — fallback for dogfooding
