"""Read-only bounded runtime for generic Debug MCP."""

from __future__ import annotations

import json
import re
import subprocess
import time
from collections.abc import Callable
from typing import Any
from urllib.error import URLError
from urllib.parse import urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener

from _debug_manifest import DebugEnvironment, DebugManifestError, DebugService

_SECRET = re.compile(
    r"(?i)(password|secret|token|authorization|cookie|api[_-]?key|jwt|set-cookie)"
)
_SECRET_VALUE = re.compile(
    r"(?i)(password|secret|token|authorization|cookie|api[_-]?key|jwt|set-cookie)"
    r"(\s*[:=]\s*(?:Bearer\s+)?|\s+Bearer\s+)(['\"]?)([^'\"\s,;]+)\3"
)


class DebugRuntimeError(RuntimeError):
    """Safe runtime operation failed."""


def redact(value: Any, patterns: tuple[str, ...] = ()) -> Any:
    if isinstance(value, dict):
        return {
            key: "[REDACTED]" if _SECRET.search(str(key)) else redact(item, patterns)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [redact(item, patterns) for item in value]
    if isinstance(value, str):
        result = value
        for pattern in patterns:
            result = re.sub(pattern, "[REDACTED]", result)
        return _SECRET_VALUE.sub(r"\1\2\3[REDACTED]\3", result)
    return value


def redact_text(value: str, patterns: tuple[str, ...] = ()) -> str:
    """Redact structured JSON and credential values from unstructured text."""
    try:
        parsed = json.loads(value)
    except json.JSONDecodeError:
        return redact(value, patterns)
    return json.dumps(redact(parsed, patterns), separators=(",", ":"))


class _ApprovedRedirects(HTTPRedirectHandler):
    def __init__(self, service: DebugService) -> None:
        self.service = service

    def redirect_request(
        self, req: Request, fp: Any, code: int, msg: str, headers: Any, newurl: str
    ) -> Request | None:
        validate_redirect(req.full_url, newurl, self.service)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def _bounded_text(value: bytes | str, maximum: int) -> str:
    text = (
        value.decode("utf-8", errors="replace") if isinstance(value, bytes) else value
    )
    return text[:maximum]


def validate_redirect(original: str, target: str, service: DebugService) -> None:
    parsed = urlparse(target)
    if (
        parsed.scheme != "http"
        or parsed.hostname not in {service.host, *service.aliases}
        or parsed.port not in {None, service.port}
    ):
        raise DebugRuntimeError("redirect destination is not approved")


class DebugRuntime:
    def __init__(
        self,
        environment: DebugEnvironment,
        runner: Callable[..., subprocess.CompletedProcess[str]] | None = None,
    ) -> None:
        self.environment = environment
        self.runner = runner or subprocess.run
        self.started_at = time.monotonic()

    def _service(self, service_id: str) -> DebugService:
        try:
            return self.environment.services[service_id]
        except KeyError as exc:
            raise DebugManifestError("unknown approved service") from exc

    def environment_metadata(self) -> dict[str, Any]:
        return self.environment.metadata()

    def service_health(self, service_id: str, path: str) -> dict[str, Any]:
        service = self._service(service_id)
        if path not in service.health_paths:
            raise DebugManifestError("health path is not approved")
        return self._http(service, path)

    def http_get(self, service_id: str, path: str) -> dict[str, Any]:
        service = self._service(service_id)
        if path not in service.http_get_paths:
            raise DebugManifestError("GET path is not approved")
        return self._http(service, path)

    def _http(self, service: DebugService, path: str) -> dict[str, Any]:
        url = f"http://{service.host}:{service.port}{path}"
        opener = build_opener(_ApprovedRedirects(service))
        opener.addheaders = []
        request = Request(url, method="GET", headers={"Accept": "application/json"})
        try:
            with opener.open(
                request, timeout=self.environment.limits["request_timeout_ms"] / 1000
            ) as response:
                body = _bounded_text(
                    response.read(self.environment.limits["max_bytes"] + 1),
                    self.environment.limits["max_bytes"],
                )
                return {
                    "status": "ok",
                    "http_status": response.status,
                    "headers": {
                        k: v
                        for k, v in response.headers.items()
                        if k.lower()
                        in {"content-type", "date", "etag", "last-modified"}
                    },
                    "body": redact_text(body, self.environment.redaction_patterns),
                }
        except TimeoutError:
            return {"status": "timeout"}
        except (DebugRuntimeError, URLError) as exc:
            return {"status": "unavailable", "error": type(exc).__name__}
        except OSError as exc:
            return {"status": "unavailable", "error": type(exc).__name__}

    def recent_logs(
        self, service_id: str, source: str, severity: str | None = None
    ) -> dict[str, Any]:
        service = self._service(service_id)
        if source not in service.log_sources:
            raise DebugManifestError("log source is not approved")
        root = self.environment.project_root.resolve()
        path = (root / source).resolve()
        if root not in path.parents or not path.is_file() or path.is_symlink():
            raise DebugRuntimeError("approved log source unavailable")
        current = path
        while current != root:
            if current.is_symlink():
                raise DebugRuntimeError("approved log source unavailable")
            current = current.parent
        lines = path.read_text(errors="replace").splitlines()[
            -self.environment.limits["max_log_lines"] :
        ]
        if severity:
            lines = [line for line in lines if severity.lower() in line.lower()]
        body = (
            "\n".join(lines)
            .encode()[: self.environment.limits["max_bytes"]]
            .decode(errors="replace")
        )
        return {
            "status": "ok",
            "service": service_id,
            "source": source,
            "lines": redact_text(
                body, self.environment.redaction_patterns
            ).splitlines(),
        }

    def database_schema(self) -> dict[str, Any]:
        if not self.environment.database.get("enabled", False):
            raise DebugRuntimeError("database capability unavailable")
        return {
            "status": "ok",
            "schemas": list(self.environment.database.get("schemas", [])),
            "metadata_only": True,
        }

    def database_query(
        self, template: str, parameters: dict[str, Any] | None = None
    ) -> dict[str, Any]:
        templates = self.environment.database.get("templates", {})
        if not isinstance(templates, dict) or template not in templates:
            raise DebugManifestError("database query template is not approved")
        query = templates[template]
        if not isinstance(query, str) or any(
            word in query.upper()
            for word in ("INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", ";")
        ):
            raise DebugManifestError("database query template is not read-only")
        return {
            "status": "unavailable",
            "reason": "database adapter not configured",
            "template": template,
        }
