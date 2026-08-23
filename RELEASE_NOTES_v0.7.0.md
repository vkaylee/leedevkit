# v0.7.0 — Claude Code Agent & Skills Standardization

## Highlights

- Standardize all LeeDevKit specialist agent definitions with official YAML frontmatter (`name`, `description`, `tools`, `model`).
- Add official `api-designer` agent definition and normalize core persona metadata across the catalog.
- Restrict agent execution tooling to supported Claude Code primitives (`Read`, `Write`, `Edit`, `Bash`, `Agent`, `Workflow`, `Grep`, `Glob`).
- Remove legacy Antigravity/Gemini residue and convert routing and skill invocations to standard `Agent(...)` and `Skill(...)` protocols.
- Upgrade resource bridge to discover and symlink community add-on skills from `skills.d/` into `.claude/skills/`.
- Add comprehensive referential-integrity and metadata test coverage.

## Upgrade

```bash
./leedevkit update --version v0.7.0
```
