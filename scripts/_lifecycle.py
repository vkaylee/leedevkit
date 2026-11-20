"""Lifecycle management — Python equivalent of _lifecycle.sh.

Provides lifecycle_up() and lifecycle_down() for test infrastructure
container orchestration. Handles profile selection, health checks,
and force-kill fallbacks for stuck containers.
"""

import contextlib
import datetime
import fcntl
import json
import os
import subprocess
import sys
import tempfile
import time
import typing
from pathlib import Path

from _bootstrap import (
    PROJECT_ROOT,
    SCRIPTS_DIR,
    bootstrap_env,
    detect_engine,
    resolve_container_provider,
    resolve_lifecycle_dependencies,
    resolve_lifecycle_profiles,
)
from _image_cache import (
    BuildMetadata,
    image_build_lock,
    resolve_build_metadata,
    service_fingerprints_match,
)
from _lock_manager import LockManager


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
    """Resolve service identity through Compose, with legacy fallback."""
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


def _go_lifecycle_shadow(
    *,
    provider: object,
    project_name: str,
    compose_base: list[str],
    profiles: list[str],
    services: list[str],
    dependencies: list[str],
    build: bool,
    native_wait: bool,
    wait_timeout: int = 120,
    owns_project: bool = True,
    no_deps: bool = False,
    override_file: str | None = None,
) -> None:
    """Compare deterministic lifecycle argv with Go when shadow mode is enabled."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return
    from _go_core import plan_lifecycle_go

    provider_config = typing.cast(typing.Any, provider)
    files: list[str] = []
    base_tail = compose_base[len(provider_config.compose) :]
    for index, value in enumerate(base_tail):
        if value == "-f" and index + 1 < len(base_tail):
            files.append(base_tail[index + 1])
    payload = {
        "provider": {
            "name": provider_config.name,
            "cli": list(provider_config.cli),
            "compose": list(provider_config.compose),
            "capabilities": sorted(provider_config.capabilities),
            "settings": dict(provider_config.settings),
            "source": provider_config.source,
        },
        "project": project_name,
        "files": files,
        "profiles": profiles,
        "services": services,
        "dependencies": dependencies,
        "build": build,
        "native_wait": native_wait,
        "wait_timeout": wait_timeout,
        "owns_project": owns_project,
        "no_deps": no_deps,
        "override_file": override_file or "",
    }
    response = plan_lifecycle_go(payload)
    expected = list(compose_base)
    if override_file:
        expected.extend(["-f", override_file])
    expected.extend(profiles)
    expected.extend(["up", "-d", "--build" if build else "--no-build"])
    if native_wait:
        expected.extend(["--wait", "--wait-timeout", str(wait_timeout)])
    if no_deps:
        expected.append("--no-deps")
    expected.extend(dict.fromkeys(services + (dependencies if native_wait else [])))
    if response is None or response.get("up") != expected:
        raise RuntimeError("Go lifecycle shadow result differs from Python result")


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
    provider = resolve_container_provider()
    required = {"compose", "config", "up", "down", "ps"}
    missing = sorted(required - provider.capabilities)
    if missing:
        raise ValueError(
            f'container provider "{provider.name}" lacks required capabilities: '
            + ", ".join(missing)
        )
    native_wait = "wait" in provider.capabilities
    env = bootstrap_env(mode, provider)
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
        startup_services = list(
            dict.fromkeys(
                active_services + dependencies if native_wait else active_services
            )
        )
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
        # Compose owns dependency ordering and health readiness when provider
        # declares native wait support. Providers without it keep legacy checks
        # until their adapter is implemented.
        if native_wait:
            up_cmd.extend(["--wait", "--wait-timeout", "120"])
        if mode.startswith(("lint-", "unit-")) and not dependencies:
            up_cmd.append("--no-deps")
        if startup_services:
            up_cmd.extend(startup_services)
        _go_lifecycle_shadow(
            provider=provider,
            project_name=project_name,
            compose_base=compose_base,
            profiles=profiles,
            services=active_services,
            dependencies=dependencies,
            build=use_build,
            native_wait=native_wait,
            no_deps=(mode.startswith(("lint-", "unit-")) and not dependencies),
            override_file=(
                str(metadata.override_file)
                if metadata.override_file is not None
                else None
            ),
        )
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
    if dependencies and not native_wait:
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
    if native_wait:
        return True

    # Legacy providers still resolve service IDs through Compose before using
    # engine health inspection. This avoids Compose v2 name assumptions.
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
        container = _compose_service_container(
            compose_base, profiles, service, project_name
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


def _project_lock_path(project_name: str) -> Path:
    return Path(tempfile.gettempdir()) / f"{project_name.removeprefix('pod_')}.lock"


def _is_project_locked(project_name: str) -> bool:
    """Check an OS lock without unlinking a path owned by another process."""
    lock_path = _project_lock_path(project_name)
    try:
        fd = os.open(str(lock_path), os.O_RDONLY)
    except OSError:
        return False
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError:
        return True
    finally:
        with contextlib.suppress(OSError):
            os.close(fd)
    return False


def _compose_base_for_project(mode: str, project_name: str) -> list[str]:
    """Build Compose argv for *project_name* without mutating process env."""
    env = bootstrap_env(mode)
    compose_base = env["DOCKER_COMPOSE_BASE"].split()
    try:
        index = compose_base.index("-p")
        compose_base[index + 1] = project_name
    except (ValueError, IndexError):
        compose_base[1:1] = ["-p", project_name]
    return compose_base


def _container_created(project_name: str) -> float | None:
    """Return oldest container creation time for project, or None if unknown."""
    listed = _run(
        [
            _get_engine(),
            "ps",
            "-a",
            "--filter",
            f"label=com.docker.compose.project={project_name}",
            "--format",
            "{{.ID}}",
        ],
        capture=True,
        timeout=15,
    )
    if listed.returncode != 0 or not isinstance(listed.stdout, str):
        return None
    ids = [value.strip() for value in listed.stdout.splitlines() if value.strip()]
    if not ids:
        return None
    inspected = _run(
        [_get_engine(), "inspect", "--format={{.Created}}", *ids],
        capture=True,
        timeout=15,
    )
    if inspected.returncode != 0 or not isinstance(inspected.stdout, str):
        return None
    values: list[float] = []
    for raw in inspected.stdout.splitlines():
        try:
            value = datetime.datetime.fromisoformat(raw.strip())
        except ValueError:
            return None
        values.append(value.timestamp())
    return min(values) if len(values) == len(ids) else None


def sweep_stale_environments() -> None:
    """Remove only old, unlocked, identifiable LeeDevKit test projects."""
    try:
        max_age_mins = int(os.environ.get("TEST_CLEANUP_MAX_AGE_MINUTES", "15"))
    except ValueError:
        max_age_mins = 15
    prefix = "leedevkit-test-"
    env = bootstrap_env("all")
    compose = env["DOCKER_COMPOSE_TOOL"].split()
    try:
        result = _run(
            [*compose, "ls", "--all", "--format", "json"],
            capture=True,
            timeout=15,
        )
        projects = json.loads(result.stdout or "[]")
        if not isinstance(projects, list):
            raise TypeError("Compose project listing is not an array")
        now = time.time()
        for project in projects:
            name = project.get("Name") if isinstance(project, dict) else None
            if not isinstance(name, str) or not name.startswith(prefix):
                continue
            if _is_project_locked(name):
                print(f"Stale sweep kept {name}: lock held")
                continue
            created = _container_created(name)
            if created is None:
                print(f"Stale sweep kept {name}: container age unknown")
                continue
            if now - created < max_age_mins * 60:
                print(f"Stale sweep kept {name}: younger than {max_age_mins}m")
                continue
            fd = LockManager.acquire(name)
            if fd is None:
                print(f"Stale sweep kept {name}: lock acquired by another process")
                continue
            try:
                # Recheck after claiming lock. New invocations can start between ls and here.
                created_again = _container_created(name)
                if (
                    created_again is None
                    or time.time() - created_again < max_age_mins * 60
                ):
                    print(f"Stale sweep kept {name}: metadata changed or age unknown")
                    continue
                cleanup_result = lifecycle_down("all", project_name=name, sweep=False)
                print(f"Stale sweep cleaned {name}: {cleanup_result}")
            finally:
                LockManager.release(fd, name)
    except Exception as exc:  # noqa: BLE001 - stale sweep must never abort teardown
        print(f"⚠️ Warning: Stale environment sweep failed: {exc}")


def _write_cleanup_log(
    project_name: str, event: str, data: dict[str, typing.Any]
) -> None:
    safe = "".join(c if c.isalnum() or c in "-_." else "_" for c in project_name)
    path = PROJECT_ROOT / ".test_logs" / safe / "cleanup.log"
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps({"event": event, **data}, sort_keys=True) + "\n")
    except OSError:
        pass


def lifecycle_down(
    mode: str = "all",
    *,
    project_name: str | None = None,
    sweep: bool = True,
) -> dict[str, typing.Any]:
    """Tear down one Compose project with bounded retries and diagnostics."""
    resolve_lifecycle_profiles(mode)
    project = project_name or os.environ.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
    compose_base = _compose_base_for_project(mode, project)
    command = compose_base + ["down", "--remove-orphans", "--timeout", "1"]
    attempts: list[dict[str, typing.Any]] = []
    for attempt in range(1, 3):
        record: dict[str, typing.Any] = {"attempt": attempt, "command": command}
        try:
            result = _run(command, timeout=10, capture=True)
            record.update(
                {
                    "exit_code": result.returncode,
                    "stdout": result.stdout
                    if isinstance(result.stdout, str)
                    else str(result.stdout or ""),
                    "stderr": result.stderr
                    if isinstance(result.stderr, str)
                    else str(result.stderr or ""),
                }
            )
            attempts.append(record)
            if result.returncode == 0:
                break
        except subprocess.TimeoutExpired as exc:
            record.update({"exit_code": 124, "stdout": "", "stderr": str(exc)})
            attempts.append(record)
        except OSError as exc:
            record.update({"exit_code": 1, "stdout": "", "stderr": str(exc)})
            attempts.append(record)
        if attempt < 2:
            time.sleep(0.1)
    success = bool(attempts and attempts[-1].get("exit_code") == 0)
    fallback: dict[str, typing.Any] = {"attempted": False, "result": "not-needed"}
    if not success:
        fallback["attempted"] = True
        rm_command = compose_base + ["rm", "--force", "--stop", "--no-deps"]
        try:
            result = _run(rm_command, timeout=10, capture=True)
            fallback.update(
                {
                    "command": rm_command,
                    "exit_code": result.returncode,
                    "stdout": result.stdout or "",
                    "stderr": result.stderr or "",
                    "result": "success" if result.returncode == 0 else "failed",
                }
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            fallback.update({"result": "failed", "error": str(exc), "exit_code": 124})
    report = {
        "project": project,
        "success": success,
        "attempts": attempts,
        "fallback": fallback,
    }
    _write_cleanup_log(project, "teardown", report)
    if sweep:
        sweep_stale_environments()
    return report


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
