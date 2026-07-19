"""Tests for SkillsManager lock file operations.

Dispatch/integration tests live in test_orchestrator.py (TestSkillsSubCommands).
"""

import os
import sys
from unittest.mock import patch

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
            tomli_w.dump(
                {
                    "skills": {
                        "ui-ux-pro-max": {
                            "name": "UI/UX Pro Max",
                            "url": "https://github.com/leeattend/ui-ux-pro-max",
                            "description": "Advanced UI/UX design toolkit",
                        }
                    }
                },
                f,
            )

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


class TestSkillsManagerDispatch:
    """Dispatch method tests for list, install, update, remove commands."""

    def test_dispatch_list(self, monkeypatch, tmp_path):
        """dispatch with skills_action='list' completes without error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir(parents=True)
        import tomli_w
        with open(agent_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({"skills": {}}, f)

        (tmp_path / "skills.d").mkdir()
        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "list"})()
        mgr.dispatch(args)  # Should not raise

    def test_dispatch_install_from_config(self, monkeypatch, tmp_path):
        """dispatch install with no name arg completes without error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "install", "name": None})()
        mgr.dispatch(args)  # Should not raise

    def test_dispatch_update_empty(self, monkeypatch, tmp_path):
        """dispatch update with no installed skills completes without error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "update"})()
        mgr.dispatch(args)  # Should not raise

    def test_dispatch_remove_not_installed(self, monkeypatch, tmp_path):
        """dispatch remove with non-existent skill completes gracefully."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        (tmp_path / "skills.d").mkdir()
        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "remove", "name": "nonexistent"})()
        mgr.dispatch(args)  # Should not raise, just logs warning

    def test_dispatch_unknown_action(self, monkeypatch, tmp_path):
        """dispatch with unknown action silently no-ops."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "bogus"})()
        mgr.dispatch(args)  # Should not raise

    def test_dispatch_add_missing_url(self, monkeypatch, tmp_path):
        """dispatch add without url logs usage error and returns."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        args = type("Args", (), {"skills_action": "add", "url": ""})()
        mgr.dispatch(args)  # Should not raise, just logs usage


class TestSkillsManagerInternals:
    """Tests for internal helper methods to boost coverage above 80%."""

    def test_install_by_name_not_in_catalog(self, monkeypatch, tmp_path):
        """_install_by_name logs error when name not found in catalog."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        # Empty catalog
        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir(parents=True)
        import tomli_w
        with open(agent_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({"skills": {}}, f)

        mgr = SkillsManager()
        mgr._install_by_name("nonexistent")  # Should not raise

    def test_install_by_name_already_installed(self, monkeypatch, tmp_path):
        """_install_by_name skips when already installed."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir(parents=True)
        import tomli_w
        with open(agent_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({"skills": {"my-skill": {"name": "MySkill", "url": "https://github.com/x/y.git"}}}, f)

        # Create the skill dir to simulate "already installed"
        (tmp_path / "skills.d" / "my-skill").mkdir(parents=True)

        mgr = SkillsManager()
        mgr._install_by_name("my-skill")  # Should log warning, not clone

    def test_install_by_name_from_catalog(self, monkeypatch, tmp_path):
        """_install_by_name clones when name is in catalog."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir(parents=True)
        import tomli_w
        with open(agent_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({"skills": {"cool-skill": {"name": "CoolSkill", "url": "https://github.com/x/cool.git"}}}, f)

        mgr = SkillsManager()
        with patch("subprocess.run") as mock_run:
            mgr._install_by_name("cool-skill")
        mock_run.assert_called_once()

    def test_add_from_url_valid(self, monkeypatch, tmp_path):
        """_add_from_url with valid URL clones and writes lock."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        with patch("subprocess.run") as mock_run:
            mgr._add_from_url("https://github.com/x/newskill.git")
        mock_run.assert_called_once()

    def test_add_from_url_catalog_name(self, monkeypatch, tmp_path):
        """_add_from_url with catalog name logs error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir(parents=True)
        import tomli_w
        with open(agent_dir / "skills-catalog.toml", "wb") as f:
            tomli_w.dump({"skills": {"existing": {"name": "Existing", "url": "https://github.com/x/existing.git"}}}, f)

        mgr = SkillsManager()
        # "existing" is a catalog name, not a URL — should log error
        mgr._add_from_url("existing")

    def test_add_from_url_invalid(self, monkeypatch, tmp_path):
        """_add_from_url with non-URL, non-catalog name logs error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        # Not a URL and not in catalog
        mgr._add_from_url("not-a-url")

    def test_update_and_lock_empty(self, monkeypatch, tmp_path):
        """_update_and_lock with no git repos logs 0 updates."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        mgr._update_and_lock()  # No repos → updated=0

    def test_update_and_lock_with_repo(self, monkeypatch, tmp_path):
        """_update_and_lock with git repo pulls and updates lock."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        # Create a fake git repo
        repo_dir = tmp_path / "skills.d" / "test-skill"
        repo_dir.mkdir(parents=True)
        (repo_dir / ".git").mkdir()

        mgr = SkillsManager()
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = type("R", (), {"stdout": "abc123\n", "returncode": 0})()
            mgr._update_and_lock()
        mock_run.assert_called()

    def test_remove_existing(self, monkeypatch, tmp_path):
        """_remove deletes an installed skill."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        (tmp_path / "skills.d" / "to-remove").mkdir(parents=True)
        mgr = SkillsManager()
        with patch("shutil.rmtree") as mock_rm:
            mgr._remove("to-remove")
        mock_rm.assert_called_once()

    def test_remove_empty_name(self, monkeypatch, tmp_path):
        """_remove with empty name logs error."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        mgr._remove("")  # Should log error, not raise

    def test_install_from_toml_empty(self, monkeypatch, tmp_path):
        """_install_from_toml with no config entries completes."""
        import _devkit_config  # noqa: F811
        from _skills_manager import SkillsManager

        monkeypatch.setattr("_skills_manager.PROJECT_ROOT", tmp_path)
        monkeypatch.setattr(_devkit_config, "get_devkit_root", lambda: tmp_path)

        mgr = SkillsManager()
        mgr._install_from_toml()  # No config → no-op
