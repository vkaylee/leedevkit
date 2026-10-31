# v0.7.29 — Deduplicate Resolved Test Targets

## Highlights

- Avoid running a service twice when `all` includes a service already covered by a configured target alias.
- Preserve service-only projects and deterministic target ordering.

## Verification

- Full infrastructure source-quality and regression gate passed.
- Release artifact acceptance gate is required before publication.
- `git diff --check` passed.

## Upgrade

```bash
./leedevkit update --version v0.7.29
```

## Rollback

```bash
./leedevkit update --version v0.7.28
```

The previous runtime remains available for rollback if an update or extraction fails.
