# v0.7.20 — Correct Per-Project Virtualenv Doctor Check

## Highlights

- Fix `leedevkit doctor` reporting the virtual environment as missing for per-project installs.
- Check `.leedevkit/.venv/`, the actual runtime location, instead of only checking a project-root `.venv/`.

## Verification

- Doctor regression tests: 16 passed.
- Full regression suite and release acceptance gate run before publishing.

## Upgrade

```bash
./leedevkit update --version v0.7.20
```

## Rollback

```bash
./leedevkit update --version v0.7.19
```
