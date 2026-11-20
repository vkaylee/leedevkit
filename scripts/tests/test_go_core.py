import json
import os
import sys
from unittest.mock import MagicMock, patch

sys.path.insert(0, "scripts")

from _go_core import (
    plan_gates_go,
    plan_lifecycle_go,
    plan_quality_gates_go,
    resolve_provider_go,
)


def test_disabled_uses_python_path():
    with patch.dict(os.environ, {}, clear=True):
        assert plan_gates_go({"schema_version": 1, "gates": []}) is None


def test_missing_binary_fails_closed():
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value=None):
            try:
                plan_gates_go({"schema_version": 1, "gates": []})
            except RuntimeError as exc:
                assert "binary missing" in str(exc)
            else:
                raise AssertionError("missing Go binary passed")


def test_missing_binary_with_invalid_runtime_fails_closed(tmp_path):
    with patch.dict(
        os.environ,
        {"LEEDEVKIT_USE_GO_CORE": "1", "DEVKIT_HOME": str(tmp_path)},
        clear=True,
    ):
        with patch("_go_core.shutil.which", return_value=None):
            try:
                plan_gates_go({"schema_version": 1, "gates": []})
            except RuntimeError as exc:
                assert "binary missing" in str(exc)
            else:
                raise AssertionError("invalid runtime passed")


def test_protocol_disabled_returns_none_for_all_planners():
    with patch.dict(os.environ, {}, clear=True):
        assert plan_quality_gates_go([]) is None
        assert plan_lifecycle_go({}) is None
        assert resolve_provider_go({}) is None


def test_protocol_timeout_fails_closed():
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch(
                "_go_core.subprocess.run",
                side_effect=__import__("subprocess").TimeoutExpired("go", 1),
            ):
                try:
                    plan_gates_go({"schema_version": 1, "gates": []})
                except RuntimeError as exc:
                    assert "timed out" in str(exc)
                else:
                    raise AssertionError("timeout passed")


def test_protocol_nonzero_exit_fails_closed():
    result = MagicMock(returncode=7, stdout="", stderr="failed")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    plan_gates_go({"schema_version": 1, "gates": []})
                except RuntimeError as exc:
                    assert "exit code 7" in str(exc)
                else:
                    raise AssertionError("failed protocol passed")


def test_protocol_malformed_response_fails_closed():
    result = MagicMock(returncode=0, stdout="not-json", stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    plan_gates_go({"schema_version": 1, "gates": []})
                except RuntimeError as exc:
                    assert "malformed" in str(exc)
                else:
                    raise AssertionError("malformed response passed")


def test_valid_response_is_returned():
    response = {"schema_version": 1, "gates": [{"name": "go-vet"}]}
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert plan_gates_go({"schema_version": 1, "gates": []}) == response
                assert run.call_args.args[0] == ["/tmp/leedevkit-core", "--plan-gates"]


def test_quality_gate_shadow_uses_normalized_protocol():
    response = {
        "schema_version": 1,
        "gates": [
            {"name": "go-vet", "phase": "lint", "command": ["go", "vet", "./..."]}
        ],
    }
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert (
                    plan_quality_gates_go(
                        [
                            {
                                "name": "go-vet",
                                "phase": "lint",
                                "command": ["go", "vet", "./..."],
                            }
                        ]
                    )
                    == response
                )
                assert run.call_args.args[0] == [
                    "/tmp/leedevkit-core",
                    "--plan-gates",
                ]


def test_provider_shadow_uses_versioned_protocol():
    response = {
        "schema_version": 1,
        "diagnostics": {"provider": "docker", "cli": ["docker"]},
    }
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert (
                    resolve_provider_go(
                        {
                            "provider": "docker",
                            "cli": ["docker"],
                            "compose": ["docker", "compose"],
                            "capabilities": [],
                            "settings": {},
                            "source": "test",
                        }
                    )
                    == response
                )
                assert run.call_args.args[0] == [
                    "/tmp/leedevkit-core",
                    "--resolve-provider",
                ]
