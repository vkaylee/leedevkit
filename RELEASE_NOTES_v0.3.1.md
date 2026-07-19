# v0.3.1 — Architecture Refactor & Technical Debt Cleanup

## What's New

### 4-round Technical Debt Resolution

This release completes 4 consecutive refactoring rounds targeting the most serious
structural issues in the codebase — all while maintaining 97.68% test coverage.

#### Round 1 — God Class Decomposition
- Extracted `SkillsManager` for community skill lifecycle (install/list/update/remove)
- Extracted `_lock_manager.py` for OS-level file locking
- Decomposed monolithic checker modules into focused single-responsibility files

#### Round 2 — SRP & DRY Cleaning
- Extracted `_logging.py` — shared logging module (eliminated copy-pasted logging functions)
- Extracted `DbHandler`, `TestHandler`, `RunHandler`, `InitHandler` from Orchestrator
- DRY checker patterns via `CheckerBase` shared class across UX and mobile auditors

#### Round 3 — DRY psql & Exception Hardening
- `_exec_psql()` single canonical psql execution in `DbHandler`
- Replaced bare `except Exception` with specific exception types
- Added release build tests (`test_release_build.py`)

#### Round 4 — Final SRP & Dependency Cleanup
- Extracted `HandlerBase` — eliminated ~73 lines of duplicated forwarding boilerplate
  across all 4 handler classes
- Extracted `_doctor.py` (system health check) and `_update_handler.py` (self-update)
  from Orchestrator — reduced from 560 to 368 lines (-34%)
- Fixed wrong import direction: `_skills_manager.py` now imports logging from `_logging`
  instead of `_orchestrator`
- `build_mode_map()` and `inject_rust_version_env()` moved to `_devkit_config.py`
  as shared config utilities

### New Test Coverage
- `test_doctor.py` — 16 tests for system health check (100% coverage)
- `test_update_handler.py` — 6 tests for self-update flow (97% coverage)
- `test_handlers.py` — 32 tests for all handler classes
- `test_skills_manager.py` — full community skill lifecycle tests

### Bug Fix
- Venv localized inside DevKit root (not CWD)
- Lazy Chromium import in Playwright runner
- Deduplicated bootstrap path resolution

## Stats

```
610 tests passed, 0 failures
97.68% code coverage (>80% threshold)
71 files changed, +9827 / -3208 lines
```

## Upgrade

```bash
./leedevkit update        # auto-update to v0.3.1
```
