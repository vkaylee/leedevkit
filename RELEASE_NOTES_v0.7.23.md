# v0.7.23 — Runtime Integrity Reconciliation

## Highlights

- Report DevKit runtime manifest drift from `leedevkit doctor` without changing files.
- Repair modified or missing runtime files with `doctor --fix` while preserving `.venv/`, `skills.d/`, and `dev-state.json`.
- Refresh AI harness projections after runtime repair without overwriting user content outside managed blocks.
- Resolve generated built-in skill and agent references through the installed `.leedevkit/` runtime.

## Verification

- Ruff format, lint, and Mypy passed.
- Full infrastructure suite: 848 passed, 1 skipped; 83.10% production coverage and 99% test coverage.
- Targeted doctor and harness tests: 28 passed.

## Upgrade

```bash
./leedevkit update --version v0.7.23
```

Existing projects can also run:

```bash
./leedevkit doctor --fix
```

## Rollback

```bash
./leedevkit update --version v0.7.22
```

Mutable runtime state remains preserved during repair; failed replacement restores the previous runtime.
