# v0.7.26 — Download Manifest-Verified Release Artifacts in Update

## Highlights

- Fix `./leedevkit update` to download official release tarballs containing `devkit.manifest.json` instead of GitHub source archive snapshots.
- Ensure updated installations pass integrity verification on subsequent `doctor` runs without reporting `manifest missing`.

## Verification

- Update URL verification test passed.
- Full infrastructure suite: 851 passed, 1 skipped; 83.46% production coverage.
- Release artifact acceptance gate passed.

## Upgrade

```bash
./leedevkit update --version v0.7.26
```

## Rollback

```bash
./leedevkit update --version v0.7.25
```
