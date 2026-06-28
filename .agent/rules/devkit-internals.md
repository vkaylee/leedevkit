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

```bash
./leedevkit test infra --lint-only   # ruff + mypy
./leedevkit test infra               # format + lint + pytest
./leedevkit test all                 # full suite
./leedevkit doctor                   # health check
./leedevkit version                  # show version
```

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
