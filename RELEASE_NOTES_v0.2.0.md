# v0.2.0 — Self-Update (`leedevkit update`)

## What's New

### `leedevkit update` — self-update from GitHub Releases

DevKit can now upgrade itself in place. No more re-running `init --force` by hand when a new version ships.

```bash
leedevkit update                 # Update to the latest release
leedevkit update --version v0.3.0   # Pin to a specific release
```

Under the hood:

- Queries the GitHub `releases/latest` API to discover the newest tag.
- Compares against the installed `VERSION`; if already current, it's a no-op.
- Backs up the current install to `<root>.bak/` before overwriting, so a manual rollback is always possible.
- Downloads the release tarball and overlays it onto the install.
- On any download or extract failure, automatically rolls back to the previous version and reports the error.

### Tests

- 13 new tests covering the update parser, GitHub version lookup, and the full update flow (already-latest, success, rollback, latest-release resolution).
- 3 subprocess smoke tests verifying `update --help`, the already-latest path, and `bin/leedevkit` routing.
