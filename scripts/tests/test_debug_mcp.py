from __future__ import annotations

import json
from pathlib import Path

import pytest

from _debug_launcher import DebugLaunchError, _network_names
from _debug_manifest import DebugManifestError, load_debug_environment
from _debug_mcp import DebugServer
from _debug_runtime import (
    DebugRuntime,
    DebugRuntimeError,
    redact,
    redact_text,
    validate_redirect,
)


FIXTURE = Path(__file__).parent / "fixtures" / "debug"


def environment():
    return load_debug_environment(
        FIXTURE / "leedevkit.toml", "fixture", project_root=FIXTURE
    )


def test_compose_network_allowlist_fails_closed():
    env = environment()
    assert _network_names(
        env, {"networks": {"fixture_backend": {}, "fixture_frontend": {}}}
    ) == [
        "fixture-project_fixture_backend",
        "fixture-project_fixture_frontend",
    ]
    with pytest.raises(DebugLaunchError):
        _network_names(env, {"networks": {"fixture_backend": {}}})


def test_fixture_is_generic_and_metadata_is_safe():
    metadata = environment().metadata()
    assert metadata["environment"] == "fixture"
    assert metadata["compose_project"] == "fixture-project"
    assert metadata["mode"] == "read-only"
    assert set(metadata["services"]) == {"fixture-api", "fixture-web"}
    assert "fixture-db" not in metadata["services"]
    assert "password" not in json.dumps(metadata).lower()


@pytest.mark.parametrize(
    ("environment_name", "message"),
    [("missing", "unknown debug environment"), ("production", "production")],
)
def test_environment_selection_fails_closed(environment_name, message):
    with pytest.raises(DebugManifestError, match=message):
        load_debug_environment(
            FIXTURE / "leedevkit.toml", environment_name, project_root=FIXTURE
        )


def test_project_mismatch_and_path_escape_rejected(tmp_path):
    manifest = tmp_path / "manifest.toml"
    manifest.write_text(
        """[debug]\nmanifest_version=1\n[debug.environments.test]\ncompose_project='safe-project'\ncompose_files=['../compose.yml']\napproved_networks=['backend']\n[debug.environments.test.services.api]\ninternal_host='api'\ninternal_port=80\n"""
    )
    with pytest.raises(DebugManifestError, match="must stay inside project root"):
        load_debug_environment(manifest, "test", project_root=tmp_path)

    manifest.write_text(manifest.read_text().replace("../compose.yml", "compose.yml"))
    (tmp_path / "compose.yml").write_text("services: {}\n")
    with pytest.raises(DebugManifestError, match="does not match"):
        load_debug_environment(
            manifest, "test", project_root=tmp_path, compose_project="other"
        )


def test_unsafe_hosts_and_limits_rejected(tmp_path):
    manifest = tmp_path / "manifest.toml"
    (tmp_path / "compose.yml").write_text("services: {}\n")
    manifest.write_text(
        """[debug]\nmanifest_version=1\n[debug.environments.test]\ncompose_project='safe-project'\ncompose_files=['compose.yml']\napproved_networks=['backend']\nrequest_timeout_ms=0\n[debug.environments.test.services.api]\ninternal_host='localhost'\ninternal_port=80\n"""
    )
    with pytest.raises(DebugManifestError):
        load_debug_environment(manifest, "test", project_root=tmp_path)


def test_absolute_log_source_rejected(tmp_path):
    manifest = tmp_path / "manifest.toml"
    (tmp_path / "compose.yml").write_text("services: {}\n")
    manifest.write_text(
        """[debug]\nmanifest_version=1\n[debug.environments.test]\ncompose_project='safe-project'\ncompose_files=['compose.yml']\napproved_networks=['backend']\n[debug.environments.test.services.api]\ninternal_host='api'\ninternal_port=80\nlog_sources=['/etc/passwd']\n"""
    )
    with pytest.raises(DebugManifestError, match="relative path"):
        load_debug_environment(manifest, "test", project_root=tmp_path)


def test_runtime_allows_only_manifest_paths_and_redacts():
    env = environment()
    runtime = DebugRuntime(env)
    with pytest.raises(DebugManifestError):
        runtime.service_health("fixture-api", "/not-approved")
    with pytest.raises(DebugManifestError):
        runtime.http_get("fixture-api", "/not-approved")
    assert redact({"Authorization": "secret", "safe": "ok"}) == {
        "Authorization": "[REDACTED]",
        "safe": "ok",
    }
    assert "sk-test-123" not in redact_text('{"api_key":"sk-test-123"}')
    assert "sk-live" not in redact_text("Authorization: Bearer sk-live")
    assert "password=hunter2" not in redact_text("password=hunter2")
    validate_redirect(
        "http://fixture-api:8080/health",
        "http://fixture-api:8080/status",
        env.services["fixture-api"],
    )
    with pytest.raises(DebugRuntimeError):
        validate_redirect(
            "http://fixture-api:8080/health",
            "http://external:8080/status",
            env.services["fixture-api"],
        )
    with pytest.raises(DebugRuntimeError):
        validate_redirect(
            "http://fixture-api:8080/health",
            "http://169.254.169.254/latest/meta-data",
            env.services["fixture-api"],
        )


def test_mcp_contract_and_errors():
    server = DebugServer(environment())
    initialized = server.handle({"jsonrpc": "2.0", "id": 1, "method": "initialize"})
    assert initialized["result"]["capabilities"] == {
        "tools": {},
        "resources": {},
        "prompts": {},
    }
    listed = server.handle({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
    assert {tool["name"] for tool in listed["result"]["tools"]} == {
        "debug_environment",
        "service_health",
        "http_get",
        "recent_logs",
        "database_schema",
        "database_query",
    }
    assert server.handle(
        {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "resources/read",
            "params": {"uri": "debug://runbook"},
        }
    )["result"]["contents"]
    assert server.handle(
        {
            "jsonrpc": "2.0",
            "id": 4,
            "method": "prompts/get",
            "params": {"name": "debug_incident", "arguments": {"symptom": "slow"}},
        }
    )["result"]["messages"]
    assert (
        server.handle(
            {
                "jsonrpc": "2.0",
                "id": 5,
                "method": "tools/call",
                "params": {"name": "unknown", "arguments": {}},
            }
        )["error"]["code"]
        == -32601
    )
    assert (
        server.handle(
            {
                "jsonrpc": "2.0",
                "id": 6,
                "method": "resources/read",
                "params": {"uri": "debug://../secret"},
            }
        )["error"]["code"]
        == -32601
    )
    assert (
        server.handle({"jsonrpc": "2.0", "method": "notifications/initialized"}) is None
    )


def test_protocol_stays_json_lines(capsys):
    server = DebugServer(environment())
    response = server.handle({"jsonrpc": "2.0", "id": 1, "method": "ping"})
    print(json.dumps(response, separators=(",", ":")))
    output = capsys.readouterr().out
    assert json.loads(output)["result"] == {}
