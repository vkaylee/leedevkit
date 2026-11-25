# Devkit Internals — Go & Bash Conventions

> **Scope:** Developing leedevkit itself (the meta-tool), NOT projects that use it.

## Project Identity

Changes here ship through release artifacts installed globally under `${DEVKIT_HOME:-$HOME/.leedevkit}`. `init` creates project configuration and copies missing rules without installing a project-local runtime. `update` changes the global release, not project data. Legacy project-local runtimes and skills must not be deleted automatically.

## Stack

| Component | Language | Tooling |
|-----------|----------|---------|
| Control plane | Go 1.24+ | `cmd/leedevkit/main.go` |
| CLI launcher | Bash | `leedevkit`, `bin/leedevkit` |
| Release/CI helpers | Bash + Go | `scripts/build-release-go.sh`, `cmd/release-manifest` |
| Shell lint | Bash | `shellcheck` |
| Container engine | | podman-compose preferred, docker fallback |

## Key Commands

leedevkit test go --lint-only   # focused feedback only
leedevkit test go --unit-only   # focused feedback only
leedevkit test go                # Go verification
leedevkit test all              # all configured targets
leedevkit test all --json           # stable final JSON report
leedevkit test all --json-stream --quiet  # JSONL events for AI agents

Use the globally installed `leedevkit` for routine repository checks. This repository's `./leedevkit` launcher requires a local Go binary and is not the default; use it only when that binary exists and is executable.

Human mode streams parallel task output live with `[task-name]` prefixes and tees stdout/stderr into `.test_logs/` (or `.test_logs/<compose-project>/`). `--quiet` suppresses live task output but preserves complete logs. AI agents MUST use JSON/JSONL fields and failure `log` paths, not scrape human terminal text.

Machine contract: parse JSON/JSONL, not human log text. Each phase carries `phase`, `target`, `command`, timestamps, duration, exit code, status, and required flag. Failed/blocked phases carry `log`, `rerun`, and diagnostic fields. `passed`, `failed`, `blocked`, and `skipped` are distinct; focused or skipped runs report `partial` and `full_regression=false`.

A dry-run constructs commands only. Its success flags are not execution evidence; report the invocation mode alongside the result.

Focused flags, `--pattern`, and `--skip-e2e` produce partial/unverified results; they cannot replace `test all` for runtime/build/API/DB/config/dependency changes. Pass `--build` after changing Dockerfiles or image build dependencies so cached images are rebuilt.

## Release Packaging Gate (MANDATORY for packaging changes)

> 🔴 Source-only unit tests are NOT sufficient for packaging/install paths.

Trigger this gate after changes to any of:
- `bootstrap.sh`, `install.sh`, `bin/leedevkit`
- `scripts/build-release-go.sh` (release build and artifact generation)
- `VERSION`, Go release layout, manifest, and artifact behavior

```bash
scripts/build-release-go.sh /tmp/dist
```

What the gate proves:
1. Artifact is built from the staged allowlist and embeds a fresh `devkit.manifest.json`, SBOM, and checksum sidecar.
2. The installed CLI runs offline without creating a project-local runtime or `.venv`.
3. `install.sh` installs from a local release mirror (`LEEDEVKIT_RELEASE_BASE_URL`); `leedevkit update` installs a verified release.
4. Failed install or update preserves the active global release and existing project data; rollback to a previous release is verified.

Notes:
- Manifest is a **build output**, not a hand-edited source file.
- `./leedevkit test go` uses Go-native formatting and test gates; it must not rewrite source unless `--fix` is explicit.
- Keep packaging acceptance independent of Docker/PyPI/GitHub.

## Go Conventions

1. Use argv-only process execution through `internal/process`.
2. Preserve stable exit codes, bounded timeouts, process-group cleanup, and stderr diagnostics.
3. Keep machine-readable JSON/JSONL on stdout.
4. Validate untrusted input before runtime mutation.

## Bash Conventions

1. `bin/*` scripts are **passthrough only** — delegate to the Go runtime.
2. Preserve exit status and clean up owned child processes/resources when a command is interrupted; do not impose terminal restoration or redirection on every harness.
