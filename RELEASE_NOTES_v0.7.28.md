# v0.7.28 — Isolate Concurrent Go Test Runs

## Highlights

- Reuse stable Go module and build cache volumes across test invocations.
- Keep per-run Compose projects while isolating temporary Go build files in a container tmpfs.
- Scope test and cleanup logs by Compose project to prevent concurrent runs overwriting each other.

## Verification

- Full source-quality gate passed.
- Full infrastructure regression suite passed.
- Release artifact acceptance gate passed.
- `git diff --check` passed.

## Upgrade

```bash
./leedevkit update --version v0.7.28
```

## Rollback

```bash
./leedevkit update --version v0.7.27
```

The previous runtime remains available for rollback if an update or extraction fails.
