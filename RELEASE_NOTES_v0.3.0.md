# v0.3.0 — Rust Container Mode

## What's New

### Zero-setup Rust projects

`leedevkit init` now auto-detects `Cargo.toml` and scaffolds the right config.  
No Dockerfile, no docker-compose.yml, no database — `./leedevkit test all` just works.

```bash
cd my-rust-crate && git init
curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
./leedevkit test all   # cargo fmt → clippy → nextest → llvm-cov
```

### Built-in container infrastructure

The devkit ships a default Docker image and compose file for Rust projects at
`.leedevkit/container/rust/`. Includes `rust:1.85`, `clippy`, `rustfmt`,
`cargo-nextest`, and `cargo-llvm-cov`. Nothing to install, nothing to configure.

**Language-specific layout** — `container/rust/`, `container/go/` (future), etc.  
Adding a new language is a directory; no code changes needed.

### Configurable Rust version

Two ways to override (higher priority first):

```bash
RUST_VERSION=1.83 ./leedevkit test all         # env var (ad-hoc, CI)
```

```toml
[services.rust]
rust_version = "1.83"                           # leedevkit.toml (committed)
```

Default: `1.85`.

### Compose file priority

```
1. .compose/docker-compose.test.yml    (your override)
2. .leedevkit/container/<lang>/        (devkit default, auto-discovered)
```

### Dynamic service detection

The pipeline reads `leedevkit.toml` and auto-detects the Rust service name
instead of hardcoding `"apiserver"`. Backward compatible — old projects
with `[services.apiserver]` work unchanged.

### Tests

- 36 new tests covering service auto-detection, init Cargo.toml detection,
  compose resolution priority, Rust version injection, and container
  subdirectory auto-discovery.
- 403 total tests passing (up from 367).
