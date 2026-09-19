# Implementation Plan: `leedevkit test` Run Isolation

## Decision

No separate parallel runner is needed. Every invocation of `./leedevkit test <target>` already receives a unique Docker Compose project name in `scripts/_orchestrator.py`:

```python
suffix = uuid.uuid4().hex[:8]
project_name = f"leedevkit-test-{suffix}"
```

The value is applied to both `COMPOSE_PROJECT_NAME` and `PODMAN_COMPOSE_PROJECT_NAME` before test execution. Docker Compose therefore isolates containers and networks for each invocation, including runs that overlap in time.

SQLite test databases remain container-internal. Compose tmpfs/ephemeral mounts discard them when `compose down --remove-orphans -v` removes the run's containers. No host DB directory or extra cleanup layer is required.

`QUIZCLASS_VOLUME_NAMESPACE` may remain shared for Go/Bun cache reuse. Override it only when full volume isolation is required. Host ports still require unique allocation by the repository's existing Compose/test contract when a test exposes ports externally.

## Verification

```bash
python3 -m pytest scripts/tests/test_orchestrator.py scripts/tests/test_lifecycle.py -q
```

No change required in the quizclass repository.
