# leedevkit

Enterprise DevKit for AI-first projects — one global CLI for testing, AI agent context, skills, and project setup.

> **Migration notice:** LeeDevKit uses one global CLI. Existing projects may need migration; back up project data and follow the migration and manual rollback steps below. Migration has been tested in isolated Linux fixtures, not representative consumer projects; compatibility remains project-specific.

LeeDevKit is migrating to one globally installed `leedevkit` CLI. The new `leedevkit init` configures project context without installing a project-local runtime.

## Install

Install the global CLI, then initialize a project:

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
export PATH="${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin:$PATH"
cd my-project && leedevkit init
```

The installer places the CLI under `${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin`. Ensure that directory is on `PATH` before running `leedevkit`. Run `leedevkit update` to install the latest global release, or `leedevkit update vX.Y.Z` for a specific release. Updates preserve the previous release and do not modify project configuration. Release checksums detect corruption but do not authenticate the publisher.

`leedevkit init` sets up project configuration and AI context. Keep existing local files and custom content; review any proposed changes before accepting them. Initialization does not install a project-local CLI or runtime.

Commit `leedevkit.toml` and `leedevkit.lock` when they belong to the project. `leedevkit.lock` records skill pins.

`bootstrap.sh` is a legacy setup path. New setups use the global installer and `leedevkit init`.

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
export PATH="${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin:$PATH"
cd my-project
leedevkit init
leedevkit test all
```

Use the global `leedevkit` command from project directories. Do not rely on a project-local `./leedevkit` runtime.

### Skills

Built-in skills come from the global installation. Project community skills live under `.agent/skills.d/`; `leedevkit.lock` records their Git revisions. `leedevkit skills list` also reports legacy `.leedevkit/skills.d/` entries, which are not moved or deleted automatically.

## Configuration

`leedevkit init` configures project context without installing a project-local runtime. Preserve local data and review changes before accepting them. Details of files managed by initialization depend on project configuration; do not remove existing project files to initialize.

Commit `leedevkit.toml` and the skills lock file `leedevkit.lock` as appropriate for the project.

The installer does not modify shell startup files or the parent shell. Add the following path to your shell configuration and export it in the current shell:

```bash
export PATH="${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin:$PATH"
```

## Migrating existing projects

1. Back up uncommitted and untracked project data, including `.leedevkit/skills.d/`, `.venv`, wrappers, custom rules and `leedevkit.lock`. Record the current working tree and executable version.
2. Install the global CLI, add it to `PATH`, and verify `command -v leedevkit` and `leedevkit version`. Run `leedevkit init` from the project root.
3. Review the project diff and untracked files. Init creates `leedevkit.toml` only if absent and copies missing rule files; it does not overwrite existing files or move legacy skills. Keep `leedevkit.lock`, which records skill revisions. Do not delete old runtimes or wrappers automatically.
4. Update project scripts and CI to invoke global `leedevkit` only after validating their behavior. Keep backups until consumers and rollback have been verified.

Project rollback is not automated: restore selected project files from the backup or Git, taking care not to overwrite later changes. Global update retains previous version directories. To roll back, request the exact previous version through the current global CLI, then verify active version and command path:

```bash
leedevkit update vX.Y.Z
leedevkit version
command -v leedevkit
```

The updater verifies the downloaded archive checksum and validates the release integrity manifest before switching `current`. Keep project backups and verify compatibility: older binaries may not understand newer project state. Rollback remains manual; migration and rollback have only been tested in isolated Linux fixtures, not representative consumer projects.

If an interrupted install leaves `${DEVKIT_HOME:-$HOME/.leedevkit}/.install.lock`, first verify no install or update is running and inspect the lock. Only then remove an empty stale lock directory with `rmdir`; never remove an active lock. `SIGKILL` or a system crash can leave one behind.

## Project Structure

Project contents vary by configuration. Common project-owned files include:

- `leedevkit.toml` — project configuration
- `leedevkit.lock` — skill pins
- `.agent/` — project AI context, where configured
- `.claude/`, `.cursor/`, or other harness context — where configured

The global CLI is installed separately from project files. `init` does not install a project-local runtime.

## Container providers

Provider selection precedence:

```text
--container-provider > CONTAINER_PROVIDER > host-local config > leedevkit.toml > docker
```

Repository config stays portable:

```toml
[container]
provider = "docker"

[container.providers.docker]
cli = ["docker"]
compose = ["docker", "compose"]

# Explicit declaration only. LeeDevKit does not infer Kiri compatibility.
[container.providers.kiri]
cli = ["kiri"]
compose = ["kiri", "compose"]
capabilities = ["compose", "config", "up", "down", "ps", "logs", "exec", "run"]
```

Host-local overrides live at `$XDG_CONFIG_HOME/leedevkit/config.toml`, or
`~/.config/leedevkit/config.toml` when `XDG_CONFIG_HOME` is unset. Use this file
for absolute executable paths, local socket/endpoint settings, and secret
references. Do not commit credentials. This configuration slice selects and
validates declarations only; provider adapters and capability enforcement must
exist before runtime commands can use a non-default provider.

## leedevkit.toml

```toml

[project]
name = "MyProject"
namespace = "myproject"
languages = ["rust", "typescript"]

[services.apiserver]
dockerfile = "./apiserver"
lang = "rust"
ports = [3000]

[services.webdashboard]
dockerfile = "./webdashboard"
lang = "typescript"
ports = [5173]

[targets]
api = ["apiserver", "agent-main"]
web = ["webdashboard"]
all = ["apiserver", "agent-main", "webdashboard"]

# Optional Compose services to start and health-check for each lifecycle mode.
# Values are Compose service names from the project's Compose file.
[test.dependencies]
# "int-go" = ["postgres"]
# "int-api" = ["postgres", "redis"]

# For Go-only projects:
# languages = ["go"]
# [targets]
# go = ["go"]
# all = ["go"]

# For Rust-only projects (no web, no DB):
# See "Rust-Only Project" section below or templates/leedevkit.rust.toml

[ai]
harnesses = ["claude"]         # configure active targets: claude, codex, cursor, gemini
rules_dir = ".agent/rules"
override_manifest = ".agent/overrides.yaml"
# Optional: route unset Claude Code subagent models by task complexity.
# Init/update wires .claude/settings.json and .mcp.json automatically.
[ai.model_routing]
enabled = false
default_model = "sonnet"

[addons.skills]
skills = [
    # { url = "https://github.com/author/skill.git", version = "main" },
]
```

## Testing

```bash
leedevkit test go --lint-only   # focused Go lint
leedevkit test go --unit-only   # focused Go unit tests
leedevkit test all              # all configured targets/phases
leedevkit test all --json        # stable final JSON report
leedevkit test all --json-stream --quiet  # JSONL events; logs stay in .test_logs/
```

Human mode streams parallel task output as `[task-name]` lines and tees it to `.test_logs/`. `--quiet` suppresses live task output but keeps complete task logs for failure diagnosis. JSONL streams emit `run_started`, `phase_started`, `phase_finished`, and `run_finished` events; skipped phases also emit start and finish events.

For TypeScript services, configure phase dependencies such as `unit-web`, `e2e-web`, and `lint-web` in `[test.dependencies]` and corresponding Compose profiles. Web phases run `bun run test`, `bun run test:e2e`, and `bun run lint`, `bun run type-check`, `bun run check-i18n`. Mixed targets report partial regression when Go integration gates are not configured. Web targets reject `--pattern`, `--coverage`, `--fix`, and `--base-ref`.

## Debug MCP

LeeDevKit provides optional generic, read-only Debug MCP over stdio. It requires explicit validated environment configuration; it never infers Compose projects or attaches to every host network.

Configure a versioned `[debug]` manifest in `leedevkit.toml` (see `templates/leedevkit.default.toml`), then launch:

```bash
leedevkit debug mcp --environment <environment>
```

Configure an MCP client with placeholders only:

```json
{
  "mcpServers": {
    "project-debug": {
      "command": "leedevkit",
      "args": ["debug", "mcp", "--environment", "<environment>"]
    }
  }
}
```

Tools: `debug_environment`, `service_health`, `http_get`, `recent_logs`, `database_schema`, and approved `database_query` templates. Resources: `debug://runbook`, `debug://safety`, `debug://environment`, `debug://capabilities`, `debug://evidence`. Prompt: `debug_incident`.

Debug MCP rejects shell execution, writes, migrations, arbitrary URLs/networks, host filesystem/socket access, credentials, cookies/tokens, unapproved paths, cross-environment access, and production selection. Responses and diagnostics are bounded and redacted. Existing task-assessor MCP remains unchanged.

```bash
leedevkit dev up
leedevkit dev down
leedevkit doctor
leedevkit --container-provider docker dev up
CONTAINER_PROVIDER=kiri leedevkit test all
```

See `leedevkit skills --help` for supported skill operations.

## Go-Only Project

For Go modules, `leedevkit init` detects `go.mod` and scaffolds Go config automatically.

```bash
cd my-go-project
leedevkit init
leedevkit test go                 # gofmt + go vet + go test
leedevkit test go --coverage      # Go coverage report
leedevkit run go test ./...       # Run Go command in container
```

Generated config uses built-in `container/go/` image. Override toolchain with `GO_VERSION=1.24` or `[services.go] go_version = "1.24"`.
Use `leedevkit test go --lint-only --fix` to rewrite formatting. Go package tests run in unit phase; integration phase runs only configured integration gates.

### Go quality gates

Existing Go projects keep `gofmt`, `go vet ./...`, and `go test ./...` defaults. Add project-owned gates without LeeDevKit knowing tool, database, or migration details:

```toml
[quality.go]
lint = ["gofmt", "go-vet", "staticcheck"]
unit = ["go-test", "go-test-race"]
integration = ["go-test-integration"]

[quality.gates.staticcheck]
phase = "lint"
command = ["staticcheck", "./..."]
working_dir = "server"
service = "go"
required = false

[quality.gates.go-test-race]
phase = "unit"
command = ["go", "test", "-race", "./..."]
working_dir = "server"
service = "go"

[quality.gates.go-test-integration]
phase = "integration"
command = ["go", "test", "-tags=integration", "-count=1", "./..."]
working_dir = "server"
service = "go"
requires = ["infra-db"]
```

Gate commands use argument-list config and existing container runner. Missing configured tools fail the gate; LeeDevKit never downloads tools. `required = false` keeps optional failures visible without failing phase. `--base-ref <git-ref>` records resolved commit and changed paths; new-findings filtering remains gate-specific.

Generated freshness checks use same generic gate model:

```toml
[quality.generated]
command = ["generator", "diff"]
working_dir = "server"
service = "go"
```

Check commands do not rewrite source. A separate `fix_command` must be declared before `--fix` can alter files.

Test lifecycle startup builds each image once per `leedevkit test` invocation by default, then uses `compose up -d --no-build` for later phases. Set `[test].build_policy = "phase"` to build during every phase. Compose/BuildKit reuses unchanged image layers and invalidates layers when Dockerfiles, build context, or dependency manifests change; no forced `build --pull` runs. Frontend `bun_init` runs `bun install --frozen-lockfile` on every startup when provided by project Compose, with its healthcheck blocking tests on failure. `--skip-build` always uses `--no-build`, so missing or stale images fail instead of being silently rebuilt; result is partial/unverified.

### Lifecycle dependencies

Configure services that must start and pass healthchecks for a lifecycle mode:

```toml
[test.dependencies]
"int-go" = ["postgres"]
```

The key is LeeDevKit lifecycle mode. Values are service names in the project's
Compose file. LeeDevKit starts the mode's primary service and listed dependencies
in the same isolated Compose project. Providers declaring `wait` delegate ordering
and health readiness to Compose via `up --wait --wait-timeout 120`; older providers
use bounded compatibility checks. Add Compose `depends_on` conditions and
healthchecks when dependency ordering matters. Missing configuration preserves
existing behavior. Unit and lint modes do not start these services unless
configured for that exact mode. For a chain, list each required service explicitly,
for example `"int-go" = ["A", "B", "C", "D"]`.

## Rust-Only Project

For Rust libraries, CLI tools, or any crate that doesn't need a web frontend or database.
No Dockerfile or docker-compose.yml setup required — the devkit provides defaults.

```bash
cd my-rust-crate
leedevkit init                 # detects Cargo.toml and scaffolds Rust config
leedevkit test all             # cargo fmt → clippy → nextest → llvm-cov
```

### Project files

`leedevkit init` configures the project for Rust using the global CLI. It does not install a project-local CLI or runtime. Preserve existing project files and local data while reviewing initialization changes.

### leedevkit.toml (Rust-only)

```toml
[project]
name = "MyRustCrate"
namespace = "myrustcrate"
languages = ["rust"]

[services.rust]
lang = "rust"
cargo = true
# rust_version = "1.83"    # optional — pin Rust version (default: 1.85)
# No dockerfile needed — devkit uses built-in container/rust/Dockerfile
# No ports — library/CLI projects don't expose network ports

[targets]
all = ["rust"]
```

### Overriding the container image

To add system dependencies (e.g., `libpq-dev` for diesel):

```bash
# Create your own Dockerfile at project root
# Create .compose/docker-compose.test.yml (takes priority over devkit default)
mkdir -p .compose
```

The devkit resolves Compose files in priority order:
1. `.compose/docker-compose.test.yml` (your override, if present)
2. Matching built-in language default, when available. Consult CLI help for current behavior.

See `container/README.md` for full architecture details.

## .agent/overrides.yaml

```yaml
replace: []   # devkit rules to replace entirely
add: []       # project-only rules
extend: []    # append to devkit rules (both loaded)
```

## AI Agent Context & Multi-Harness Adapters

LeeDevKit projects the single source of truth in `.agent/` (rules, skills, agents, workflows) into each target coding harness:

| Harness | Generated / Managed Path | Discovery Mechanism |
|---|---|---|
| **Claude Code** | `.claude/skills/`, `.claude/agents/`, `CLAUDE.md` | Relative symlinks + managed context block |
| **Codex / Copilot / Aider / Cline** | `AGENTS.md` | Root instruction file with managed marker block and skill index |
| **Cursor** | `.cursor/rules/leedevkit.mdc` | Project rule file with `alwaysApply: true` |
| **Gemini CLI** | `GEMINI.md` | Root instruction file with inlined context and skill index |
| **Pi** | `.pi/skills/`, `AGENTS.md` | Native Pi skills + managed project context |
| **Oh My Pi** | `.omp/skills/`, `.omp/agents/`, `.omp/AGENTS.md` | Native OMP skills, task agents, and context |

Configure active harnesses in `leedevkit.toml`:

```toml
[ai]
harnesses = ["claude", "codex", "cursor", "gemini", "pi", "omp"]
rules_dir = ".agent/rules"
override_manifest = ".agent/overrides.yaml"

```
Managed files use delimiter comments (`<!-- leedevkit:begin -->` ... `<!-- leedevkit:end -->`) so custom instructions outside the block survive re-sync. Pi and OMP skill directories contain managed relative symlinks; user files with conflicting names are preserved.

Context is split by responsibility: core safety and scope in the base context, executable facts in project configuration/CI, behavior verification in `development-workflow.md`, execution/coverage gates in `testing-standards.md`, and domain guidance loaded only for matching risks. Rulebooks do not override harness permissions or select an application's auth, database, or operational architecture.

Consumer projects resolve the base under `.leedevkit/templates/CLAUDE.base.md`; the LeeDevKit source checkout uses `templates/CLAUDE.base.md`. Generated Claude hook/MCP commands follow the same source-versus-installed layout. Process control uses the active harness's capabilities, not blanket bans on shell syntax or forced log redirection. Existing configured isolation wrappers still apply.

Use harness-native skill discovery when available; load relevant skill bodies only. Scenario checks should keep Q&A lightweight, verify observable behavior for small fixes, and load authorization, migration, or release controls only when those risks apply. A dry-run is not test evidence, even if its machine summary reports success.

The context cleanup reduced the base template from 92 lines / 5,448 bytes to 37 lines / 3,119 bytes, and the 26 rulebooks from 1,636 lines / 105,297 bytes to 936 lines / 71,333 bytes. These are on-disk sizes, not measured injected tokens. Claude and OMP use native discovery without a duplicated full skill index; Pi retains the same fallback index as Codex because both manage `AGENTS.md`.

## Skills

`leedevkit.lock` records skill pins. Use `leedevkit skills --help` for available commands. Skill storage locations and update behavior depend on implementation details not specified here.

## LeeDevKit development project

This repository uses Go package tests and project-owned Compose gates.

```toml
[project]
name = "leedevkit"
languages = ["go", "bash"]

[targets]
all = ["go"]
go = ["go"]
```

```bash
leedevkit test go --lint-only   # focused Go checks
leedevkit test go --unit-only   # Go package tests
leedevkit test all              # full configured pipeline
```

## License

MIT
