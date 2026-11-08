# Optimize AI Context for Modern Harnesses

Status: Implemented; verification limited by unrelated repository formatting

## Goal and boundaries
Keep project invariants explicit; remove obsolete executor workarounds, duplicated gates, and universalized product policy. Preserve prior redirect-removal changes, user-owned context, configured isolation, data safety, authorization, and compatibility controls. No new dependency, runner, router, or compliance framework. Do not assume every harness exposes the same capabilities.

## Tasks
1. [x] **Define ownership and precedence.** Project policy is subordinate to harness/system controls; external content is data. Core, project facts, behavior verification, execution gates, and risk-triggered domain rules now have explicit owners.
2. [x] **Remove executor folklore.** Execution rules now defer process control to available harness mechanisms, preserve quoting/exit status and discovered wrappers, and remove blanket shell-syntax bans and mandatory redirection. Affected shell/routing/clean-code skills were reconciled; nonexistent runner advice and wording-only tests were removed.
3. [x] **Deduplicate development/testing guidance.** `development-workflow.md` owns the risk-proportionate behavior loop; `testing-standards.md` owns execution, coverage, and evidence. Coding standards reference these owners; no universal checklist or percentage is imposed.
4. [x] **Scope all domain policy.** All 26 rulebooks reviewed across owners; 18 domain books reviewed and 17 changed. Auth, tenant isolation, secrets, data integrity, and destructive approval remain; architecture choices and operational/legal limits are project-owned, with safe rollback/roll-forward guidance.
5. [x] **Fix source/install context resolution.** Context, Claude hook, and MCP commands now use source, installed, or external runtime paths; migrated callers and previously generated entries while preserving unrelated configuration. Removed unused legacy context helper. Six-adapter sync and actual source/installed hooks/MCP passed in disposable fixtures with spaces in paths.
6. [x] **Slim and reconcile the entry context.** Base is 37 lines / 3,119 bytes (was 92 / 5,448); 26 rulebooks total 936 lines / 71,333 bytes (was 1,636 / 105,297). Updated affected skills/agents, README, release notes, and source root context; no forced rescans or unsupported public DB command in base. Native Claude/OMP discovery avoids duplicated skill tables; Pi shares Codex's fallback index to keep shared `AGENTS.md` idempotent. Sizes are on disk, not injected tokens.
7. [x] **Verification — final phase.** Source/install path, native discovery, shared Pi/Codex context, repeated sync, and user-content preservation are covered by behavior tests and smoke. Q&A, small-fix, auth, migration, and release trigger guidance reviewed; policy and path reviews found no material issues. Verification outcomes and limitations below; no live six-harness claim.

Dependencies: 1 first; 2–4 can proceed independently with distinct file ownership; 5 follows the agreed path contract; 6 integrates all changes; 7 runs after integration. One owner handles shared files and final verification.

## Verification commands and acceptance
```bash
./leedevkit --help
./leedevkit test all --json
.venv/bin/python3 scripts/_release_build.py --repo-root . --output /tmp/leedevkit-context-dist
.venv/bin/python3 scripts/_release_acceptance.py --repo-root . --artifact /tmp/leedevkit-context-dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```
Done when source and installed references resolve, project-owned content survives sync, obsolete terminal requirements and contradictory policy are gone across affected consumers, entry context is smaller, and executed checks have accurate evidence. Dry-run output is command-construction evidence only, never a test pass.

## Observed verification
- `./leedevkit --help`: passed.
- Disposable source/installed fixtures: six adapter projections and repeated sync passed; actual routing hook returned `haiku` for lookup input; MCP `tools/list` exposed `assess_task`; user context/settings survived. Throwaway script removed.
- `./leedevkit test all --json`: failed at repository-wide formatting. Diagnostic `ruff format --diff scripts` identified only `scripts/tests/test_orchestrator.py` and `scripts/tests/test_usage.py`, which are outside this change; left user-owned formatting untouched.
- `./leedevkit test all --unit-only --json`: 856 passed, 1 existing Compose integration test skipped (requires `LEEDEVKIT_RUN_COMPOSE_INTEGRATION=1`); production coverage 83.28% meets configured 80%. Wrapper correctly reports partial, not full regression.
- Scoped Ruff format check (11 changed Python files): passed. `.venv/bin/ruff check scripts`: passed. `.venv/bin/mypy scripts`: success across 67 files; informational note about untyped function bodies remains.
- Final release build and acceptance commands above: passed for `/tmp/leedevkit-context-dist/leedevkit-0.8.0.tar.gz`.
- Full regression remains unverified because the full gate did not pass; no formatter, test gate, or assertion was suppressed. Removed obsolete prose/implementation-pinning tests instead of re-pinning wording.

Out of scope: changing production auth/DB behavior, enforcing new operational SLAs, exposing additional CLI commands, or fixing the independently observed `--dry-run` JSON success/full-regression classification. Report that CLI defect separately; do not conceal it with context wording.
