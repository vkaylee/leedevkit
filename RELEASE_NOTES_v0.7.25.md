# v0.7.25 — Fix Project Doctor Wrapper Dispatch

## Highlights

- Fix project launcher `./leedevkit doctor` passing the `doctor` command name twice to the Python doctor parser.
- Preserve `doctor --fix` behavior and runtime manifest repair from v0.7.24.

## Verification

- Wrapper doctor acceptance regression test passed.
- Ruff format, lint, and Mypy passed.
- Full infrastructure suite and release acceptance gate passed.

## Upgrade

```bash
./leedevkit update --version v0.7.25
```

## Rollback

```bash
./leedevkit update --version v0.7.24
```
