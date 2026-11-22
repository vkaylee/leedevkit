# LeeDevKit Container Mode

Container support for Rust and Go projects that need toolchains without manual
Dockerfile or Compose setup. Use the globally installed `leedevkit` CLI; project
initialization does not install a project-local runtime. Provider selection is
explicit and configured through LeeDevKit; built-in adapters remain provider-specific.

## Provider configuration

```toml
[container]
provider = "docker"

[container.providers.docker]
cli = ["docker"]
compose = ["docker", "compose"]
```

Selection precedence is `--container-provider`, `CONTAINER_PROVIDER`, host-local
`$XDG_CONFIG_HOME/leedevkit/config.toml` (or `~/.config/leedevkit/config.toml`),
repository `leedevkit.toml`, then Docker default. Kiri and custom providers need
explicit command arrays; LeeDevKit never infers compatibility from provider names.
Host-local files can hold absolute paths and local endpoint settings. Keep secret
values out of repository configuration and logs.

Configuration alone does not add an adapter or prove provider capabilities. Lifecycle
commands use explicitly resolved Compose argv and fail before mutation when required
capabilities are absent. Providers declaring `wait` receive native Compose readiness;
others use bounded compatibility checks until their adapter is verified. Kiri
commands in examples are placeholders until its official CLI contract is verified.

| Provider | Native adapter | Docker-compatible wrapper | Required capabilities | CI status |
|---|---:|---:|---|---|
| Docker | yes | no | full | required |
| Kiri | explicit declaration | only if verified | documented | conditional |
| Podman | optional | no fake wrapper | documented | optional |
| Unsupported tool | no | no | fail fast | unsupported |

## Language profiles

Go projects use `go.mod`, `languages = ["go"]`, and `[services.go] lang = "go", go = true`. Built-in Go image includes `gofmt`, `go vet`, `go test`, and `go tool cover`. Set `GO_VERSION` to override image version.

```bash
leedevkit test go --lint-only
leedevkit test go --unit-only
leedevkit test go --coverage
leedevkit run go test ./...
```

Go `go test ./...` runs during unit phase. Integration phase skips duplicate package tests. Rust uses analogous `cargo fmt`, `cargo clippy`, `cargo nextest`, and `cargo llvm-cov` commands.

Go-only and Rust-only projects use built-in single-service Compose files. Mixed projects need `.compose/docker-compose.test.yml` defining all required services; built-in language files cannot run mixed stacks.

## Layout

Both profiles mount project root at `/workspace`, persist dependency/build caches, and expose stable built-in service names: `rust` and `go`.

```text
project/
├── go.mod or Cargo.toml
└── leedevkit.toml
```

## Compose resolution

1. `.compose/docker-compose.test.yml` — project override
2. Matching single-language default, when available

Without override, project manifest or explicit test mode selects a matching language container. Mixed projects must provide an override. Default asset locations are implementation details; use project configuration and CLI help for current behavior.

## Service configuration

```toml
[services.go]
lang = "go"
go = true
# go_version = "1.24"
```

```toml
[services.rust]
lang = "rust"
cargo = true
# rust_version = "1.85"
```

Built-in files expose `go` and `rust`. Custom service names require project override Compose defining those names.

## Docker images

- `container/go/Dockerfile`: official Go image; standard Go formatting, lint, test, and coverage tools.
- `container/go/docker-compose.core.yml`: Compose-owned Go core build/test service with module and build caches.
- `container/rust/Dockerfile`: Rust toolchain, clippy, rustfmt, nextest, and llvm-cov.

Environment overrides:

```bash
GO_VERSION=1.24 leedevkit test go
RUST_VERSION=1.85 leedevkit test all
```

## Custom container

Create `.compose/docker-compose.test.yml` to add services or system dependencies. Project override always wins built-in defaults.

Image freshness fingerprints hash full build context by default, while preserving `.dockerignore` rules. For a Dockerfile that intentionally reads only selected files, add Compose extension field `x-leedevkit-build-inputs` beside `build`:

```yaml
services:
  toolchain:
    x-leedevkit-build-inputs:
      - Dockerfile
      - scripts/install-tools.sh
    build:
      context: .
      dockerfile: Dockerfile
```

Paths are relative to build context. Missing declared inputs invalidate image freshness. Do not use this field unless Dockerfile and build tooling do not read other context files; omitted field keeps safe full-context hashing.

## Debug MCP isolation

Debug MCP runs as a temporary read-only capability selected by explicit environment manifest. Use internal Compose DNS and approved project networks only; do not use published host ports, host network aliases, arbitrary host paths, or container sockets. Keep root filesystem read-only, use only bounded temporary writable paths, run non-root where compatible, drop capabilities, and shut down on EOF or SIGTERM. Projects must provide approved service aliases, paths, logs, limits, and database metadata in the versioned `[debug]` contract.

## Compatibility

| Project type | Compose source | Service name |
|---|---|---|
| Rust crate | Built-in language default, when available | `rust` |
| Go module | Built-in language default, when available | `go` |
| Existing API/Web | `.compose/` project override | From config |
| Mixed Rust + Go/Web | `.compose/` project override | From config |
