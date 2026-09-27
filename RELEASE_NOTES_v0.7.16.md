# v0.7.16 — Compose Provider Detection

## Highlights

- Prefer native `podman compose` when Podman is installed without standalone `podman-compose`.
- Preserve standalone `podman-compose` and Docker Compose detection.
- Replace the unsafe missing-tool fallback with an explicit dependency error.
- Add regression coverage for native Podman Compose and missing Compose implementations.

## Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Lifecycle and bootstrap tests: passed; 71 passed, 1 skipped.
- Full CI quality and release acceptance gates: required before publication.

## Upgrade

```bash
./leedevkit update --version v0.7.16
```

## Rollback

```bash
./leedevkit update --version v0.7.15
```

## Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
