# v0.7.22 — Per-Project Worktree Bootstrap

## Highlights

- Commit a root `leedevkit` launcher while keeping `.leedevkit/` runtime data ignored.
- Bootstrap missing runtimes from the matching main Git worktree or `DEVKIT_HOME` only when versions match.
- Atomically install pinned releases with concurrent-bootstrap locking and rollback-safe replacement.
- Make `doctor` read-only by default; `doctor --fix` repairs safe DevKit runtime, virtualenv, and missing AI rulebook state.
- Migrate generated `.gitignore` files to ignore only runtime and bootstrap staging artifacts, never the committed launcher.

## Verification

- Full wrapper suite: 843 passed, 1 skipped; 83.12% coverage.
- Ruff format and lint passed.
- Mypy passed for 68 source files.
- Release artifact acceptance passed.
- Fresh Git worktree smoke test passed for read-only `doctor` and runtime repair.

## Upgrade

```bash
./leedevkit update --version v0.7.22
```

Existing projects can also run:

```bash
./leedevkit doctor --fix
```

## Rollback

```bash
./leedevkit update --version v0.7.21
```

The previous runtime remains preserved when a bootstrap download or extraction fails.
