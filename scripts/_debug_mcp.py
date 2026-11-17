"""Generic read-only Debug MCP stdio transport."""

from __future__ import annotations

import argparse
import json
import signal
import sys
from pathlib import Path
from typing import Any

from _debug_manifest import DebugEnvironment, DebugManifestError, load_debug_environment
from _debug_runtime import DebugRuntime, DebugRuntimeError, redact

PROTOCOL_VERSION = "2024-11-05"
MAX_REQUEST_BYTES = 256 * 1024
MAX_STRING = 4096

TOOLS = [
    {
        "name": "debug_environment",
        "description": "Return safe selected read-only environment metadata.",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "service_health",
        "description": "Check an approved internal service health path.",
        "inputSchema": {
            "type": "object",
            "properties": {"service": {"type": "string"}, "path": {"type": "string"}},
            "required": ["service", "path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "http_get",
        "description": "GET an approved internal service path only.",
        "inputSchema": {
            "type": "object",
            "properties": {"service": {"type": "string"}, "path": {"type": "string"}},
            "required": ["service", "path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "recent_logs",
        "description": "Read bounded approved logs with redaction.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "service": {"type": "string"},
                "source": {"type": "string"},
                "severity": {"type": "string"},
            },
            "required": ["service", "source"],
            "additionalProperties": False,
        },
    },
    {
        "name": "database_schema",
        "description": "Return approved database metadata only.",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "database_query",
        "description": "Run an approved read-only query template.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "template": {"type": "string"},
                "parameters": {"type": "object"},
            },
            "required": ["template"],
            "additionalProperties": False,
        },
    },
]

RESOURCES = [
    {
        "uri": "debug://runbook",
        "name": "runbook",
        "description": "Generic evidence-first debugging runbook.",
        "mimeType": "text/plain",
    },
    {
        "uri": "debug://safety",
        "name": "safety",
        "description": "Read-only safety restrictions.",
        "mimeType": "text/plain",
    },
    {
        "uri": "debug://environment",
        "name": "environment",
        "description": "Selected safe environment metadata.",
        "mimeType": "application/json",
    },
    {
        "uri": "debug://capabilities",
        "name": "capabilities",
        "description": "Approved capability metadata.",
        "mimeType": "application/json",
    },
    {
        "uri": "debug://evidence",
        "name": "evidence",
        "description": "Evidence source guidance.",
        "mimeType": "text/plain",
    },
]

PROMPTS = [
    {
        "name": "debug_incident",
        "description": "Evidence-first generic incident debugging workflow.",
        "arguments": [
            {
                "name": "symptom",
                "description": "Bounded symptom description.",
                "required": True,
            }
        ],
    }
]

RUNBOOK = """1. Inspect debug_environment.\n2. Check service_health.\n3. Inspect recent_logs.\n4. Use http_get only for approved paths.\n5. Use database evidence only when needed.\n6. Separate observations, hypotheses, and conclusions.\n7. Cite timestamps, request IDs, tool results, or equivalent evidence.\n8. Mask PII.\n9. Never mutate data.\n10. Never inspect credentials.\n11. Never claim root cause without evidence."""
SAFETY = "Read-only only. No code changes, writes, migrations, shell, arbitrary URLs, credentials, cookies, tokens, authorization bypass, cross-environment access, or conclusions without evidence."


def _response(msg_id: Any, result: Any) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": msg_id, "result": result}


def _error(msg_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": msg_id, "error": {"code": code, "message": message}}


def _content(value: Any) -> dict[str, Any]:
    return {
        "content": [
            {"type": "text", "text": json.dumps(redact(value), separators=(",", ":"))}
        ],
        "isError": False,
    }


def _required_string(args: dict[str, Any], name: str) -> str:
    value = args.get(name)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required and must be a non-empty string")
    return value


def _optional_string(args: dict[str, Any], name: str) -> str | None:
    value = args.get(name)
    if value is not None and not isinstance(value, str):
        raise ValueError(f"{name} must be a string")
    return value


def _args(params: Any) -> dict[str, Any]:
    if not isinstance(params, dict):
        raise ValueError("params must be an object")
    args = params.get("arguments", {})
    if not isinstance(args, dict):
        raise ValueError("arguments must be an object")
    if any(not isinstance(k, str) or len(k) > 128 for k in args):
        raise ValueError("invalid argument keys")
    for value in args.values():
        if isinstance(value, str) and len(value) > MAX_STRING:
            raise ValueError("argument string exceeds limit")
    return args


class DebugServer:
    def __init__(self, environment: DebugEnvironment) -> None:
        self.runtime = DebugRuntime(environment)

    def handle(self, req: dict[str, Any]) -> dict[str, Any] | None:
        if not isinstance(req, dict) or req.get("jsonrpc") != "2.0":
            return _error(None, -32600, "Invalid Request")
        method, msg_id = req.get("method"), req.get("id")
        if not isinstance(method, str):
            return _error(msg_id, -32600, "Invalid Request")
        if msg_id is None:
            if method in {"notifications/initialized", "notifications/cancelled"}:
                return None
            return None
        try:
            if method == "initialize":
                return _response(
                    msg_id,
                    {
                        "protocolVersion": PROTOCOL_VERSION,
                        "capabilities": {"tools": {}, "resources": {}, "prompts": {}},
                        "serverInfo": {"name": "leedevkit-debug", "version": "1.0"},
                    },
                )
            if method == "ping":
                return _response(msg_id, {})
            if method == "tools/list":
                return _response(msg_id, {"tools": TOOLS})
            if method == "resources/list":
                return _response(msg_id, {"resources": RESOURCES})
            if method == "prompts/list":
                return _response(msg_id, {"prompts": PROMPTS})
            if method == "tools/call":
                params = req.get("params")
                if not isinstance(params, dict) or params.get("name") not in {
                    tool["name"] for tool in TOOLS
                }:
                    return _error(msg_id, -32601, "Unknown tool")
                args = _args(params)
                name = params["name"]
                if name == "debug_environment":
                    value = self.runtime.environment_metadata()
                elif name == "service_health":
                    value = self.runtime.service_health(
                        _required_string(args, "service"),
                        _required_string(args, "path"),
                    )
                elif name == "http_get":
                    value = self.runtime.http_get(
                        _required_string(args, "service"),
                        _required_string(args, "path"),
                    )
                elif name == "recent_logs":
                    value = self.runtime.recent_logs(
                        _required_string(args, "service"),
                        _required_string(args, "source"),
                        _optional_string(args, "severity"),
                    )
                elif name == "database_schema":
                    value = self.runtime.database_schema()
                else:
                    value = self.runtime.database_query(
                        _required_string(args, "template"), args.get("parameters")
                    )
                return _response(msg_id, _content(value))
            if method == "resources/read":
                uri = (
                    req.get("params", {}).get("uri")
                    if isinstance(req.get("params"), dict)
                    else None
                )
                if uri not in {resource["uri"] for resource in RESOURCES}:
                    return _error(msg_id, -32601, "Unknown resource")
                resource_value: str | dict[str, Any]
                if uri == "debug://runbook":
                    resource_value = RUNBOOK
                elif uri == "debug://safety":
                    resource_value = SAFETY
                elif uri == "debug://environment":
                    resource_value = self.runtime.environment_metadata()
                elif uri == "debug://capabilities":
                    resource_value = self.runtime.environment_metadata()["capabilities"]
                else:
                    resource_value = "Evidence sources are bounded tool results with timestamps and request IDs where available."
                return _response(
                    msg_id,
                    {
                        "contents": [
                            {
                                "uri": uri,
                                "mimeType": "text/plain",
                                "text": resource_value
                                if isinstance(resource_value, str)
                                else json.dumps(resource_value),
                            }
                        ]
                    },
                )
            if method == "prompts/get":
                params = req.get("params", {})
                if (
                    not isinstance(params, dict)
                    or params.get("name") != "debug_incident"
                ):
                    return _error(msg_id, -32601, "Unknown prompt")
                symptom = (params.get("arguments") or {}).get("symptom")
                if (
                    not isinstance(symptom, str)
                    or not symptom.strip()
                    or len(symptom) > MAX_STRING
                ):
                    return _error(msg_id, -32602, "symptom is required and bounded")
                text = f"Symptom: {symptom}\n\nWorkflow: environment -> service health -> logs -> approved HTTP evidence -> approved database evidence -> root-cause analysis -> verification plan.\n{SAFETY}"
                return _response(
                    msg_id,
                    {
                        "description": "Generic incident debugging",
                        "messages": [
                            {"role": "user", "content": {"type": "text", "text": text}}
                        ],
                    },
                )
            return _error(msg_id, -32601, "Method not found")
        except (DebugManifestError, DebugRuntimeError, ValueError) as exc:
            return _error(msg_id, -32602, str(exc))
        except Exception:
            return _error(msg_id, -32603, "Debug operation failed")


def main(
    manifest: str | None = None,
    environment: str | None = None,
    compose_project: str | None = None,
    project_root: str | None = None,
) -> int:
    if not environment:
        print("debug MCP requires --environment", file=sys.stderr)
        return 2
    try:
        server = DebugServer(
            load_debug_environment(
                Path(manifest) if manifest else None,
                environment,
                project_root=Path(project_root) if project_root else None,
                compose_project=compose_project,
            )
        )
    except DebugManifestError as exc:
        print(str(exc), file=sys.stderr)
        return 2
    stopping = False

    def stop(_signum: int, _frame: Any) -> None:
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, stop)
    for line in sys.stdin:
        if stopping:
            break
        if len(line.encode()) > MAX_REQUEST_BYTES:
            sys.stdout.write(
                json.dumps(_error(None, -32600, "request exceeds size limit")) + "\n"
            )
            sys.stdout.flush()
            continue
        if not line.strip():
            continue
        try:
            req = json.loads(line)
            response = server.handle(req)
        except json.JSONDecodeError:
            response = _error(None, -32700, "Parse error")
        if response is not None:
            sys.stdout.write(json.dumps(response, separators=(",", ":")) + "\n")
            sys.stdout.flush()
    return 0


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Run read-only Debug MCP")
    parser.add_argument("--environment", required=True)
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--project-root", default=None)
    parser.add_argument("--compose-project", default=None)
    options = parser.parse_args()
    raise SystemExit(
        main(
            options.manifest,
            options.environment,
            options.compose_project,
            options.project_root,
        )
    )
