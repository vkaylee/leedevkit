# leedevkit

Enterprise Developer Experience Kit for AI-first multi-service projects.

## What it does

- **Test Orchestrator** — `./test.sh` with lint → unit → e2e pipeline, parallel execution, Docker isolation
- **AI Agent Context** — `.agent/rules/` + `overrides.yaml` with inheritance cascade
- **Project Scaffolding** — `.devkit.toml` defines services, targets, and phases
- **Integrity Verification** — SHA256 manifest + tamper detection
- **Multi-service** — Rust + TypeScript + any Dockerized service

## Quick Start

```bash
# Install (choose one)
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
# OR: pip install leedevkit

# In your project root
leedevkit init

# Start developing
./test.sh all
```

## Architecture

```
~/.leedevkit/
├── v0.1.0/                    ← versioned install
│   ├── scripts/               ← orchestrator core
│   ├── .agent/                ← AI rules + agents
│   ├── templates/             ← project templates
│   └── bin/                   ← test.sh, manage.sh
└── current → v0.1.0/          ← active version symlink

my-project/
├── .devkit.toml               ← project profile (override defaults)
├── .devkit-version            ← pin "0.1.0"
├── .agent/
│   ├── overrides.yaml         ← rule inheritance manifest
│   └── rules/                 ← project-specific AI rules
├── test.sh → ~/.leedevkit/current/bin/test.sh
└── manage.sh → ~/.leedevkit/current/bin/manage.sh
```

## Inheritance & Override

```yaml
# .agent/overrides.yaml
replace:                       # Replace devkit rule entirely
  - coding-standards.md
add:                           # Project-only rules
  - security/pci-dss.md
extend:                        # Load both (devkit → project)
  - database-rules.md
```

## CLI

```bash
./test.sh api --lint-only      # Quick format check (local <1s if cargo available)
./test.sh api --lint-only --fix # Auto-fix formatting
./test.sh all --json            # Machine-readable output for AI agents
./test.sh api --pattern auth    # Filter tests by name
./manage.sh up dev              # Start dev environment
leedevkit doctor         # Verify installation health
leedevkit verify         # Check file integrity (tamper detection)
leedevkit upgrade        # Update to latest version
```

## License

MIT
