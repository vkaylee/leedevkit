# v0.7.5 — Claude Skill and Agent Optimization

## Highlights

- Reduce redundant prompt choreography across Claude-facing skills, agents, and orchestration workflows.
- Align agent and skill tool declarations with supported Claude Code tools.
- Repair documented skill script paths so validation commands resolve from the repository root.
- Make community skill installation fail closed on clone, fetch, checkout, and pull failures.
- Remove partial failed clones and preserve existing repositories during failed updates.
- Warn when duplicate skill IDs would otherwise be silently shadowed.
- Add compatibility, path, duplicate-ID, and Git failure-path tests.

## Verification

- Full test suite: 769 passed, 1 skipped.
- Release acceptance gate: passed.
- Security scan: no real secrets detected; scanner self-patterns and isolated worktree copies produce false positives.
- Lint runner: no linters configured for this project type.

## Upgrade

```bash
./leedevkit update --version v0.7.5
```

Existing community skill repositories remain in `.leedevkit/skills.d/` during updates.

## Rollback

If an update fails, the previous installation is preserved. To pin the previous release manually:

```bash
./leedevkit update --version v0.7.4
```

## Known Risks

- Community repositories must expose a root `SKILL.md` or Claude Code skills under `.claude/skills/`.
- Existing repositories with failed `git pull --ff-only` remain unchanged and are reported as failed.
