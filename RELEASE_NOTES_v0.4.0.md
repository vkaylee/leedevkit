# v0.4.0 — First-class Go support

## Highlights

leedevkit now supports Go modules as a built-in project type, alongside Rust and web projects.

## Features

- Auto-detects `go.mod` during `init` and generates Go project configuration.
- Adds built-in Go Dockerfile and Compose profile with dependency and build caches.
- Runs `gofmt`, `go vet`, `go test ./...`, and Go coverage through project-local containers.
- Supports Go test pattern filtering through shell-safe `--pattern` handling.
- Adds `run go ...` with Go-specific service, profile, and compose selection.
- Supports `GO_VERSION` environment overrides and `[services.<name>] go_version` configuration.
- Selects language-specific built-in Compose files without guessing in mixed Rust/Go projects.
- Adds Go-only and mixed-language pipeline regression coverage.
- Documents Go setup, commands, container overrides, and mixed-project requirements.

## Compatibility

Existing Rust, TypeScript, JavaScript, and Python workflows retain prior behavior. Mixed Rust/Go or Rust/Go/Web projects must provide `.compose/docker-compose.test.yml` defining required services.

## Upgrade

```bash
./leedevkit update --version v0.4.0
```

## Rollback

If needed, pin previous release:

```bash
./leedevkit update --version v0.3.18
```

## Verification

- Python modules compile successfully.
- Go-focused regression tests pass.
- Targeted suite passes: 326 tests.
- Release artifact integrity verification runs through existing release pipeline.
