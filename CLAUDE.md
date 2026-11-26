<!-- leedevkit:start -->
## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`templates/CLAUDE.base.md`.

Project policy lives in `.agent/`; devkit sources live at:
- Built-in skills: `.agent/skills/*/SKILL.md`
- Community skills: `skills.d/*/SKILL.md`
- Built-in agents: `.agent/agents/*.md`
- Project agents: `.agent/agents/*.md`

Use globally installed `leedevkit --help` for available project commands. This repository's `./leedevkit` launcher requires a local Go binary and is not the default entry point; use it only when that binary exists and is executable.
<!-- leedevkit:end -->

The rules below add specific constraints. Apply both.

## Repository verification

- Prefer `leedevkit test all --compact` for configured regression checks (use `--lint-only` or `--unit-only` for fast iteration). Use Docker directly only when the target cannot exercise the required case; state why.
- The global CLI runs checks against the current source but may be older than it. To verify changes to CLI behavior, run `scripts/build-local.sh` (builds gitignored `build/leedevkit-go` via host Go or Compose) and then invoke `./leedevkit ...`. If `build/leedevkit-go` is absent, use the global CLI for regression checks rather than assuming `./leedevkit` runs current CLI code.
- Focused checks supplement, not replace, full regression.
