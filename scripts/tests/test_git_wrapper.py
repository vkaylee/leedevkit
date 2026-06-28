import atexit as atexit_module
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any
from unittest import mock

# Import the wrapper module
import _git_wrapper
import pytest
from _arg_sanitizer import ArgSanitizeError


def test_non_commit_command(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git command other than commit passes through untouched."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "status", "--short"])

    with (
        mock.patch("subprocess.call", return_value=0) as mock_call,
        mock.patch("sys.exit") as mock_exit,
    ):
        _git_wrapper.main()

    # Check that subprocess.call was called with correct arguments
    called_cmd = mock_call.call_args[0][0]
    assert "git" in called_cmd
    assert "status" in called_cmd
    assert "--short" in called_cmd
    assert "-F" not in called_cmd

    mock_exit.assert_called_once_with(0)


def test_commit_without_message(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit without -m."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-a"])

    with (
        mock.patch("subprocess.call", return_value=0) as mock_call,
        mock.patch("sys.exit") as mock_exit,
    ):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "commit" in called_cmd
    assert "-a" in called_cmd
    assert "-F" not in called_cmd
    mock_exit.assert_called_once_with(0)


def test_commit_with_space_separated_m(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit -m 'message'."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-a", "-m", "Hello world"])

    with (
        mock.patch("subprocess.call", return_value=0) as mock_call,
        mock.patch("sys.exit"),
    ):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "commit" in called_cmd
    assert "-a" in called_cmd
    assert "-m" not in called_cmd
    assert "Hello world" not in called_cmd

    # Verify -F is injected and file contains the message
    assert "-F" in called_cmd
    f_index = called_cmd.index("-F")
    temp_file = called_cmd[f_index + 1]

    assert Path(temp_file).exists()
    with Path(temp_file).open(encoding="utf-8") as f:
        content = f.read()
    assert content == "Hello world\n"

    # Manual cleanup as we're mocking exit
    Path(temp_file).unlink()


def test_commit_with_attached_m(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit -m'message'."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-mFix issue"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "-F" in called_cmd
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Fix issue\n"
    Path(temp_file).unlink()


def test_commit_with_attached_message(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit --message='message'."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "--message=Hello"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "-F" in called_cmd
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Hello\n"
    Path(temp_file).unlink()


def test_commit_with_multiple_messages(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with multiple -m."""
    monkeypatch.setattr(
        sys, "argv", ["_git_wrapper.py", "commit", "-m", "Title", "--message", "Body"]
    )

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "-F" in called_cmd
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Title\n\nBody\n"
    Path(temp_file).unlink()


def test_commit_with_special_characters(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with emojis, quotes, and symbols."""
    special_msg = "Fix bug 🐛\nAdded 'quotes' and \"double quotes\"!\n$VAR and `backticks`"
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m", special_msg])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == special_msg + "\n"
    Path(temp_file).unlink()


def test_commit_with_multiline_string(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with actual newlines in a single -m."""
    multiline_msg = "Line 1\nLine 2\n\nLine 4"
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "--message", multiline_msg])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Line 1\nLine 2\n\nLine 4\n"
    Path(temp_file).unlink()


def test_commit_with_empty_message(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with an empty string message."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m", ""])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "\n"
    Path(temp_file).unlink()


def test_commit_with_missing_message_value(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with -m as the very last argument (missing value).
    The wrapper shouldn't crash, it should just ignore it or pass it along,
    but since it checks i+1 < len, it shouldn't IndexError."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    # In this case, messages array is empty because we guarded against IndexError.
    # So -F should not be present!
    assert "-F" not in called_cmd
    # Instead, -m gets passed down and git will handle the error.
    assert "-m" in called_cmd


def test_commit_with_mixed_message_flags(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test mixing all possible ways to provide a message."""
    monkeypatch.setattr(
        sys,
        "argv",
        [
            "_git_wrapper.py",
            "commit",
            "-m",
            "Msg1",
            "--message=Msg2",
            "--message",
            "Msg3",
            "-mMsg4",
        ],
    )

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Msg1\n\nMsg2\n\nMsg3\n\nMsg4\n"
    Path(temp_file).unlink()


def test_commit_with_complex_unicode(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test git commit with complex non-ascii unicode."""
    unicode_msg = "Thử nghiệm unicode 測試 🚀 (CJK, Viet, Emoji, RTL ‮test‬)"  # noqa: PLE2502
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m", unicode_msg])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == unicode_msg + "\n"
    Path(temp_file).unlink()


def test_commit_with_interleaved_flags(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test that unrelated flags before and after -m are preserved."""
    monkeypatch.setattr(
        sys,
        "argv",
        ["_git_wrapper.py", "commit", "--amend", "--quiet", "-m", "Msg", "--author=Test"],
    )

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "--amend" in called_cmd
    assert "--quiet" in called_cmd
    assert "--author=Test" in called_cmd

    temp_file = called_cmd[called_cmd.index("-F") + 1]
    with Path(temp_file).open(encoding="utf-8") as f:
        assert f.read() == "Msg\n"
    Path(temp_file).unlink()


def test_atexit_cleanup(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test that the cleanup function correctly deletes files, and ignores OSErrors."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m", "Msg"])

    # Spy on atexit.register to capture the cleanup function
    cleanup_func = None

    original_register = atexit_module.register

    def fake_register(func: Callable[..., Any], *args: Any, **kwargs: Any) -> Any:
        nonlocal cleanup_func
        cleanup_func = func
        return original_register(func, *args, **kwargs)

    with (
        mock.patch("atexit.register", side_effect=fake_register),
        mock.patch("subprocess.call", return_value=0) as mock_call,
        mock.patch("sys.exit"),
    ):
        _git_wrapper.main()

    assert cleanup_func is not None
    called_cmd = mock_call.call_args[0][0]
    temp_file = called_cmd[called_cmd.index("-F") + 1]

    # 1. Normal cleanup
    assert Path(temp_file).exists()
    cleanup_func()
    assert not Path(temp_file).exists()

    # 2. Ignore OSError cleanup (file already deleted)
    # This shouldn't raise any exception
    cleanup_func()


def test_commit_auto_inject_no_gpg_sign(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test that --no-gpg-sign is automatically injected."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-m", "Msg"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "--no-gpg-sign" in called_cmd


def test_commit_with_existing_no_gpg_sign(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test that --no-gpg-sign is not injected twice."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "--no-gpg-sign", "-m", "Msg"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert called_cmd.count("--no-gpg-sign") == 1


def test_commit_with_s_flag(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test that --no-gpg-sign is not injected if -S or --gpg-sign is present."""
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "-S", "-m", "Msg"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    assert "--no-gpg-sign" not in called_cmd

    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "commit", "--gpg-sign", "-m", "Msg"])

    with mock.patch("subprocess.call", return_value=0) as mock_call2, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd2 = mock_call2.call_args[0][0]
    assert "--no-gpg-sign" not in called_cmd2


def test_timeout_env_var(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test GIT_TIMEOUT env var."""
    monkeypatch.setenv("GIT_TIMEOUT", "300")
    monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "status"])

    with mock.patch("subprocess.call", return_value=0) as mock_call, mock.patch("sys.exit"):
        _git_wrapper.main()

    called_cmd = mock_call.call_args[0][0]
    # In the command constructed: [sys.executable, safe_run_path, timeout, 'git']
    assert "300" in called_cmd


class TestSanitizerFallbackSafety:
    """Verify _git_wrapper.py never passes empty args to git (would hang)."""

    def test_sanitize_failure_keeps_original_args(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """When sanitize raises, args MUST be preserved — never replaced with [].

        Empty args → git interactive mode + stdin=DEVNULL → infinite hang.
        """
        monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "status"])

        with (
            mock.patch("subprocess.call", return_value=0) as mock_call,
            mock.patch("sys.exit"),
            mock.patch("_git_wrapper.sanitize", side_effect=ArgSanitizeError("test")),
        ):
            _git_wrapper.main()

        called_cmd = mock_call.call_args[0][0]
        # ["git", "status"] must be in the command (not just ["git"])
        assert "status" in called_cmd
        assert called_cmd.count("git") == 1  # "git" appears once, no empty fallback

    def test_git_add_many_files_keeps_all_args(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Simulate 'git add 13 files' — all paths must survive sanitize."""
        files = [
            "webdashboard/src/components/features/leave/leave-request-row.tsx",
            "webdashboard/src/components/features/reports/use-daily-drawer.ts",
            "webdashboard/src/components/payroll/payslip-modal.tsx",
            "webdashboard/src/hooks/use-report-stream.ts",
            "webdashboard/src/hooks/use-report-stream-socket.ts",
            "webdashboard/src/hooks/use-report-stream-utils.ts",
        ]
        monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "add"] + files)

        with (
            mock.patch("subprocess.call", return_value=0) as mock_call,
            mock.patch("sys.exit"),
        ):
            _git_wrapper.main()

        called_cmd = mock_call.call_args[0][0]
        for f in files:
            assert f in called_cmd

    def test_non_commit_path_does_not_inject_gpg_flag(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """git add should NOT get --no-gpg-sign injected (only commit does)."""
        monkeypatch.setattr(sys, "argv", ["_git_wrapper.py", "add", "file.tsx"])

        with (
            mock.patch("subprocess.call", return_value=0) as mock_call,
            mock.patch("sys.exit"),
        ):
            _git_wrapper.main()

        called_cmd = mock_call.call_args[0][0]
        assert "--no-gpg-sign" not in called_cmd
