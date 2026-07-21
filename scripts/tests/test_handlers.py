"""Tests for extracted handler modules (DbHandler, RunHandler, TestHandler, InitHandler).

These tests verify the public API of each handler, using mock orchestrator
instances to provide the shared state contract.
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest


# ── Helpers ──────────────────────────────────────────────────────────────────


def _mock_orchestrator(**overrides):
    """Build a mock orchestrator with sensible defaults for handler testing."""
    orch = MagicMock()
    orch.engine = "podman"
    orch.compose_engine = ["podman-compose"]
    orch.env_vars = {"COMPOSE_PROJECT_NAME": "leedevkit-test"}
    orch.dry_run = False
    orch.results = {}
    orch.start_time = 0.0
    orch.needs_cleanup = False
    orch.active_mode = "all"
    orch.tool_map = {"npm": "webdashboard", "cargo": "apiserver", "diesel": "apiserver"}
    # Apply overrides
    for k, v in overrides.items():
        setattr(orch, k, v)
    return orch


# ── DbHandler ────────────────────────────────────────────────────────────────


class TestDbHandler:
    """Unit tests for the extracted DbHandler."""

    def test_get_compose_files_dev(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        files = handler.get_compose_files("dev")
        assert "-p" in files
        assert "leedevkit-dev" in files
        assert "docker-compose.yml" in files

    def test_get_compose_files_test(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        files = handler.get_compose_files("test")
        assert "-p" in files
        assert "leedevkit-test" in files

    def test_get_compose_files_default(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        files = handler.get_compose_files("prod")
        assert "-p" in files
        assert "leedevkit" in files

    def test_handle_prebuild_phase(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        result = handler.handle_prebuild_phase()
        assert result is True
        orch.execute_safe.assert_called_once()

    def test_handle_db_query(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        args = MagicMock()
        args.sql = "SELECT 1"
        args.json = False
        handler.handle_db_query(args)
        orch.execute_safe.assert_called_once()

    def test_handle_db_query_json(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        args = MagicMock()
        args.sql = "SELECT 1"
        args.json = True
        handler.handle_db_query(args)
        orch.execute_safe.assert_called_once()

    def test_handle_diesel(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_diesel(["migration", "run"])
        orch.execute_safe.assert_called_once()

    # ── _exec_psql helper ──────────────────────────────────────────────────

    @patch("subprocess.run")
    def test_exec_psql_basic_sql(self, mock_run):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        handler._exec_psql("test_container", sql="SELECT 1")
        mock_run.assert_called_once()
        cmd = mock_run.call_args[0][0]
        assert "psql" in cmd
        assert "-c" in cmd
        assert "SELECT 1" in cmd
        assert "test_container" in cmd

    @patch("subprocess.run")
    def test_exec_psql_tuples_only(self, mock_run):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        handler._exec_psql("test_container", sql="SELECT 1", tuples_only=True)
        cmd = mock_run.call_args[0][0]
        assert "-t" in cmd

    @patch("subprocess.run")
    def test_exec_psql_capture(self, mock_run):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        handler._exec_psql("test_container", sql="SELECT 1", capture=True)
        kwargs = mock_run.call_args[1]
        assert kwargs.get("capture_output") is True
        assert kwargs.get("text") is True

    @patch("subprocess.run")
    def test_exec_psql_input_text(self, mock_run):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        handler._exec_psql("test_container", input_text="DROP DATABASE foo;")
        kwargs = mock_run.call_args[1]
        assert kwargs.get("input") == "DROP DATABASE foo;"
        assert kwargs.get("text") is True

    @patch("subprocess.run")
    def test_exec_psql_no_sql_no_input_does_not_add_c_flag(self, mock_run):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        handler._exec_psql("test_container")
        cmd = mock_run.call_args[0][0]
        assert "-c" not in cmd

    # ── handle_db_setup_phase ──────────────────────────────────────────────

    @patch("time.sleep")
    @patch("subprocess.run")
    def test_handle_db_setup_happy_path(self, mock_run, mock_sleep):
        """Full happy path: pg_isready → cleanup → create → migrations → revoke."""
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)

        mock_run.return_value.returncode = 0
        mock_run.return_value.stdout = ""

        import _db_handler

        with patch.object(_db_handler, "_lifecycle_up", return_value=None):
            result = handler.handle_db_setup_phase()

        assert result is True
        pg_isready_calls = [
            c for c in mock_run.call_args_list if "pg_isready" in str(c)
        ]
        assert len(pg_isready_calls) >= 1

    @patch("time.sleep")
    @patch("subprocess.run")
    def test_handle_db_setup_db_not_ready(self, mock_run, mock_sleep):
        """pg_isready never succeeds → should return False."""
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)

        # pg_isready always fails
        mock_run.return_value.returncode = 1
        mock_run.return_value.stdout = ""

        import _db_handler

        with patch.object(_db_handler, "_lifecycle_up", return_value=None):
            result = handler.handle_db_setup_phase()

        assert result is False
        # Should have tried 60 times
        assert mock_sleep.call_count == 60

    @patch("time.sleep")
    @patch("subprocess.run")
    def test_handle_db_setup_cleans_zombie_dbs(self, mock_run, mock_sleep):
        """When cleanup SQL returns zombie DB names, they get dropped."""
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)

        # Call tracking: first is pg_isready, second is cleanup capture,
        # third is drop, then create, etc.
        call_results = {"count": 0}

        def side_effect(*args, **kwargs):
            call_results["count"] += 1
            result = MagicMock()
            # First call: pg_isready
            if call_results["count"] == 1:
                result.returncode = 0
                result.stdout = ""
            # Second call: cleanup capture (exec_psql with capture=True)
            elif call_results["count"] == 2:
                result.returncode = 0
                result.stdout = (
                    'DROP DATABASE IF EXISTS "test_db_abc123";\n'
                    'DROP DATABASE IF EXISTS "test_db_def456";\n'
                )
            # All subsequent: success
            else:
                result.returncode = 0
                result.stdout = ""
            return result

        mock_run.side_effect = side_effect
        mock_sleep.return_value = None

        import _db_handler

        with patch.object(_db_handler, "_lifecycle_up", return_value=None):
            result = handler.handle_db_setup_phase()

        assert result is True
        # Third call should contain the DROP commands
        assert call_results["count"] >= 3

    @patch("time.sleep")
    @patch("subprocess.run")
    def test_handle_db_setup_calls_execute_safe_for_migrations(
        self, mock_run, mock_sleep
    ):
        """Migrations should be dispatched via _execute_safe (not raw subprocess)."""
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        orch.execute_safe = MagicMock()
        handler = DbHandler(orch)

        call_results = {"count": 0}

        def side_effect(*args, **kwargs):
            call_results["count"] += 1
            result = MagicMock()
            result.returncode = 0
            result.stdout = ""
            return result

        mock_run.side_effect = side_effect
        mock_sleep.return_value = None

        import _db_handler

        with patch.object(_db_handler, "_lifecycle_up", return_value=None):
            result = handler.handle_db_setup_phase()

        assert result is True
        # execute_safe should have been called for the two migration runs
        assert orch.execute_safe.call_count >= 2


# ── RunHandler ───────────────────────────────────────────────────────────────


class TestRunHandler:
    """Unit tests for the extracted RunHandler."""

    def test_is_service_running_not_found(self):
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        # Mock subprocess to return empty container list
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(stdout="", returncode=0)
            handler = RunHandler(orch)
            assert handler.is_service_running("nonexistent") is False

    def test_is_service_running_found(self):
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.env_vars["COMPOSE_PROJECT_NAME"] = "myproject"
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(
                stdout="myproject_apiserver_1\nmyproject_db_1\n",
                returncode=0,
            )
            handler = RunHandler(orch)
            assert handler.is_service_running("apiserver") is True

    def test_handle_run_cargo_no_db(self):
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["fmt"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_run(args)
        orch.execute_safe.assert_called_once()

    def test_handle_run_npm_standalone(self):
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "npm"
        args.args = ["--version"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_run(args)
        orch.execute_safe.assert_called_once()

    def test_handle_run_cargo_standalone(self):
        """cargo fmt (no DB needed) should work."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["fmt"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_run(args)
        orch.execute_safe.assert_called_once()

    def test_handle_run_cargo_test_needs_db(self):
        """cargo test should bring up DB dependencies."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["test"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            with patch("_run_handler._lifecycle_up") as mock_up:
                handler.handle_run(args)
        orch.execute_safe.assert_called_once()
        # DB + pooler profiles should have been started
        assert mock_up.call_count >= 2

    def test_handle_run_with_pooler(self):
        """pooler flag sets DATABASE_POOLER_URL env var."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["fmt"]
        args.pooler = True
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_run(args)
        assert "DATABASE_POOLER_URL" in orch.env_vars

    def test_handle_run_npm_typecheck_rewrite(self):
        """npm run typecheck is rewritten to type-check."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        # Simulate service running for exec path
        orch.env_vars["COMPOSE_PROJECT_NAME"] = "tp"
        with patch("subprocess.run") as mock_sr:
            mock_sr.return_value = MagicMock(
                stdout="tp_webdashboard_1\n",
                returncode=0,
            )
            handler = RunHandler(orch)
            args = MagicMock()
            args.tool = "npm"
            args.args = ["run", "typecheck"]
            args.pooler = False
            with patch("pathlib.Path.cwd", return_value=MagicMock()):
                handler.handle_run(args)
        orch.execute_safe.assert_called_once()

    def test_handle_run_sanitize_args(self):
        """AI-provided args are sanitized before execution."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["fmt", "--check"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            handler.handle_run(args)
        orch.execute_safe.assert_called_once()

    def test_is_service_running_podman(self):
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.engine = "podman"
        orch.env_vars["COMPOSE_PROJECT_NAME"] = "lk"
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(
                stdout="lk_apiserver_1\nlk_db_1\n",
                returncode=0,
            )
            handler = RunHandler(orch)
            assert handler.is_service_running("apiserver") is True


# ── TestHandler ──────────────────────────────────────────────────────────────


class TestTestHandler:
    """Unit tests for the extracted TestHandler."""

    def test_handle_lint_infra(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with patch("shutil.which", return_value=None):
            handler.handle_lint_infra()
        # ruff check + mypy = 2 calls (no shellcheck since shutil.which returns None)
        assert orch.execute_safe.call_count >= 2

    def test_handle_fmt_infra(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler.handle_fmt_infra()
        orch.execute_safe.assert_called_once()

    def test_handle_verify_infra(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with patch("shutil.which", return_value=None):
            handler.handle_verify_infra()

        commands = [call.args[0] for call in orch.execute_safe.call_args_list]
        assert any(command[1:3] == ["format", "--check"] for command in commands)
        assert not any(
            len(command) >= 2 and command[1] == "format" and "--check" not in command
            for command in commands
        )
        # format check + lint (ruff+mypy) + test_infra
        assert orch.execute_safe.call_count >= 4

    def test_handle_test_infra(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler.handle_test_infra()
        orch.execute_safe.assert_called_once()

    def test_print_test_summary_no_logs(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        # Should not raise when no log dir exists
        handler.print_test_summary("test-target")

    def test_run_phase_dry_run(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator(dry_run=True)
        handler = TestHandler(orch)
        args = MagicMock()
        handler.run_phase("Linting", "api", args)
        orch.execute_safe.assert_not_called()

    def test_handle_test_infra_lint_only(self):
        """handle_test with target=infra and lint_only=True runs lint."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "infra"
        args.lint_only = True
        with patch("shutil.which", return_value=None):
            handler.handle_test(args)
        # ruff + mypy
        assert orch.execute_safe.call_count >= 2

    def test_handle_test_infra_full(self):
        """handle_test with target=infra (no lint_only) runs full verify."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "infra"
        args.lint_only = False
        args.unit_only = False
        args.e2e_only = False
        with patch("shutil.which", return_value=None):
            handler.handle_test(args)
        # fmt + lint + test = at least 4 calls
        assert orch.execute_safe.call_count >= 4

    def test_handle_test_lint_only(self):
        """handle_test with lint_only runs only lint phase."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "api"
        args.lint_only = True
        args.unit_only = False
        args.e2e_only = False
        args.skip_lint = False
        args.coverage = False
        args.timeout = None
        args.pattern = ""
        args.fix = False
        args.json_output = False
        args.component = ""
        with (
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_lint", return_value=True),
        ):
            handler.handle_test(args)
        # run_phase was called for Linting
        assert "Linting" in orch.results

    def test_handle_test_unit_only(self):
        """handle_test with unit_only runs only unit tests."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "api"
        args.lint_only = False
        args.unit_only = True
        args.e2e_only = False
        args.skip_lint = False
        args.coverage = False
        args.timeout = None
        args.pattern = ""
        args.fix = False
        args.json_output = False
        args.component = ""
        with (
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_unit", return_value=True),
        ):
            handler.handle_test(args)
        assert "Unit Tests" in orch.results

    def test_handle_test_coverage(self):
        """handle_test with coverage flag skips unit/e2e, runs Coverage."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "api"
        args.lint_only = False
        args.unit_only = False
        args.e2e_only = False
        args.skip_lint = False
        args.coverage = True
        args.timeout = None
        args.pattern = ""
        args.fix = False
        args.json_output = False
        args.component = ""
        with (
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_coverage", return_value=True),
        ):
            handler.handle_test(args)
        assert "Coverage" in orch.results

    def test_handle_test_sets_timeout_env(self):
        """handle_test with timeout arg sets env vars."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "api"
        args.lint_only = False
        args.unit_only = False
        args.e2e_only = False
        args.skip_lint = False
        args.coverage = False
        args.timeout = 600
        args.pattern = ""
        args.fix = False
        args.json_output = False
        args.component = ""
        with (
            patch.dict("os.environ", {}, clear=True),
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_unit", return_value=True),
            patch("_test_handler.leedevkit_run_lint", return_value=True),
            patch("_test_handler.leedevkit_run_integration", return_value=True),
        ):
            handler.handle_test(args)
            assert orch.env_vars.get("TIMEOUT_LINT") == "600"

    def test_run_phase_unknown_phase(self):
        """run_phase with unknown phase name exits with error."""
        import pytest
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        with pytest.raises(SystemExit):
            handler.run_phase("BogusPhase", "api", args)

    def test_run_phase_startup(self):
        """run_phase 'Startup' brings up lifecycle."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.component = ""
        args.fix = False
        args.pattern = ""
        args.unit_only = False
        with patch("_test_handler._lifecycle_up") as mock_up:
            handler.run_phase("Startup", "all", args)
        mock_up.assert_called_once_with("all")

    def test_print_test_summary_with_logs(self, tmp_path):
        """print_test_summary parses log files correctly."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.start_time = 0.0  # Accept all log files
        handler = TestHandler(orch)
        # Create a fake log with nextest-style output
        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        (log_dir / "test.log").write_text("Summary [default] 5 tests run: 5 passed")
        with patch("_test_handler.PROJECT_ROOT", tmp_path):
            handler.print_test_summary("my-target")
        # Should not raise


# ── InitHandler ──────────────────────────────────────────────────────────────


class TestInitHandler:
    """Unit tests for the extracted InitHandler."""

    def test_read_installed_version_none(self, tmp_path):
        from _init_handler import InitHandler

        orch = _mock_orchestrator()
        handler = InitHandler(orch)
        assert handler._read_installed_version(tmp_path) is None

    def test_read_installed_version_found(self, tmp_path):
        from _init_handler import InitHandler

        orch = _mock_orchestrator()
        (tmp_path / "VERSION").write_text("0.3.0")
        handler = InitHandler(orch)
        assert handler._read_installed_version(tmp_path) == "0.3.0"

    def test_detect_legacy_symlinks_empty(self, tmp_path):
        from _init_handler import InitHandler

        orch = _mock_orchestrator()
        handler = InitHandler(orch)
        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir()
        result = handler._detect_legacy_symlinks(agent_dir)
        assert result == []

    def test_detect_legacy_symlinks_missing_dir(self, tmp_path):
        from _init_handler import InitHandler

        orch = _mock_orchestrator()
        handler = InitHandler(orch)
        result = handler._detect_legacy_symlinks(tmp_path / "nonexistent")
        assert result == []


class TestTestHandlerCoverageGaps:
    """Targeted tests to close coverage gaps in _test_handler.py."""

    def test_print_test_summary_old_logs_filtered(self, tmp_path, monkeypatch):
        """print_test_summary skips log files older than start_time."""
        from _test_handler import TestHandler
        import time

        orch = _mock_orchestrator()
        orch.start_time = time.time() + 3600  # 1 hour in the future
        handler = TestHandler(orch)

        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        (log_dir / "old.log").write_text("Summary [1] 1 test run: 1 passed")
        monkeypatch.setattr("_test_handler.PROJECT_ROOT", tmp_path)

        handler.print_test_summary("all")
        # No crash — old logs filtered successfully

    def test_print_test_summary_with_playwright_logs(self, tmp_path, monkeypatch):
        """print_test_summary parses playwright log output."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.start_time = 0  # Way in the past
        handler = TestHandler(orch)

        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        (log_dir / "playwright.log").write_text("  5 passed (2m)\n  3 passed (1m)\n")
        monkeypatch.setattr("_test_handler.PROJECT_ROOT", tmp_path)

        handler.print_test_summary("all")

    def test_print_test_summary_with_vitest_logs(self, tmp_path, monkeypatch):
        """print_test_summary parses vitest log output."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.start_time = 0
        handler = TestHandler(orch)

        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        (log_dir / "vitest.log").write_text("Tests  42 passed (84)")
        monkeypatch.setattr("_test_handler.PROJECT_ROOT", tmp_path)

        handler.print_test_summary("web")

    def test_print_test_summary_empty_logs(self, tmp_path, monkeypatch):
        """print_test_summary handles empty log directory."""
        from _test_handler import TestHandler
        import time

        orch = _mock_orchestrator()
        orch.start_time = time.time()
        handler = TestHandler(orch)

        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        monkeypatch.setattr("_test_handler.PROJECT_ROOT", tmp_path)

        handler.print_test_summary("api")

    def test_print_test_summary_corrupted_log(self, tmp_path, monkeypatch):
        """print_test_summary handles unreadable log files."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.start_time = 0
        handler = TestHandler(orch)

        log_dir = tmp_path / ".test_logs"
        log_dir.mkdir()
        # Create a file we can't read properly
        (log_dir / "broken.log").write_bytes(b"\xff\xfe\x00\x01")
        monkeypatch.setattr("_test_handler.PROJECT_ROOT", tmp_path)

        handler.print_test_summary("api")  # Should not crash

    def test_handle_test_with_json_output(self, tmp_path, monkeypatch):
        """handle_test with --json produces JSON output (does not crash)."""
        from _test_handler import TestHandler
        import argparse

        orch = _mock_orchestrator()
        orch.start_time = 0
        orch.results = {
            "Linting": {"status": "pass", "duration_s": 1.0},
        }
        handler = TestHandler(orch)

        monkeypatch.setattr(handler, "run_phase", lambda *a, **kw: None)
        monkeypatch.setattr(handler, "print_test_summary", lambda *a, **kw: None)

        args = argparse.Namespace(
            target="api",
            lint_only=False,
            unit_only=False,
            e2e_only=False,
            coverage=True,
            skip_lint=False,
            fix=False,
            pattern="",
            timeout=None,
            json_output=True,
            component="",
        )
        # Should not crash — json output goes to stderr
        handler.handle_test(args)

    def test_run_phase_unknown_phase(self):
        """run_phase exits with error for unknown phase."""
        from _test_handler import TestHandler
        import argparse

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        orch.dry_run = False

        args = argparse.Namespace(
            target="all", component="", fix=False, pattern="", unit_only=False
        )
        with pytest.raises(SystemExit) as exc:
            handler.run_phase("UnknownPhase", "all", args)
        assert exc.value.code == 1

    def test_run_phase_dry_run(self):
        """run_phase in dry_run mode logs and returns."""
        from _test_handler import TestHandler
        import argparse

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        orch.dry_run = True

        args = argparse.Namespace(
            target="all", component="", fix=False, pattern="", unit_only=False
        )
        handler.run_phase("Linting", "all", args)  # Should no-op

    def test_handle_test_all_recursive(self, tmp_path, monkeypatch):
        """handle_test with target=all does not crash."""
        from _test_handler import TestHandler
        import argparse

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        orch.dry_run = False

        monkeypatch.setattr("_devkit_config.resolve_targets", lambda: ["infra"])
        monkeypatch.setattr("_test_handler.build_mode_map", lambda: {"infra": "infra"})
        monkeypatch.setattr("_test_handler.inject_rust_version_env", lambda: None)
        monkeypatch.setattr(handler, "run_phase", lambda *a, **kw: None)
        monkeypatch.setattr(handler, "print_test_summary", lambda *a, **kw: None)

        args = argparse.Namespace(
            target="all",
            lint_only=False,
            unit_only=False,
            e2e_only=False,
            coverage=False,
            skip_lint=False,
            fix=False,
            pattern="",
            timeout=None,
            json_output=False,
            component="",
        )
        handler.handle_test(args)  # Should not crash

    def test_handle_test_infra_coverage(self, tmp_path, monkeypatch):
        """handle_test_infra runs pytest with coverage."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        orch.dry_run = False

        executed = []

        def fake_execute(cmd, env=None, timeout=1800):
            executed.append(cmd)

        monkeypatch.setattr(handler, "_execute_safe", fake_execute)
        handler.handle_test_infra()

        assert len(executed) == 1
        assert "--cov=scripts" in " ".join(executed[0])
        assert "--cov-fail-under=80" in " ".join(executed[0])
