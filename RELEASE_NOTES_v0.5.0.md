# v0.5.0 — Configurable lifecycle dependencies

## Highlights

- Add project-configured Compose lifecycle dependencies through `[test.dependencies]`.
- Start the lifecycle mode's primary service and configured dependencies in one isolated Compose project.
- Wait for healthchecks of primary and dependency services.
- Support Docker Compose and Podman Compose.
- Validate malformed configuration and unknown Compose services with clear diagnostics.
- Preserve existing unit/lint behavior, Compose networking, and dynamic project isolation.

## Configuration

```toml
[test.dependencies]
"int-go" = ["postgres"]
```

The key is LeeDevKit lifecycle mode. Values are Compose service names. For a
multi-service chain, list each required service explicitly:

```toml
[test.dependencies]
"int-go" = ["A", "B", "C", "D"]
```

## Verification

- `740 passed, 1 skipped`
- `./leedevkit test infra --unit-only` passed
- Docker Compose lifecycle integration test passed

## Upgrade

```bash
./leedevkit update --version v0.5.0
```
