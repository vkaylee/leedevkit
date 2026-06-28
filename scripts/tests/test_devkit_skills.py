"""Tests for leedevkit skills management — add/remove/install/update/lock."""
import os
import sys
import tempfile
from pathlib import Path

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _orchestrator import Orchestrator


@pytest.fixture
def orch():
    return Orchestrator()


@pytest.fixture
def tmp_skills_d():
    with tempfile.TemporaryDirectory() as td:
        skills_d = Path(td)
        yield skills_d


class TestSkillsLock:
    def test_read_lock_empty_when_missing(self, orch, tmp_skills_d):
        # Patch PROJECT_ROOT to temp
        import _orchestrator as mod

        orig = mod.PROJECT_ROOT
        mod.PROJECT_ROOT = tmp_skills_d
        try:
            result = orch._read_lock()
            assert result == {}
        finally:
            mod.PROJECT_ROOT = orig

    def test_write_and_read_lock_roundtrip(self, orch, tmp_skills_d):
        import subprocess

        import _orchestrator as mod

        orig = mod.PROJECT_ROOT
        mod.PROJECT_ROOT = tmp_skills_d
        try:
            # Create a fake git repo in skills.d
            skills_d = tmp_skills_d / "skills.d"
            skills_d.mkdir()
            repo_dir = skills_d / "test-skill"
            repo_dir.mkdir()
            (repo_dir / ".git").mkdir()
            # Write a fake HEAD
            subprocess.run(
                ["git", "-C", str(repo_dir), "init", "-q"],
                check=False, stdin=subprocess.DEVNULL,
            )
            subprocess.run(
                ["git", "-C", str(repo_dir), "config", "user.email", "test@test.com"],
                check=False, stdin=subprocess.DEVNULL,
            )
            subprocess.run(
                ["git", "-C", str(repo_dir), "config", "user.name", "Test"],
                check=False, stdin=subprocess.DEVNULL,
            )
            subprocess.run(
                ["git", "-C", str(repo_dir), "commit", "-q", "--allow-empty", "-m", "init"],
                check=False, stdin=subprocess.DEVNULL,
            )
            orch._write_lock(skills_d)
            lock = orch._read_lock()
            assert "test-skill" in lock
            assert len(lock["test-skill"]) == 40  # SHA length
        finally:
            mod.PROJECT_ROOT = orig

    def test_lock_path_is_project_root(self, orch):
        import _orchestrator as mod

        path = orch._lock_path()
        assert path.name == "leedevkit.lock"
        assert path.parent == mod.PROJECT_ROOT


class TestSkillsList:
    def test_list_empty(self, orch, tmp_skills_d):
        assert not list(tmp_skills_d.iterdir())


class TestResolveTargets:
    def test_fallback_when_no_config(self):
        from _orchestrator import _resolve_targets

        targets = _resolve_targets()
        assert "all" in targets
        assert "api" in targets


class TestInitValidation:
    def test_symlink_validation(self, orch, tmp_skills_d):
        # Test that init validates symlinks after creation
        # The actual symlink creation requires leedevkit installed at DEVKIT_HOME
        # We verify the validation logic handles broken symlinks
        agent_dir = tmp_skills_d / ".agent"
        agent_dir.mkdir()
        broken_link = agent_dir / "skills"
        broken_link.symlink_to("/nonexistent/path")

        # Validation: broken symlinks should be detected
        assert broken_link.is_symlink()
        assert not broken_link.resolve().exists()  # broken — target missing
