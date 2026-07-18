# LeeDevKit Container Mode

Built-in Docker infrastructure for Rust projects that need the toolchain
in a container without manual Dockerfile/compose setup.

## How it works

```
my-rust-crate/
├── Cargo.toml
├── src/
├── leedevkit.toml          ← [services.rust] with lang="rust", cargo=true
├── leedevkit               ← wrapper script
├── .leedevkit/
│   ├── container/
│   │   └── rust/                        ← language-specific
│   │       ├── Dockerfile               ← Rust 1.85 + clippy + nextest + llvm-cov
│   │       └── docker-compose.test.yml  ← single service, mounts project
│   ├── scripts/
│   └── ...
└── .agent/rules/
```

## Compose file resolution

```
Priority 1: .compose/docker-compose.test.yml    (user override, project root)
Priority 2: .leedevkit/container/<lang>/docker-compose.test.yml (devkit default, auto-discovered)
```

- If you create `.compose/docker-compose.test.yml` → devkit uses your file (full control)
- If you don't → devkit falls back to its built-in container compose (zero setup)

## Service auto-detection

The pipeline reads `leedevkit.toml` and finds the first service with:

```toml
[services.<name>]
lang = "rust"
cargo = true
```

This service name is used for all `docker-compose exec` calls.
Falls back to `"apiserver"` for projects created before auto-detection existed.

## Docker image

The default image (`container/rust/Dockerfile`) includes:
- `rust:${RUST_VERSION}-slim-bookworm` base (default: 1.85)
- `pkg-config`, `libssl-dev` (system deps for common crates)
- `clippy`, `rustfmt`, `llvm-tools-preview` (Rust components)
- `cargo-nextest` (faster test runner)
- `cargo-llvm-cov` (LLVM source-based coverage)

### Changing Rust version

Two ways, in priority order:

```toml
# 1. leedevkit.toml (recommended — committed, everyone gets same version)
[services.rust]
rust_version = "1.83"
```

```bash
# 2. Environment variable (ad-hoc override, CI-friendly)
RUST_VERSION=1.83 ./leedevkit test all
```

Default is `1.85` if neither is set.

## Overriding the container

To use a custom image (e.g., add system dependencies like `libpq-dev`):

1. Create a `Dockerfile` at your project root
2. Create `.compose/docker-compose.test.yml` referencing your Dockerfile
3. The devkit automatically uses your files (Priority 1)

Example override:
```yaml
# .compose/docker-compose.test.yml
services:
  rust:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - .:/workspace
      - cargo-registry:/usr/local/cargo/registry
    working_dir: /workspace
    environment:
      - CARGO_BUILD_JOBS=2

volumes:
  cargo-registry:
```

## Compatibility

| Project type | Compose source | Service name |
|---|---|---|
| Rust crate (new) | `.leedevkit/container/rust/` | `rust` (from config) |
| API server (existing) | `.compose/` (user-managed) | `apiserver` (backward compat) |
| Mixed (override) | `.compose/` (Priority 1) | From config |
