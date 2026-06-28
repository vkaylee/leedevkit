"""Lifecycle management — Python equivalent of _lifecycle.sh.

Provides lifecycle_up() and lifecycle_down() for test infrastructure
container orchestration. Handles profile selection, health checks,
and force-kill fallbacks for stuck containers.
"""

import contextlib
import fcntl
import os
import subprocess
import sys
import tempfile
import time
import typing

from _bootstrap import (
    PROJECT_ROOT,
    SCRIPTS_DIR,
    bootstrap_env,
    detect_engine,
    resolve_lifecycle_profiles,
)


def _get_engine() -> str:
    """Return the container engine (podman or docker) at call time."""
    return os.environ.get("CONTAINER_ENGINE") or detect_engine()


def _run(
    cmd: list[str],
    timeout: int | None = None,
    capture: bool = True,
    check: bool = False,
    silent: bool = False,
) -> subprocess.CompletedProcess[str]:
    """Run a command with project env and PTY-safe stdin."""
    env = os.environ.copy()
    env.update(bootstrap_env())

    kwargs: dict[str, typing.Any] = {
        "cwd": PROJECT_ROOT,
        "env": env,
        "stdin": subprocess.DEVNULL,
        "text": True,
        "check": check,
    }
    if silent:
        kwargs["stdout"] = subprocess.DEVNULL
        kwargs["stderr"] = subprocess.DEVNULL
    elif capture:
        kwargs["stdout"] = subprocess.PIPE
        kwargs["stderr"] = subprocess.PIPE

    if timeout is not None:  # pragma: no cover
        kwargs["timeout"] = timeout

    check_val = kwargs.pop("check", False)
    return subprocess.run(cmd, check=check_val, **kwargs)


def _container_healthy(container_name: str) -> bool:
    """Check if a container reports healthy status."""
    result = _run(
        [_get_engine(), "inspect", "--format={{.State.Health.Status}}", container_name],
        silent=False,
        capture=True,
    )
    return result.stdout.strip() == "healthy"


def lifecycle_up(mode: str = "all") -> bool:
    """Bring up test infrastructure containers for the given mode.

    Returns True if all health checks pass, False otherwise.
    Idempotent — safe for concurrent agents sharing the same project.
    """
    profiles = resolve_lifecycle_profiles(mode)
    env = bootstrap_env(mode)
    compose_base = env["DOCKER_COMPOSE_BASE"].split()

    # Bring services up (idempotent — no pre-cleanup needed)
    # CRITICAL: We must use silent=True (which sets stdout/stderr to DEVNULL)
    # to prevent podman system service and conmon daemons from inheriting the PTY!
    # Inheriting the PTY causes the terminal to hang indefinitely after the script exits.
    up_cmd = ["up", "-d"]
    if mode.startswith("lint-") or mode.startswith("unit-"):
        up_cmd.append("--no-deps")
    _run(compose_base + profiles + up_cmd, silent=True, capture=False)

    # Health check
    project_name = os.environ.get("COMPOSE_PROJECT_NAME", "leedevkit-test")

    containers_to_check = []
    if mode in ("web", "unit-web", "lint-web", "e2e-web"):
        containers_to_check.append(f"{project_name}_bun_init_1")
        timeout = 120
    elif mode == "infra-pooler":
        containers_to_check.append(f"{project_name}_pgbouncer_tx_1")  # pragma: no cover
        timeout = 60  # pragma: no cover
    elif mode == "infra-db":  # pragma: no cover
        containers_to_check.append(f"{project_name}_db_init_1")  # pragma: no cover
        timeout = 60  # pragma: no cover
    elif mode == "infra-redis":  # pragma: no cover
        containers_to_check.append(f"{project_name}_redis_1")  # pragma: no cover
        timeout = 30  # pragma: no cover
    else:
        containers_to_check.append(f"{project_name}_apiserver_1")
        if mode in ("api", "int-api", "integration", "all"):
            containers_to_check.append(f"{project_name}_db_system_1")
            containers_to_check.append(f"{project_name}_redis_1")
        timeout = 60

    for container in containers_to_check:
        healthy = False
        for _ in range(timeout):
            if _container_healthy(container):
                healthy = True
                break
            time.sleep(1)  # pragma: no cover

        if not healthy:
            # Diagnostic
            _run(
                [_get_engine(), "ps", "-a", "--filter", f"name={project_name}"],
                capture=False,
            )  # pragma: no cover
            return False  # pragma: no cover

    return True


def _is_project_locked(project_name: str) -> bool:
    """Check if an OS-level file lock is held for the given project name."""
    from pathlib import Path

    # We must strip the 'pod_' prefix for podman pods
    if project_name.startswith("pod_"):
        project_name = project_name[4:]  # pragma: no cover

    lock_path = Path(tempfile.gettempdir()) / f"{project_name}.lock"
    if not lock_path.exists():
        return False

    try:
        fd = os.open(str(lock_path), os.O_RDONLY)
    except OSError:
        return False

    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        # Lock acquired, meaning it wasn't locked!
        os.close(fd)
        with contextlib.suppress(OSError):
            lock_path.unlink(missing_ok=True)
        return False
    except OSError:
        os.close(fd)
        return True


def sweep_stale_environments() -> None:
    """Sweep and destroy any test pods/containers older than the configured max age.
    This acts as a garbage collector for crashed test runs.
    """
    max_age_mins = int(os.environ.get("TEST_CLEANUP_MAX_AGE_MINUTES", "15"))
    engine = _get_engine()
    age_filter = f"until={max_age_mins}m"
    prefix = "leedevkit-test"

    try:
        if engine == "podman":
            # Sweep stale pods
            res = _run(
                [
                    engine,
                    "pod",
                    "ps",
                    "-q",
                    "--format",
                    "{{.Name}}",
                    "--filter",
                    f"name=pod_{prefix}",
                    "--filter",
                    age_filter,
                ],
                capture=True,
            )
            pods = [p for p in res.stdout.strip().split() if not _is_project_locked(p)]
            if pods:
                print(
                    f"🧹 Sweeping {len(pods)} stale test pods older than {max_age_mins}m..."
                )
                _run([engine, "pod", "rm", "-f"] + pods, silent=True)

            # Sweep stray containers
            res_c = _run(
                [
                    engine,
                    "ps",
                    "-aq",
                    "--format",
                    "{{.Names}}",
                    "--filter",
                    f"name={prefix}",
                    "--filter",
                    age_filter,
                ],
                capture=True,
            )
            containers = [
                c
                for c in res_c.stdout.strip().split()
                if not _is_project_locked(c.split("_")[0])
            ]
            if containers:
                _run([engine, "rm", "-f"] + containers, silent=True)
        else:
            # Docker: sweep stale containers
            res_c = _run(
                [
                    engine,
                    "ps",
                    "-aq",
                    "--format",
                    "{{.Names}}",
                    "--filter",
                    f"name={prefix}",
                    "--filter",
                    age_filter,
                ],
                capture=True,
            )
            containers = [
                c
                for c in res_c.stdout.strip().split()
                if not _is_project_locked(c.split("_")[0])
            ]
            if containers:
                print(
                    f"🧹 Sweeping {len(containers)} stale test containers older than {max_age_mins}m..."
                )
                _run([engine, "rm", "-f"] + containers, silent=True)
    except Exception as e:
        print(f"⚠️ Warning: Stale environment sweep failed: {e}")


# Containers per lifecycle profile — used for force-kill fallback
# when compose down fails to stop PID 1 (e.g. tail -f /dev/null ignoring SIGTERM)
_PROFILE_CONTAINERS: dict[str, list[str]] = {
    "frontend": ["bun_init", "webdashboard"],
    "backend": [
        "apiserver",
        "agent-main",
        "db_system",
        "db_init",
        "redis",
        "pgbouncer_tx",
        "pgbouncer_sess",
    ],
}


def lifecycle_down(mode: str = "all") -> None:
    """Tear down test infrastructure for the given mode.

    Stops containers via compose for the specified profiles only.
    Safe for concurrent agents — never touches containers from other profiles.

    After graceful down, force-removes profile-specific containers that
    compose could not stop (e.g. PID 1 tail -f /dev/null ignoring SIGTERM).
    """
    resolve_lifecycle_profiles(mode)
    env = bootstrap_env(mode)
    compose_base = env["DOCKER_COMPOSE_BASE"].split()
    project_name = os.environ.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
    engine = _get_engine()

    # With dynamic project names (per-run isolation), we can safely and aggressively
    # destroy all containers tied to this project to prevent any "dependent container" hangs.
    if engine == "podman":
        pod_name = f"pod_{project_name}"
        _run([engine, "pod", "rm", "-f", pod_name], silent=True)

        # Also catch any stray containers not in the pod but labeled with the project
        res = _run(
            [
                engine,
                "ps",
                "-aq",
                "--filter",
                f"label=com.docker.compose.project={project_name}",
            ],
            capture=True,
        )
        if res.stdout.strip():  # pragma: no cover
            _run([engine, "rm", "-f"] + res.stdout.strip().split(), silent=True)
    else:
        res = _run(
            [
                engine,
                "ps",
                "-aq",
                "--filter",
                f"label=com.docker.compose.project={project_name}",
            ],
            capture=True,
        )  # pragma: no cover
        if res.stdout.strip():  # pragma: no cover
            _run([engine, "rm", "-f"] + res.stdout.strip().split(), silent=True)

    # Finally run compose down to clean up networks and any dangling state
    # We MUST NOT use -v here, otherwise we will delete the shared Cargo/Bun cache volumes!
    # CRITICAL: Use silent=True to prevent PTY hang from leftover daemons.
    _run(
        compose_base + ["down", "--remove-orphans", "--timeout", "1"],
        silent=True,
    )

    # Perform global garbage collection of crashed tests
    sweep_stale_environments()


def start_watchdog(timeout_sec: int, parent_pid: int) -> int:
    """Spawn a fully detached watchdog that kills parent_pid after timeout.

    Returns the watchdog's own PID so it can be cancelled later.
    All FDs severed from PTY so the watchdog itself never causes a PTY hang.
    """
    watchdog_script = SCRIPTS_DIR / "_watchdog.py"
    proc = subprocess.Popen(
        [
            sys.executable,
            str(watchdog_script),
            str(timeout_sec),
            str(parent_pid),
        ],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        start_new_session=True,
    )
    return proc.pid
