"""Tests for general CLI usage — integration with orchestrator."""

import os
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))


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
            Path(__file__).resolve().parent.parent.parent / "VERSION"
        ).read_text().strip()

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
        from _devkit_config import load_project_config

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
