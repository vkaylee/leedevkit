# Devkit Internals — Python & Bash Conventions

> **Scope:** Developing leedevkit itself (the meta-tool), NOT projects that use it.

## Project Identity

Changes here ship to downstream projects through release artifacts installed into each project's `.leedevkit/`. Init/update synchronize project wrappers, rulebooks, and harness discovery paths. `install.sh` retains a legacy global installation path; it is not the source checkout layout.

## Stack

| Component | Language | Tooling |
|-----------|----------|---------|
| Control plane | Go 1.24+ | `cmd/leedevkit/main.go` |
| CLI launcher | Bash | `leedevkit`, `bin/leedevkit` |
| Release/CI helpers | Transitional Python | `scripts/_release_*.py`, `scripts/_go_core_*.py` |
| Shell lint | Bash | `shellcheck` |
| Container engine | | podman-compose preferred, docker fallback |

## Key Commands

./leedevkit test infra --lint-only   # focused feedback only
./leedevkit test infra --unit-only    # focused feedback only
./leedevkit test infra                # infra verification
./leedevkit test all                  # all configured targets; this checkout configures infra
./leedevkit test all --json           # stable final JSON report
./leedevkit test all --json-stream --quiet  # JSONL events for AI agents

Human mode streams parallel task output live with `[task-name]` prefixes and tees stdout/stderr into `.test_logs/` (or `.test_logs/<compose-project>/`). `--quiet` suppresses live task output but preserves complete logs. AI agents MUST use JSON/JSONL fields and failure `log` paths, not scrape human terminal text.

Machine contract: parse JSON/JSONL, not human log text. Each phase carries `phase`, `target`, `command`, timestamps, duration, exit code, status, and required flag. Failed/blocked phases carry `log`, `rerun`, and diagnostic fields. `passed`, `failed`, `blocked`, and `skipped` are distinct; focused or skipped runs report `partial` and `full_regression=false`.

A dry-run constructs commands only. Its success flags are not execution evidence; report the invocation mode alongside the result.

Focused flags, `--pattern`, `--skip-e2e`, and `--skip-build` produce partial/unverified results; they cannot replace `test all` for runtime/build/API/DB/config/dependency changes.

## Release Packaging Gate (MANDATORY for packaging changes)

> 🔴 Source-only unit tests are NOT sufficient for packaging/install paths.

Trigger this gate after changes to any of:
- `bootstrap.sh`, `install.sh`, `bin/leedevkit`
- `scripts/_release_build.py`, `scripts/_release_acceptance.py` (transitional build tooling)
- `VERSION`, Go release layout, manifest, and artifact behavior

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

## Go Conventions

1. Use argv-only process execution through `internal/process`.
2. Preserve stable exit codes, bounded timeouts, process-group cleanup, and stderr diagnostics.
3. Keep machine-readable JSON/JSONL on stdout.
4. Validate untrusted input before runtime mutation.

## Bash Conventions

1. `bin/*` scripts are **passthrough only** — set up venv, delegate to orchestrator.
2. Preserve exit status and clean up owned child processes/resources when a command is interrupted; do not impose terminal restoration or redirection on every harness.
