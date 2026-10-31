# v0.7.30 — Fix Project Launcher Update Lock

## Highlights

- Initialize the project launcher lock path before stale-runtime bootstrap.
- Allow `./leedevkit update` to recover projects whose runtime version is outdated.

## Verification

- Full infrastructure source-quality and regression gate passed: `858 passed, 1 skipped`.
- Release artifact acceptance gate passed.
- Project update smoke test passed from `0.7.28` to `0.7.29`.
- `git diff --check` passed.

## Upgrade

```bash
./leedevkit update --version v0.7.30
```

## Rollback

```bash
./leedevkit update --version v0.7.29
```

The previous runtime remains available for rollback if an update or extraction fails.
