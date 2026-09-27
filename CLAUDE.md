## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`.leedevkit/templates/CLAUDE.base.md`.

The rules below add specific constraints. Apply both.

<!-- leedevkit:begin -->
## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`.leedevkit/templates/CLAUDE.base.md`.

Source of truth for this project's AI context lives in `.agent/`:

| Purpose | Location |
|---------|----------|
| Rulebooks (lazy-load by domain) | `.agent/rules/*.md` |
| Built-in skills | `.agent/skills/*/SKILL.md` |
| Installed community skills | `.leedevkit/skills.d/*/SKILL.md` |
| Specialist agents | `.agent/agents/*.md` |
| Devkit commands | `./leedevkit --help` |

Capability fallback: if this harness cannot run shell commands, dispatch
subagents, or track todos, follow the skill's prose directly and report the
missing capability. Never invent a tool call the harness does not provide.

### Available skills

_No skills discovered in this devkit._
<!-- leedevkit:end -->
