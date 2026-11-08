## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`templates/CLAUDE.base.md`.

The rules below add specific constraints. Apply both.

<!-- leedevkit:begin -->
## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`.leedevkit/templates/CLAUDE.base.md`.

Project policy lives in `.agent/`; devkit sources live at:
- Built-in skills: `.leedevkit/.agent/skills/*/SKILL.md`
- Community skills: `.leedevkit/skills.d/*/SKILL.md`
- Built-in agents: `.leedevkit/.agent/agents/*.md`
- Project agents: `.agent/agents/*.md`

Run `./leedevkit --help` for available project commands.
<!-- leedevkit:end -->
