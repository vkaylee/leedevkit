"""Validated, project-agnostic Debug MCP manifest contract."""

from __future__ import annotations

import ipaddress
import re
import tomllib
from dataclasses import dataclass
from pathlib import Path
from typing import Any


class DebugManifestError(ValueError):
    """Manifest is missing, ambiguous, or unsafe."""


_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$")
_DEFAULT_LIMITS = {
    "request_timeout_ms": 5000,
    "query_timeout_ms": 3000,
    "max_rows": 100,
    "max_bytes": 65536,
    "max_log_lines": 200,
    "session_idle_timeout_ms": 900000,
}


def _table(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise DebugManifestError(f"{name} must be a table")
    return value


def _strings(value: Any, name: str, *, nonempty: bool = True) -> list[str]:
    if not isinstance(value, list) or any(not isinstance(v, str) for v in value):
        raise DebugManifestError(f"{name} must be a string list")
    if nonempty and not value:
        raise DebugManifestError(f"{name} must not be empty")
    if any(not v.strip() for v in value):
        raise DebugManifestError(f"{name} contains an empty value")
    return list(value)


def _id(value: Any, name: str) -> str:
    if not isinstance(value, str) or not _ID.fullmatch(value):
        raise DebugManifestError(f"{name} must be a safe identifier")
    return value


def _relative_path(value: Any, name: str) -> str:
    if not isinstance(value, str) or not value or Path(value).is_absolute():
        raise DebugManifestError(f"{name} must be a relative path")
    if ".." in Path(value).parts:
        raise DebugManifestError(f"{name} must stay inside project root")
    return value


def _positive_int(value: Any, name: str, *, maximum: int) -> int:
    if (
        isinstance(value, bool)
        or not isinstance(value, int)
        or not 0 < value <= maximum
    ):
        raise DebugManifestError(f"{name} must be an integer in 1..{maximum}")
    return value


def _safe_host(value: Any, name: str) -> str:
    if (
        not isinstance(value, str)
        or not value
        or value.lower()
        in {
            "localhost",
            "host.docker.internal",
            "host.containers.internal",
        }
    ):
        raise DebugManifestError(f"{name} is not an approved internal hostname")
    try:
        address = ipaddress.ip_address(value)
    except ValueError:
        return value
    if (
        address.is_loopback
        or address.is_link_local
        or address.is_unspecified
        or address.is_reserved
    ):
        raise DebugManifestError(f"{name} is not an approved internal address")
    if str(address) in {"169.254.169.254", "100.100.100.200"}:
        raise DebugManifestError(f"{name} targets a metadata address")
    return value


def _safe_patterns(value: Any) -> list[str]:
    patterns = _strings(value, "redaction.patterns", nonempty=False)
    for pattern in patterns:
        try:
            re.compile(pattern)
        except re.error as exc:
            raise DebugManifestError(
                "redaction.patterns contains invalid regex"
            ) from exc
    return patterns


@dataclass(frozen=True)
class DebugService:
    service_id: str
    host: str
    port: int
    aliases: tuple[str, ...]
    health_paths: tuple[str, ...]
    http_get_paths: tuple[str, ...]
    log_sources: tuple[str, ...]


@dataclass(frozen=True)
class DebugEnvironment:
    name: str
    compose_project: str
    compose_files: tuple[str, ...]
    profiles: tuple[str, ...]
    approved_networks: tuple[str, ...]
    services: dict[str, DebugService]
    database: dict[str, Any]
    redaction_patterns: tuple[str, ...]
    sensitive_columns: tuple[str, ...]
    limits: dict[str, int]
    project_root: Path
    source_path: Path

    def metadata(self) -> dict[str, Any]:
        return {
            "environment": self.name,
            "compose_project": self.compose_project,
            "mode": "read-only",
            "networks": list(self.approved_networks),
            "services": sorted(self.services),
            "capabilities": {
                "health": bool(self.services),
                "http_get": any(s.http_get_paths for s in self.services.values()),
                "logs": any(s.log_sources for s in self.services.values()),
                "database_schema": bool(self.database.get("enabled", False)),
                "database_query": bool(self.database.get("templates")),
            },
            "limits": dict(self.limits),
            "evidence_sources": [
                "compose_state",
                "approved_health",
                "approved_logs",
                "approved_database_metadata",
            ],
        }


def _load_data(path: Path) -> dict[str, Any]:
    try:
        with path.open("rb") as stream:
            return tomllib.load(stream)
    except FileNotFoundError as exc:
        raise DebugManifestError(f"manifest not found: {path}") from exc
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise DebugManifestError("manifest is not valid TOML") from exc


def load_debug_environment(
    manifest_path: Path | None,
    environment: str,
    *,
    project_root: Path | None = None,
    compose_project: str | None = None,
    validate_compose_files: bool = True,
) -> DebugEnvironment:
    if not isinstance(environment, str) or not _ID.fullmatch(environment):
        raise DebugManifestError("environment must be a safe identifier")
    if "prod" in environment.lower() or "production" in environment.lower():
        raise DebugManifestError(
            "production environments are not eligible for Debug MCP"
        )
    source = (manifest_path or Path.cwd() / "leedevkit.toml").resolve()
    data = _load_data(source)
    debug = _table(data.get("debug"), "[debug]")
    if debug.get("manifest_version") != 1:
        raise DebugManifestError("[debug].manifest_version must equal 1")
    environments = _table(debug.get("environments"), "[debug.environments]")
    raw = environments.get(environment)
    if raw is None:
        raise DebugManifestError(f"unknown debug environment: {environment}")
    cfg = _table(raw, f"[debug.environments.{environment}]")
    root = (project_root or source.parent).resolve()
    project = _id(cfg.get("compose_project"), "compose_project")
    if "prod" in project.lower() or "production" in project.lower():
        raise DebugManifestError(
            "production Compose projects are not eligible for Debug MCP"
        )
    if compose_project is not None and compose_project != project:
        raise DebugManifestError("explicit Compose project does not match manifest")
    files = tuple(
        _relative_path(v, "compose_files[]")
        for v in _strings(cfg.get("compose_files"), "compose_files")
    )
    if validate_compose_files and any(not (root / item).is_file() for item in files):
        raise DebugManifestError(
            "one or more Compose files do not exist inside project root"
        )
    profiles = tuple(
        _id(v, "profiles[]")
        for v in _strings(cfg.get("profiles", ["debug"]), "profiles")
    )
    networks = tuple(
        _id(v, "approved_networks[]")
        for v in _strings(cfg.get("approved_networks"), "approved_networks")
    )
    services_cfg = _table(cfg.get("services"), "services")
    services: dict[str, DebugService] = {}
    for service_id, raw_service in services_cfg.items():
        sid = _id(service_id, "service id")
        service = _table(raw_service, f"services.{sid}")
        host = _safe_host(
            service.get("internal_host", sid), f"services.{sid}.internal_host"
        )
        port = _positive_int(
            service.get("internal_port"), f"services.{sid}.internal_port", maximum=65535
        )
        aliases = tuple(
            _id(v, f"services.{sid}.aliases[]")
            for v in _strings(service.get("aliases", [sid]), f"services.{sid}.aliases")
        )
        health = tuple(
            _relative_url(v, f"services.{sid}.health_paths[]")
            for v in _strings(
                service.get("health_paths", []),
                f"services.{sid}.health_paths",
                nonempty=False,
            )
        )
        get_paths = tuple(
            _relative_url(v, f"services.{sid}.http_get_paths[]")
            for v in _strings(
                service.get("http_get_paths", []),
                f"services.{sid}.http_get_paths",
                nonempty=False,
            )
        )
        sources = tuple(
            _relative_path(v, f"services.{sid}.log_sources[]")
            for v in _strings(
                service.get("log_sources", []),
                f"services.{sid}.log_sources",
                nonempty=False,
            )
        )
        services[sid] = DebugService(
            sid, host, port, aliases, health, get_paths, sources
        )
    database = _table(cfg.get("database", {"enabled": False}), "database")
    if database.get("enabled", False):
        _id(database.get("service"), "database.service")
        _id(database.get("role"), "database.role")
        _strings(database.get("schemas"), "database.schemas")
        if database.get("templates") is not None and not isinstance(
            database["templates"], dict
        ):
            raise DebugManifestError("database.templates must be a table")
    redaction = _table(cfg.get("redaction", {}), "redaction")
    patterns = tuple(_safe_patterns(redaction.get("patterns", [])))
    columns = tuple(
        _id(v, "redaction.sensitive_columns[]")
        for v in _strings(
            redaction.get("sensitive_columns", []),
            "redaction.sensitive_columns",
            nonempty=False,
        )
    )
    limits = {}
    for key, default in _DEFAULT_LIMITS.items():
        limits[key] = _positive_int(
            cfg.get(key, default),
            key,
            maximum=3_600_000
            if key.endswith("timeout_ms")
            else (1_000_000 if key == "max_bytes" else 100_000),
        )
    return DebugEnvironment(
        environment,
        project,
        files,
        profiles,
        networks,
        services,
        database,
        patterns,
        columns,
        limits,
        root,
        source,
    )


def _relative_url(value: Any, name: str) -> str:
    if (
        not isinstance(value, str)
        or not value.startswith("/")
        or value.startswith("//")
        or "?" in value
        or "#" in value
    ):
        raise DebugManifestError(f"{name} must be an internal path")
    return value
