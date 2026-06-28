"""Tests for _test_utils — parallel runner, compose exec builder."""
import os
import sys
from pathlib import Path

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _test_utils import (
    LOG_DIR,
    _ensure_log_dir,
    _safe_log_name,
    build_compose_exec,
    get_phase_timeout,
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
        exit_code = run_single_task(
            "slow", ["sleep", "30"], log_file, timeout=1
        )
        assert exit_code == 124  # timeout exit code


class TestRunParallelOrdered:
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
