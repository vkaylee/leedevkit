# v0.7.24 — Repair Missing Runtime Manifests

## Highlights

- Make `doctor --fix` restore a standalone per-project runtime when `devkit.manifest.json` is missing.
- Preserve `.venv/` and other mutable runtime state during manifest recovery.
- Keep shared worktree-linked runtimes read-only during repair.

## Verification

- Doctor regression suite: 21 passed.
- Ruff format, lint, and Mypy passed.
- Full infrastructure suite and release acceptance gate passed.

## Upgrade

```bash
./leedevkit update --version v0.7.24
```

Existing projects can repair a missing manifest with:

```bash
./leedevkit doctor --fix
```

## Rollback

```bash
./leedevkit update --version v0.7.23
```
