#!/usr/bin/env python3
"""Safe merge utilities for Claude Code settings and MCP server configs."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from _logging import log_warn

DEVKIT_PYTHON = ".leedevkit/.venv/bin/python3"
ROUTER_HOOK_COMMAND = f"{DEVKIT_PYTHON} .leedevkit/scripts/_model_router.py"
MCP_SERVER_NAME = "leedevkit-task-assessor"


def _read_json(path: Path) -> tuple[dict[str, Any] | None, bool]:
    """Return (data, is_valid_or_missing)."""
    if not path.exists():
        return {}, True
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        if isinstance(data, dict):
            return data, True
        return None, False
    except Exception:
        return None, False


def _write_json_atomic(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(f"{path.suffix}.tmp")
    text = json.dumps(data, indent=2, sort_keys=True) + "\n"
    temp.write_text(text, encoding="utf-8")
    temp.replace(path)


def merge_settings_json(root: Path) -> bool:
    """Add LeeDevKit PreToolUse router hook to .claude/settings.json without overwriting user data."""
    target = root / ".claude" / "settings.json"
    data, valid = _read_json(target)
    if not valid or data is None:
        log_warn(f"⚠️  {target} contains invalid JSON — skipping hook installation")
        return False

    hooks = data.setdefault("hooks", {})
    if not isinstance(hooks, dict):
        log_warn(
            f"⚠️  'hooks' in {target} is not a dictionary — skipping hook installation"
        )
        return False

    pre_tool_list = hooks.setdefault("PreToolUse", [])
    if not isinstance(pre_tool_list, list):
        log_warn(
            f"⚠️  'hooks.PreToolUse' in {target} is not a list — skipping hook installation"
        )
        return False

    for entry in pre_tool_list:
        if isinstance(entry, dict):
            inner_hooks = entry.get("hooks", [])
            if isinstance(inner_hooks, list):
                for hook in inner_hooks:
                    if (
                        isinstance(hook, dict)
                        and hook.get("command") == ROUTER_HOOK_COMMAND
                    ):
                        return False

    pre_tool_list.append(
        {
            "matcher": "Agent|Task",
            "hooks": [{"type": "command", "command": ROUTER_HOOK_COMMAND}],
        }
    )
    _write_json_atomic(target, data)
    return True


def merge_mcp_json(root: Path) -> bool:
    """Add leedevkit-task-assessor to .mcp.json without overwriting user servers."""
    target = root / ".mcp.json"
    data, valid = _read_json(target)
    if not valid or data is None:
        log_warn(f"⚠️  {target} contains invalid JSON — skipping MCP installation")
        return False

    servers = data.setdefault("mcpServers", {})
    if not isinstance(servers, dict):
        log_warn(
            f"⚠️  'mcpServers' in {target} is not a dictionary — skipping MCP installation"
        )
        return False

    if MCP_SERVER_NAME in servers:
        return False

    servers[MCP_SERVER_NAME] = {
        "command": DEVKIT_PYTHON,
        "args": [".leedevkit/scripts/_task_assessor_mcp.py"],
    }
    _write_json_atomic(target, data)
    return True


def install_ai_integrations(root: Path) -> tuple[bool, bool]:
    """Install integrations; runtime routing remains controlled by ``enabled``."""
    return merge_settings_json(root), merge_mcp_json(root)
