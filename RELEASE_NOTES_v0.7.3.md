# v0.7.3 — Community Plugin Skill Discovery

## Highlights

- Bridge community skills from Claude Code plugin repositories into `.claude/skills/`.
- Support repositories containing multiple nested skills under `.claude/skills/<skill-id>/`.
- Make `skills list` reflect the same runtime skill registry Claude Code loads.
- Remove all runtime bridges when a community plugin is removed.
- Remove undeclared `tomli_w` usage from tests; test fixtures now use stdlib-compatible TOML.

## Upgrade

```bash
./leedevkit update --version v0.7.3
```

Existing community skill repositories remain in `.leedevkit/skills.d/`; the update syncs valid Claude Code skill bridges automatically.

## Rollback

If an update fails, the previous installation is restored automatically. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.2
```

## Known Risks

- Community repositories must expose a root `SKILL.md` or Claude Code skills under `.claude/skills/`.
- User-owned entries in `.claude/skills/` are preserved and never overwritten.
