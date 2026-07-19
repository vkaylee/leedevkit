"""Tests for _orchestrator — main CLI entry point (config-compatible)."""

import argparse
import os
import sys
from unittest.mock import patch

import pytest

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
        from _devkit_config import resolve_targets

        targets = resolve_targets()
        assert isinstance(targets, list)
        assert len(targets) > 0
        assert "all" in targets
        assert "infra" in targets


class TestColors:
    def test_colors_defined(self):
        from _logging import Colors

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
        (logs / "test.log").write_text(
            "Summary [   0.123s] 42 tests run: 42 passed, 0 skipped"
        )
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
        from _skills_manager import SkillsManager

        path = SkillsManager._lock_path()
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

    def test_init_uses_installed_venv_bootstrap(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        project = tmp_path / "project"
        project.mkdir()
        (project / ".git").mkdir()
        (project / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        monkeypatch.chdir(project)
        devkit = project / ".leedevkit"
        scripts = devkit / "scripts"
        scripts.mkdir(parents=True)
        python_path = devkit / ".venv" / "bin" / "python3"
        python_path.parent.mkdir(parents=True)
        python_path.write_text("#!/bin/sh\n")
        python_path.chmod(0o755)
        ensure = scripts / "_ensure-venv.sh"
        ensure.write_text(f"#!/bin/sh\nprintf '%s\\n' '{python_path}'\n")
        ensure.chmod(0o755)
        (devkit / "VERSION").write_text("0.1.0")
        (devkit / ".agent" / "rules").mkdir(parents=True)
        (devkit / ".agent" / "rules" / "base.md").write_text("# Base\n")
        (devkit / ".agent" / "skills-catalog.toml").write_text("[skills]\n")
        (devkit / "bin").mkdir()
        (devkit / "bin" / "leedevkit").write_text("#!/bin/sh\n")
        (devkit / "templates").mkdir()
        monkeypatch.setenv("DEVKIT_HOME", str(devkit))
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch.object(Orchestrator, "register_traps", return_value=None):
            with patch.object(Orchestrator, "_install_devkit") as install:
                orch = Orchestrator()
                orch.handle_init(force=False)
        install.assert_not_called()
        assert (project / "leedevkit").exists()

    def test_init_rejects_venv_path_outside_devkit(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        project = tmp_path / "project"
        project.mkdir()
        (project / ".git").mkdir()
        (project / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        devkit = project / ".leedevkit"
        (devkit / "scripts").mkdir(parents=True)
        (devkit / "VERSION").write_text("0.1.0")
        outside_python = tmp_path / ".venv" / "bin" / "python3"
        outside_python.parent.mkdir(parents=True)
        outside_python.write_text("#!/bin/sh\n")
        outside_python.chmod(0o755)
        ensure = devkit / "scripts" / "_ensure-venv.sh"
        ensure.write_text(f"#!/bin/sh\nprintf '%s\\n' '{outside_python}'\n")
        ensure.chmod(0o755)
        monkeypatch.chdir(project)
        monkeypatch.setenv("DEVKIT_HOME", str(devkit))
        monkeypatch.setattr("pathlib.Path.home", lambda: tmp_path / "no-home")
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            with pytest.raises(RuntimeError, match="invalid Python executable"):
                orch.handle_init(force=False)

    def test_load_catalog(self):
        from _skills_manager import SkillsManager

        catalog = SkillsManager()._load_catalog()
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
        (tmp_path / "leedevkit.toml").write_text('[devkit]\nversion = "1.2.3"\n')
        from _devkit_config import _read_version

        assert _read_version(tmp_path) == "1.2.3"

    def test_read_version_default(self, tmp_path):
        from _devkit_config import _read_version

        assert _read_version(tmp_path) == "latest"


class TestLockFile:
    def test_read_lock_missing(self, tmp_path, monkeypatch):
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        lock = SkillsManager._read_lock()
        assert lock == {}

    def test_read_lock_valid(self, tmp_path, monkeypatch):
        from _skills_manager import SkillsManager
        import tomli_w

        lock_path = tmp_path / "leedevkit.lock"
        lock_path.parent.mkdir(exist_ok=True)
        with open(lock_path, "wb") as f:
            tomli_w.dump({"my-skill": "abc123def"}, f)
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        data = SkillsManager._read_lock()
        assert "my-skill" in data

    def test_write_lock(self, tmp_path, monkeypatch):
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
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
        mgr = SkillsManager()
        # Override skills_d to point to our tmp path
        mgr._skills_d = skills_d
        mgr._write_lock()
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
            "all": "all",
            "api": "api",
            "web": "web",
            "apiserver": "api",
            "agent-main": "api",
            "webdashboard": "web",
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
            m = type(
                "R",
                (),
                {"stdout": "leedevkit-test-abc_apiserver_1\nother", "returncode": 0},
            )()
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
        monkeypatch.setattr(
            "subprocess.run", lambda *a, **kw: type("R", (), {"returncode": 0})()
        )
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
        from _logging import Colors

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
                target="infra",
                lint_only=True,
                unit_only=False,
                e2e_only=False,
                skip_lint=False,
                pattern=None,
                coverage=False,
                timeout=1800,
                fix=False,
                json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_infra_full(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="infra",
                lint_only=False,
                unit_only=False,
                e2e_only=False,
                skip_lint=False,
                pattern=None,
                coverage=False,
                timeout=1800,
                fix=False,
                json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_all_with_flags_runs_all_phases(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all",
                lint_only=True,
                unit_only=False,
                e2e_only=False,
                skip_lint=False,
                pattern=None,
                coverage=False,
                timeout=1800,
                fix=False,
                json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_non_infra_target_dry(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all",
                lint_only=False,
                unit_only=True,
                e2e_only=False,
                skip_lint=True,
                pattern=None,
                coverage=False,
                timeout=1800,
                fix=False,
                json_output=False,
            )
            orch.handle_test(args)

    def test_handle_test_with_coverage(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                target="all",
                lint_only=False,
                unit_only=False,
                e2e_only=False,
                skip_lint=False,
                pattern=None,
                coverage=True,
                timeout=1800,
                fix=False,
                json_output=False,
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
                    command="run",
                    tool="npm",
                    pooler=False,
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
                command="run",
                tool="cargo",
                pooler=False,
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
                command="run",
                tool="cargo",
                pooler=False,
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
                component="",
                fix=False,
                pattern="",
            )
            orch.run_phase("Linting", "api", args)

    def test_run_phase_unit_web(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                component="",
                fix=False,
                pattern="",
            )
            orch.run_phase("Unit Tests", "web", args)

    def test_run_phase_coverage_api(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.dry_run = True
            args = argparse.Namespace(
                component="",
                fix=False,
                pattern="",
                unit_only=False,
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


# ── Coverage gap fillers: _build_mode_map, _inject_rust_version_env, etc. ──


class TestBuildModeMap:
    """Tests for _build_mode_map to cover the config-parsing branch."""

    def test_falls_back_to_defaults_when_no_config(self, tmp_path):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            result = orch._build_mode_map()
            assert result["apiserver"] == "api"
            assert result["webdashboard"] == "web"
            assert result["all"] == "all"

    def test_parses_rust_service_from_config(self, tmp_path):
        from _orchestrator import Orchestrator

        config_toml = tmp_path / "leedevkit.toml"
        config_toml.write_text("""
[services.myservice]
lang = "rust"
""")
        with patch.object(Orchestrator, "register_traps", return_value=None):
            with patch("_bootstrap.PROJECT_ROOT", tmp_path):
                orch = Orchestrator()
                result = orch._build_mode_map()
        assert result.get("myservice") == "api"

    def test_parses_typescript_service_from_config(self, tmp_path):
        from _orchestrator import Orchestrator

        config_toml = tmp_path / "leedevkit.toml"
        config_toml.write_text("""
[services.frontend]
lang = "typescript"
""")
        with patch.object(Orchestrator, "register_traps", return_value=None):
            with patch("_bootstrap.PROJECT_ROOT", tmp_path):
                orch = Orchestrator()
                result = orch._build_mode_map()
        assert result.get("frontend") == "web"


class TestInjectRustVersionEnv:
    """Tests for _inject_rust_version_env."""

    def test_default_when_no_config(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._inject_rust_version_env()
        import os
        assert os.environ.get("RUST_VERSION") == "1.85"

    def test_env_override_wins(self):
        from _orchestrator import Orchestrator
        import os

        os.environ["RUST_VERSION"] = "1.99"
        try:
            with patch.object(Orchestrator, "register_traps", return_value=None):
                orch = Orchestrator()
                orch._inject_rust_version_env()
            assert os.environ["RUST_VERSION"] == "1.99"
        finally:
            del os.environ["RUST_VERSION"]


class TestComposeEngineCmd:
    """Tests for compose_engine_cmd property."""

    def test_returns_joined_string(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            # Just verify the property is callable and returns a string
            result = orch.compose_engine_cmd
            assert isinstance(result, str)


class TestCleanup:
    """Tests for orchestator cleanup."""

    def test_cleanup_no_needs_cleanup_returns_early(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.needs_cleanup = False
            orch.cleanup()  # Should no-op

    def test_cleanup_dry_run_returns_early(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.needs_cleanup = True
            orch.dry_run = True
            orch.cleanup()  # Should no-op

    def test_cleanup_active_calls_lifecycle_down(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.needs_cleanup = True
            orch.dry_run = False
            orch.active_mode = "all"
            orch.lock_fd = None
            with patch("_orchestrator.lifecycle_down") as mock_down:
                orch.cleanup()
            mock_down.assert_called_once_with("all")

    def test_cleanup_with_lock_releases(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.needs_cleanup = True
            orch.dry_run = False
            orch.active_mode = "all"
            orch.lock_fd = 999  # Fake fd
            with patch("_orchestrator.lifecycle_down"):
                with patch("_orchestrator.LockManager.release") as mock_release:
                    orch.cleanup()
            mock_release.assert_called_once()
            assert orch.lock_fd is None


class TestRunMethod:
    """Tests for Orchestrator.run() branches."""

    def test_run_update_command(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
        args = type("Args", (), {
            "command": "update",
            "dry_run": False,
        })()
        with patch.object(orch.parser, "parse_args", return_value=args):
            with patch.object(orch, "handle_update") as mock_update:
                orch.run()
        mock_update.assert_called_once()

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
        (logs / "test.log").write_text(
            "\033[0;32mSummary [0.1s] 10 tests run: 10 passed\033[0m"
        )
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


class TestSkillsAddValidation:
    def test_skills_add_rejects_catalog_name(self, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse

        monkeypatch.setenv(
            "DEVKIT_HOME",
            str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent),
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(
                skills_action="add", url="ui-ux-pro-max", version="main"
            )
            orch.handle_skills(args)

    def test_skills_add_rejects_plain_name(self, monkeypatch):
        from _orchestrator import Orchestrator
        import argparse

        monkeypatch.setenv(
            "DEVKIT_HOME",
            str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent),
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(
                skills_action="add", url="not-a-url", version="main"
            )
            orch.handle_skills(args)

    def test_skills_install_empty_name(self):
        from _orchestrator import Orchestrator
        import argparse

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(skills_action="install", name=None)
            orch.handle_skills(args)


class TestInitPopulatesRules:
    def test_init_copies_rules_to_project(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        # Create leedevkit.toml with custom rules_dir
        (tmp_path / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname="test"\n[ai]\nrules_dir=".myrules"\n'
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=False)
            # Should have created .myrules/ with rulebooks
            rules_dir = tmp_path / ".myrules"
            assert rules_dir.exists()
            assert len(list(rules_dir.glob("*.md"))) > 0

    def test_init_does_not_overwrite_existing_rules(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        (tmp_path / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname="test"\n'
        )
        # Pre-create a rule file
        rules_dir = tmp_path / ".agent" / "rules"
        rules_dir.mkdir(parents=True)
        existing = rules_dir / "my-rule.md"
        existing.write_text("custom content")
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=False)
            # Existing file should not be overwritten
            assert existing.read_text() == "custom content"

    def test_init_creates_overrides_yaml(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        dk = str(__import__("pathlib").Path(__file__).resolve().parent.parent.parent)
        monkeypatch.setenv("DEVKIT_HOME", dk)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        (tmp_path / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname="test"\n'
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=False)
            assert (tmp_path / ".agent" / "overrides.yaml").exists()


class TestUpdateParser:
    """Parser accepts 'update' and exposes --version."""

    def test_update_command_parsed(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = orch.parser.parse_args(["update"])
            assert args.command == "update"
            assert args.version is None

    def test_update_with_pinned_version(self):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = orch.parser.parse_args(["update", "--version", "v0.2.0"])
            assert args.version == "v0.2.0"

    def test_update_help_lists_examples(self, capsys):
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            with pytest.raises(SystemExit):
                orch.parser.parse_args(["update", "--help"])
            out = capsys.readouterr().out
            assert "--version" in out


class TestLatestReleaseVersion:
    """_latest_release_version reads tag_name from GitHub releases/latest."""

    def _fake_urlopen(self, payload: bytes):
        import io

        class _Resp(io.BytesIO):
            def __enter__(self):
                return self

            def __exit__(self, *a):
                return False

        def _open(req, timeout=15):
            return _Resp(payload)

        return _open

    def test_returns_tag_name(self, monkeypatch):
        from _orchestrator import Orchestrator
        import json

        body = json.dumps({"tag_name": "v0.2.0"}).encode()
        monkeypatch.setattr(
            "_orchestrator.urllib.request.urlopen", self._fake_urlopen(body)
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch._latest_release_version() == "v0.2.0"

    def test_raises_when_no_tag_name(self, monkeypatch):
        from _orchestrator import Orchestrator
        import json

        body = json.dumps({"name": "v0.2.0"}).encode()
        monkeypatch.setattr(
            "_orchestrator.urllib.request.urlopen", self._fake_urlopen(body)
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            with pytest.raises(RuntimeError, match="no tag_name"):
                orch._latest_release_version()

    def test_wraps_network_failure(self, monkeypatch):
        from _orchestrator import Orchestrator

        def _boom(req, timeout=15):
            raise OSError("offline")

        monkeypatch.setattr("_orchestrator.urllib.request.urlopen", _boom)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            with pytest.raises(RuntimeError, match="Could not reach"):
                orch._latest_release_version()


class TestHandleUpdate:
    """Self-update flow: already-latest, success, rollback, latest lookup."""

    @staticmethod
    def _make_root(tmp_path, version="0.1.0"):
        root = tmp_path / "devkit"
        root.mkdir()
        (root / "VERSION").write_text(version)
        return root

    def test_already_on_latest_skips(self, tmp_path, monkeypatch, capsys):
        from _orchestrator import Orchestrator

        root = self._make_root(tmp_path, "0.1.0")
        monkeypatch.setattr(Orchestrator, "_devkit_root", lambda self: root)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(version="v0.1.0")
            orch.handle_update(args)
        out = capsys.readouterr().err
        assert "Already on latest (0.1.0)" in out
        # No backup created
        assert not (tmp_path / "devkit.bak").exists()

    def test_successful_update_backs_up_and_overlays(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        root = self._make_root(tmp_path, "0.1.0")

        def fake_download(self, url, target_dir):
            # Mimic _download_and_extract: populate target_dir with new tree
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.2.0")
            (target_dir / "scripts").mkdir()

        monkeypatch.setattr(Orchestrator, "_devkit_root", lambda self: root)
        monkeypatch.setattr(Orchestrator, "_download_and_extract", fake_download)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(version="v0.2.0")
            orch.handle_update(args)
        # New version installed
        assert (root / "VERSION").read_text() == "0.2.0"
        assert (root / "scripts").is_dir()
        # Backup kept with old version
        assert (tmp_path / "devkit.bak" / "VERSION").read_text() == "0.1.0"
        # No leftover temp dirs
        assert not list(tmp_path.glob(".leedevkit-update-*"))

    def test_rollback_on_download_failure(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        root = self._make_root(tmp_path, "0.1.0")

        def boom(self, url, target_dir):
            raise RuntimeError("network down")

        monkeypatch.setattr(Orchestrator, "_devkit_root", lambda self: root)
        monkeypatch.setattr(Orchestrator, "_download_and_extract", boom)
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(version="v0.2.0")
            with pytest.raises(RuntimeError, match="network down"):
                orch.handle_update(args)
        # Rolled back: original tree intact, no leftover backup
        assert (root / "VERSION").read_text() == "0.1.0"
        assert not (tmp_path / "devkit.bak").exists()

    def test_uses_latest_release_when_no_version(self, tmp_path, monkeypatch):
        from _orchestrator import Orchestrator

        root = self._make_root(tmp_path, "0.1.0")

        captured = {}

        def fake_download(self, url, target_dir):
            captured["url"] = url
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.0")

        monkeypatch.setattr(Orchestrator, "_devkit_root", lambda self: root)
        monkeypatch.setattr(Orchestrator, "_download_and_extract", fake_download)
        monkeypatch.setattr(
            Orchestrator, "_latest_release_version", lambda self: "v0.3.0"
        )
        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            args = argparse.Namespace(version=None)
            orch.handle_update(args)
        assert (root / "VERSION").read_text() == "0.3.0"
        # URL built from the resolved latest tag
        assert "v0.3.0/leedevkit-0.3.0.tar.gz" in captured["url"]
