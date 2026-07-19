"""Tests for general CLI usage — integration with orchestrator."""

import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))


class TestCliFastPaths:
    @staticmethod
    def _make_artifact(tmp_path):
        root = tmp_path / "artifact"
        (root / "bin").mkdir(parents=True)
        (root / "scripts").mkdir()
        source = Path(__file__).resolve().parents[2] / "bin" / "leedevkit"
        cli = root / "bin" / "leedevkit"
        cli.write_bytes(source.read_bytes())
        cli.chmod(0o755)
        (root / "VERSION").write_text("9.8.7\n")
        ensure = root / "scripts" / "_ensure-venv.sh"
        ensure.write_text('#!/bin/bash\ntouch "$(dirname "$0")/CALLED"\nexit 99\n')
        ensure.chmod(0o755)
        return root, cli

    def test_identity_and_help_do_not_initialize_venv(self, tmp_path):
        root, cli = self._make_artifact(tmp_path)

        for args in ([], ["version"], ["--version"], ["--help"]):
            result = subprocess.run(
                [str(cli), *args], capture_output=True, text=True, timeout=10
            )
            assert result.returncode == 0

        assert not (root / "scripts" / "CALLED").exists()
        assert not (root / ".venv").exists()

    def test_unknown_command_does_not_initialize_venv(self, tmp_path):
        root, cli = self._make_artifact(tmp_path)

        result = subprocess.run(
            [str(cli), "not-a-command"], capture_output=True, text=True, timeout=10
        )

        assert result.returncode == 1
        assert "Unknown command" in result.stdout
        assert not (root / "scripts" / "CALLED").exists()

    def test_python_command_initializes_venv(self, tmp_path):
        root, cli = self._make_artifact(tmp_path)

        result = subprocess.run(
            [str(cli), "test"], capture_output=True, text=True, timeout=10
        )

        assert result.returncode != 0
        assert (root / "scripts" / "CALLED").exists()


class TestCliHelp:
    def test_test_help(self):
        orch = Path(__file__).resolve().parent.parent / "_orchestrator.py"
        r = subprocess.run(
            [sys.executable, str(orch), "test", "--help"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode == 0

    def test_manage_help(self):
        orch = Path(__file__).resolve().parent.parent / "_orchestrator.py"
        r = subprocess.run(
            [sys.executable, str(orch), "manage", "--help"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode == 0


class TestInitFlow:
    def test_init_dry_run(self, tmp_path, monkeypatch):
        orch = Path(__file__).resolve().parent.parent / "_orchestrator.py"
        monkeypatch.chdir(tmp_path)
        dk = str(Path(__file__).resolve().parent.parent.parent)
        r = subprocess.run(
            [sys.executable, str(orch), "manage", "init", "--dry-run"],
            capture_output=True,
            text=True,
            timeout=30,
            env={**os.environ, "DEVKIT_HOME": dk},
        )
        assert r.returncode == 0

    def test_init_creates_files(self, tmp_path, monkeypatch):
        orch = Path(__file__).resolve().parent.parent / "_orchestrator.py"
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        dk = str(Path(__file__).resolve().parent.parent.parent)
        subprocess.run(
            [sys.executable, str(orch), "manage", "init"],
            capture_output=True,
            text=True,
            timeout=30,
            env={**os.environ, "DEVKIT_HOME": dk},
        )
        assert (tmp_path / "leedevkit").exists()


class TestDoctor:
    def test_doctor_runs(self, tmp_path, monkeypatch):
        orch = Path(__file__).resolve().parent.parent / "_orchestrator.py"
        (tmp_path / "leedevkit.toml").write_text("[project]\nname = 'test'")
        monkeypatch.chdir(tmp_path)
        dk = str(Path(__file__).resolve().parent.parent.parent)
        r = subprocess.run(
            [sys.executable, str(orch), "manage", "doctor"],
            capture_output=True,
            text=True,
            timeout=10,
            env={**os.environ, "DEVKIT_HOME": dk},
        )
        assert r.returncode == 0


class TestUpdateCli:
    """Subprocess smoke tests for the 'update' subcommand.

    Note: the destructive/rollback update paths are covered by the in-process
    tests in test_orchestrator.py (which redirect _devkit_root to tmp_path), so
    these subprocess checks stick to non-mutating cases.
    """

    @staticmethod
    def _orch():
        return Path(__file__).resolve().parent.parent / "_orchestrator.py"

    @staticmethod
    def _repo_version():
        return (
            (Path(__file__).resolve().parent.parent.parent / "VERSION")
            .read_text()
            .strip()
        )

    def test_update_help(self):
        r = subprocess.run(
            [sys.executable, str(self._orch()), "update", "--help"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode == 0
        assert "--version" in r.stdout

    def test_update_already_latest(self):
        version = self._repo_version()
        r = subprocess.run(
            [sys.executable, str(self._orch()), "update", "--version", f"v{version}"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        assert r.returncode == 0
        assert f"Already on latest ({version})" in r.stderr

    def test_bin_leedevkit_routes_update(self):
        """bin/leedevkit forwards the update subcommand to the orchestrator."""
        bin_path = Path(__file__).resolve().parent.parent.parent / "bin" / "leedevkit"
        r = subprocess.run(
            [str(bin_path), "update", "--help"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        assert r.returncode == 0
        assert "--version" in r.stdout


class TestConfigLoading:
    def test_config_loads(self, tmp_path, monkeypatch):
        (tmp_path / "leedevkit.toml").write_text("""
[project]
name = "TestProject"
languages = ["python"]
""")
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv(
            "DEVKIT_HOME", str(Path(__file__).resolve().parent.parent.parent)
        )
        from _devkit_config import load_project_config
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        cfg = load_project_config()
        assert cfg["project"]["name"] == "TestProject"


class TestSafeRunIntegration:
    def test_echo(self):
        sr = Path(__file__).resolve().parent.parent / "_safe_run.py"
        r = subprocess.run(
            [sys.executable, str(sr), "10", "echo", "hello"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode == 0

    def test_false(self):
        sr = Path(__file__).resolve().parent.parent / "_safe_run.py"
        r = subprocess.run(
            [sys.executable, str(sr), "10", "false"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode != 0

    def test_timeout(self):
        sr = Path(__file__).resolve().parent.parent / "_safe_run.py"
        r = subprocess.run(
            [sys.executable, str(sr), "1", "sleep", "30"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert r.returncode != 0

    def test_requires_timeout_arg(self):
        sr = Path(__file__).resolve().parent.parent / "_safe_run.py"
        r = subprocess.run(
            [sys.executable, str(sr)], capture_output=True, text=True, timeout=5
        )
        assert r.returncode == 1
