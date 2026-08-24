#!/usr/bin/env python3
"""Minimal stdio JSON-RPC 2.0 MCP server for task complexity assessment."""

from __future__ import annotations

import json
import sys
from typing import Any

from _model_router import assess_task

TOOL_NAME = "assess_task"
TOOL_SCHEMA = {
    "name": TOOL_NAME,
    "description": "Assess task complexity and return a recommended Claude model tier (haiku, sonnet, opus, fable) with rationale.",
    "inputSchema": {
        "type": "object",
        "properties": {
            "task": {
                "type": "string",
                "description": "Task prompt, instruction, or problem description.",
            },
            "files": {
                "type": "integer",
                "description": "Estimated count of involved files (default 0).",
                "default": 0,
            },
            "risk": {
                "type": "string",
                "description": "Optional risk keywords (e.g. auth, migration, payment).",
                "default": "",
            },
        },
        "required": ["task"],
    },
}


def _response(msg_id: Any, result: Any) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": msg_id, "result": result}


def _error(msg_id: Any, code: int, message: str) -> dict[str, Any]:
    return {
        "jsonrpc": "2.0",
        "id": msg_id,
        "error": {"code": code, "message": message},
    }


def handle_request(req: dict[str, Any]) -> dict[str, Any] | None:
    if not isinstance(req, dict) or req.get("jsonrpc") != "2.0":
        return _error(None, -32600, "Invalid Request: expected JSON-RPC 2.0 object")

    method = req.get("method")
    msg_id = req.get("id")

    if method == "notifications/initialized":
        return None
    if msg_id is None:
        return None

    if method == "initialize":
        return _response(
            msg_id,
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {
                    "name": "leedevkit-task-assessor",
                    "version": "0.1.0",
                },
            },
        )

    if method == "tools/list":
        return _response(msg_id, {"tools": [TOOL_SCHEMA]})

    if method == "tools/call":
        params = req.get("params") or {}
        if params.get("name") != TOOL_NAME:
            return _error(msg_id, -32601, f"Unknown tool: {params.get('name')!r}")
        args = params.get("arguments") or {}
        task = args.get("task")
        if not isinstance(task, str) or not task.strip():
            return _error(
                msg_id,
                -32602,
                "Invalid params: 'task' is required and must be a non-empty string",
            )
        assessment = assess_task(
            text=task,
            files=args.get("files", 0),
            risk=str(args.get("risk", "")),
        )
        return _response(
            msg_id,
            {
                "content": [{"type": "text", "text": json.dumps(assessment, indent=2)}],
                "isError": False,
            },
        )

    return _error(msg_id, -32601, f"Method not found: {method!r}")


def main() -> int:
    for line in sys.stdin:
        text = line.strip()
        if not text:
            continue
        resp: dict[str, Any] | None
        try:
            req = json.loads(text)
        except json.JSONDecodeError:
            resp = _error(None, -32700, "Parse error")
        else:
            resp = handle_request(req)
        if resp is not None:
            sys.stdout.write(json.dumps(resp, separators=(",", ":")) + "\n")
            sys.stdout.flush()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
