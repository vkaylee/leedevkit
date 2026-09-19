"""Podman Compose startup sequencing for dependency conditions."""

from __future__ import annotations

import json
import subprocess
import time
from collections.abc import Mapping
from typing import Any, Callable

import yaml


class ComposeConfigError(ValueError):
    """The resolved compose graph is invalid before startup mutation."""


_VALID_CONDITIONS = {
    "service_started",
    "service_healthy",
    "service_completed_successfully",
}


def _dependencies(service: Mapping[str, Any]) -> dict[str, str]:
    raw = service.get("depends_on", {})
    if isinstance(raw, list):
        return {str(name): "service_started" for name in raw}
    if not isinstance(raw, Mapping):
        return {}

    result: dict[str, str] = {}
    for name, value in raw.items():
        condition = (
            value.get("condition", "service_started")
            if isinstance(value, Mapping)
            else "service_started"
        )
        condition = str(condition)
        if condition not in _VALID_CONDITIONS:
            raise ComposeConfigError(f"unsupported dependency condition: {condition}")
        result[str(name)] = condition
    return result


def _topological_order(services: Mapping[str, Any]) -> list[str]:
    state: dict[str, int] = {}
    order: list[str] = []

    def visit(name: str) -> None:
        if name not in services:
            raise ComposeConfigError(f"unknown dependency service: {name}")
        if state.get(name) == 1:
            raise ComposeConfigError(f"dependency cycle involving service: {name}")
        if state.get(name) == 2:
            return
        state[name] = 1
        for dependency in _dependencies(services[name]):
            visit(dependency)
        state[name] = 2
        order.append(name)

    for name in services:
        visit(str(name))
    return order


def _compose_config(compose: list[str], env: Mapping[str, str]) -> dict[str, Any]:
    result = subprocess.run(
        [*compose, "config"],
        env=dict(env),
        capture_output=True,
        text=True,
        check=False,
        timeout=30,
    )
    if result.returncode != 0:
        raise ComposeConfigError("compose config failed")
    try:
        config = yaml.safe_load(result.stdout) or {}
    except yaml.YAMLError as exc:
        raise ComposeConfigError("compose config returned invalid YAML") from exc
    services = config.get("services") if isinstance(config, Mapping) else None
    if not isinstance(services, Mapping):
        raise ComposeConfigError("compose config has no services")
    _topological_order(services)
    return {"services": services}


def _inspect_project_containers(
    compose: list[str], env: Mapping[str, str]
) -> list[dict[str, Any]]:
    result = subprocess.run(
        [*compose, "ps", "-q"],
        env=dict(env),
        capture_output=True,
        text=True,
        check=False,
        timeout=30,
    )
    if result.returncode != 0:
        raise ComposeConfigError("compose ps failed")
    ids = result.stdout.split()
    if not ids:
        return []
    result = subprocess.run(
        ["podman", "inspect", *ids],
        env=dict(env),
        capture_output=True,
        text=True,
        check=False,
        timeout=30,
    )
    if result.returncode != 0:
        raise ComposeConfigError("podman inspect failed")
    try:
        inspected = json.loads(result.stdout)
    except (TypeError, ValueError) as exc:
        raise ComposeConfigError("podman inspect returned invalid JSON") from exc
    if not isinstance(inspected, list):
        raise ComposeConfigError("podman inspect returned an invalid container list")
    return [container for container in inspected if isinstance(container, dict)]


def _remove_stale_dependency_containers(
    compose: list[str], env: Mapping[str, str], execute: Callable[[list[str]], None]
) -> None:
    """Remove stale containers from leaves to roots, preserving volumes."""
    containers = _inspect_project_containers(compose, env)
    remaining = {
        str(container["Id"]): set(container.get("Dependencies", []))
        for container in containers
        if container.get("Dependencies")
    }
    while remaining:
        referenced = {
            dependency
            for dependencies in remaining.values()
            for dependency in dependencies
            if dependency in remaining
        }
        leaves = [
            container_id for container_id in remaining if container_id not in referenced
        ]
        if not leaves:
            raise ComposeConfigError("stale container dependency cycle")
        for container_id in leaves:
            execute(["podman", "rm", "-f", container_id])
            remaining.pop(container_id)


def _wait_for_healthy(
    compose: list[str], env: Mapping[str, str], service: str, timeout: float = 120.0
) -> None:
    """Wait for a running container's healthcheck without compose text parsing."""
    deadline = time.monotonic() + timeout
    while True:
        containers = []
        for container in _inspect_project_containers(compose, env):
            labels = container.get("Config", {}).get("Labels", {})
            if labels.get("com.docker.compose.service") == service:
                containers.append(container)
        if containers:
            for container in containers:
                state = container.get("State", {})
                if not state.get("Running", False):
                    raise ComposeConfigError(
                        f"dependency exited before becoming healthy: {service}"
                    )
                health = state.get("Health", {}).get("Status")
                if health == "unhealthy":
                    raise ComposeConfigError(f"dependency became unhealthy: {service}")
            if all(
                container.get("State", {}).get("Health", {}).get("Status") == "healthy"
                for container in containers
            ):
                return
        if time.monotonic() >= deadline:
            raise ComposeConfigError(
                f"timed out waiting for healthy dependency: {service}"
            )
        time.sleep(1.0)


def start_podman_compose(
    compose: list[str],
    env: Mapping[str, str],
    execute: Callable[[list[str]], None],
) -> None:
    """Start Podman Compose services in dependency order."""
    config = _compose_config(compose, env)
    services: Mapping[str, Any] = config["services"]
    order = _topological_order(services)
    completed: set[str] = set()
    started: set[str] = set()
    healthy: set[str] = set()
    jobs = {
        dependency
        for service in services.values()
        for dependency, condition in _dependencies(service).items()
        if condition == "service_completed_successfully"
    }

    _remove_stale_dependency_containers(compose, env, execute)

    for service in order:
        dependencies = _dependencies(services[service])
        for dependency, condition in dependencies.items():
            if (
                condition == "service_completed_successfully"
                and dependency not in completed
            ):
                raise ComposeConfigError(
                    f"completed dependency was not run: {dependency}"
                )
            if condition == "service_healthy" and dependency not in healthy:
                if dependency not in started:
                    raise ComposeConfigError(
                        f"healthy dependency was not started: {dependency}"
                    )
                _wait_for_healthy(compose, env, dependency)
                healthy.add(dependency)
            if (
                condition == "service_started"
                and dependency not in started
                and dependency not in completed
            ):
                raise ComposeConfigError(
                    f"started dependency was not started: {dependency}"
                )

        if service in jobs:
            execute([*compose, "run", "--rm", "--no-deps", service])
            completed.add(service)
        else:
            execute([*compose, "up", "-d", "--no-deps", service])
            started.add(service)
