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
    resolve_lifecycle_dependencies,
    resolve_lifecycle_profiles,
)
from _image_cache import (
    BuildMetadata,
    image_build_lock,
    service_fingerprints_match,
    resolve_build_metadata,
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


def _compose_service_names(
    compose_base: list[str], profiles: list[str]
) -> set[str] | None:
    """Read all Compose service names when Compose metadata is available."""
    result = _run(compose_base + profiles + ["config", "--services"], capture=True)
    if result.returncode != 0 or not isinstance(result.stdout, str):
        return None
    services = {line.strip() for line in result.stdout.splitlines() if line.strip()}
    return services or None


def _compose_service_container(
    compose_base: list[str], profiles: list[str], service: str, project_name: str
) -> str:
    """Resolve Compose service to engine inspect target, with legacy fallback."""
    result = _run(compose_base + profiles + ["ps", "-q", service], capture=True)
    if isinstance(result.stdout, str):
        container_id = result.stdout.strip().splitlines()
        if container_id:
            return container_id[0]
    return f"{project_name}_{service}_1"


_ACTIVE_SERVICES: dict[str, list[str]] = {
    "api": ["apiserver"],
    "lint-api": ["apiserver"],
    "unit-api": ["apiserver"],
    "int-api": ["apiserver"],
    "integration": ["apiserver"],
    "web": ["bun_init", "webdashboard"],
    "lint-web": ["bun_init", "webdashboard"],
    "unit-web": ["bun_init", "webdashboard"],
    "e2e-web": ["bun_init", "webdashboard"],
    "go": ["go"],
    "lint-go": ["go"],
    "unit-go": ["go"],
    "int-go": ["go"],
}


def _configured_service(mode: str) -> str | None:
    """Return configured primary service for lifecycle mode, if one exists."""
    language = (
        "rust"
        if mode in {"api", "lint-api", "unit-api", "int-api", "integration"}
        else "typescript"
        if mode in {"web", "lint-web", "unit-web", "e2e-web"}
        else "go"
        if mode in {"go", "lint-go", "unit-go", "int-go"}
        else ""
    )
    if not language:
        return None
    try:
        from _devkit_config import load_project_config

        services = load_project_config().get("services", {})
        if not isinstance(services, dict):
            return None
        for name, service in services.items():
            if not isinstance(service, dict):
                continue
            service_language = service.get("lang")
            if language == "typescript":
                matches = service_language in {"typescript", "javascript", "web"}
            else:
                matches = service_language == language
            if matches:
                return str(name)
    except (OSError, ValueError, KeyError):
        return None
    return None


def _active_services(mode: str) -> list[str]:
    """Resolve lifecycle services while preserving legacy service fallbacks."""
    configured = _configured_service(mode)
    if configured is None:
        return list(_ACTIVE_SERVICES.get(mode, []))
    if mode in {"web", "lint-web", "unit-web", "e2e-web"}:
        return (
            ["bun_init", configured] if configured == "webdashboard" else [configured]
        )
    return [configured]


def _has_legacy_api_services() -> bool:
    """Return whether API health checks should include legacy DB/Redis services."""
    try:
        from _devkit_config import load_project_config

        services = load_project_config().get("services", {})
        if not isinstance(services, dict) or not services:
            return True
        return "apiserver" in services
    except (OSError, ValueError, KeyError):
        return True


def lifecycle_up(
    mode: str = "all",
    *,
    build: bool = True,
    build_metadata: BuildMetadata | None = None,
    skip_build: bool = False,
    force_build: bool = False,
) -> bool:
    """Bring up selected services after native image freshness validation.

    ``build=True`` requests Compose build only when image metadata is missing or
    stale. ``build=False`` always uses ``--no-build``. ``skip_build`` preserves
    the explicit unverified path and never performs freshness-triggered builds.
    """
    profiles = resolve_lifecycle_profiles(mode)
    dependencies = list(dict.fromkeys(resolve_lifecycle_dependencies(mode)))
    env = bootstrap_env(mode)
    compose_base = env["DOCKER_COMPOSE_BASE"].split()
    project_name = env.get(
        "COMPOSE_PROJECT_NAME", os.environ.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
    )

    if dependencies:
        available_services = _compose_service_names(compose_base, profiles)
        if available_services is not None:
            unknown = [
                service for service in dependencies if service not in available_services
            ]
            if unknown:
                raise ValueError(
                    "Unknown lifecycle dependency service(s) "
                    f"{', '.join(unknown)} for mode {mode!r} in {env['DOCKER_COMPOSE_BASE']}"
                )
    services = list(dict.fromkeys(_active_services(mode) + dependencies))
    metadata = build_metadata
    if metadata is None and build:
        metadata = resolve_build_metadata(compose_base, profiles, services, _run)
    elif metadata is None:
        metadata = BuildMetadata({}, {}, [], None)
    image_names = metadata.images
    lock_context = (
        image_build_lock(image_names) if image_names else contextlib.nullcontext()
    )
    with lock_context:
        # Recheck after lock acquisition. Another invocation may have published a
        # valid image while this process waited.
        previous_override = metadata.override_file
        if build_metadata is None and build:
            metadata = resolve_build_metadata(compose_base, profiles, services, _run)
            if (
                previous_override is not None
                and previous_override != metadata.override_file
            ):
                with contextlib.suppress(OSError):
                    previous_override.unlink()
        active_services = _active_services(mode)
        if skip_build:
            use_build = False
        elif force_build:
            use_build = True
        elif build and metadata.images:
            use_build = not service_fingerprints_match(
                metadata, active_services + dependencies, _get_engine(), _run
            )
        else:
            use_build = build
        effective_compose = list(compose_base)
        if metadata.override_file is not None:
            effective_compose.extend(["-f", str(metadata.override_file)])
        up_cmd = ["up", "-d", "--build" if use_build else "--no-build"]
        if (mode.startswith("lint-") or mode.startswith("unit-")) and not dependencies:
            up_cmd.append("--no-deps")
        if active_services:
            up_cmd.extend(active_services)
        up_result = _run(
            effective_compose + profiles + up_cmd, silent=True, capture=False
        )
    if isinstance(up_result.returncode, int) and up_result.returncode != 0:
        if metadata.override_file is not None:
            with contextlib.suppress(OSError):
                metadata.override_file.unlink()
        print(
            f"Compose startup failed: mode={mode!r}, exit={up_result.returncode}",
            file=sys.stderr if os.environ.get("LEEDEVKIT_QUIET") == "1" else sys.stdout,
        )
        return False
    if (
        use_build
        and metadata.images
        and not service_fingerprints_match(
            metadata, active_services, _get_engine(), _run
        )
    ):
        if metadata.override_file is not None:
            with contextlib.suppress(OSError):
                metadata.override_file.unlink()
        print(
            f"Compose image metadata verification failed: mode={mode!r}",
            file=sys.stderr if os.environ.get("LEEDEVKIT_QUIET") == "1" else sys.stdout,
        )
        return False
    if dependencies:
        dependency_build = False
        if not skip_build and build:
            dependency_build = (
                not service_fingerprints_match(
                    metadata, dependencies, _get_engine(), _run
                )
                if metadata.images
                else True
            )
        dependency_compose = list(compose_base)
        if metadata.override_file is not None:
            dependency_compose.extend(["-f", str(metadata.override_file)])
        dependency_result = _run(
            dependency_compose
            + profiles
            + [
                "up",
                "-d",
                "--build" if dependency_build else "--no-build",
                *dependencies,
            ],
            silent=True,
            capture=False,
        )
        if (
            isinstance(dependency_result.returncode, int)
            and dependency_result.returncode != 0
        ):
            if metadata.override_file is not None:
                with contextlib.suppress(OSError):
                    metadata.override_file.unlink()
            print(
                f"Compose dependency startup failed: mode={mode!r}, exit={dependency_result.returncode}",
                file=sys.stderr
                if os.environ.get("LEEDEVKIT_QUIET") == "1"
                else sys.stdout,
            )
            return False
        if (
            dependency_build
            and metadata.fingerprints
            and not service_fingerprints_match(
                metadata, dependencies, _get_engine(), _run
            )
        ):
            if metadata.override_file is not None:
                with contextlib.suppress(OSError):
                    metadata.override_file.unlink()
            print(
                f"Compose dependency image metadata verification failed: mode={mode!r}",
                file=sys.stderr
                if os.environ.get("LEEDEVKIT_QUIET") == "1"
                else sys.stdout,
            )
            return False
    if metadata.override_file is not None:
        with contextlib.suppress(OSError):
            metadata.override_file.unlink()
    health_services: list[tuple[str, str, str]] = []
    if mode in ("web", "unit-web", "lint-web", "e2e-web"):
        service = _configured_service(mode) or "bun_init"
        health_services.append(("main service", f"{project_name}_{service}_1", service))
        timeout = 120
    elif mode in ("go", "unit-go", "lint-go", "int-go"):
        service = _configured_service(mode) or "go"
        health_services.append(("main service", f"{project_name}_{service}_1", service))
        timeout = 60
    elif (
        mode == "all"
        and (PROJECT_ROOT / "go.mod").exists()
        and not (PROJECT_ROOT / "Cargo.toml").exists()
    ):
        health_services.append(("main service", f"{project_name}_go_1", "go"))
        timeout = 60
    elif mode == "infra-pooler":
        health_services.append(
            ("main service", f"{project_name}_pgbouncer_tx_1", "pgbouncer_tx")
        )  # pragma: no cover
        timeout = 60  # pragma: no cover
    elif mode == "infra-db":  # pragma: no cover
        health_services.append(
            ("main service", f"{project_name}_db_init_1", "db_init")
        )  # pragma: no cover
        timeout = 60  # pragma: no cover
    elif mode == "infra-redis":  # pragma: no cover
        health_services.append(
            ("main service", f"{project_name}_redis_1", "redis")
        )  # pragma: no cover
        timeout = 30  # pragma: no cover
    else:
        service = _configured_service(mode) or "apiserver"
        health_services.append(("main service", f"{project_name}_{service}_1", service))
        if (
            mode in ("api", "int-api", "integration", "all")
            and _has_legacy_api_services()
        ):
            health_services.append(
                ("main service", f"{project_name}_db_system_1", "db_system")
            )
            health_services.append(("main service", f"{project_name}_redis_1", "redis"))
        timeout = 60

    containers_to_check: list[tuple[str, str]] = []
    for label, fallback, service in health_services:
        container = (
            _compose_service_container(compose_base, profiles, service, project_name)
            if dependencies
            else fallback
        )
        containers_to_check.append((label, container))

    for service in dependencies:
        containers_to_check.append(
            (
                f"dependency service {service!r}",
                _compose_service_container(
                    compose_base, profiles, service, project_name
                ),
            )
        )

    checked: set[str] = set()
    for label, container in containers_to_check:
        if container in checked:
            continue
        checked.add(container)
        healthy = False
        for _ in range(timeout):
            if _container_healthy(container):
                healthy = True
                break
            time.sleep(1)  # pragma: no cover

        if not healthy:
            diagnostic = _run(
                [_get_engine(), "ps", "-a", "--filter", f"name={project_name}"],
                capture=True,
            )  # pragma: no cover
            details = (
                diagnostic.stdout.strip() if isinstance(diagnostic.stdout, str) else ""
            )
            output = sys.stdout
            print(
                f"Healthcheck timeout: mode={mode!r}, {label}, "
                f"container={container!r}, project={project_name!r}",
                file=output,
            )
            if details:
                print(details, file=output)
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
