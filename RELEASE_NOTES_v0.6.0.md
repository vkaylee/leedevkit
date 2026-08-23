# v0.6.0 — Claude Code Resource Bridge

## Highlights

- Bridge LeeDevKit specialist agents (`.agent/agents/*.md`) into `.claude/agents/` automatically on `init` and `update`.
- Bridge built-in skills (`.agent/skills/*/SKILL.md`) into `.claude/skills/` automatically on `init` and `update`.
- Preserve user-authored files, agents, and custom skills in `.claude/` without overwrite or deletion.
- Prune only stale devkit-managed symlinks when devkit resources are removed or renamed.
- Keep `.claude/settings.local.json` untracked for local permissions, document project settings standard in `.claude/settings.json`.

## Upgrade

```bash
./leedevkit update --version v0.6.0
```
