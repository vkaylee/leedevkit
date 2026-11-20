# leedevkit

Enterprise DevKit for AI-first projects — unified CLI, test orchestrator, AI agent context, skill catalog, and integrity verification.

Global CLI plus per-project runtime: `leedevkit` is installed globally, while each project keeps pinned runtime state in `.leedevkit/` (like `node_modules/`).

## Install

### Bootstrap (one-time, installs the `leedevkit` command)

Two ways to get started — pick one:

### Option A: bootstrap.sh (project runtime)

```bash
cd my-project && git init
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
```

Downloads pinned runtime into `.leedevkit/` inside project. Install global `leedevkit` separately with `install.sh`.

### Option B: install.sh + init (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
cd my-project && git init
leedevkit init                      # ← uses global binary for this step only
```

Installs native `leedevkit` to `~/.leedevkit/current/bin`, then `init` downloads pinned runtime into `.leedevkit/`.

### After install

`init` or `bootstrap` creates this structure:

- `.leedevkit/` — pinned project runtime and isolated Python environment
- `.agent/rules/` — project's AI rulebooks (copied from devkit base)
- `leedevkit` — global executable

`.leedevkit/` and bootstrap lock files are gitignored. Commit `leedevkit`, `leedevkit.toml`, and `leedevkit.lock`.

## Quick Start

```bash
# Recommended: bootstrap.sh (no global, per-project only)
cd my-project && git init
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash

# Focused commands provide fast feedback; select completion checks by affected risk.
./leedevkit test infra --lint-only  # focused lint
./leedevkit test infra --unit-only  # focused unit
./leedevkit test all                # all configured targets/phases
./leedevkit test all --json         # stable final JSON report
./leedevkit test all --json-stream --quiet  # JSONL events for AI agents; logs stay in .test_logs/
# Human mode streams parallel task output as [task-name] lines and tees it to .test_logs/.
# --quiet suppresses live task output but keeps complete task logs for failure diagnosis.
# ./leedevkit init auto-detects Cargo.toml and scaffolds the right config.
./leedevkit test all                # Rust targets use configured Rust verification

# Development environment

## Debug MCP

LeeDevKit provides optional generic, read-only Debug MCP over stdio. It requires explicit validated environment configuration; it never infers Compose projects or attaches to every host network.

Configure a versioned `[debug]` manifest in `leedevkit.toml` (see `templates/leedevkit.default.toml`), then launch:

```bash
./leedevkit debug mcp --environment <environment>
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
./leedevkit dev up                   # start dev environment
./leedevkit dev down                 # stop dev environment
./leedevkit doctor                   # health check: config, installed runtimes, selected runtime, containers

# Explicit container-provider selection
./leedevkit --container-provider docker dev up
CONTAINER_PROVIDER=kiri ./leedevkit test all

# Skills catalog
./leedevkit skills list             # browse built-in + community catalog
./leedevkit skills install <name>   # install community skill by name
./leedevkit skills add <git-url>    # install external skill
./leedevkit skills update           # pull latest for all installed skills

# Version
./leedevkit version                 # show devkit version
```

### Why `leedevkit` instead of `./leedevkit`?
`leedevkit` is one global native executable. `leedevkit.toml` pins project runtime version; `.leedevkit/` stores that runtime locally and stays gitignored.

Existing projects may keep their old `./leedevkit` wrapper during migration. New projects do not create one.

Installer updates existing `$HOME/.bashrc` (preferred) or `$HOME/.zshrc` with the actual `${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin` path. It does not create startup files or change the parent shell running the installer. Source the updated file, or export the path in the current shell:

```bash
export PATH="${DEVKIT_HOME:-$HOME/.leedevkit}/current/bin:$PATH"
```

If installer found no supported startup file, run that export manually or add it to your shell startup file before using `leedevkit`.

## Migrating existing projects

If your project was initialized with the old global install (has `.agent/skills`, `.agent/workflows`, etc. as symlinks to `~/.leedevkit/`):

```bash
cd my-project
rm -rf .agent                      # Remove legacy symlinks + stale content
leedevkit init                      # Re-init clean from per-project install
```

Running `leedevkit init` will also warn you if legacy symlinks are detected.

## Project Structure

```
my-project/
├── leedevkit.toml            # project config (commit this)
├── leedevkit.lock            # skill version pins (commit this)
├── .leedevkit/               # gitignored — devkit engine (downloaded on init)
│   ├── scripts/              # orchestrator, bootstrap, etc.
│   ├── templates/            # default config templates
│   ├── bin/leedevkit         # CLI entry point
│   ├── .agent/               # base rules, agents, built-in skills catalog
│   ├── .venv/                # project-local Python env
│   ├── skills.d/             # cloned community skills (like node_modules)
│   ├── VERSION
│   └── dev-state.json        # { version, source }
├── .agent/                   # project AI context (real directory, not symlinks)
│   ├── rules/                # rulebooks (copied from devkit + project custom)
│   ├── overrides.yaml        # which devkit rules to replace/extend/add
│   └── agents/, workflows/   # project-specific (optional)
├── .claude/                  # Claude Code discovery paths (active when claude harness enabled)
│   ├── agents/                # symlinks to .leedevkit/.agent/agents/
│   ├── skills/                # symlinks to .leedevkit/.agent/skills/
│   └── settings.local.json    # optional machine-local permissions (do not commit)
├── AGENTS.md                 # root instructions for Codex, Copilot, Aider, Cline
├── GEMINI.md                 # root instructions for Gemini CLI
├── .cursor/rules/            # Cursor project rules (leedevkit.mdc)
└── leedevkit                   # committed self-bootstrapping CLI launcher
```

`.claude/settings.local.json` is for local Claude Code permissions and may contain
machine-specific paths. Keep it untracked; shared project settings belong in
`.claude/settings.json` when needed.

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
[devkit]
version = "0.1.0"              # pinned devkit version

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

# For Python-only projects:
# [targets]
# infra = ["infra"]
# all = ["infra"]

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

### Claude Code model routing

LeeDevKit automatically wires a fail-open `PreToolUse` hook into `.claude/settings.json` and registers the `assess_task` MCP tool in `.mcp.json`. Enable active subagent model routing by setting `ai.model_routing.enabled = true` in `leedevkit.toml`. When disabled (`enabled = false`, the default), the hook is dormant and exits immediately without modifying tool inputs. Explicit `model` values always win.

The default routing tiers are `haiku` for lookups, `sonnet` for routine implementation, `opus` for debugging or security work, and `fable` for ambiguous cross-system design.

## Go-Only Project

For Go modules, `leedevkit init` detects `go.mod` and scaffolds Go config automatically.

```bash
cd my-go-project && git init
touch go.mod
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
./leedevkit test go                 # gofmt + go vet + go test
./leedevkit test go --coverage      # Go coverage report
./leedevkit run go test ./...       # Run Go command in container
```

Generated config uses built-in `container/go/` image. Override toolchain with `GO_VERSION=1.24` or `[services.go] go_version = "1.24"`.
Use `./leedevkit test go --lint-only --fix` to rewrite formatting. Go package tests run in unit phase; integration phase runs only configured integration gates.

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
cd my-rust-crate && git init
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash

# init auto-detects Cargo.toml → scaffolds leedevkit.rust.toml
./leedevkit test all   # cargo fmt → clippy → nextest → llvm-cov
```

### How it works

```
my-rust-crate/
├── Cargo.toml
├── src/
├── leedevkit.toml              ← auto-generated (rust template)
├── leedevkit                   ← wrapper script
├── .leedevkit/
│   │   └── container/rust/     ← language-specific Dockerfile + compose (zero setup)
├── .agent/rules/               ← AI context (copied from devkit)
└── .compose/                   ← (optional) create to override the default image
```

### leedevkit.toml (Rust-only)

```toml
[devkit]
version = "latest"

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

The devkit resolves compose files in priority order:
1. `.compose/docker-compose.test.yml` (your override, if present)
2. `.leedevkit/container/<lang>/docker-compose.test.yml` (devkit default, auto-discovered)

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

### Built-in Skills (38 top-level packages, 49 skill files)
Shipped with devkit inside `.leedevkit/.agent/skills/`. During `init` and `update`,
LeeDevKit recursively discovers skills and syncs them across active harness adapters.
Pi and OMP consume the standard `SKILL.md` layout directly; Claude receives
additional symlink projections for plugin compatibility.
User-owned files in destination directories are never overwritten or removed.

### Community Catalog
Installable via `leedevkit skills install <name>`. Cloned into `.leedevkit/skills.d/` per-project. Catalog at `.agent/skills-catalog.toml`. Includes UI/UX Pro Max, Claude Code Model Router, and Superpowers Methodology.

```bash
./leedevkit skills list              # show built-in + catalog + installed
./leedevkit skills install <name>    # install from catalog by name
./leedevkit skills add <git-url>     # install external (not in catalog)
./leedevkit skills update            # git pull all installed skills
./leedevkit skills remove <name>     # remove
```

## Python Project (dogfooding devkit itself)

```toml
# leedevkit.toml
[devkit]
version = "0.1.0"

[project]
name = "leedevkit"
languages = ["python", "bash"]

[targets]
all = ["infra"]
infra = ["infra"]
```

```bash
leedevkit test infra --lint-only   # ruff + mypy
leedevkit test infra               # format + lint + pytest --cov
leedevkit test all                 # full pipeline with an 80% coverage gate
```

## License

MIT
