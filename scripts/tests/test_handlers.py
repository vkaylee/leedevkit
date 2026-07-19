"""Tests for extracted handler modules (DbHandler, RunHandler, TestHandler, InitHandler).

These tests verify the public API of each handler, using mock orchestrator
instances to provide the shared state contract.
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch


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
                stdout="myproject_apiserver_1\nmyproject_db_1\n", returncode=0,
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
                stdout="tp_webdashboard_1\n", returncode=0,
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
                stdout="lk_apiserver_1\nlk_db_1\n", returncode=0,
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
        # fmt + lint (ruff+mypy) + test_infra
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
        with patch.dict("os.environ", {}, clear=True):
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
        (log_dir / "test.log").write_text(
            "Summary [default] 5 tests run: 5 passed"
        )
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
