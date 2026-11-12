"""Tests for _test_utils — parallel runner, compose exec builder."""

import os
import subprocess
import sys
import time

import psutil

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _test_utils import (
    _ensure_log_dir,
    _go_pattern_evidence,
    _safe_log_name,
    build_compose_exec,
    get_last_phase_commands,
    get_phase_timeout,
    get_run_log_dir,
    run_parallel_ordered,
    run_single_task,
)


class TestGetPhaseTimeout:
    def test_lint_phase(self):
        t = get_phase_timeout("Linting")
        assert t > 0

    def test_unit_phase(self):
        t = get_phase_timeout("Unit Tests")
        assert t > 0

    def test_integration_phase(self):
        t = get_phase_timeout("Integration & E2E")
        assert t > 0

    def test_coverage_phase(self):
        t = get_phase_timeout("Coverage")
        assert t > 0

    def test_default_timeout(self):
        t = get_phase_timeout("Unknown Phase")
        assert t > 0


class TestSafeLogName:
    def test_spaces_replaced(self):
        name = _safe_log_name("Unit Tests", "my task")
        assert " " not in name
        assert "_" in name

    def test_slashes_replaced(self):
        name = _safe_log_name("Integration/E2E", "task/name")
        assert "/" not in name
        assert "_" in name

    def test_basic_name(self):
        name = _safe_log_name("Linting", "format-check")
        assert name.endswith(".log")
        assert "Linting" in name
        assert "format-check" in name


class TestBuildComposeExec:
    def test_builds_command_list(self, monkeypatch):
        monkeypatch.setenv("CONTAINER_ENGINE", "docker")
        monkeypatch.setenv("DOCKER_COMPOSE_CMD", "docker compose")
        cmd = build_compose_exec("apiserver", "echo hello", mode="api")
        assert cmd[0] in ("docker", "podman-compose")
        assert "exec" in cmd
        assert "apiserver" in cmd
        assert "echo hello" in cmd or "bash" in cmd

    def test_with_workdir(self, monkeypatch):
        monkeypatch.setenv("CONTAINER_ENGINE", "docker")
        monkeypatch.setenv("DOCKER_COMPOSE_CMD", "docker compose")
        cmd = build_compose_exec("apiserver", "ls", workdir="/workspace", mode="api")
        assert "-w" in cmd
        assert "/workspace" in cmd


class TestGoPatternEvidence:
    def test_requires_matching_run_or_skip_event(self, tmp_path):
        log = tmp_path / "go.json"
        log.write_text('{"Action":"pass","Test":"TestOther"}\n')
        assert not _go_pattern_evidence(log, "TestAuth")

    def test_accepts_skipped_test_event(self, tmp_path):
        log = tmp_path / "go.json"
        log.write_text('{"Action":"skip","Test":"TestAuth"}\n')
        assert _go_pattern_evidence(log, "Auth")

    def test_ignores_package_only_events(self, tmp_path):
        log = tmp_path / "go.json"
        log.write_text('{"Action":"pass","Package":"example"}\n')
        assert not _go_pattern_evidence(log, "example")


class TestRunSingleTask:
    def test_echo_command_succeeds(self, tmp_path, monkeypatch):
        log_file = tmp_path / "test.log"
        exit_code = run_single_task("test", ["echo", "hello"], log_file, timeout=10)
        assert exit_code == 0
        assert log_file.exists()
        content = log_file.read_text()
        assert "hello" in content

    def test_failing_command(self, tmp_path):
        log_file = tmp_path / "fail.log"
        exit_code = run_single_task("fail", ["false"], log_file, timeout=10)
        assert exit_code != 0

    def test_timeout(self, tmp_path):
        log_file = tmp_path / "timeout.log"
        exit_code = run_single_task("slow", ["sleep", "30"], log_file, timeout=1)
        assert exit_code == 124  # timeout exit code

    def test_timeout_terminates_descendant(self, tmp_path):
        child_pid_file = tmp_path / "child.pid"
        child_code = "import time; time.sleep(30)"
        parent_code = (
            "import pathlib, subprocess, sys, time; "
            f"p=subprocess.Popen([sys.executable, '-c', {child_code!r}]); "
            f"pathlib.Path({str(child_pid_file)!r}).write_text(str(p.pid)); "
            "time.sleep(30)"
        )
        log_file = tmp_path / "tree-timeout.log"

        exit_code = run_single_task(
            "tree-timeout", [sys.executable, "-c", parent_code], log_file, timeout=1
        )

        assert exit_code == 124
        child_pid = int(child_pid_file.read_text())
        assert not psutil.pid_exists(child_pid)

    def test_timeout_preserves_unrelated_process(self, tmp_path):
        unrelated = subprocess.Popen(
            [sys.executable, "-c", "import time; time.sleep(30)"]
        )
        try:
            exit_code = run_single_task(
                "isolated-timeout",
                [sys.executable, "-c", "import time; time.sleep(30)"],
                tmp_path / "isolated-timeout.log",
                timeout=1,
            )

            assert exit_code == 124
            assert psutil.pid_exists(unrelated.pid)
        finally:
            unrelated.terminate()
            unrelated.wait()

    def test_serial_timeout_terminates_descendant(self, tmp_path, monkeypatch):
        child_pid_file = tmp_path / "serial-child.pid"
        marker_file = tmp_path / "child-finished"
        child_code = (
            "import pathlib, time; "
            f"time.sleep(2); pathlib.Path({str(marker_file)!r}).write_text('finished')"
        )
        parent_code = (
            "import pathlib, subprocess, sys, time; "
            f"p=subprocess.Popen([sys.executable, '-c', {child_code!r}]); "
            f"pathlib.Path({str(child_pid_file)!r}).write_text(str(p.pid)); "
            "time.sleep(1.5)"
        )
        monkeypatch.setattr("_test_utils.TIMEOUT_LINT", 1)

        started = time.monotonic()
        result = run_parallel_ordered(
            "Linting",
            "",
            [("serial-tree-timeout", "svc", [sys.executable, "-c", parent_code])],
            num_workers=1,
        )

        assert result is False
        assert time.monotonic() - started < 1.8
        child_pid = int(child_pid_file.read_text())
        assert not psutil.pid_exists(child_pid)
        assert not marker_file.exists()

    def test_serial_timeout_preserves_unrelated_process(self, tmp_path, monkeypatch):
        unrelated = subprocess.Popen(
            [sys.executable, "-c", "import time; time.sleep(10)"]
        )
        try:
            monkeypatch.setattr("_test_utils.TIMEOUT_LINT", 1)
            result = run_parallel_ordered(
                "Linting",
                "",
                [
                    (
                        "serial-isolated-timeout",
                        "svc",
                        [sys.executable, "-c", "import time; time.sleep(2)"],
                    )
                ],
                num_workers=1,
            )

            assert result is False
            assert psutil.pid_exists(unrelated.pid)
        finally:
            unrelated.terminate()
            unrelated.wait()


class TestRunParallelOrdered:
    def test_parallel_output_streams_to_terminal_and_log(
        self, capsys, tmp_path, monkeypatch
    ):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)
        script = "import sys; print('stdout line', flush=True); print('stderr line', file=sys.stderr, flush=True)"
        result = run_parallel_ordered(
            "Linting",
            "",
            [("webdashboard", "srv", [sys.executable, "-c", script])],
            num_workers=2,
        )

        assert result is True
        output = capsys.readouterr().out
        assert "[webdashboard] stdout line\n" in output
        assert "[webdashboard] stderr line\n" in output
        assert (
            get_run_log_dir() / "Linting_webdashboard.log"
        ).read_text() == "stdout line\nstderr line\n"

    def test_parallel_tasks_keep_prefixes_and_lines(
        self, capsys, tmp_path, monkeypatch
    ):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)
        tasks = [
            ("task-a", "srv", ["printf", "a1\na2\n"]),
            ("task-b", "srv", ["printf", "b1\nb2\n"]),
        ]

        assert run_parallel_ordered("Linting", "", tasks, num_workers=2)
        output = capsys.readouterr().out
        assert output.count("[task-a] a1\n") == 1
        assert output.count("[task-a] a2\n") == 1
        assert output.count("[task-b] b1\n") == 1
        assert output.count("[task-b] b2\n") == 1
        for name, lines in (("task-a", "a1\na2\n"), ("task-b", "b1\nb2\n")):
            assert (get_run_log_dir() / f"Linting_{name}.log").read_text() == lines

    def test_parallel_quiet_keeps_log_without_live_output(
        self, capsys, tmp_path, monkeypatch
    ):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)
        monkeypatch.setattr("_test_utils.QUIET_OUTPUT", True)

        assert run_parallel_ordered(
            "Linting",
            "",
            [("quiet-task", "srv", ["printf", "hidden\n"])],
            num_workers=2,
        )
        assert capsys.readouterr().out == ""
        assert (get_run_log_dir() / "Linting_quiet-task.log").read_text() == "hidden\n"

    def test_parallel_failure_keeps_output(self, capsys, tmp_path, monkeypatch):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)
        script = "import sys; print('before failure', flush=True); sys.exit(7)"

        assert not run_parallel_ordered(
            "Linting",
            "",
            [("failed-task", "srv", [sys.executable, "-c", script])],
            num_workers=2,
        )
        assert "[failed-task] before failure\n" in capsys.readouterr().out
        assert (
            get_run_log_dir() / "Linting_failed-task.log"
        ).read_text() == "before failure\n"

    def test_parallel_timeout_keeps_output_and_returns_124(
        self, capsys, tmp_path, monkeypatch
    ):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)
        monkeypatch.setattr("_test_utils.TIMEOUT_LINT", 1)
        monkeypatch.setattr("_test_utils.QUIET_OUTPUT", False)
        script = "import time; print('before timeout', flush=True); time.sleep(5)"

        assert not run_parallel_ordered(
            "Linting",
            "",
            [("timeout-task", "srv", [sys.executable, "-c", script])],
            num_workers=2,
        )
        assert "[timeout-task] before timeout\n" in capsys.readouterr().out
        assert (
            get_run_log_dir() / "Linting_timeout-task.log"
        ).read_text() == "before timeout\n"
        from _test_utils import get_last_phase_exit_code

        assert get_last_phase_exit_code() == 124

    def test_parallel_output_without_trailing_newline_is_flushed(
        self, capsys, tmp_path, monkeypatch
    ):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path)

        assert run_parallel_ordered(
            "Linting",
            "",
            [("partial-task", "srv", ["printf", "last line"])],
            num_workers=2,
        )
        assert "[partial-task] last line" in capsys.readouterr().out
        assert (
            get_run_log_dir() / "Linting_partial-task.log"
        ).read_text() == "last line"

    def test_empty_tasks_returns_true(self):
        result = run_parallel_ordered("Linting", "", [])
        assert result is True

    def test_single_task_succeeds(self):
        tasks = [("echo-test", "any", ["echo", "ok"])]
        result = run_parallel_ordered("Linting", "", tasks, num_workers=1)
        assert result is True

    def test_single_task_fails(self):
        tasks = [("fail-test", "any", ["false"])]
        result = run_parallel_ordered("Linting", "", tasks, num_workers=1)
        assert result is False

    def test_component_filter(self):
        tasks = [
            ("api-test", "apiserver", ["echo", "api"]),
            ("web-test", "webdashboard", ["echo", "web"]),
        ]
        # With filter "api", only api-test should run
        result = run_parallel_ordered("Unit Tests", "api", tasks, num_workers=1)
        assert result is True

    def test_phase_commands_capture_filtered_commands(self):
        tasks = [
            ("api-test", "apiserver", ["echo", "api"]),
            ("web-test", "webdashboard", ["echo", "web"]),
        ]
        assert run_parallel_ordered("Unit Tests", "api", tasks, num_workers=1)
        assert get_last_phase_commands() == {"api-test": ["echo", "api"]}

    def test_parallel_execution(self):
        tasks = [
            ("task-a", "srv", ["echo", "a"]),
            ("task-b", "srv", ["echo", "b"]),
        ]
        result = run_parallel_ordered("Linting", "", tasks, num_workers=2)
        assert result is True

    def test_format_tasks_prioritized(self):
        """Format tasks should be moved to the beginning and executed first."""
        tasks = [
            ("slow-task", "srv", ["sleep", "0.1"]),
            ("format-foo", "srv", ["echo", "fmt"]),
        ]
        result = run_parallel_ordered("Linting", "", tasks, num_workers=1)
        assert result is True


class TestEnsureLogDir:
    def test_creates_dir(self, tmp_path, monkeypatch):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path / ".test_logs")
        log_dir = _ensure_log_dir()
        assert log_dir.exists()
        assert log_dir.is_dir()

    def test_run_log_dir_isolated_by_compose_project(self, tmp_path, monkeypatch):
        monkeypatch.setattr("_test_utils.LOG_DIR", tmp_path / ".test_logs")
        monkeypatch.setenv("COMPOSE_PROJECT_NAME", "leedevkit-test-a1b2")

        first = get_run_log_dir()
        monkeypatch.setenv("COMPOSE_PROJECT_NAME", "leedevkit-test-c3d4")
        second = get_run_log_dir()

        assert first != second
        assert first == tmp_path / ".test_logs" / "leedevkit-test-a1b2"
        assert second == tmp_path / ".test_logs" / "leedevkit-test-c3d4"


class TestPrintSuccessFailure:
    def test_print_success(self, capsys):
        import tempfile
        from pathlib import Path

        from _test_utils import _print_success

        log = Path(tempfile.gettempdir()) / "test_success.log"
        log.write_text("all good")
        _print_success("mytask", log)
        captured = capsys.readouterr()
        assert "PASSED" in captured.out

    def test_print_failure(self, capsys):
        import tempfile
        from pathlib import Path

        from _test_utils import _print_failure

        log = Path(tempfile.gettempdir()) / "test_fail.log"
        log.write_text("error line 1\nerror line 2")
        _print_failure("mytask", log, 1)
        captured = capsys.readouterr()
        assert "FAILED" in captured.out

    def test_print_failure_includes_complete_log(self, capsys, tmp_path):
        from _test_utils import _print_failure

        lines = [f"error line {index}" for index in range(1, 36)]
        log = tmp_path / "long_fail.log"
        log.write_text("\n".join(lines) + "\n")

        _print_failure("mytask", log, 1)

        captured = capsys.readouterr()
        assert "error line 1" in captured.out
        assert "error line 35" in captured.out

    def test_print_failure_no_log(self, capsys):
        import tempfile
        from pathlib import Path

        from _test_utils import _print_failure

        log = Path(tempfile.gettempdir()) / "nonexistent.log"
        _print_failure("mytask", log, 1)
        captured = capsys.readouterr()
        assert "FAILED" in captured.out

    def test_quiet_failure_keeps_status_without_log_dump(
        self, capsys, tmp_path, monkeypatch
    ):
        from _test_utils import _print_failure

        monkeypatch.setattr("_test_utils.QUIET_OUTPUT", True)
        log = tmp_path / "quiet.log"
        log.write_text("secret diagnostic")
        _print_failure("quiet-task", log, 1)
        captured = capsys.readouterr()
        assert "FAILED (exit=1)" in captured.out
        assert "secret diagnostic" not in captured.out


class TestBuildComposeExecEdgeCases:
    def test_no_workdir(self, monkeypatch):
        monkeypatch.setenv("CONTAINER_ENGINE", "docker")
        monkeypatch.setenv("DOCKER_COMPOSE_CMD", "docker compose")
        cmd = build_compose_exec("myservice", "ls", mode="api")
        assert "-w" not in cmd


class TestParallelEdgeCases:
    def test_component_filter_all(self):
        tasks = [
            ("keep-me", "srv", ["echo", "hi"]),
            ("drop-me", "other", ["echo", "no"]),
        ]
        result = run_parallel_ordered("Linting", "srv", tasks, num_workers=1)
        assert result is True

    def test_quiet_success_suppresses_progress(self, capsys, monkeypatch, tmp_path):
        from _test_utils import _print_success

        monkeypatch.setattr("_test_utils.QUIET_OUTPUT", True)
        _print_success("quiet-task", tmp_path / "quiet.log")
        assert capsys.readouterr().out == ""

    def test_no_matching_component(self):
        tasks = [("a", "srv", ["echo", "hi"])]
        result = run_parallel_ordered("Linting", "no-match", tasks, num_workers=1)
        assert result is True

    def test_single_task_timeout(self, tmp_path):
        log_file = tmp_path / "slow.log"
        from _test_utils import run_single_task

        rc = run_single_task("slow", ["sleep", "5"], log_file, timeout=1)
        assert rc == 124
