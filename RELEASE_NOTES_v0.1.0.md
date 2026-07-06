# v0.1.0 — Per-Project DevKit Install

## Breaking Change

DevKit no longer requires global installation. Each project now self-contains its own devkit copy at `.leedevkit/`.

## What's New

### Per-project install (no sharing)

Running `leedevkit init` downloads the devkit release into your project:

```
my-project/
├── .leedevkit/           # Self-contained devkit (gitignored)
│   ├── scripts/, templates/, bin/, .agent/
│   ├── .venv/            # Project-local Python env
│   ├── skills.d/         # Community skills
│   └── VERSION
├── .agent/rules/         # Project rules (real files, not symlinks)
└── leedevkit → .leedevkit/bin/leedevkit
```

### AI Agent friendly

All directories an AI Agent needs to write (`.venv/`, `skills.d/`, `.agent/`) now live **inside the project root** — no permission issues with global install.

### Legacy symlink detection

`init` detects old symlinks from global install and warns with cleanup instructions (no auto-delete).

### Release tarball builder

`scripts/_release_build.py` builds release artifacts for GitHub releases.

## Migration

### New projects

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
cd my-project && git init
leedevkit init
```

### Existing projects (with old global symlinks)

```bash
cd my-project
rm -rf .agent                      # Remove legacy symlinks
leedevkit init                      # Re-init clean
```

`init` will warn if legacy symlinks are detected.

## Important: Use `./leedevkit` not `leedevkit`

After `init`, always use `./leedevkit` (project-local wrapper). Using plain `leedevkit`
runs the global bootstrap binary at `~/.leedevkit/`, which is outside your project.

## Files Changed

- `scripts/_orchestrator.py` — Rewrite `handle_init()`, per-project install flow
- `scripts/_devkit_config.py` — Per-project root resolution priority
- `scripts/_bootstrap.py` — Add `DEVKIT_ROOT` constant
- `scripts/_ensure-venv.sh` — Venv path → `.leedevkit/.venv/`
- `scripts/_release_build.py` — New: release tarball builder
- `bin/leedevkit` — Self-resolving wrapper
- `templates/leedevkit.default.toml` — Add `install` + `source` config
- `.gitignore` — Ignore `.leedevkit/` and `leedevkit` wrapper
- `README.md` — Updated Quick Start + migration guide

## Tests

- 372 tests pass (358 existing + 14 new)
- New tests cover: per-project init, tarball download, legacy symlink detection, devkit root resolution priority

## Install Strategies (in order)

1. `DEVKIT_LOCAL_PATH` env — for development
2. GitHub release tarball — for production
3. Copy from current source — fallback for dogfooding
