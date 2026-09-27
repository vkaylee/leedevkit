# v0.7.15 — Runtime Guard and Dead-Code Cleanup

## Highlights

- Enforced project-local `.venv/bin/python3` for direct LeeDevKit Python entrypoints.
- Preserved host-Python execution only for explicit bootstrap paths.
- Migrated CI, release, and documentation examples to project-local Python.
- Removed unused `_docker_ops` Compose wrapper and its obsolete test suite.
- Kept legacy install, global-resolution, and configuration compatibility paths because repository evidence still shows active migration support.

## Verification

- `./leedevkit test infra --lint-only`: passed.
- `./leedevkit test infra --unit-only`: passed.
- `./leedevkit test all`: passed; 849 passed, 1 skipped.
- `./leedevkit test all --json`: passed; `full_regression=true`, exit code `0`.
- Release build and black-box acceptance: passed.
- Host interpreter smoke: rejected with exit code `125`.
- Project-local interpreter smoke: passed.

## Upgrade

```bash
./leedevkit update --version v0.7.15
```

## Rollback

```bash
./leedevkit update --version v0.7.14
```

## Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
- Legacy install and compatibility paths remain until an explicit deprecation window and migration plan retire them.
