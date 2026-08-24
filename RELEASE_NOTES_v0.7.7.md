# v0.7.7 — Claude Base Context & Release Hygiene

## Highlights

- Add the **Subagent model selection** section to the `CLAUDE.base.md` template: agents should call `mcp__leedevkit-task-assessor__assess_task` before delegating ambiguous or non-trivial work.
- Ensure every project `CLAUDE.md` references the LeeDevKit base context during `init` and `update`. The check is idempotent and preserves existing user content; it only appends the missing block.
- Fix a mypy type error in the task-assessor MCP entry point (`resp` annotation).
- Reformat `_model_router.py`, `_claude_config.py`, and `_task_assessor_mcp.py` to be Ruff-clean so `leedevkit test all` passes its gate.

## Verification

- Model routing and base-context tests: passed.
- Full test suite: 779 passed, 1 skipped.
- Coverage: 95.71% (threshold 80%).
- Ruff format + lint: clean. Mypy: clean.
- Release acceptance gate: passed.

## Upgrade

```bash
./leedevkit update --version v0.7.7
```

## Rollback

```bash
./leedevkit update --version v0.7.6
```

## Known Risks

- Existing projects only gain the base-context block on their next `init` or `update`; projects that never run either keep their `CLAUDE.md` untouched.
- The base-context block is appended, not prepended, so it does not reorder existing project instructions.
