"""Launch Debug MCP in a temporary, isolated Compose container."""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import uuid
from pathlib import Path
from typing import Any

import yaml
from _bootstrap import DEVKIT_ROOT, detect_compose_cmd, detect_engine
from _debug_manifest import DebugEnvironment, DebugManifestError, load_debug_environment


class DebugLaunchError(RuntimeError):
    """Debug container cannot be safely launched."""


def _compose_args(
    environment: DebugEnvironment, override: Path | None = None
) -> list[str]:
    args = [*detect_compose_cmd(), "-p", environment.compose_project]
    for compose_file in environment.compose_files:
        args.extend(["-f", str(environment.project_root / compose_file)])
    if override is not None:
        args.extend(["-f", str(override)])
    for profile in environment.profiles:
        args.extend(["--profile", profile])
    return args


def _cleanup_owned_run(environment: DebugEnvironment, override: Path) -> bool:
    """Remove only this manifest's one-off service and verify it is gone."""
    base = _compose_args(environment, override)
    command = [*base, "rm", "--force", "--stop", "--no-deps", "leedevkit-debug"]
    try:
        result = subprocess.run(
            command,
            cwd=environment.project_root,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            check=False,
            timeout=10,
        )
        if result.returncode != 0:
            return False
        verify = subprocess.run(
            [*base, "ps", "-q", "leedevkit-debug"],
            cwd=environment.project_root,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            check=False,
            timeout=10,
        )
        return not verify.stdout.strip()
    except (OSError, subprocess.TimeoutExpired):
        return False


def _terminate_owned_process(environment: DebugEnvironment, override: Path) -> None:
    """Stop only this Compose project's debug service after launcher timeout."""
    command = [
        *_compose_args(environment, override),
        "kill",
        "--signal",
        "SIGTERM",
        "leedevkit-debug",
    ]
    try:
        subprocess.run(
            command,
            cwd=environment.project_root,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
            timeout=5,
        )
    except (OSError, subprocess.TimeoutExpired):
        pass


def _compose_config(environment: DebugEnvironment) -> dict[str, Any]:
    command = detect_compose_cmd()
    args = [*command, "-p", environment.compose_project]
    for compose_file in environment.compose_files:
        args.extend(["-f", str(environment.project_root / compose_file)])
    for profile in environment.profiles:
        args.extend(["--profile", profile])
    args.append("config")
    result = subprocess.run(
        args,
        cwd=environment.project_root,
        capture_output=True,
        text=True,
        stdin=subprocess.DEVNULL,
        check=False,
        timeout=30,
    )
    if result.returncode != 0:
        raise DebugLaunchError("Compose config validation failed")
    try:
        config = yaml.safe_load(result.stdout) or {}
    except yaml.YAMLError as exc:
        raise DebugLaunchError("Compose config returned invalid YAML") from exc
    if not isinstance(config, dict) or not isinstance(config.get("services"), dict):
        raise DebugLaunchError("Compose config has no services")
    return config


def _network_names(environment: DebugEnvironment, config: dict[str, Any]) -> list[str]:
    networks = config.get("networks", {})
    if not isinstance(networks, dict):
        raise DebugLaunchError("Compose config has no networks")
    result: list[str] = []
    for approved in environment.approved_networks:
        network = networks.get(approved)
        if not isinstance(network, dict):
            raise DebugLaunchError(
                f"approved network is not in selected Compose project: {approved}"
            )
        actual = network.get("name") or f"{environment.compose_project}_{approved}"
        if not isinstance(actual, str) or not actual:
            raise DebugLaunchError("Compose network name is invalid")
        result.append(actual)
    return result


def _validate_services(environment: DebugEnvironment, config: dict[str, Any]) -> None:
    services = config.get("services", {})
    names = set(services)
    for service in environment.services.values():
        if service.host not in names and not any(
            alias in names for alias in service.aliases
        ):
            raise DebugLaunchError(
                f"approved service is not in selected Compose project: {service.service_id}"
            )


def _inspect_networks(environment: DebugEnvironment, names: list[str]) -> None:
    engine = detect_engine()
    for name in names:
        result = subprocess.run(
            [engine, "network", "inspect", name],
            capture_output=True,
            text=True,
            stdin=subprocess.DEVNULL,
            check=False,
            timeout=15,
        )
        if result.returncode != 0:
            raise DebugLaunchError(f"approved network is unavailable: {name}")
        try:
            inspected = json.loads(result.stdout)
            item = inspected[0]
            labels = item.get("Labels", {})
            owner = labels.get("com.docker.compose.project")
        except (ValueError, IndexError, AttributeError, TypeError) as exc:
            raise DebugLaunchError("network inspection returned invalid data") from exc
        if owner != environment.compose_project:
            raise DebugLaunchError(
                f"network is not owned by selected Compose project: {name}"
            )


def _safe_mounts(environment: DebugEnvironment) -> list[str]:
    root = environment.project_root.resolve()
    paths = {environment.source_path.resolve()}
    for service in environment.services.values():
        for source in service.log_sources:
            path = (root / source).resolve()
            if root not in path.parents or not path.is_file() or path.is_symlink():
                raise DebugLaunchError("approved log source is outside project root")
            paths.add(path)
    mounts: list[str] = []
    for path in sorted(paths):
        relative = path.relative_to(root).as_posix()
        mounts.append(f"{path}:/workspace/{relative}:ro")
    return mounts


def _write_override(
    environment: DebugEnvironment,
    networks: list[str],
    mounts: list[str],
    directory: Path,
) -> Path:
    network_keys = {
        f"debug_net_{index}": {"external": True, "name": name}
        for index, name in enumerate(networks)
    }
    service = {
        "image": f"leedevkit-debug:{uuid.uuid4().hex[:12]}",
        "build": {
            "context": str(DEVKIT_ROOT),
            "dockerfile": "container/debug/Dockerfile",
        },
        "profiles": ["debug"],
        "read_only": True,
        "user": "65532:65532",
        "cap_drop": ["ALL"],
        "security_opt": ["no-new-privileges:true"],
        "pids_limit": 64,
        "mem_limit": "256m",
        "tmpfs": ["/tmp:rw,nosuid,nodev,noexec,size=16m"],
        "volumes": mounts,
        "networks": list(network_keys),
        "working_dir": "/workspace",
        "entrypoint": ["python", "/app/_debug_mcp.py"],
        "stdin_open": False,
        "tty": False,
        "environment": ["PYTHONDONTWRITEBYTECODE=1"],
    }
    path = directory / "debug-compose.override.yml"
    path.write_text(
        yaml.safe_dump(
            {"services": {"leedevkit-debug": service}, "networks": network_keys},
            sort_keys=False,
        )
    )
    return path


def launch(
    manifest: Path | None, environment_name: str, compose_project: str | None = None
) -> int:
    environment = load_debug_environment(
        manifest, environment_name, compose_project=compose_project
    )
    config = _compose_config(environment)
    _validate_services(environment, config)
    networks = _network_names(environment, config)
    _inspect_networks(environment, networks)
    mounts = _safe_mounts(environment)
    relative_manifest = (
        environment.source_path.resolve()
        .relative_to(environment.project_root.resolve())
        .as_posix()
    )
    command = detect_compose_cmd()
    with tempfile.TemporaryDirectory(prefix="leedevkit-debug-") as temp_dir:
        override = _write_override(environment, networks, mounts, Path(temp_dir))
        args = [*command, "-p", environment.compose_project]
        for compose_file in environment.compose_files:
            args.extend(["-f", str(environment.project_root / compose_file)])
        args.extend(
            [
                "-f",
                str(override),
                "--profile",
                "debug",
                "run",
                "--rm",
                "-T",
                "--no-deps",
                "--build",
                "leedevkit-debug",
                "--environment",
                environment.name,
                "--manifest",
                f"/workspace/{relative_manifest}",
                "--project-root",
                "/workspace",
            ]
        )
        try:
            result = subprocess.run(
                args,
                cwd=environment.project_root,
                stdin=None,
                check=False,
                timeout=environment.limits["request_timeout_ms"] / 1000,
            )
            return result.returncode
        except subprocess.TimeoutExpired:
            _terminate_owned_process(environment, override)
            raise
        finally:
            _cleanup_owned_run(environment, override)


def main(
    manifest: str | None,
    environment: str,
    compose_project: str | None = None,
) -> int:
    try:
        return launch(
            Path(manifest) if manifest else None, environment, compose_project
        )
    except (
        DebugManifestError,
        DebugLaunchError,
        OSError,
        subprocess.TimeoutExpired,
    ) as exc:
        print(str(exc), file=sys.stderr)
        return 2
