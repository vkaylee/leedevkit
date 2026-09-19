# v0.7.8 — Compose Dependency Sequencing & Dev Project Runs

## Highlights

- Sequence Podman Compose services according to `depends_on` conditions, including `service_completed_successfully` and `service_healthy`.
- Detect dependency cycles, unknown services, invalid conditions, stale dependency containers, and failed health checks before downstream startup.
- Add `leedevkit run --project dev` support for attaching commands to the long-lived development Compose project without tearing it down.
- Add explicit project selection for `run`, including safe cleanup ownership and service-aware `exec` versus `run` behavior.
- Preserve isolated ephemeral projects for default `test` and `run` invocations.

## Verification

- Focused orchestration and Compose tests: passed.
- Release acceptance gate: pending.

## Upgrade

```bash
./leedevkit update --version v0.7.8
```

## Rollback

```bash
./leedevkit update --version v0.7.7
```

## Known Risks

- `run --project dev` requires the development Compose files and a running target service when using `exec` paths.
- Podman Compose startup now rejects invalid dependency graphs rather than starting services opportunistically.
