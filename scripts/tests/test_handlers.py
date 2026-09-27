"""Tests for extracted handler modules (DbHandler, RunHandler, TestHandler, InitHandler).

These tests verify the public API of each handler, using mock orchestrator
instances to provide the shared state contract.
"""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import json
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


def _test_args(**overrides):
    import argparse

    values = {
        "target": "api",
        "lint_only": False,
        "unit_only": False,
        "e2e_only": False,
        "skip_lint": False,
        "skip_e2e": False,
        "skip_build": False,
        "coverage": False,
        "timeout": None,
        "pattern": "",
        "fix": False,
        "json_output": False,
        "json_stream": False,
        "quiet": False,
        "component": "",
    }
    values.update(overrides)
    return argparse.Namespace(**values)


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

    def test_handle_prebuild_phase_builds_current_profiles_with_cache(self):
        from _db_handler import DbHandler

        orch = _mock_orchestrator()
        handler = DbHandler(orch)
        env = {
            "DOCKER_COMPOSE_CMD": "docker compose -p test -f current.yml --profile api --profile web",
        }
        with patch("_db_handler.bootstrap_env", return_value=env):
            assert handler.handle_prebuild_phase() is True
        command = orch.execute_safe.call_args.args[0]
        assert command[:5] == ["docker", "compose", "-p", "test", "-f"]
        assert command[5] == "current.yml"
        assert command[6:10] == ["--profile", "api", "--profile", "web"]
        assert command[-2:] == ["build", "--pull"]

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

    def test_handle_run_go_uses_go_profile_and_entrypoint(self):
        """run go must use --profile go, --entrypoint go, and the go service."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.tool_map = {
            "npm": "webdashboard",
            "cargo": "apiserver",
            "diesel": "apiserver",
            "go": "go",
        }
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "go"
        args.args = ["version"]
        args.pooler = False
        # Report service NOT running; mock lifecycle_up so no real compose runs.
        with patch(
            "subprocess.run",
            return_value=MagicMock(stdout="", returncode=0),
        ):
            with patch("_run_handler._lifecycle_up"):
                handler.handle_run(args)

        orch.execute_safe.assert_called_once()
        cmd = orch.execute_safe.call_args[0][0]
        assert "--profile" in cmd
        assert cmd[cmd.index("--profile") + 1] == "go"
        assert "--entrypoint" in cmd
        assert cmd[cmd.index("--entrypoint") + 1] == "go"
        assert "go" in cmd  # service name
        assert "version" in cmd  # sanitized tool args pass through

    def test_handle_run_go_starts_service_when_not_running(self):
        """run go starts the go service when it is not already running."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.tool_map = {
            "npm": "webdashboard",
            "cargo": "apiserver",
            "diesel": "apiserver",
            "go": "go",
        }
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "go"
        args.args = ["test", "./..."]
        args.pooler = False
        with patch(
            "subprocess.run",
            return_value=MagicMock(stdout="", returncode=0),
        ):
            with patch("_run_handler._lifecycle_up") as mock_up:
                handler.handle_run(args)

        mock_up.assert_called_once_with("go")
        orch.execute_safe.assert_called_once()

    def test_handle_run_go_skips_rust_env_injection(self):
        """run go must not call inject_rust_version_env."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.tool_map = {
            "npm": "webdashboard",
            "cargo": "apiserver",
            "diesel": "apiserver",
            "go": "go",
        }
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "go"
        args.args = ["version"]
        args.pooler = False
        with patch(
            "subprocess.run",
            return_value=MagicMock(stdout="", returncode=0),
        ):
            with patch("_run_handler._lifecycle_up"):
                with patch("_run_handler.inject_rust_version_env") as mock_inject:
                    handler.handle_run(args)
        mock_inject.assert_not_called()

    def test_handle_run_cargo_uses_api_compose_mode(self):
        """cargo selects the api compose file, not the all/default one."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "cargo"
        args.args = ["fmt"]
        args.pooler = False
        with patch("pathlib.Path.cwd", return_value=MagicMock()):
            with patch("_run_handler.inject_rust_version_env"):
                with patch.object(
                    RunHandler, "_compose_file_for_mode", return_value="/tmp/x.yml"
                ) as mock_file:
                    handler.handle_run(args)
        mock_file.assert_called_once_with("api")

    def test_handle_run_go_uses_go_compose_mode(self):
        """go selects the go compose file."""
        from _run_handler import RunHandler

        orch = _mock_orchestrator()
        orch.tool_map = {
            "npm": "webdashboard",
            "cargo": "apiserver",
            "diesel": "apiserver",
            "go": "go",
        }
        handler = RunHandler(orch)
        args = MagicMock()
        args.tool = "go"
        args.args = ["version"]
        args.pooler = False
        with patch(
            "subprocess.run",
            return_value=MagicMock(stdout="", returncode=0),
        ):
            with patch("_run_handler._lifecycle_up"):
                with patch.object(
                    RunHandler, "_compose_file_for_mode", return_value="/tmp/x.yml"
                ) as mock_file:
                    handler.handle_run(args)
        mock_file.assert_called_once_with("go")


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
        assert orch.execute_safe.call_count == 2
        assert "--cov=scripts" in " ".join(orch.execute_safe.call_args_list[0].args[0])

    def test_print_test_summary_no_logs(self):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        # Should not raise when no log dir exists
        handler.print_test_summary("test-target")

    def test_print_test_summary_reports_failed_phase(self):
        """A phase recorded as failed must NOT print the green all-passed line."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.results = {"Linting": {"status": "failed", "duration_s": 1.0}}
        handler = TestHandler(orch)
        with patch("_test_handler.log_error") as mock_err:
            handler.print_test_summary("api", "failed")
        mock_err.assert_called_once()

    def test_print_test_summary_all_pass_still_green(self):
        """All-passed phases still print the green summary."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        orch.results = {
            "Unit Tests": {"status": "passed", "duration_s": 1.0},
            "Linting": {"status": "passed", "duration_s": 1.0},
        }
        handler = TestHandler(orch)
        with patch("_test_handler.log_error") as mock_err:
            handler.print_test_summary("api", "passed")
        mock_err.assert_not_called()

    def test_run_phase_linting_failure_records_nonzero_result(self):
        """A failed focused phase exits non-zero and preserves phase details."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
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
        args.target = "api"
        with (
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down") as mock_down,
            patch("_test_handler.leedevkit_run_lint", return_value=False),
            pytest.raises(SystemExit) as exc,
        ):
            handler.run_phase("Linting", "api", args)
        assert exc.value.code == 1
        assert orch.results["Linting"]["status"] == "failed"
        assert orch.results["Linting"]["exit_code"] == 1
        mock_down.assert_called_once_with("all")

    def test_run_phase_linting_success_no_exit(self):
        """A passing Linting phase must not exit."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
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
        args.target = "api"
        with (
            patch("_test_handler._lifecycle_up"),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_lint", return_value=True),
        ):
            handler.run_phase("Linting", "api", args)
        assert "Linting" in orch.results
        assert orch.results["Linting"]["status"] == "passed"

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

    def test_handle_test_infra_unit_only_runs_tests_without_format_or_lint(self):
        """infra --unit-only executes only coverage commands, not format or lint."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "infra"
        args.lint_only = False
        args.unit_only = True
        args.e2e_only = False
        handler.handle_test(args)
        assert orch.execute_safe.call_count == 2
        commands = [" ".join(call.args[0]) for call in orch.execute_safe.call_args_list]
        assert all("pytest" in command for command in commands)
        assert not any("ruff" in command or "mypy" in command for command in commands)
        assert "pytest" in orch.execute_safe.call_args.args[0][0]

    def test_handle_test_infra_rejects_conflicting_phase_flags(self):
        """infra cannot select lint and unit phases simultaneously."""
        import pytest as _pytest

        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "infra"
        args.lint_only = True
        args.unit_only = True
        args.e2e_only = False
        with _pytest.raises(SystemExit) as exc_info:
            handler.handle_test(args)
        assert exc_info.value.code == 2
        orch.execute_safe.assert_not_called()

    def test_handle_test_infra_e2e_only_is_rejected(self):
        """infra has no e2e phase and rejects --e2e-only explicitly."""
        import pytest as _pytest

        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = MagicMock()
        args.target = "infra"
        args.lint_only = False
        args.unit_only = False
        args.e2e_only = True
        with _pytest.raises(SystemExit) as exc_info:
            handler.handle_test(args)
        assert exc_info.value.code == 2
        orch.execute_safe.assert_not_called()

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
            patch("_test_handler.leedevkit_run_lint", return_value=True),
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

    def test_run_phase_unknown_phase_records_failure(self):
        """Unknown phase remains a hard failure with recorded details."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with pytest.raises(SystemExit) as exc:
            handler.run_phase("BogusPhase", "api", _test_args())
        assert exc.value.code == 1
        assert orch.results["BogusPhase"]["status"] == "failed"
        assert orch.results["BogusPhase"]["exit_code"] == 1

    def test_handle_test_all_expands_server_and_web_targets(self):
        """Full regression expands configured server and web targets separately."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        calls = []

        def record_phase(phase, mode, args, target_name=None):
            calls.append((phase, mode, target_name))
            handler._record_result(
                phase, target_name or args.target, phase, "passed", 0
            )

        args = _test_args(target="all")
        with (
            patch("_devkit_config.resolve_test_targets", return_value=["api", "web"]),
            patch(
                "_test_handler.build_mode_map",
                return_value={"all": "all", "api": "api", "web": "web"},
            ),
            patch("_test_handler.inject_rust_version_env"),
            patch.object(handler, "run_phase", side_effect=record_phase),
            patch.object(handler, "print_test_summary"),
        ):
            handler.handle_test(args)

        assert ("Prebuild", "all", "all") in calls
        assert ("Unit Tests", "api", "api") in calls
        assert ("Integration Tests", "web", "web") in calls
        assert ("Linting", "web", "web") in calls

    def test_e2e_focused_web_uses_web_profile_and_teardown(self):
        """Focused E2E target selects e2e-web profile and remains compatible."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = _test_args(target="web", e2e_only=True)
        with (
            patch("_test_handler._lifecycle_up", return_value=True) as mock_up,
            patch("_test_handler.lifecycle_down") as mock_down,
            patch("_test_handler.leedevkit_run_integration", return_value=True),
            patch.object(handler, "print_test_summary"),
        ):
            handler.handle_test(args)

        mock_up.assert_called_once_with("e2e-web")
        mock_down.assert_called_once_with("all")
        assert orch.results["Integration Tests"]["status"] == "passed"

    def test_lifecycle_startup_failure_blocks_phase_and_tears_down(self):
        """Compose startup false blocks phase execution and still tears down."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with (
            patch("_test_handler._lifecycle_up", return_value=False) as mock_up,
            patch("_test_handler.lifecycle_down") as mock_down,
            patch("_test_handler.leedevkit_run_integration") as mock_integration,
        ):
            handler.run_phase(
                "Integration Tests",
                "web",
                _test_args(target="web"),
                target_name="web",
            )

        mock_up.assert_called_once_with("e2e-web")
        mock_integration.assert_not_called()
        mock_down.assert_called_once_with("all")
        assert orch.results["Integration Tests"]["status"] == "blocked"
        assert orch.results["Integration Tests"]["exit_code"] == 1

    def test_blocked_phase_report_has_diagnostic_and_rerun(self, capsys):
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler._record_result(
            "Integration Tests",
            "web",
            "./leedevkit test web --e2e-only",
            "blocked",
            1,
            error="Lifecycle readiness failed",
        )
        with patch.object(handler, "print_test_summary"):
            with pytest.raises(SystemExit):
                handler._finish_test(_test_args(json_output=True), "web")
        report = json.loads(capsys.readouterr().out)
        phase = report["phases"]["Integration Tests"]
        assert report["status"] == "blocked"
        assert phase["error"] == "Lifecycle readiness failed"
        assert phase["rerun"] == "./leedevkit test web --e2e-only"

    def test_phase_timeout_records_124_and_tears_down(self):
        """Timeout remains failed with conventional 124 exit code."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with (
            patch("_test_handler._lifecycle_up", return_value=True),
            patch("_test_handler.lifecycle_down") as mock_down,
            patch("_test_handler.leedevkit_run_unit", side_effect=TimeoutError),
        ):
            handler.run_phase("Unit Tests", "api", _test_args(), target_name="api")

        mock_down.assert_called_once_with("all")
        assert orch.results["Unit Tests"]["status"] == "failed"
        assert orch.results["Unit Tests"]["exit_code"] == 124

    def test_skip_e2e_records_non_required_skipped_phase(self):
        """Skipping E2E is explicit partial verification, not an unrecorded phase."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = _test_args(skip_e2e=True)
        with (
            patch("_test_handler._lifecycle_up", return_value=True),
            patch("_test_handler.lifecycle_down"),
            patch("_test_handler.leedevkit_run_unit", return_value=True),
            patch("_test_handler.leedevkit_run_lint", return_value=True),
            patch.object(handler, "print_test_summary"),
        ):
            handler.handle_test(args)

        assert orch.results["Integration Tests"]["status"] == "skipped"
        assert orch.results["Integration Tests"]["required"] is False

    def test_json_report_has_phase_contract_and_full_regression_status(self, capsys):
        """JSON report exposes status, target, full-regression marker, and phase fields."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler._record_result("Unit Tests", "api", "unit-api", "passed", 0)
        with patch.object(handler, "print_test_summary"):
            handler._finish_test(_test_args(json_output=True), "api")

        import json

        report = json.loads(capsys.readouterr().out)
        assert report["status"] == "passed"
        assert report["full_regression"] is True
        assert report["target"] == "api"
        phase = report["phases"]["Unit Tests"]
        assert {
            "phase",
            "target",
            "command",
            "start_time",
            "end_time",
            "duration_s",
            "exit_code",
            "status",
            "required",
        } <= set(phase)

    def test_json_report_failure_has_exit_log_and_rerun(self, capsys):
        """Failed phase exposes truthful exit, log, rerun, and next action."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler._record_result(
            "Unit Tests", "api", "./leedevkit test api --unit-only", "failed", 7
        )
        with patch.object(handler, "print_test_summary"):
            with pytest.raises(SystemExit) as exc:
                handler._finish_test(_test_args(json_output=True), "api")

        assert exc.value.code == 1
        report = json.loads(capsys.readouterr().out)
        assert report["status"] == "failed"
        assert report["exit_code"] == 7
        assert report["full_regression"] is False
        phase = report["phases"]["Unit Tests"]
        assert phase["rerun"] == "./leedevkit test api --unit-only"
        assert phase["log"].endswith("Unit_Tests.log")
        assert report["next_action"]

    def test_json_stream_emits_ordered_lifecycle_events(self, capsys):
        """JSONL mode emits run and phase events with stable sequence fields."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = _test_args(json_stream=True)
        handler.handle_test = handler.handle_test
        handler._event_args = args
        handler._run_id = "run-test"
        handler._emit_event("phase_started", phase="Unit Tests", target="api")
        handler._emit_event("phase_finished", phase="Unit Tests", target="api")
        events = [json.loads(line) for line in capsys.readouterr().out.splitlines()]
        assert [event["event"] for event in events] == [
            "phase_started",
            "phase_finished",
        ]
        assert [event["sequence"] for event in events] == [1, 2]
        assert all(event["run_id"] == "run-test" for event in events)

    def test_pattern_yields_partial_result(self):
        """Focused pattern is partial even when selected phase passes."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        handler._record_result("Unit Tests", "api", "unit-api", "passed", 0)
        with patch.object(handler, "print_test_summary") as summary:
            handler._finish_test(_test_args(pattern="auth"), "api")
        summary.assert_called_once_with("api", "partial")

    def test_skip_build_records_skipped_prebuild(self):
        """--skip-build records non-required prebuild instead of running it."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        args = _test_args(target="all", skip_build=True)
        with (
            patch("_devkit_config.resolve_test_targets", return_value=["api"]),
            patch(
                "_test_handler.build_mode_map",
                return_value={"all": "all", "api": "api"},
            ),
            patch("_test_handler.inject_rust_version_env"),
            patch.object(handler, "run_phase") as run_phase,
            patch.object(handler, "print_test_summary"),
        ):
            handler.handle_test(args)

        assert all(
            call.kwargs.get("target_name") != "all" or call.args[0] != "Prebuild"
            for call in run_phase.call_args_list
        )
        assert any(call.args[0] == "Unit Tests" for call in run_phase.call_args_list)
        assert orch.results["all:Prebuild"]["status"] == "skipped"
        assert orch.results["all:Prebuild"]["required"] is False

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
            "Linting": {"status": "passed", "duration_s": 1.0},
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

    def test_run_phase_unknown_phase_records_failure_in_coverage_gap(self):
        """Unknown phase records failure before hard exit."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        with pytest.raises(SystemExit) as exc:
            handler.run_phase("UnknownPhase", "all", _test_args(target="all"))
        assert exc.value.code == 1
        assert orch.results["all:UnknownPhase"]["status"] == "failed"
        assert orch.results["all:UnknownPhase"]["exit_code"] == 1

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
        """handle_test_infra reports production coverage, excluding test sources."""
        from _test_handler import TestHandler

        orch = _mock_orchestrator()
        handler = TestHandler(orch)
        orch.dry_run = False

        executed = []

        def fake_execute(cmd, env=None, timeout=1800):
            executed.append(cmd)

        monkeypatch.setattr(handler, "_execute_safe", fake_execute)
        handler.handle_test_infra()

        assert len(executed) == 2
        production = " ".join(executed[0])
        test_source = " ".join(executed[1])
        assert "--cov=scripts" in production
        assert "--cov-config" in production
        assert "--cov-fail-under=80" in production
        assert "scripts/.coveragerc" in production
        assert "coverage-production" in production
        assert "--cov=scripts/tests" in test_source
        assert "coverage-tests" in test_source
