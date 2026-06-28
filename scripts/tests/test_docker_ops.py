"""Tests for _docker_ops.py — compose command building, dc(), dc_detached()."""

import os
import subprocess
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))

from _docker_ops import (  # noqa: E402
    _build_compose_cmd,
    dc,
    dc_detached,
)


class TestBuildComposeCmd:
    def test_basic_command(self) -> None:
        with patch.dict(os.environ, {}, clear=True):
            cmd = _build_compose_cmd(["up", "-d"])
            cmd_str = " ".join(cmd)
            assert cmd[0] in ("podman-compose", "docker")
            assert "-p" in cmd
            assert any("test" in c for c in cmd)  # project name contains 'test'
            assert "-f" in cmd
            assert "docker-compose.test.yml" in cmd_str
            assert "up" in cmd
            assert "-d" in cmd

    def test_custom_project_name(self) -> None:
        cmd = _build_compose_cmd(["ps"], project_name="my-project")
        assert "my-project" in cmd

    def test_custom_compose_file(self) -> None:
        custom_file = Path("/tmp/test.yml")
        cmd = _build_compose_cmd(["down"], compose_file=custom_file)
        assert "/tmp/test.yml" in cmd


class TestDc:
    def test_strips_stdin(self) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0)
            dc("up", "-d")
            call_kwargs = mock_run.call_args.kwargs
            assert call_kwargs["stdin"] == subprocess.DEVNULL

    def test_passes_all_args(self) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0)
            dc("exec", "apiserver", "cargo", "check")
            cmd = mock_run.call_args.args[0]
            # Find the args after the compose base
            assert "exec" in cmd
            assert "apiserver" in cmd
            assert "cargo" in cmd
            assert "check" in cmd

    def test_raises_on_failure(self) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.side_effect = subprocess.CalledProcessError(1, "cmd")
            import pytest

            with pytest.raises(subprocess.CalledProcessError):
                dc("invalid-command")

    def test_no_raise_when_check_false(self) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=1)
            result = dc("down", check=False)
            assert result.returncode == 1

    def test_uses_project_root_as_cwd(self) -> None:
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0)
            dc("ps")
            call_kwargs = mock_run.call_args.kwargs
            assert call_kwargs["cwd"] == PROJECT_ROOT


class TestDcDetached:
    def test_starts_new_session(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            mock_proc.returncode = 0
            mock_popen.return_value = mock_proc
            dc_detached("down", "--remove-orphans")
            call_kwargs = mock_popen.call_args.kwargs
            assert call_kwargs["start_new_session"] is True

    def test_null_fds(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            mock_proc.returncode = 0
            mock_popen.return_value = mock_proc
            dc_detached("down")
            call_kwargs = mock_popen.call_args.kwargs
            assert call_kwargs["stdin"] == subprocess.DEVNULL
            assert call_kwargs["stdout"] == subprocess.DEVNULL
            assert call_kwargs["stderr"] == subprocess.DEVNULL

    def test_returns_exit_code(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            mock_proc.returncode = 7
            mock_popen.return_value = mock_proc
            result = dc_detached("down")
            assert result == 7

    def test_timeout_kills_process_group(self) -> None:
        with patch("subprocess.Popen") as mock_popen:
            mock_proc = MagicMock()
            # First call to wait() raises Timeout, second call succeeds
            mock_proc.wait.side_effect = [
                subprocess.TimeoutExpired("cmd", 1),
                None,
            ]
            mock_popen.return_value = mock_proc

            with patch("os.killpg") as mock_killpg:
                result = dc_detached("down", timeout=1)
                mock_killpg.assert_called_once_with(mock_proc.pid, 9)
                assert result == 1

    def test_default_timeout_from_env(self) -> None:
        with patch.dict(os.environ, {"DOCKER_COMPOSE_TIMEOUT": "30"}):
            from importlib import reload

            import _docker_ops

            reload(_docker_ops)
            assert _docker_ops.DEFAULT_TIMEOUT == 30
