# leedevkit

Enterprise DevKit for AI-first projects — unified CLI, test orchestrator, AI agent context, skill catalog, and integrity verification.

## Install

```bash
git clone https://github.com/vkaylee/leedevkit.git ~/.leedevkit/v0.1.0
ln -sfn ~/.leedevkit/v0.1.0 ~/.leedevkit/current
~/.leedevkit/current/bin/leedevkit version
```

Or via curl (when published):

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
```

Installs to `~/.leedevkit/<version>/` with `current` symlink.

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash

# Initialize a project
cd my-project && git init
leedevkit init

# Run tests
leedevkit test infra --lint-only    # ruff + mypy (Python) / cargo fmt + clippy (Rust)
leedevkit test infra --unit-only    # pytest / cargo nextest / bun test
leedevkit test infra                # format + lint + test + coverage
leedevkit test all                  # full suite across all targets

# Infrastructure management
leedevkit manage up dev             # start dev environment
leedevkit manage down dev           # stop dev environment
leedevkit doctor                    # health check: config, containers, .agent symlinks

# Skills catalog
leedevkit skills list               # browse built-in + community catalog
leedevkit skills install <name>     # install community skill by name
leedevkit skills add <git-url>      # install external skill
leedevkit skills update             # pull latest for all installed skills

# Version
leedevkit version                   # show devkit version
```

## Project Structure

```
my-project/
├── leedevkit.toml            # ALL project config
├── leedevkit.lock            # skill version pins (commit this)
├── .leedevkit/               # gitignored — regenerated cache
│   └── skills.d/             # cloned community skills (like node_modules)
├── .agent/ → devkit          # AI context (symlinks to devkit)
└── leedevkit → devkit          # CLI wrapper
```

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

# For Python-only projects:
# [targets]
# infra = ["infra"]
# all = ["infra"]

[ai]
rules_dir = ".agent/rules"
override_manifest = ".agent/overrides.yaml"

[addons.skills]
skills = [
    # { url = "https://github.com/author/skill.git", version = "main" },
]
```

## .agent/overrides.yaml

```yaml
replace: []   # devkit rules to replace entirely
add: []       # project-only rules
extend: []    # append to devkit rules (both loaded)
```

## AI Agent Context

Three entry points — all symlink to the same template:

| File | AI Agent |
|------|----------|
| `CLAUDE.md` | Claude Code |
| `AGENTS.md` | Cursor, Copilot, Aider, Cline |
| `GEMINI.md` | Gemini CLI |

Template at `templates/CLAUDE.base.md` uses lazy-load pattern:
- Layer 0: Core principles (Enterprise quality, security, structured errors)
- Layer 4: Domain rulebooks (`.agent/rules/*.md`)
- Layer 5: Specialist agents (`.agent/agents/*.md`)
- Layer 6: Devkit commands

## Skills

Two kinds of skills:

### Built-in (38 skills)
Shipped with devkit, always available via `.agent/skills → devkit` symlink. No install needed.

```
api-patterns  app-builder  architecture  bash-linux  clean-code
database-design  frontend-design  game-development  mobile-design
nextjs-react-expert  python-patterns  rust-pro  tailwind-patterns
... (38 total)
```

### Community Catalog
Installable via `leedevkit skills install <name>`. Cloned into `.leedevkit/skills.d/` per-project. Catalog at `.agent/skills-catalog.toml`.

```bash
leedevkit skills list              # show built-in + catalog + installed
leedevkit skills install <name>    # install from catalog by name
leedevkit skills add <git-url>     # install external (not in catalog)
leedevkit skills update            # git pull all installed skills
leedevkit skills remove <name>     # remove
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
leedevkit test all                 # full pipeline (291 tests, 46% coverage)
```

## License

MIT
