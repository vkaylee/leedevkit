# v0.7.10 — Nested Go Module Workdir Support

## Highlights

- Honor `[services.go].workdir` for Go formatting, vetting, unit tests, and coverage.
- Preserve `/workspace` as the fallback when the configured path is absent or does not contain `go.mod`.
- Remove the invalid GitHub Actions pip-cache assumption for repositories without `requirements.txt` or `pyproject.toml`.

## Verification

- Focused source compilation passed.
- `scripts/tests/test_test_modules.py`: 40 passed.
- Focused Ruff formatting check passed for the changed Python files.
- Release artifact acceptance must pass before publication.

## Upgrade

```bash
./leedevkit update --version v0.7.10
```

## Rollback

```bash
./leedevkit update --version v0.7.9
```
