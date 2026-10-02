# v0.7.19 — Worktree-Safe Per-Project Install

## Highlights

- Keep the project-root `leedevkit` launcher tracked by Git.
- Make the launcher self-bootstrapping for fresh clones and missing runtimes.
- Reuse a matching `.leedevkit/` runtime from the main checkout in Git worktrees.
- Add `leedevkit doctor --fix` for runtime links, virtualenvs, and missing AI rulebooks.
- Detect main-repository runtime from `git rev-parse --git-common-dir`.
- Add local-source support to the release builder.

## Verification

- Full regression suite: 839 passed, 1 skipped.
- Bootstrap, release, usage, and acceptance tests: 76 passed.
- Worktree launcher smoke check passed with a clean worktree.

## Upgrade

```bash
./leedevkit update --version v0.7.19
```

## Rollback

```bash
./leedevkit update --version v0.7.18
```

## Known limitations

- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
