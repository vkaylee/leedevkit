# Devkit Internals — Python & Bash Conventions

> **Scope:** Developing leedevkit itself (the meta-tool), NOT projects that use it.

## Project Identity

This is a **meta-tool factory**. Changes here ship to ALL downstream projects via:
- `~/.leedevkit/current` symlink (updated by `install.sh`)
- `leedevkit init` (writes wrappers + symlinks into project roots)

## Stack

| Component | Language | Tooling |
|-----------|----------|---------|
| Orchestrator | Python 3.11+ | `scripts/_orchestrator.py` |
| CLI wrappers | Bash | `bin/test.sh`, `bin/manage.sh`, `bin/leedevkit` |
| Lint/format | Python | `ruff` + `mypy` (strict) |
| Shell lint | Bash | `shellcheck` |
| Container engine | | podman-compose preferred, docker fallback |

## Key Commands

./leedevkit test infra --lint-only   # focused feedback only
./leedevkit test infra --unit-only    # focused feedback only
./leedevkit test infra                # infra verification
./leedevkit test all                  # completion gate: configured server/web/build/integration/E2E phases
./leedevkit test all --json           # stable final JSON report
./leedevkit test all --json-stream --quiet  # JSONL events for AI agents

Machine contract: parse JSON/JSONL, not human log text. Each phase carries `phase`, `target`, `command`, timestamps, duration, exit code, status, and required flag. Failed/blocked phases carry `log`, `rerun`, and diagnostic fields. `passed`, `failed`, `blocked`, and `skipped` are distinct; focused or skipped runs report `partial` and `full_regression=false`.

Focused flags, `--pattern`, `--skip-e2e`, and `--skip-build` produce partial/unverified results; they cannot replace `test all` for runtime/build/API/DB/config/dependency changes.

## Release Packaging Gate (MANDATORY for packaging changes)

> 🔴 Source-only unit tests are NOT sufficient for packaging/install paths.

Trigger this gate after changes to any of:
- `bootstrap.sh`, `install.sh`, `bin/leedevkit`
- `scripts/_release_build.py`, `scripts/_release_acceptance.py`
- `scripts/_download.py`, `scripts/_update_handler.py`, `scripts/_init_handler.py`
- `scripts/_devkit_integrity.py`, `VERSION`, release layout/manifest behavior

```bash
.venv/bin/python3 scripts/_release_build.py --repo-root . --output /tmp/dist
.venv/bin/python3 scripts/_release_acceptance.py \
  --repo-root . \
  --artifact /tmp/dist/leedevkit-$(tr -d '\n' < VERSION).tar.gz
```

What the gate proves:
1. Artifact is built from the staged allowlist and embeds a fresh `devkit.manifest.json`
2. Extracted `bin/leedevkit version|help` works offline without creating `.venv`
3. `bootstrap.sh` can install from a local release mirror (`LEEDEVKIT_RELEASE_BASE_URL`)
4. Failed bootstrap does not destroy an existing `.leedevkit`

Notes:
- Manifest is a **build output**, not a hand-edited source file.
- `./leedevkit test infra` uses `ruff format --check` and must not rewrite source.
- Keep packaging acceptance independent of Docker/PyPI/GitHub.

## Python Conventions

1. **`_` prefix on all modules** — internal implementation, NOT public API.
2. **Type hints mandatory** — mypy strict mode.
3. **No shell=True** — subprocess calls use argument lists.
4. **All CLI output to stderr** — structured JSON to stdout.
5. **PTY safety** — `_safe_run.py` wraps ALL subprocess execution.
6. **Ruff for formatting** — `ruff format scripts/`, not black.
7. **Argument sanitization** — AI-provided args go through `_arg_sanitizer.sanitize()`.

## Bash Conventions

1. `bin/*` scripts are **passthrough only** — set up venv, delegate to orchestrator.
2. Signal traps in `test.sh` for cleanup.
3. PTY restoration (`stty sane`) on exit.
