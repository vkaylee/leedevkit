"""Tests for SkillsManager lock file operations.

Dispatch/integration tests live in test_orchestrator.py (TestSkillsSubCommands).
"""

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))


class TestSkillsManagerLock:
    """Lock file read/write — no devkit-config dependency needed."""

    def test_lock_path(self, monkeypatch, tmp_path):
        from _skills_manager import SkillsManager
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        assert SkillsManager._lock_path() == tmp_path / "leedevkit.lock"

    def test_read_lock_missing(self, monkeypatch, tmp_path):
        from _skills_manager import SkillsManager
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        assert SkillsManager._read_lock() == {}

    def test_read_lock_valid_toml(self, monkeypatch, tmp_path):
        import tomli_w
        from _skills_manager import SkillsManager
        lock_path = tmp_path / "leedevkit.lock"
        with open(lock_path, "wb") as f:
            tomli_w.dump({"my-skill": "abc123def"}, f)
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        data = SkillsManager._read_lock()
        assert "my-skill" in data
        assert data["my-skill"] == "abc123def"

    def test_read_lock_valid_json_fallback(self, monkeypatch, tmp_path):
        import json
        from _skills_manager import SkillsManager
        lock_path = tmp_path / "leedevkit.lock"
        lock_path.write_text(json.dumps({"other-skill": "xyz"}))
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        data = SkillsManager._read_lock()
        assert "other-skill" in data

    def test_write_lock_produces_file(self, monkeypatch, tmp_path):
        import subprocess
        from _skills_manager import SkillsManager
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        skills_d = tmp_path / "skills.d"
        skills_d.mkdir()
        (skills_d / ".git").mkdir()
        orig = subprocess.run

        def _fake(*a, **kw):
            if "rev-parse" in str(a):
                return type("R", (), {"stdout": "abc123\n", "returncode": 0})()
            return orig(*a, **kw)

        monkeypatch.setattr("subprocess.run", _fake)
        mgr = SkillsManager()
        mgr._skills_d = skills_d
        mgr._write_lock()
        assert (tmp_path / "leedevkit.lock").exists()


class TestSkillsManagerCatalog:
    """Catalog loading tests."""

    def test_load_catalog_has_ui_ux_pro_max(self, monkeypatch, tmp_path):
        """Catalog must contain the ui-ux-pro-max skill."""
        import _devkit_config  # noqa: F811 - ensure module is in sys.modules
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)
        # Create a minimal catalog file
        catalog_dir = tmp_path / ".agent"
        catalog_dir.mkdir(parents=True)
        import tomli_w
        with open(catalog_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({
                "skills": {
                    "ui-ux-pro-max": {
                        "name": "UI/UX Pro Max",
                        "url": "https://github.com/leeattend/ui-ux-pro-max",
                        "description": "Advanced UI/UX design toolkit",
                    }
                }
            }, f)

        from _skills_manager import SkillsManager
        mgr = SkillsManager()
        catalog = mgr._load_catalog()
        assert isinstance(catalog, dict)
        assert "ui-ux-pro-max" in catalog

    def test_load_catalog_missing_file(self, monkeypatch, tmp_path):
        """Returns empty dict when catalog file doesn't exist."""
        import _devkit_config  # noqa: F811 - ensure module is in sys.modules
        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)
        from _skills_manager import SkillsManager
        mgr = SkillsManager()
        assert mgr._load_catalog() == {}
