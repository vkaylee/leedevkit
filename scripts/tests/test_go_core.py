import json
import os
import sys
from unittest.mock import MagicMock, patch

sys.path.insert(0, "scripts")

from _go_core import (
    plan_gates_go,
    plan_lifecycle_go,
    plan_quality_gates_go,
    resolve_modes_go,
    resolve_provider_go,
    resolve_test_targets_go,
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
        assert resolve_test_targets_go({}) is None
        assert resolve_modes_go({}) is None


def test_mode_shadow_uses_versioned_protocol():
    response = {"schema_version": 1, "modes": {"backend": "go"}}
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert resolve_modes_go({"services": []}) == response
                assert run.call_args.args[0] == [
                    "/tmp/leedevkit-core",
                    "--resolve-modes",
                ]


def test_mode_protocol_rejects_invalid_modes():
    result = MagicMock(
        returncode=0,
        stdout=json.dumps({"schema_version": 1, "modes": []}),
        stderr="",
    )
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    resolve_modes_go({"services": []})
                except RuntimeError as exc:
                    assert "invalid mode map" in str(exc)
                else:
                    raise AssertionError("invalid mode response passed")


def test_test_target_shadow_uses_versioned_protocol():
    response = {"schema_version": 1, "targets": ["go", "standalone"]}
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert resolve_test_targets_go({"target": "all"}) == response
                assert run.call_args.args[0] == [
                    "/tmp/leedevkit-core",
                    "--resolve-test-targets",
                ]


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


def test_protocol_schema_mismatch_fails_closed():
    result = MagicMock(
        returncode=0,
        stdout=json.dumps({"schema_version": 2}),
        stderr="",
    )
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    plan_gates_go({"schema_version": 1, "gates": []})
                except RuntimeError as exc:
                    assert "unsupported schema" in str(exc)
                else:
                    raise AssertionError("schema mismatch passed")


def test_target_protocol_rejects_invalid_targets():
    result = MagicMock(
        returncode=0,
        stdout=json.dumps({"schema_version": 1, "targets": "go"}),
        stderr="",
    )
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    resolve_test_targets_go({"target": "all"})
                except RuntimeError as exc:
                    assert "invalid test target" in str(exc)
                else:
                    raise AssertionError("invalid target response passed")


def test_gate_protocol_rejects_invalid_gates():
    result = MagicMock(
        returncode=0,
        stdout=json.dumps({"schema_version": 1, "gates": "go"}),
        stderr="",
    )
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result):
                try:
                    plan_gates_go({"schema_version": 1, "gates": []})
                except RuntimeError as exc:
                    assert "invalid gate plan" in str(exc)
                else:
                    raise AssertionError("invalid gate response passed")


def test_lifecycle_protocol_returns_response():
    response = {"schema_version": 1, "operation": "up", "args": ["compose", "up"]}
    result = MagicMock(returncode=0, stdout=json.dumps(response), stderr="")
    with patch.dict(os.environ, {"LEEDEVKIT_USE_GO_CORE": "1"}, clear=True):
        with patch("_go_core.shutil.which", return_value="/tmp/leedevkit-core"):
            with patch("_go_core.subprocess.run", return_value=result) as run:
                assert plan_lifecycle_go({"provider": {}}, operation="up") == response
                assert run.call_args.args[0] == [
                    "/tmp/leedevkit-core",
                    "--plan-lifecycle",
                ]


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
