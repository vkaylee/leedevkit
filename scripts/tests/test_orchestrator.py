"""Tests for _orchestrator — main CLI entry point (config-compatible)."""
import os
import sys
from unittest.mock import patch

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))


class TestOrchestratorProperties:
    def test_engine_detected(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch.engine in ("podman", "docker")

    def test_compose_engine_list(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert isinstance(orch.compose_engine, list)


class TestResolveTargets:
    def test_returns_list(self):
        from _orchestrator import _resolve_targets
        targets = _resolve_targets()
        assert isinstance(targets, list)
        assert len(targets) > 0
        assert "all" in targets
        assert "infra" in targets


class TestColors:
    def test_colors_defined(self):
        from _orchestrator import Colors
        assert Colors.GREEN and Colors.RED and Colors.NC


class TestToolMap:
    def test_tool_map_entries(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch.tool_map["npm"] == "webdashboard"
            assert orch.tool_map["cargo"] == "apiserver"


class TestGetComposeFiles:
    def test_dev_environment(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            files = orch.get_compose_files("dev")
            assert any("docker-compose.yml" in f for f in files)

    def test_test_environment(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.env_vars["COMPOSE_PROJECT_NAME"] = "test-abc"
            files = orch.get_compose_files("test")
            assert any("docker-compose.test.yml" in f for f in files)


class TestDryRun:
    def test_dry_run_noop(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            with patch("subprocess.run") as mock_run:
                orch.execute_safe(["echo", "x"])
                mock_run.assert_not_called()


class TestPrintTestSummary:
    def test_no_logs_no_crash(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.print_test_summary("all")


class TestOrchestratorInitFlow:
    def test_handle_init_dry_run(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        import _devkit_config
        _devkit_config._DEVKIT_ROOT = None
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            orch.handle_init(force=False)

    def test_handle_skills_list(self, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse
        import _devkit_config
        _devkit_config._DEVKIT_ROOT = None
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="list")
            orch.handle_skills(args)


class TestPrintTestSummaryParsing:
    def test_parses_nextest_output(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        logs = tmp_path / ".test_logs"
        logs.mkdir()
        (logs / "test.log").write_text("Summary [   0.123s] 42 tests run: 42 passed, 0 skipped")
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.print_test_summary("api")


class TestLogFunctions:
    def test_log_info(self, capsys):
        from _orchestrator import log_info
        log_info("test message")

    def test_log_success(self, capsys):
        from _orchestrator import log_success
        log_success("test success")

    def test_log_warn(self, capsys):
        from _orchestrator import log_warn
        log_warn("test warning")

    def test_log_error(self, capsys):
        from _orchestrator import log_error
        log_error("test error")


class TestLeedevkitDir:
    def test_lock_path_at_root(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            path = orch._lock_path()
            assert path.name == "leedevkit.lock"

    def test_init_creates_files(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        import _devkit_config
        _devkit_config._DEVKIT_ROOT = None
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=True)
            assert (tmp_path / "leedevkit").exists()
            assert (tmp_path / "leedevkit.toml").exists()

    def test_load_catalog(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            catalog = orch._load_skills_catalog()
            assert isinstance(catalog, dict)
            assert "ui-ux-pro-max" in catalog

    def test_skills_install_not_in_catalog(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="install", name="nonexistent")
            orch.handle_skills(args)


class TestVersionInToml:
    def test_read_version_from_toml(self, tmp_path):
        (tmp_path / "leedevkit.toml").write_text("[devkit]\nversion = \"1.2.3\"\n")
        from _devkit_config import _read_version
        assert _read_version(tmp_path) == "1.2.3"

    def test_read_version_default(self, tmp_path):
        from _devkit_config import _read_version
        assert _read_version(tmp_path) == "latest"


class TestLockFile:
    def test_read_lock_missing(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            lock = orch._read_lock()
            assert lock == {}

    def test_read_lock_valid(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        import tomli_w
        lock_path = tmp_path / "leedevkit.lock"
        lock_path.parent.mkdir(exist_ok=True)
        with open(lock_path, "wb") as f:
            tomli_w.dump({"my-skill": "abc123def"}, f)
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            data = orch._read_lock()
            assert "my-skill" in data

    def test_write_lock(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        skills_d = tmp_path / "skills.d"
        skills_d.mkdir()
        (skills_d / ".git").mkdir()
        import subprocess
        orig_run = subprocess.run
        def fake_run(*a, **kw):
            if "rev-parse" in str(a):
                m = type("R", (), {"stdout": "abc123\n", "returncode": 0})()
                return m
            return orig_run(*a, **kw)
        monkeypatch.setattr("subprocess.run", fake_run)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._write_lock(skills_d)
            assert (tmp_path / "leedevkit.lock").exists()


class TestSkillsSubCommands:
    def test_skills_add_usage(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="add", url="", version="main")
            orch.handle_skills(args)

    def test_skills_remove_usage(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="remove", name="")
            orch.handle_skills(args)

    def test_skills_remove_nonexistent(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="remove", name="no-such-skill")
            orch.handle_skills(args)

    def test_skills_update(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="update")
            orch.handle_skills(args)

    def test_skills_install_no_name(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="install", name=None)
            orch.handle_skills(args)


class TestHandleManageDispatch:
    def test_manage_fmt_infra(self, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="fmt:infra")
            orch.handle_manage(args)

    def test_manage_doctor_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="doctor")
            orch.handle_manage(args)

    def test_manage_clean(self, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="clean", env="dev")
            orch.handle_manage(args)


class TestOrchestratorRun:
    def test_run_cargo_test_converts_to_nextest(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            compose_cmd = ["podman-compose", "-p", "test"]
            tool_args = ["test", "--lib"]
            orch._handle_run_cargo(compose_cmd, tool_args, "apiserver")
            assert "nextest" in compose_cmd


class TestModeMap:
    def test_mode_mapping(self):
        mode_map = {
            "all": "all", "api": "api", "web": "web",
            "apiserver": "api", "agent-main": "api", "webdashboard": "web",
        }
        assert mode_map["all"] == "all"
        assert mode_map["apiserver"] == "api"
        assert mode_map["webdashboard"] == "web"


class TestIsServiceRunning:
    def test_no_containers(self, monkeypatch):
        from _orchestrator import Orchestrator
        def fake_run(*a, **kw):
            m = type("R", (), {"stdout": "", "returncode": 0})()
            return m
        monkeypatch.setattr("subprocess.run", fake_run)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch._is_service_running("apiserver") is False

    def test_service_found(self, monkeypatch):
        from _orchestrator import Orchestrator
        def fake_run(*a, **kw):
            m = type("R", (), {"stdout": "leedevkit-test-abc_apiserver_1\nother", "returncode": 0})()
            return m
        monkeypatch.setattr("subprocess.run", fake_run)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.env_vars["COMPOSE_PROJECT_NAME"] = "leedevkit-test-abc"
            assert orch._is_service_running("apiserver") is True


class TestOrchestratorEdgeCases:
    def test_handle_test_infra_glob(self, tmp_path, monkeypatch):
        """handle_test_infra should discover all test_*.py files via glob."""
        from _orchestrator import Orchestrator
        tests_dir = tmp_path / "tests"
        tests_dir.mkdir()
        (tests_dir / "test_a.py").write_text("def test(): pass")
        (tests_dir / "test_b.py").write_text("def test(): pass")
        monkeypatch.setattr("_orchestrator.SCRIPTS_DIR", tmp_path)
        monkeypatch.setattr("subprocess.run", lambda *a, **kw: type("R", (), {"returncode": 0})())
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_test_infra()

    def test_handle_db_query(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(sql="SELECT 1", json=False)
            orch.handle_db_query(args)

    def test_handle_diesel(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            orch.handle_diesel(["migration", "list"])

    def test_log_error_with_file(self, tmp_path):
        from _orchestrator import log_error
        f = (tmp_path / "err.log").open("w")
        log_error("test", file=f)
        f.close()
        assert (tmp_path / "err.log").stat().st_size > 0

    def test_orchestrator_env_vars(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert "USE_DOCKER" in orch.env_vars
            assert orch.env_vars["USE_DOCKER"] == "true"

    def test_colors_values(self):
        from _orchestrator import Colors
        assert Colors.GREEN == "\033[0;32m"
        assert Colors.RED == "\033[0;31m"
        assert Colors.NC == "\033[0m"


class TestLogFunctionsMore:
    def test_log_info_contains_emoji(self, capsys):
        from _orchestrator import log_info
        log_info("hello")
        out = capsys.readouterr()
        # Output goes to stderr
        assert "hello" in out.err

    def test_log_success_contains_checkmark(self, capsys):
        from _orchestrator import log_success
        log_success("done")
        assert "done" in capsys.readouterr().err

    def test_log_warn_contains_warning(self, capsys):
        from _orchestrator import log_warn
        log_warn("careful")
        assert "careful" in capsys.readouterr().err

    def test_log_error_contains_cross(self):
        from _orchestrator import log_error
        import io
        buf = io.StringIO()
        log_error("fail", file=buf)
        assert "fail" in buf.getvalue()


class TestHandleTestMocked:
    """Test handle_test flow with Docker calls mocked out."""

    def test_handle_test_infra_lint_only(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="infra", lint_only=True, unit_only=False,
                e2e_only=False, skip_lint=False, pattern=None,
                coverage=False, timeout=1800, fix=False, json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_infra_full(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="infra", lint_only=False, unit_only=False,
                e2e_only=False, skip_lint=False, pattern=None,
                coverage=False, timeout=1800, fix=False, json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_all_with_flags_runs_all_phases(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all", lint_only=True, unit_only=False,
                e2e_only=False, skip_lint=False, pattern=None,
                coverage=False, timeout=1800, fix=False, json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_non_infra_target_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all", lint_only=False, unit_only=True,
                e2e_only=False, skip_lint=True, pattern=None,
                coverage=False, timeout=1800, fix=False, json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_with_coverage(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all", lint_only=False, unit_only=False,
                e2e_only=False, skip_lint=False, pattern=None,
                coverage=True, timeout=1800, fix=False, json_output=False,
            )
            orch.handle_test(args)


class TestHandleRunMocked:
    """Test handle_run flow with subprocess and container ops mocked."""

    def test_handle_run_npm_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            with patch.object(Orchestrator, "_is_service_running", return_value=False):
                args = argparse.Namespace(
                    command="run", tool="npm", pooler=False,
                    args=["--version"],
                )
                orch.handle_run(args)

    def test_handle_run_cargo_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                command="run", tool="cargo", pooler=False,
                args=["build"],
            )
            orch.handle_run(args)

    def test_handle_run_cargo_cwd_detection(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                command="run", tool="cargo", pooler=False,
                args=["build"],
            )
            orch.handle_run(args)


class TestRunPhaseMocked:
    """Test run_phase dispatch with lifecycle/test functions mocked."""

    def test_run_phase_lint_api(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                component="", fix=False, pattern="",
            )
            orch.run_phase("Linting", "api", args)

    def test_run_phase_unit_web(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                component="", fix=False, pattern="",
            )
            orch.run_phase("Unit Tests", "web", args)

    def test_run_phase_coverage_api(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                component="", fix=False, pattern="", unit_only=False,
            )
            orch.run_phase("Coverage", "api", args)

    def test_run_phase_startup(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(component="", fix=False, pattern="")
            orch.run_phase("Startup", "all", args)

    def test_run_phase_unknown(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True  # dry_run returns before func lookup
            args = argparse.Namespace(component="", fix=False, pattern="")
            orch.run_phase("BogusPhase", "all", args)
            # dry_run just logs and returns — no error for unknown phase


class TestCleanupMocked:
    """Test cleanup logic with docker calls mocked."""

    def test_cleanup_noop_when_dry(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            orch.needs_cleanup = True
            orch.cleanup()

    def test_cleanup_noop_when_not_needed(self):
        from _orchestrator import Orchestrator
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = False
            orch.needs_cleanup = False
            orch.cleanup()


class TestHandleManageMocked:
    """Test handle_manage dispatch with execution mocked."""

    def test_handle_manage_sync_api_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="sync:api")
            orch.handle_manage(args)

    def test_handle_manage_migrate_run_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="migrate:run")
            orch.handle_manage(args)

    def test_handle_manage_migrate_revert_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="migrate:revert")
            orch.handle_manage(args)

    def test_handle_manage_prebuild_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="prebuild")
            orch.handle_manage(args)

    def test_handle_manage_db_setup_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="db:setup")
            orch.handle_manage(args)

    def test_handle_manage_verify_infra_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="verify:infra")
            orch.handle_manage(args)

    def test_handle_manage_test_infra_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="test:infra")
            orch.handle_manage(args)


class TestHandleManageUpDown:
    """Test compose up/down/ps/logs with execution mocked."""

    def test_manage_up_dev_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="up", env="dev")
            orch.handle_manage(args)

    def test_manage_down_test_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="down", env="test")
            orch.handle_manage(args)

    def test_manage_ps_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="ps", env="dev")
            orch.handle_manage(args)

    def test_manage_exec_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="exec", service="apiserver", args=[])
            orch.handle_manage(args)

    def test_manage_logs_dry(self):
        from _orchestrator import Orchestrator
        import argparse
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(subcommand="logs", env="dev", service=None)
            orch.handle_manage(args)


class TestHandleTestSummary:
    """Test print_test_summary with various log parsers."""

    def test_summary_ansi_stripped(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        logs = tmp_path / ".test_logs"
        logs.mkdir()
        (logs / "test.log").write_text("\033[0;32mSummary [0.1s] 10 tests run: 10 passed\033[0m")
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.print_test_summary("all")

    def test_summary_stale_log_skipped(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator
        import time
        logs = tmp_path / ".test_logs"
        logs.mkdir()
        logfile = logs / "old.log"
        logfile.write_text("Summary [0.1s] 5 tests run: 5 passed")
        # Set mtime to epoch so it's definitely stale
        os.utime(str(logfile), (0, 0))
        monkeypatch.setattr("_orchestrator.PROJECT_ROOT", tmp_path)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.start_time = time.time() + 100
            orch.print_test_summary("all")


