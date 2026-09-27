# v0.7.17 — Preserve Compose Lookup Environment

## Highlights

- Preserve the parent process environment when executing commands with custom variables.
- Keep `PATH` available to `_safe_run.py`, including user-installed `podman-compose`.
- Add regression coverage for custom environment merging.

## Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Bootstrap, lifecycle, and orchestrator tests: passed; 206 passed, 1 skipped.
- Full CI quality and release acceptance gates: required before publication.

## Upgrade

```bash
./leedevkit update --version v0.7.17
```

## Rollback

```bash
./leedevkit update --version v0.7.16
```

## Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
