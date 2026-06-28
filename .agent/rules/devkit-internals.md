# Devkit Internals — Python & Bash Conventions

> **Scope:** Developing leedevkit itself (the meta-tool), NOT projects that use it.

## Project Identity

This is a **meta-tool factory**. Changes here ship to ALL downstream projects via:
- `~/.leedevkit/current` symlink (updated by `install.sh`)
- `leedevkit init` (writes wrappers + symlinks into project roots)

## Stack

| Component | Language | Tooling |
|-----------|----------|---------|
| Orchestrator | Python 3.11+ | `scripts/_orchestrator.py` (1483 lines) |
| CLI wrappers | Bash | `bin/test.sh`, `bin/manage.sh` |
| Package manager | None | stdlib-only where possible (tomllib, argparse) |
| Lint/format | Python | `ruff` + `mypy` (strict) |
| Shell lint | Bash | `shellcheck` |
| Container engine | | podman-compose preferred, docker fallback |

## Key Conventions

### Python

1. **`_` prefix on all modules** — they are internal implementation, NOT public API.
2. **Type hints mandatory** — mypy strict mode, no `Any` without explicit `# noqa: ANN401`.
3. **No shell=True** — all subprocess calls use argument lists.
4. **All CLI output to stderr** — `print(..., file=sys.stderr)`. Only structured JSON goes to stdout.
5. **PTY safety** — `_safe_run.py` wraps ALL subprocess execution. Never use `python -c`, heredocs, or inline code.
6. **100% coverage required** on `scripts/` — enforced by `test:infra`.
7. **Ruff for formatting** — `ruff format scripts/`, not black.
8. **Argument sanitization** — all AI-provided args go through `_arg_sanitizer.sanitize()` before execution.

### Bash

1. **bin/* scripts are PASSTHROUGH only** — set up venv, delegate to `_orchestrator.py`. Zero logic.
2. **`set -euo pipefail`** in all shell scripts.
3. **Signal traps** — `test.sh` runs orchestrator in background and traps TERM/INT/HUP for cleanup.
4. **PTY restoration** — `stty sane` on exit to prevent terminal hang.

### Config Cascade

```
Project leedevkit.toml  (highest priority)
  ↓ deep_merge
Devkit default template  (templates/leedevkit.default.toml)
  ↓
Auto-detected values    (container engine, project root)
```

## Testing the Devkit

```bash
# Format only
.venv/bin/ruff format scripts/

# Lint only (Python + shell)
.venv/bin/ruff check scripts/
.venv/bin/mypy scripts/
shellcheck bin/*.sh scripts/*.sh

# Unit tests (100% coverage required)
.venv/bin/pytest --cov=scripts --cov-report=term-missing --cov-fail-under=100 scripts/tests/

# Full suite (format → lint → test)
python3 scripts/_orchestrator.py manage verify:infra
```

## Testing Init Flow End-to-End

```bash
cd /tmp && rm -rf test-init && mkdir test-init && cd test-init && git init
python3 /home/elt1541/leeattend-devkit/scripts/_orchestrator.py manage init
ls -la .agent/          # verify symlinks
cat leedevkit.toml      # verify config
cat .devkit-version    # verify version pin
```

## Design Invariants

1. **`.agent/` is L1 cache** — symlinked into projects. Changes are instant and global. Backward compat is critical.
2. **`_orchestrator.py` is the monolith** — prefer extracting to focused `_*.py` modules when adding features.
3. **Profiles, not flags** — docker-compose profiles (`api`, `web`, `infra-db`, `infra-redis`, `infra-pooler`) are injected by `_bootstrap.LIFECYCLE_PROFILES`. Never hardcode service names in commands.
4. **Run isolation** — each `./test.sh` gets a unique `COMPOSE_PROJECT_NAME` (UUID v4 suffix) + OS-level file lock.
5. **Signal safety** — SIGINT/SIGTERM/SIGHUP must always trigger `Orchestrator.cleanup()`.
6. **`templates/CLAUDE.base.md`** uses `@[.agent/...]` lazy-load syntax — keep it consistent when adding new rules/agents.

## Files to NEVER touch casually

- `templates/CLAUDE.base.md` — shapes ALL AI behavior in ALL projects
- `.agent/agents/` — 21 agent definitions, symlinked everywhere
- `VERSION` — must match semver, triggers GitHub release
