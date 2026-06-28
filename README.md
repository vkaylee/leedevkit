# leedevkit

Enterprise DevKit for AI-first multi-service projects — test orchestrator, AI agent context, integrity verification.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
```

## Quick Start

```bash
./test.sh api --lint-only      # format + lint
./test.sh api --unit-only      # unit tests only
./test.sh all                  # full suite
./test.sh api --lint-only --fix # auto-fix
./test.sh all --json            # machine-readable for AI
./manage.sh up dev              # start dev env
```

## Project Setup

```toml
# .devkit.toml
[services.apiserver]
dockerfile = "./apiserver"
lang = "rust"
ports = [3000]

[targets]
api = ["apiserver"]
```
```yaml
# .agent/overrides.yaml
replace: []   # rules to override from devkit
add: []       # project-only rules
extend: []    # append to devkit rules
```

## License

MIT
