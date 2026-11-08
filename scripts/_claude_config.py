#!/usr/bin/env python3
"""Safe merge utilities for Claude Code settings and MCP server configs."""

from __future__ import annotations

import json
import shlex
from pathlib import Path
from typing import Any

from _logging import log_warn

MCP_SERVER_NAME = "leedevkit-task-assessor"
LEGACY_ROUTER_HOOK_COMMAND = (
    ".leedevkit/.venv/bin/python3 .leedevkit/scripts/_model_router.py"
)
LEGACY_MCP_COMMAND = ".leedevkit/.venv/bin/python3"
LEGACY_MCP_ARGS = [".leedevkit/scripts/_task_assessor_mcp.py"]


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


def _runtime_paths(root: Path, devkit: Path) -> tuple[str, str, str]:
    """Return interpreter, router, and MCP script paths for this runtime."""
    root, devkit = root.resolve(), devkit.resolve()
    if devkit == root:
        prefix = Path()
    elif devkit == root / ".leedevkit":
        prefix = Path(".leedevkit")
    else:
        return (
            str(devkit / ".venv" / "bin" / "python3"),
            str(devkit / "scripts" / "_model_router.py"),
            str(devkit / "scripts" / "_task_assessor_mcp.py"),
        )
    return (
        (prefix / ".venv" / "bin" / "python3").as_posix(),
        (prefix / "scripts" / "_model_router.py").as_posix(),
        (prefix / "scripts" / "_task_assessor_mcp.py").as_posix(),
    )


def merge_settings_json(root: Path, devkit: Path) -> bool:
    """Install or migrate the LeeDevKit router hook without overwriting user data."""
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

    python, router, _ = _runtime_paths(root, devkit)
    command = shlex.join([python, router])
    changed = False
    current_found = False
    for entry in pre_tool_list:
        if entry == {
            "matcher": "Agent|Task",
            "hooks": [{"type": "command", "command": command}],
        }:
            current_found = True
        elif entry == {
            "matcher": "Agent|Task",
            "hooks": [{"type": "command", "command": LEGACY_ROUTER_HOOK_COMMAND}],
        }:
            entry["hooks"][0]["command"] = command
            changed = True

    if changed:
        _write_json_atomic(target, data)
        return True
    if current_found:
        return False

    pre_tool_list.append(
        {
            "matcher": "Agent|Task",
            "hooks": [{"type": "command", "command": command}],
        }
    )
    _write_json_atomic(target, data)
    return True


def merge_mcp_json(root: Path, devkit: Path) -> bool:
    """Install or migrate the managed task assessor, preserving user servers."""
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

    python, _, assessor = _runtime_paths(root, devkit)
    generated = {"command": python, "args": [assessor]}
    existing = servers.get(MCP_SERVER_NAME)
    if MCP_SERVER_NAME in servers:
        if not isinstance(existing, dict):
            return False
        command, args = existing.get("command"), existing.get("args")
        is_managed = (command, args) in (
            (LEGACY_MCP_COMMAND, LEGACY_MCP_ARGS),
            (generated["command"], generated["args"]),
        )
        if not is_managed:
            return False
        if command == generated["command"] and args == generated["args"]:
            return False
        existing.update(generated)
        _write_json_atomic(target, data)
        return True

    servers[MCP_SERVER_NAME] = generated
    _write_json_atomic(target, data)
    return True


def install_ai_integrations(root: Path, devkit: Path) -> tuple[bool, bool]:
    """Install integrations; runtime routing remains controlled by ``enabled``."""
    return merge_settings_json(root, devkit), merge_mcp_json(root, devkit)
