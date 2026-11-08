"""Tests for project-local Claude model routing and task assessment."""

from __future__ import annotations

import json
import shlex
import subprocess
import sys
from pathlib import Path

from _claude_config import (
    LEGACY_MCP_ARGS,
    LEGACY_MCP_COMMAND,
    LEGACY_ROUTER_HOOK_COMMAND,
    MCP_SERVER_NAME,
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
    assert (
        assess_task("debug the production race condition")["suggested_tier"] == "opus"
    )
    assert (
        assess_task("design an ambiguous multi-system migration")["suggested_tier"]
        == "fable"
    )
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
    assert (
        route_pretool_input(
            {"tool_name": "Agent", "tool_input": {"description": "grep config"}},
            {"enabled": False},
        )
        is None
    )


def test_routing_adds_model_only_to_agent_or_task():
    payload = {
        "tool_name": "Task",
        "tool_input": {"description": "security audit for auth"},
    }
    updated = route_pretool_input(payload, {"enabled": True})
    assert updated["model"] == "opus"
    assert (
        route_pretool_input(
            {"tool_name": "Bash", "tool_input": {"description": "security audit"}},
            {"enabled": True},
        )
        is None
    )


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
    assert (
        handle_request({"jsonrpc": "2.0", "id": 4, "method": "unknown"})["error"][
            "code"
        ]
        == -32601
    )


def test_settings_and_mcp_merges_are_safe_and_idempotent(tmp_path):
    root = tmp_path / "project"
    devkit = root / ".leedevkit"
    settings = root / ".claude" / "settings.json"
    settings.parent.mkdir(parents=True)
    settings.write_text(
        json.dumps(
            {
                "permissions": {"allow": ["Bash(git status)"]},
                "hooks": {"UserPromptSubmit": []},
            }
        )
    )
    mcp = root / ".mcp.json"
    mcp.write_text(json.dumps({"mcpServers": {"user-server": {"command": "user"}}}))

    assert merge_settings_json(root, devkit)
    assert merge_mcp_json(root, devkit)
    first_settings = settings.read_text()
    first_mcp = mcp.read_text()
    assert not merge_settings_json(root, devkit)
    assert not merge_mcp_json(root, devkit)
    assert settings.read_text() == first_settings
    assert mcp.read_text() == first_mcp
    settings_data = json.loads(first_settings)
    assert settings_data["permissions"]["allow"] == ["Bash(git status)"]
    hook = settings_data["hooks"]["PreToolUse"][0]["hooks"][0]
    assert shlex.split(hook["command"]) == [
        ".leedevkit/.venv/bin/python3",
        ".leedevkit/scripts/_model_router.py",
    ]
    mcp_data = json.loads(first_mcp)
    assert "user-server" in mcp_data["mcpServers"]
    assert mcp_data["mcpServers"][MCP_SERVER_NAME] == {
        "command": ".leedevkit/.venv/bin/python3",
        "args": [".leedevkit/scripts/_task_assessor_mcp.py"],
    }


def test_source_runtime_commands_resolve_without_nested_install(tmp_path):
    root = tmp_path / "source checkout"
    (root / "scripts").mkdir(parents=True)
    (root / "scripts" / "_model_router.py").touch()
    (root / "scripts" / "_task_assessor_mcp.py").touch()
    (root / ".venv" / "bin" / "python3").parent.mkdir(parents=True)
    (root / ".venv" / "bin" / "python3").touch()

    assert install_ai_integrations(root, root) == (True, True)
    settings = json.loads((root / ".claude" / "settings.json").read_text())
    command = settings["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
    interpreter, router = shlex.split(command)
    assert (root / interpreter).resolve() == (root / ".venv/bin/python3").resolve()
    assert (root / router).resolve() == (root / "scripts/_model_router.py").resolve()
    server = json.loads((root / ".mcp.json").read_text())["mcpServers"][MCP_SERVER_NAME]
    assert server == {
        "command": ".venv/bin/python3",
        "args": ["scripts/_task_assessor_mcp.py"],
    }
    assert not (root / ".leedevkit").exists()


def test_installed_generated_entries_migrate_in_place(tmp_path):
    root = tmp_path / "project"
    devkit = root
    settings_path = root / ".claude" / "settings.json"
    settings_path.parent.mkdir(parents=True)
    settings_path.write_text(
        json.dumps(
            {
                "hooks": {
                    "PreToolUse": [
                        {
                            "matcher": "Agent|Task",
                            "hooks": [
                                {
                                    "type": "command",
                                    "command": LEGACY_ROUTER_HOOK_COMMAND,
                                }
                            ],
                        },
                        {
                            "matcher": "Bash",
                            "hooks": [{"type": "command", "command": "user command"}],
                        },
                        {
                            "matcher": "Custom",
                            "hooks": [
                                {
                                    "type": "command",
                                    "command": LEGACY_ROUTER_HOOK_COMMAND,
                                }
                            ],
                        },
                    ],
                    "UserPromptSubmit": [{"hooks": [{"command": "preserve"}]}],
                },
                "permissions": {"allow": ["preserve"]},
            }
        )
    )
    mcp_path = root / ".mcp.json"
    mcp_path.write_text(
        json.dumps(
            {
                "mcpServers": {
                    "user-server": {"command": "preserve"},
                    MCP_SERVER_NAME: {
                        "command": LEGACY_MCP_COMMAND,
                        "args": LEGACY_MCP_ARGS,
                        "env": {"KEEP": "yes"},
                        "custom": True,
                    },
                }
            }
        )
    )

    assert merge_settings_json(root, devkit)
    assert merge_mcp_json(root, devkit)
    settings = json.loads(settings_path.read_text())
    assert len(settings["hooks"]["PreToolUse"]) == 3
    assert settings["hooks"]["PreToolUse"][0]["hooks"][0]["command"] == (
        ".venv/bin/python3 scripts/_model_router.py"
    )
    assert settings["hooks"]["PreToolUse"][1]["hooks"][0]["command"] == "user command"
    assert settings["hooks"]["PreToolUse"][2]["hooks"][0]["command"] == (
        LEGACY_ROUTER_HOOK_COMMAND
    )
    assert settings["hooks"]["UserPromptSubmit"][0]["hooks"][0]["command"] == "preserve"
    assert settings["permissions"]["allow"] == ["preserve"]
    mcp = json.loads(mcp_path.read_text())
    assert mcp["mcpServers"]["user-server"]["command"] == "preserve"
    managed = mcp["mcpServers"][MCP_SERVER_NAME]
    assert managed["command"] == ".venv/bin/python3"
    assert managed["args"] == ["scripts/_task_assessor_mcp.py"]
    assert managed["env"] == {"KEEP": "yes"}
    assert managed["custom"] is True
    settings_after = settings_path.read_text()
    mcp_after = mcp_path.read_text()
    assert not merge_settings_json(root, devkit)
    assert not merge_mcp_json(root, devkit)
    assert settings_path.read_text() == settings_after
    assert mcp_path.read_text() == mcp_after


def test_mcp_name_collision_preserves_user_config(tmp_path):
    root = tmp_path / "project"
    root.mkdir()
    path = root / ".mcp.json"
    path.write_text(json.dumps({"mcpServers": {MCP_SERVER_NAME: {"command": "user"}}}))
    assert not merge_mcp_json(root, root / ".leedevkit")
    assert (
        json.loads(path.read_text())["mcpServers"][MCP_SERVER_NAME]["command"] == "user"
    )


def test_external_runtime_shell_hook_quotes_paths_with_spaces(tmp_path):
    root = tmp_path / "consumer project"
    devkit = tmp_path / "external devkit"

    assert install_ai_integrations(root, devkit) == (True, True)
    settings = json.loads((root / ".claude" / "settings.json").read_text())
    command = settings["hooks"]["PreToolUse"][0]["hooks"][0]["command"]
    assert shlex.split(command) == [
        str(devkit / ".venv" / "bin" / "python3"),
        str(devkit / "scripts" / "_model_router.py"),
    ]
    assert "'" in command
    server = json.loads((root / ".mcp.json").read_text())["mcpServers"][MCP_SERVER_NAME]
    assert server == {
        "command": str(devkit / ".venv" / "bin" / "python3"),
        "args": [str(devkit / "scripts" / "_task_assessor_mcp.py")],
    }


def test_disabled_runtime_routing_still_installs_dormant_integrations(tmp_path):
    assert install_ai_integrations(tmp_path, tmp_path / ".leedevkit") == (True, True)
    assert (tmp_path / ".claude" / "settings.json").exists()
    assert (tmp_path / ".mcp.json").exists()
    assert (
        route_pretool_input(
            {"tool_name": "Agent", "tool_input": {"description": "implement feature"}},
            {"enabled": False},
        )
        is None
    )
    assert (
        route_pretool_input(
            {"tool_name": "Agent", "tool_input": {"description": "implement feature"}},
            {"enabled": True},
        )["model"]
        == "sonnet"
    )
