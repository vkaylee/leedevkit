# v0.3.4 — Black-Box Release Acceptance Gate

## What's New

### End-to-end release acceptance

LeeDevKit now builds and verifies the real release artifact before packaging is considered complete:

```bash
python3 scripts/_release_build.py --repo-root . --output /tmp/dist
python3 scripts/_release_acceptance.py \
  --repo-root . \
  --artifact /tmp/dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```

The gate proves:

- the tarball contains the staged allowlist and a fresh integrity manifest
- extracted `bin/leedevkit version|help` works offline without creating `.venv`
- `bootstrap.sh` can install from a local release mirror
- failed bootstrap does not destroy an existing `.leedevkit`

### Safer packaging and install paths

- Release builds stage payload first, generate `devkit.manifest.json` from that payload, verify it, then archive
- Runtime-generated paths (`.venv/`, `skills.d/`, `dev-state.json`) are ignored by integrity checks
- `bootstrap.sh` validates artifact structure/version/integrity before promoting into `.leedevkit`
- Existing installs are preserved when bootstrap fails
- `LEEDEVKIT_RELEASE_BASE_URL` supports deterministic local bootstrap tests

### Offline CLI identity paths

`leedevkit version`, `--help`, and unknown top-level commands no longer initialize a virtualenv first. Python-backed commands still bootstrap lazily.

### Non-mutating verification

`./leedevkit test infra` now runs `ruff format --check` instead of rewriting source during verification.

### AI agent guidance

Shipped Claude/base context and rulebooks now require the release packaging gate for packaging, bootstrap, install, update, versioning, and release-layout changes. Source-only unit tests are no longer treated as sufficient for those paths.

### Correctness fixes

- Devkit discovery no longer mistakes the project-local `leedevkit` CLI symlink for a bundled install root
- Manifest version/structure mismatches fail integrity verification

## Upgrade

```bash
./leedevkit update --version v0.3.4
```
