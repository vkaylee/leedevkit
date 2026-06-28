import os
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path
from unittest.mock import MagicMock, patch

import psutil
import pytest

# Add scripts to path so we can import _safe_run
sys.path.append(str(Path(__file__).parent.parent))
import _safe_run


def test_kill_process_tree() -> None:
    """Test killing a process tree."""
    # Start a dummy process that stays alive
    proc = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(10)"])
    pid = proc.pid
    assert psutil.pid_exists(pid)

    _safe_run.kill_process_tree(pid)

    # Wait a bit for it to be killed
    time.sleep(0.5)
    assert not psutil.pid_exists(pid)


def test_read_from_pty_eof() -> None:
    """Test reading from PTY on EOF."""
    master, slave = os.openpty()
    stop_event = threading.Event()

    # Write something to master (from slave side)
    os.write(slave, b"hello")
    os.close(slave)

    # This should read "hello" and then exit because slave is closed
    _safe_run.read_from_pty(master, stop_event)
    os.close(master)


def test_read_from_pty_stop_event() -> None:
    """Test reading from PTY when stop_event is set."""
    master, slave = os.openpty()
    stop_event = threading.Event()
    stop_event.set()

    # Should exit immediately
    _safe_run.read_from_pty(master, stop_event)

    os.close(slave)
    os.close(master)


def test_main_not_enough_args() -> None:
    """Test main with insufficient arguments."""
    with patch.object(sys, "argv", ["safe-run.py"]), pytest.raises(SystemExit) as exc:
        _safe_run.main()
    assert exc.value.code == 1


def test_main_invalid_timeout() -> None:
    """Test main with invalid timeout value."""
    with (
        patch.object(sys, "argv", ["safe-run.py", "abc", "ls"]),
        pytest.raises(SystemExit) as exc,
    ):
        _safe_run.main()
    assert exc.value.code == 1


@patch("subprocess.Popen")
def test_main_command_not_found(mock_popen: MagicMock) -> None:
    """Test main when command is not found."""
    mock_popen.side_effect = FileNotFoundError()
    with (
        patch.object(sys, "argv", ["safe-run.py", "5", "nonexistent-cmd"]),
        pytest.raises(SystemExit) as exc,
    ):
        _safe_run.main()
    assert exc.value.code == 127


def test_main_success() -> None:
    """Test main executing command successfully."""
    with (
        patch.object(sys, "argv", ["safe-run.py", "2", "echo", "hello"]),
        patch("sys.stdout.buffer.write"),  # Avoid printing to real stdout
        pytest.raises(SystemExit) as exc,
    ):
        _safe_run.main()
    assert exc.value.code == 0


def test_main_timeout() -> None:
    """Test main when command times out."""
    with (
        patch.object(sys, "argv", ["safe-run.py", "0.1", "sleep", "1"]),
        pytest.raises(SystemExit) as exc,
    ):
        _safe_run.main()
    assert exc.value.code == 124


def test_validate_command_python_c(capsys: pytest.CaptureFixture[str]) -> None:
    """Test validate_command blocks python -c."""
    with pytest.raises(SystemExit) as exc:
        _safe_run.validate_command(["python3", "-c", "print(1)"])
    assert exc.value.code == 1
    captured = capsys.readouterr()
    assert "PTY SECURITY ERROR" in captured.err


def test_validate_command_valid() -> None:
    """Test validate_command allows valid commands."""
    _safe_run.validate_command(["echo", "hello"])  # Should not raise


class TestExecuteCommandProcessGroupCleanup:
    """Verify _safe_run kills leftover children to release PTY slave FD."""

    def test_normal_exit_kills_process_group(self) -> None:
        """After proc.wait() returns, os.killpg must be called with SIGKILL."""
        with (
            patch("subprocess.Popen") as mock_popen,
            patch("os.killpg") as mock_killpg,
            patch("pty.openpty", return_value=(3, 4)),
            patch("threading.Thread"),
            patch("os.close"),
        ):
            mock_proc = MagicMock()
            mock_proc.returncode = 0
            mock_proc.pid = 12345
            mock_popen.return_value = mock_proc

            exit_code = _safe_run.execute_command(["echo", "hello"], timeout_sec=30)

            assert exit_code == 0
            # Must kill process group on normal exit
            mock_killpg.assert_called_once_with(12345, signal.SIGKILL)

    def test_timeout_uses_kill_process_tree(self) -> None:
        """On timeout, kill_process_tree handles the process tree + children."""
        with (
            patch("subprocess.Popen") as mock_popen,
            patch("_safe_run.kill_process_tree") as mock_kill_tree,
            patch("os.killpg") as mock_killpg,
            patch("pty.openpty", return_value=(3, 4)),
            patch("threading.Thread"),
            patch("os.close"),
        ):
            mock_proc = MagicMock()
            mock_proc.returncode = 0
            mock_proc.pid = 12345
            mock_proc.wait.side_effect = subprocess.TimeoutExpired("cmd", 1)
            mock_popen.return_value = mock_proc

            exit_code = _safe_run.execute_command(["sleep", "10"], timeout_sec=1)

            assert exit_code == 124
            # kill_process_tree should be called for timeout
            mock_kill_tree.assert_called_once_with(12345)
            # os.killpg should NOT be called for timeout (kill_tree handles it)
            mock_killpg.assert_not_called()

    def test_process_group_already_gone_is_handled(self) -> None:
        """When process group doesn't exist, OSError is caught gracefully."""
        with (
            patch("subprocess.Popen") as mock_popen,
            patch("os.killpg", side_effect=OSError("no such process")),
            patch("pty.openpty", return_value=(3, 4)),
            patch("threading.Thread"),
            patch("os.close"),
        ):
            mock_proc = MagicMock()
            mock_proc.returncode = 0
            mock_proc.pid = 12345
            mock_popen.return_value = mock_proc

            exit_code = _safe_run.execute_command(["true"], timeout_sec=30)

            assert exit_code == 0  # Should not raise


class TestArgSanitizerIntegration:
    """Verify _safe_run.validate_command uses _arg_sanitizer."""

    def test_blocks_dollar_paren(self, capsys: pytest.CaptureFixture[str]) -> None:
        with pytest.raises(SystemExit) as exc:
            _safe_run.validate_command(["echo", "$(whoami)"])
        assert exc.value.code == 1
        assert "PTY SAFETY ERROR" in capsys.readouterr().err

    def test_blocks_unmatched_quote(self, capsys: pytest.CaptureFixture[str]) -> None:
        with pytest.raises(SystemExit) as exc:
            _safe_run.validate_command(["echo", 'hello"world'])
        assert exc.value.code == 1
        assert "PTY SAFETY ERROR" in capsys.readouterr().err

    def test_allows_clean_args(self) -> None:
        _safe_run.validate_command(["podman-compose", "run", "apiserver"])


class TestLongArgList:
    """Simulate AI agent passing 24+ file paths to npm run lint."""

    def test_long_arg_list_passes_sanitizer(self) -> None:
        """24 valid file paths should pass sanitizer without issues."""
        files = [f"/workspace/webdashboard/src/test_{i:03d}.tsx" for i in range(30)]
        args = ["run", "lint", "--"] + files
        _safe_run.validate_command(["bun"] + args)

    def test_long_arg_list_with_dangerous_injection_blocked(self) -> None:
        """24 files + 1 shell injection should be caught."""
        files = [f"/workspace/webdashboard/src/test_{i:03d}.tsx" for i in range(29)]
        files.append("$(whoami)")
        args = ["run", "lint", "--"] + files
        with pytest.raises(SystemExit):
            _safe_run.validate_command(["bun"] + args)
