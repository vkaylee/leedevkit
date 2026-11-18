import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _devkit_config import (
    container_provider_diagnostics,
    require_container_capabilities,
    resolve_container_provider,
)

DOCKER = {"cli": ["docker"], "compose": ["docker", "compose"]}
KIRI = {
    "cli": ["/opt/kiri/bin/kiri"],
    "compose": ["/opt/kiri/bin/kiri", "compose"],
    "capabilities": ["compose", "config"],
    "endpoint": "unix:///run/kiri.sock",
}


def test_default_is_explicit_docker_without_tool_probe() -> None:
    provider = resolve_container_provider({}, environ={}, host_config={})
    assert provider.name == "docker"
    assert provider.cli == ("docker",)
    assert provider.compose == ("docker", "compose")


def test_precedence_cli_env_host_repo() -> None:
    config = {"container": {"provider": "repo", "providers": {"repo": DOCKER}}}
    host = {
        "container": {
            "provider": "host",
            "providers": {"host": {"cli": ["host"], "compose": ["host", "compose"]}},
        }
    }
    with pytest.raises(ValueError, match='provider "cli" is not defined'):
        resolve_container_provider(
            config, cli_provider="cli", environ={}, host_config=host
        )

    env_provider = resolve_container_provider(
        config,
        environ={"CONTAINER_PROVIDER": "host"},
        host_config=host,
    )
    assert env_provider.name == "host"

    repo_provider = resolve_container_provider(config, environ={}, host_config={})
    assert repo_provider.name == "repo"


def test_host_definition_overlays_repository_definition() -> None:
    config = {
        "container": {
            "provider": "kiri",
            "providers": {"kiri": {"cli": ["kiri"], "compose": ["kiri", "compose"]}},
        }
    }
    provider = resolve_container_provider(
        config, environ={}, host_config={"container": {"providers": {"kiri": KIRI}}}
    )
    assert provider.cli == ("/opt/kiri/bin/kiri",)
    assert provider.settings["endpoint"] == "unix:///run/kiri.sock"
    assert provider.capabilities == frozenset({"compose", "config"})


def test_custom_provider_requires_explicit_definition() -> None:
    with pytest.raises(ValueError, match='provider "custom" is not defined'):
        resolve_container_provider(
            {}, cli_provider="custom", environ={}, host_config={}
        )


def test_invalid_provider_command_is_rejected() -> None:
    config = {
        "container": {
            "provider": "bad",
            "providers": {"bad": {"cli": "docker", "compose": ["docker", "compose"]}},
        }
    }
    with pytest.raises(ValueError, match="cli must be a non-empty string list"):
        resolve_container_provider(config, environ={}, host_config={})


def test_unknown_capability_is_rejected() -> None:
    config = {
        "container": {
            "provider": "bad",
            "providers": {"bad": {**DOCKER, "capabilities": ["teleport"]}},
        }
    }
    with pytest.raises(ValueError, match="unknown names"):
        resolve_container_provider(config, environ={}, host_config={})


def test_required_capabilities_fail_closed() -> None:
    config = {
        "container": {
            "provider": "docker",
            "providers": {"docker": {**DOCKER, "capabilities": ["compose"]}},
        }
    }
    provider = resolve_container_provider(config, environ={}, host_config={})
    with pytest.raises(ValueError, match="lacks required capabilities: config"):
        require_container_capabilities(provider, {"compose", "config"})


def test_required_capabilities_accept_declared_set() -> None:
    config = {
        "container": {
            "provider": "docker",
            "providers": {
                "docker": {
                    **DOCKER,
                    "capabilities": ["compose", "config"],
                }
            },
        }
    }
    provider = resolve_container_provider(config, environ={}, host_config={})
    require_container_capabilities(provider, {"compose", "config"})


def test_diagnostics_masks_secret_settings_and_url_credentials() -> None:
    config = {
        "container": {
            "provider": "docker",
            "providers": {
                "docker": {
                    **DOCKER,
                    "password": "secret",
                    "endpoint": "https://user:pass@example.test/socket",
                }
            },
        }
    }
    diagnostics = container_provider_diagnostics(
        resolve_container_provider(config, environ={}, host_config={})
    )
    assert diagnostics["settings"]["password"] == "<redacted>"
    assert "user" not in diagnostics["settings"]["endpoint"]
    assert "pass" not in diagnostics["settings"]["endpoint"]


def test_cli_override_beats_environment() -> None:
    config = {
        "container": {
            "provider": "docker",
            "providers": {"docker": DOCKER, "kiri": KIRI},
        }
    }
    provider = resolve_container_provider(
        config,
        cli_provider="kiri",
        environ={"CONTAINER_PROVIDER": "docker"},
        host_config={},
    )
    assert provider.name == "kiri"
    assert provider.source == "CLI override"
