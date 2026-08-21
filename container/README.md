# LeeDevKit Container Mode

Built-in Docker infrastructure for Rust and Go projects that need toolchains in containers without manual Dockerfile or Compose setup.

## Language profiles

Go projects use `go.mod`, `languages = ["go"]`, and `[services.go] lang = "go", go = true`. Built-in Go image includes `gofmt`, `go vet`, `go test`, and `go tool cover`. Set `GO_VERSION` to override image version.

```bash
./leedevkit test go --lint-only
./leedevkit test go --unit-only
./leedevkit test go --coverage
./leedevkit run go test ./...
```

Go `go test ./...` runs during unit phase. Integration phase skips duplicate package tests. Rust uses analogous `cargo fmt`, `cargo clippy`, `cargo nextest`, and `cargo llvm-cov` commands.

Go-only and Rust-only projects use built-in single-service Compose files. Mixed projects need `.compose/docker-compose.test.yml` defining all required services; built-in language files cannot run mixed stacks.

## Layout

Both profiles mount project root at `/workspace`, persist dependency/build caches, and expose stable built-in service names: `rust` and `go`.

```text
project/
├── go.mod or Cargo.toml
├── leedevkit.toml
└── .leedevkit/container/<language>/
    ├── Dockerfile
    └── docker-compose.test.yml
```

## Compose resolution

1. `.compose/docker-compose.test.yml` — project override
2. `.leedevkit/container/<language>/docker-compose.test.yml` — matching single-language default

Without override, project manifest or explicit test mode selects matching language container. Mixed projects must provide override.

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
- `container/rust/Dockerfile`: Rust toolchain, clippy, rustfmt, nextest, and llvm-cov.

Environment overrides:

```bash
GO_VERSION=1.24 ./leedevkit test go
RUST_VERSION=1.85 ./leedevkit test all
```

## Custom container

Create `.compose/docker-compose.test.yml` to add services or system dependencies. Project override always wins built-in defaults.

## Compatibility

| Project type | Compose source | Service name |
|---|---|---|
| Rust crate | `.leedevkit/container/rust/` | `rust` |
| Go module | `.leedevkit/container/go/` | `go` |
| Existing API/Web | `.compose/` | From config |
| Mixed Rust + Go/Web | `.compose/` | From config |
