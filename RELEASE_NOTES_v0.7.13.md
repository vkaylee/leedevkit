# v0.7.13 — Multi-Harness AI Adapters & Superpowers Methodology

## Highlights

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

## Verification

- Full test suite: 814 passed, 1 skipped.
- Harness engine tests: 7/7 passed (including Pi, OMP, and alias resolution).
- Real binary end-to-end: verified on `omp` v18.2.5 (`omp read skill://api-patterns`, `omp read .omp/AGENTS.md`).
- Release packaging acceptance gate: passed.

## Upgrade

```bash
./leedevkit update --version v0.7.13
```

## Rollback

```bash
./leedevkit update --version v0.7.12
```
