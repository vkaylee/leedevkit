# v0.7.18 — Docker Layer Cache in Test Builds

## Highlights

- Allow Docker layer caching during test image prebuilds by removing forced `--no-cache`.
- Keep `--pull` so updated base images are still fetched when available.
- Update prebuild regression test to verify cached build invocation.

## Verification

- `ruff format --check scripts`: passed.
- `ruff check scripts`: passed.
- Prebuild and handler regression tests: passed; 79 passed.
- Full regression suite and release acceptance gate: passing before release.

## Upgrade

```bash
./leedevkit update --version v0.7.18
```

## Rollback

```bash
./leedevkit update --version v0.7.17
```

## Known limitations

- One existing lifecycle test remains skipped because Compose integration is opt-in.
- Release artifacts remain unsigned; the in-artifact SHA-256 manifest provides tamper detection, not publisher authenticity.
