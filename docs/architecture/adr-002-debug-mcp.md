# ADR-002: Generic Read-Only Debug MCP

- Status: proposed
- Date: 2026-11-17
- Owner: LeeDevKit maintainers

## Context

LeeDevKit needs project-agnostic debugging evidence over local stdio MCP. Existing task-assessor MCP and Compose test lifecycle must remain compatible. Debug access crosses MCP, project configuration, Compose, container, network, and sensitive-data boundaries.

## Decision

Add Debug MCP as separate core modules: validated versioned `[debug]` manifest, transport/tool dispatch, and bounded runtime. Environment selection is explicit. Core code consumes capability IDs, allowlists, limits, and redaction policy from manifest; it never names consumer services, routes, schemas, networks, credentials, or PII fields. Default behavior is read-only and fail-closed. Existing task-assessor MCP remains unchanged.

## Alternatives

- Extend task-assessor: rejected; unrelated public contract and trust boundary.
- Arbitrary shell/SQL: rejected; prompt restrictions cannot enforce read-only behavior.
- Ambient dev/test project selection: rejected; risks cross-worktree and cross-user access.
- Network transport: deferred; v1 remains stdio-only and local.

## Security and operational impact

Manifest validation rejects unsafe paths, destinations, production identifiers, invalid limits, and ambiguous selection. Tool inputs are allowlisted. Responses redact credential-like fields and configured patterns. Runtime/container integration must use temporary non-privileged containers, explicit selected networks, no host/socket mounts, bounded resources, and cleanup on EOF, signal, timeout, and error. No database writes, migrations, restart, stop, remove, or arbitrary command execution are exposed.

Current implementation has deterministic unit/contract coverage. Engine-backed container/network integration remains a required follow-up until a Docker/Podman fixture environment is available in CI.

## Rollback

Feature is opt-in. Remove Debug MCP configuration or pin prior LeeDevKit release. Preserve existing task-assessor registration, user MCP servers, project files, and Compose volumes. Do not remove unrelated containers or networks.

## Compatibility

`[debug].manifest_version = 1` is additive. Projects without `[debug]` retain prior behavior. Future incompatible schema changes require a new manifest version and migration guidance.
