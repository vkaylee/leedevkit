# v0.7.6 — Claude Code Model Routing

## Highlights

- Add opt-in project-local model routing for unset Claude Code `Agent` and `Task` models.
- Add the `assess_task` MCP tool for deterministic task-complexity assessment.
- Automatically merge the routing hook into `.claude/settings.json` and the assessor server into `.mcp.json` during init/update.
- Run both integrations with the project-local `.leedevkit/.venv/bin/python3` executable.
- Preserve explicit model selections, existing Claude settings, and user MCP servers.
- Register the community `claude-code-model-router` skill.

## Configuration

```toml
[ai.model_routing]
enabled = true
default_model = "sonnet"
```

`enabled = false` keeps the installed hook dormant. Explicit `model` values always take precedence.

## Verification

- Model routing tests: 8 passed.
- Routing and lifecycle tests: 64 passed.
- Full test suite: 778 passed, 1 skipped.
- Ruff: clean.
- Release acceptance gate: passed.

## Upgrade

```bash
./leedevkit update --version v0.7.6
```

## Rollback

```bash
./leedevkit update --version v0.7.5
```

## Known Risks

- Routing uses deterministic keyword and scope heuristics; review explicit model assignments for critical work.
- The feature is disabled by default and requires `enabled = true` to modify unset subagent inputs.
