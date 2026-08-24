"""Tests for project-local Claude model routing and task assessment."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

from _claude_config import (
    DEVKIT_PYTHON,
    MCP_SERVER_NAME,
    ROUTER_HOOK_COMMAND,
    install_ai_integrations,
    merge_mcp_json,
    merge_settings_json,
)
from _model_router import assess_task, route_pretool_input
from _task_assessor_mcp import handle_request


ROOT = Path(__file__).resolve().parents[2]


def test_assessment_covers_tiers_and_is_deterministic():
    assert assess_task("grep where config is defined")["suggested_tier"] == "haiku"
    assert assess_task("implement a routine API endpoint")["suggested_tier"] == "sonnet"
    assert assess_task("debug the production race condition")["suggested_tier"] == "opus"
    assert assess_task("design an ambiguous multi-system migration")["suggested_tier"] == "fable"
    result = assess_task("implement a routine API endpoint")
    assert result == assess_task("implement a routine API endpoint")
    assert 0 <= result["confidence"] <= 1
    assert result["rationale"]


def test_routing_preserves_explicit_model_and_disabled_config():
    payload = {
        "tool_name": "Agent",
        "tool_input": {"description": "grep config", "model": "opus"},
    }
    assert route_pretool_input(payload, {"enabled": True}) is None
    assert route_pretool_input(
        {"tool_name": "Agent", "tool_input": {"description": "grep config"}},
        {"enabled": False},
    ) is None


def test_routing_adds_model_only_to_agent_or_task():
    payload = {
        "tool_name": "Task",
        "tool_input": {"description": "security audit for auth"},
    }
    updated = route_pretool_input(payload, {"enabled": True})
    assert updated["model"] == "opus"
    assert route_pretool_input(
        {"tool_name": "Bash", "tool_input": {"description": "security audit"}},
        {"enabled": True},
    ) is None


def test_hook_fails_open_on_bad_input():
    process = subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "_model_router.py")],
        input="not json\n",
        text=True,
        capture_output=True,
        check=False,
    )
    assert process.returncode == 0
    assert process.stdout == ""


def test_mcp_protocol_and_assess_task():
    initialized = handle_request({"jsonrpc": "2.0", "id": 1, "method": "initialize"})
    assert initialized["result"]["capabilities"]["tools"] == {}
    listed = handle_request({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
    assert listed["result"]["tools"][0]["name"] == "assess_task"
    called = handle_request(
        {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "tools/call",
            "params": {"name": "assess_task", "arguments": {"task": "grep files"}},
        }
    )
    content = called["result"]["content"][0]["text"]
    assert json.loads(content)["suggested_tier"] == "haiku"
    assert handle_request({"jsonrpc": "2.0", "id": 4, "method": "unknown"})["error"]["code"] == -32601


def test_settings_and_mcp_merges_are_safe_and_idempotent(tmp_path):
    settings = tmp_path / ".claude" / "settings.json"
    settings.parent.mkdir()
    settings.write_text(
        json.dumps({"permissions": {"allow": ["Bash(git status)"]}, "hooks": {"UserPromptSubmit": []}})
    )
    mcp = tmp_path / ".mcp.json"
    mcp.write_text(json.dumps({"mcpServers": {"user-server": {"command": "user"}}}))

    assert merge_settings_json(tmp_path)
    assert merge_mcp_json(tmp_path)
    first_settings = settings.read_text()
    first_mcp = mcp.read_text()
    assert not merge_settings_json(tmp_path)
    assert not merge_mcp_json(tmp_path)
    assert settings.read_text() == first_settings
    assert mcp.read_text() == first_mcp
    settings_data = json.loads(first_settings)
    assert settings_data["permissions"]["allow"] == ["Bash(git status)"]
    assert any(
        hook.get("command") == ROUTER_HOOK_COMMAND
        for entry in settings_data["hooks"]["PreToolUse"]
        for hook in entry["hooks"]
    )
    mcp_data = json.loads(first_mcp)
    assert "user-server" in mcp_data["mcpServers"]
    assert MCP_SERVER_NAME in mcp_data["mcpServers"]
    assert mcp_data["mcpServers"][MCP_SERVER_NAME]["command"] == DEVKIT_PYTHON


def test_mcp_name_collision_preserves_user_config(tmp_path):
    path = tmp_path / ".mcp.json"
    path.write_text(json.dumps({"mcpServers": {MCP_SERVER_NAME: {"command": "user"}}}))
    assert not merge_mcp_json(tmp_path)
    assert json.loads(path.read_text())["mcpServers"][MCP_SERVER_NAME]["command"] == "user"


def test_disabled_runtime_routing_still_installs_dormant_integrations(tmp_path):
    assert install_ai_integrations(tmp_path) == (True, True)
    assert (tmp_path / ".claude" / "settings.json").exists()
    assert (tmp_path / ".mcp.json").exists()
    assert route_pretool_input(
        {"tool_name": "Agent", "tool_input": {"description": "implement feature"}},
        {"enabled": False},
    ) is None
    assert route_pretool_input(
        {"tool_name": "Agent", "tool_input": {"description": "implement feature"}},
        {"enabled": True},
    )["model"] == "sonnet"
