"""Tests for per-project devkit init (Option B: no sharing).

Covers the new handle_init() flow:
  - Installs devkit into .leedevkit/ from local source fallback
  - Creates project-local .venv inside .leedevkit/
  - Copies rules to project (not symlinks)
  - Creates ./leedevkit wrapper
  - Idempotent (no overwrite on re-init)
  - DevKit root resolution priority
  - skills.d inside devkit root
"""

from __future__ import annotations

import json
import tarfile
from pathlib import Path
from unittest.mock import patch

import pytest


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_project(tmp_path: Path, version: str = "0.1.0") -> Path:
    """Create a minimal project directory with leedevkit.toml."""
    tmp_path.mkdir(parents=True, exist_ok=True)
    (tmp_path / ".git").mkdir()
    (tmp_path / "leedevkit.toml").write_text(
        f'[devkit]\nversion = "{version}"\n[project]\nname = "test"\n'
    )
    return tmp_path


def _make_devkit_source(tmp_path: Path, version: str = "0.1.0") -> Path:
    """Create a fake devkit source directory (simulates the repo root)."""
    src = tmp_path / "devkit-src"
    src.mkdir(parents=True, exist_ok=True)
    (src / "VERSION").write_text(version)
    scripts = src / "scripts"
    scripts.mkdir()
    (scripts / "_orchestrator.py").write_text("# orchestrator stub\n")
    (scripts / "_bootstrap.py").write_text("# bootstrap stub\n")
    # Create a proper .venv structure so InitHandler validation passes
    venv_bin = src / ".venv" / "bin"
    venv_bin.mkdir(parents=True)
    python3 = venv_bin / "python3"
    python3.touch()
    python3.chmod(0o755)
    # ensure-venv.sh creates the venv and outputs the Python path
    (scripts / "_ensure-venv.sh").write_text(
        '#!/bin/bash\n'
        'VENV="$(dirname "$(dirname "$0")")/.venv"\n'
        'mkdir -p "$VENV/bin"\n'
        'touch "$VENV/bin/python3"\n'
        'chmod +x "$VENV/bin/python3"\n'
        'echo "$VENV/bin/python3"\n'
    )
    bin_dir = src / "bin"
    bin_dir.mkdir()
    (bin_dir / "leedevkit").write_text("#!/bin/bash\necho stub\n")
    (src / "templates").mkdir()
    agent = src / ".agent"
    agent.mkdir()
    rules = agent / "rules"
    rules.mkdir()
    (rules / "coding-standards.md").write_text("# Coding Standards\n")
    (rules / "testing-standards.md").write_text("# Testing Standards\n")
    (agent / "skills-catalog.toml").write_text(
        '[skills]\nfoo = { name = "foo", url = "https://example.com/foo.git" }\n'
    )
    return src


# ---------------------------------------------------------------------------
# Tests: devkit root resolution priority
# ---------------------------------------------------------------------------


class TestDevKitRootPriority:
    def test_per_project_overrides_global(self, tmp_path, monkeypatch):
        """When .leedevkit/ exists in project, it takes priority over global."""
        project = _make_project(tmp_path / "project")
        # Create .leedevkit/ in project
        leedevkit = project / ".leedevkit"
        leedevkit.mkdir()
        (leedevkit / "scripts").mkdir()
        (leedevkit / "scripts" / "_orchestrator.py").write_text("# stub\n")
        # Set a global install too (should be ignored)
        global_install = tmp_path / "global" / "current"
        global_install.mkdir(parents=True)
        (global_install / "scripts").mkdir()
        (global_install / "scripts" / "_orchestrator.py").write_text("# global stub\n")
        monkeypatch.setenv("DEVKIT_HOME", str(global_install))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        root = _devkit_config.get_devkit_root()
        assert root == leedevkit

    def test_env_var_overrides_legacy(self, tmp_path, monkeypatch):
        """DEVKIT_HOME takes priority over ~/.leedevkit/current symlink."""
        project = _make_project(tmp_path / "project")
        custom = tmp_path / "custom-install"
        custom.mkdir()
        (custom / "scripts").mkdir()
        (custom / "scripts" / "_orchestrator.py").write_text("# custom\n")
        monkeypatch.setenv("DEVKIT_HOME", str(custom))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        root = _devkit_config.get_devkit_root()
        assert root == custom

    def test_raises_when_no_devkit_found(self, tmp_path, monkeypatch):
        """Raises FileNotFoundError when no devkit is resolvable."""
        project = _make_project(tmp_path / "project")
        monkeypatch.setenv("DEVKIT_HOME", "")
        # Prevent legacy ~/.leedevkit/current fallback from resolving
        monkeypatch.setattr("pathlib.Path.home", lambda: tmp_path / "fake-home")
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with pytest.raises(FileNotFoundError, match="Cannot locate leedevkit"):
            _devkit_config.get_devkit_root()


# ---------------------------------------------------------------------------
# Tests: handle_init from local source
# ---------------------------------------------------------------------------


class TestHandleInitFromSource:
    @pytest.mark.parametrize("configured_version", ["v0.3.14", "latest"])
    def test_equivalent_version_does_not_reinstall(
        self, tmp_path, monkeypatch, configured_version
    ):
        """A prefixed or latest pin must not reinstall an existing release."""
        project = _make_project(tmp_path / "project", version=configured_version)
        source = _make_devkit_source(tmp_path / "source", version="0.3.14")
        target = project / ".leedevkit"
        import shutil

        shutil.copytree(source, target)
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with (
            patch("_orchestrator.Orchestrator.register_traps", return_value=None),
            patch("_init_handler.InitHandler._install_devkit") as install,
        ):
            from _orchestrator import Orchestrator

            Orchestrator().handle_init(force=False)

        install.assert_not_called()

    def test_unprefixed_version_downloads_v_tag(self, tmp_path, monkeypatch):
        """Config versions without ``v`` still resolve GitHub's v-prefixed tag."""
        project = _make_project(tmp_path / "project", version="0.3.11")
        target = project / ".leedevkit"
        requested_urls = []

        def fake_download(url, destination):
            requested_urls.append(url)
            destination.mkdir(parents=True)
            (destination / "VERSION").write_text("0.3.11")

        monkeypatch.setenv("DEVKIT_LOCAL_PATH", "")
        monkeypatch.setattr(
            "_init_handler.download_and_extract_tarball", fake_download
        )

        from _init_handler import InitHandler

        InitHandler(None)._install_devkit(project, target, "0.3.11", force=False)

        assert requested_urls == [
            "https://github.com/vkaylee/leedevkit/archive/refs/tags/"
            "v0.3.11.tar.gz"
        ]

    def test_extract_rejects_copying_target_onto_itself(self, tmp_path):
        """A failed download must not delete a project's existing devkit."""
        target = _make_devkit_source(tmp_path)
        orchestrator = target / "scripts" / "_orchestrator.py"

        from _init_handler import InitHandler

        with pytest.raises(RuntimeError, match="target directory itself"):
            InitHandler(None)._extract_from_source(target, target)

        assert orchestrator.exists()

    def test_failed_download_does_not_destroy_running_install(
        self, tmp_path, monkeypatch
    ):
        """Exercise the original 404 → fallback failure through _install_devkit."""
        project = _make_project(tmp_path / "project", version="0.3.11")
        target = _make_devkit_source(project, version="0.3.14")
        orchestrator = target / "scripts" / "_orchestrator.py"

        import _init_handler

        monkeypatch.setenv("DEVKIT_LOCAL_PATH", "")
        monkeypatch.setattr(
            _init_handler,
            "__file__",
            str(target / "scripts" / "_init_handler.py"),
        )

        def fail_download(_url, _destination):
            raise OSError("simulated 404")

        monkeypatch.setattr(
            _init_handler, "download_and_extract_tarball", fail_download
        )

        with pytest.raises(RuntimeError, match="target directory itself"):
            _init_handler.InitHandler(None)._install_devkit(
                project, target, "0.3.11", force=False
            )

        assert orchestrator.read_text() == "# orchestrator stub\n"

    def test_installs_devkit_into_leedevkit_dir(self, tmp_path, monkeypatch):
        """Init copies devkit artifacts from source into .leedevkit/."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        leedevkit = project / ".leedevkit"
        assert leedevkit.exists()
        assert (leedevkit / "scripts" / "_orchestrator.py").exists()
        assert (leedevkit / "bin" / "leedevkit").exists()
        assert (leedevkit / ".agent" / "rules" / "coding-standards.md").exists()

    def test_creates_dev_state_json(self, tmp_path, monkeypatch):
        """Init writes dev-state.json with version + source."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        state_path = project / ".leedevkit" / "dev-state.json"
        assert state_path.exists()
        state = json.loads(state_path.read_text())
        assert state["version"] == "0.1.0"
        assert state["source"] == "local"

    def test_copies_rules_to_project_not_symlink(self, tmp_path, monkeypatch):
        """Rules are copied as real files, not symlinks."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        project_rule = project / ".agent" / "rules" / "coding-standards.md"
        assert project_rule.exists()
        assert not project_rule.is_symlink()
        assert project_rule.read_text() == "# Coding Standards\n"

    def test_idempotent_does_not_overwrite_existing_rules(self, tmp_path, monkeypatch):
        """Re-running init does not overwrite existing project rules."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        # Pre-create a custom rule
        rules_dir = project / ".agent" / "rules"
        rules_dir.mkdir(parents=True)
        custom_rule = rules_dir / "coding-standards.md"
        custom_rule.write_text("# My Custom Rules\n")
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
            # Should still have custom content
            assert custom_rule.read_text() == "# My Custom Rules\n"

    def test_force_overwrites_existing_rules(self, tmp_path, monkeypatch):
        """Force flag overwrites existing rules."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        rules_dir = project / ".agent" / "rules"
        rules_dir.mkdir(parents=True)
        custom_rule = rules_dir / "coding-standards.md"
        custom_rule.write_text("# My Custom Rules\n")
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=True)
            assert custom_rule.read_text() == "# Coding Standards\n"

    def test_creates_leedevkit_wrapper_symlink(self, tmp_path, monkeypatch):
        """Init creates ./leedevkit symlink to .leedevkit/bin/leedevkit."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        wrapper = project / "leedevkit"
        assert wrapper.exists()
        # Wrapper is a real executable script (not symlink) by design
        assert wrapper.is_file()
        assert "exec" in wrapper.read_text()

    def test_pins_devkit_version_in_toml(self, tmp_path, monkeypatch):
        """Init pins the actual devkit version in leedevkit.toml."""
        project = _make_project(tmp_path / "project", version="latest")
        source = _make_devkit_source(tmp_path / "source", version="0.1.0")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        import _devkit_config

        cfg = _devkit_config._load_toml(project / "leedevkit.toml")
        assert cfg.get("devkit", {}).get("version") == "0.1.0"

    def test_creates_overrides_yaml(self, tmp_path, monkeypatch):
        """Init copies overrides.yaml from devkit to project if missing."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        # Add overrides.yaml to source devkit
        (source / ".agent" / "overrides.yaml").write_text(
            "replace: []\nextend: []\nadd: []\n"
        )
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        override = project / ".agent" / "overrides.yaml"
        assert override.exists()
        assert not override.is_symlink()


# ---------------------------------------------------------------------------
# Tests: handle_init from tarball download (mocked)
# ---------------------------------------------------------------------------


class TestHandleInitFromTarball:
    def test_downloads_and_extracts_tarball(self, tmp_path, monkeypatch):
        """When no local source, init downloads release tarball."""
        project = _make_project(tmp_path / "project", version="0.1.0")
        source = _make_devkit_source(tmp_path / "source", version="0.1.0")

        # Build a real tarball from source
        tarball_path = tmp_path / "release.tar.gz"
        with tarfile.open(tarball_path, "w:gz") as tf:
            for item in ["scripts", "bin", ".agent"]:
                tf.add(source / item, arcname=f"leedevkit-0.1.0/{item}")
            tf.add(source / "VERSION", arcname="leedevkit-0.1.0/VERSION")
            # Include venv so init validation passes
            tf.add(source / ".venv", arcname="leedevkit-0.1.0/.venv")

        # Mock urllib to return our tarball
        class FakeResponse:
            def __init__(self, path):
                self._path = path

            def read(self, *a, **kw):
                return b""

            def __enter__(self):
                return self

            def __exit__(self, *a):
                pass

        def fake_urlretrieve(url, filename):
            import shutil

            shutil.copy(str(tarball_path), str(filename))

        monkeypatch.setenv("DEVKIT_LOCAL_PATH", "")  # disable local fallback

        # download_and_extract_tarball uses urllib.request (via _download module)
        import urllib.request as _urllib_req

        monkeypatch.setattr(_urllib_req, "urlretrieve", fake_urlretrieve)
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None

        with (
            patch("_orchestrator.Orchestrator.register_traps", return_value=None),
        ):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)

        assert (project / ".leedevkit" / "scripts" / "_orchestrator.py").exists()
        assert (project / ".leedevkit" / "VERSION").exists()


# ---------------------------------------------------------------------------
# Tests: skills.d location
# ---------------------------------------------------------------------------


class TestSkillsDLocation:
    def test_skills_d_inside_devkit_root(self, tmp_path, monkeypatch):
        """skills.d is created inside .leedevkit/, not at project root."""
        project = _make_project(tmp_path / "project")
        # Pre-install .leedevkit/
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
            # Trigger skills list to ensure skills.d path resolves
            import argparse

            args = argparse.Namespace(skills_action="list")
            orch.handle_skills(args)
        # skills.d should be inside .leedevkit/
        assert (project / ".leedevkit" / "skills.d").exists()
        # Should NOT be at project/.leedevkit/skills.d (old pattern was same, check no stray)
        # Old pattern: <project>/.leedevkit/skills.d — same path, but the key is it's
        # relative to devkit root which IS .leedevkit/ now


# ---------------------------------------------------------------------------
# Tests: doctor reflects per-project install
# ---------------------------------------------------------------------------


class TestDoctorPerProject:
    def test_doctor_shows_devkit_location(self, tmp_path, monkeypatch, capsys):
        """Doctor command reports the devkit install path."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
            _devkit_config._DEVKIT_ROOT = None  # reset cache
            orch.handle_doctor()
        captured = capsys.readouterr()
        # Doctor should mention DevKit location
        combined = captured.out + captured.err
        assert ".leedevkit" in combined or "DevKit" in combined


# ---------------------------------------------------------------------------
# Tests: legacy symlink detection (detect + warn, no auto-delete)
# ---------------------------------------------------------------------------


class TestLegacySymlinkDetection:
    def test_detects_symlinks_to_global_install(self, tmp_path, monkeypatch):
        """Detects symlinks pointing to ~/.leedevkit/ (legacy global install)."""
        project = _make_project(tmp_path / "project")
        agent_dir = project / ".agent"
        agent_dir.mkdir()
        # Create a fake global install path
        fake_global = tmp_path / "fake-home" / ".leedevkit" / "current"
        fake_global.mkdir(parents=True)
        (fake_global / "scripts").mkdir()
        # Symlink to fake global (simulating old init)
        (agent_dir / "scripts").symlink_to(fake_global / "scripts")
        monkeypatch.chdir(project)
        from _init_handler import InitHandler
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
        handler = InitHandler(orch)
        # Patch Path.home to point to fake home
        monkeypatch.setattr("pathlib.Path.home", lambda: tmp_path / "fake-home")
        detected = handler._detect_legacy_symlinks(agent_dir)
        assert len(detected) == 1
        assert detected[0][0] == "scripts"

    def test_ignores_real_directories(self, tmp_path):
        """Real directories in .agent/ are not flagged as legacy symlinks."""
        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir()
        (agent_dir / "custom-stuff").mkdir()
        (agent_dir / "custom-stuff" / "file.txt").write_text("hello")
        from _init_handler import InitHandler
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
        handler = InitHandler(orch)
        detected = handler._detect_legacy_symlinks(agent_dir)
        assert detected == []

    def test_ignores_symlinks_to_other_targets(self, tmp_path):
        """Symlinks not pointing to ~/.leedevkit/ are not flagged."""
        agent_dir = tmp_path / ".agent"
        agent_dir.mkdir()
        other_target = tmp_path / "other-location"
        other_target.mkdir()
        (agent_dir / "ext").symlink_to(other_target)
        from _init_handler import InitHandler
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
        handler = InitHandler(orch)
        detected = handler._detect_legacy_symlinks(agent_dir)
        assert detected == []

    def test_init_warns_about_legacy_symlinks(self, tmp_path, monkeypatch, capsys):
        """Init prints warning when legacy symlinks detected."""
        project = _make_project(tmp_path / "project")
        source = _make_devkit_source(tmp_path / "source")
        # Create legacy symlinks in .agent/
        agent_dir = project / ".agent"
        agent_dir.mkdir(parents=True)
        fake_global = tmp_path / "fake-home" / ".leedevkit" / "current"
        fake_global.mkdir(parents=True)
        (fake_global / "scripts").mkdir()
        (agent_dir / "scripts").symlink_to(fake_global / "scripts")
        (agent_dir / "rules").mkdir()  # real dir, not symlink
        (agent_dir / "rules" / "my-rule.md").write_text("custom")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.setattr("pathlib.Path.home", lambda: tmp_path / "fake-home")
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        captured = capsys.readouterr()
        combined = captured.out + captured.err
        assert "legacy symlink" in combined.lower() or "legacy" in combined.lower()
        # Verify symlink was NOT deleted
        assert (agent_dir / "scripts").is_symlink()
        # Verify custom rule was preserved
        assert (agent_dir / "rules" / "my-rule.md").exists()


# ---------------------------------------------------------------------------
# Tests: custom rules_dir symlink (CLAUDE.md compatibility)
# ---------------------------------------------------------------------------


class TestCustomRulesDirSymlink:
    def test_creates_symlink_when_rules_dir_is_custom(self, tmp_path, monkeypatch):
        """When rules_dir is custom (e.g., .attendagent/rules), create .agent/rules symlink."""
        project = _make_project(tmp_path / "project")
        # Configure custom rules_dir
        (project / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname = "test"\n'
            '[ai]\nrules_dir = ".attendagent/rules"\n'
        )
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        # Verify custom rules_dir has files
        custom_rules = project / ".attendagent" / "rules"
        assert custom_rules.exists()
        assert (custom_rules / "coding-standards.md").exists()
        # Verify .agent/rules symlink exists and points to custom rules
        default_rules = project / ".agent" / "rules"
        assert default_rules.is_symlink()
        assert default_rules.resolve() == custom_rules.resolve()
        # Verify symlink is accessible
        assert (default_rules / "coding-standards.md").exists()

    def test_fixes_broken_symlink(self, tmp_path, monkeypatch):
        """If .agent/rules symlink is broken, init fixes it."""
        project = _make_project(tmp_path / "project")
        (project / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname = "test"\n'
            '[ai]\nrules_dir = ".attendagent/rules"\n'
        )
        source = _make_devkit_source(tmp_path / "source")
        # Create broken symlink
        agent_dir = project / ".agent"
        agent_dir.mkdir(parents=True)
        broken_target = tmp_path / "nonexistent"
        (agent_dir / "rules").symlink_to(broken_target)
        assert (agent_dir / "rules").is_symlink()
        assert not (agent_dir / "rules").exists()  # broken
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        # Verify symlink is fixed
        default_rules = project / ".agent" / "rules"
        custom_rules = project / ".attendagent" / "rules"
        assert default_rules.is_symlink()
        assert default_rules.resolve() == custom_rules.resolve()
        assert (default_rules / "coding-standards.md").exists()

    def test_fixes_wrong_target_symlink(self, tmp_path, monkeypatch):
        """If .agent/rules symlink points to wrong target, init fixes it."""
        project = _make_project(tmp_path / "project")
        (project / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname = "test"\n'
            '[ai]\nrules_dir = ".attendagent/rules"\n'
        )
        source = _make_devkit_source(tmp_path / "source")
        # Create symlink pointing to wrong target
        agent_dir = project / ".agent"
        agent_dir.mkdir(parents=True)
        wrong_target = tmp_path / "wrong-location"
        wrong_target.mkdir()
        (agent_dir / "rules").symlink_to(wrong_target)
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        # Verify symlink is fixed
        default_rules = project / ".agent" / "rules"
        custom_rules = project / ".attendagent" / "rules"
        assert default_rules.is_symlink()
        assert default_rules.resolve() == custom_rules.resolve()

    def test_preserves_real_directory(self, tmp_path, monkeypatch):
        """If .agent/rules is a real directory (not symlink), don't touch it."""
        project = _make_project(tmp_path / "project")
        (project / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname = "test"\n'
            '[ai]\nrules_dir = ".attendagent/rules"\n'
        )
        source = _make_devkit_source(tmp_path / "source")
        # Create real directory at .agent/rules
        agent_dir = project / ".agent"
        agent_dir.mkdir(parents=True)
        real_rules = agent_dir / "rules"
        real_rules.mkdir()
        (real_rules / "custom-rule.md").write_text("# Custom\n")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        # Verify real directory is preserved
        default_rules = project / ".agent" / "rules"
        assert not default_rules.is_symlink()
        assert default_rules.is_dir()
        assert (default_rules / "custom-rule.md").exists()
        assert (default_rules / "custom-rule.md").read_text() == "# Custom\n"

    def test_no_symlink_when_rules_dir_is_default(self, tmp_path, monkeypatch):
        """When rules_dir is .agent/rules (default), no symlink is created."""
        project = _make_project(tmp_path / "project")
        # Use default rules_dir
        (project / "leedevkit.toml").write_text(
            '[devkit]\nversion = "0.1.0"\n[project]\nname = "test"\n'
            '[ai]\nrules_dir = ".agent/rules"\n'
        )
        source = _make_devkit_source(tmp_path / "source")
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(project)
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with patch("_orchestrator.Orchestrator.register_traps", return_value=None):
            from _orchestrator import Orchestrator

            orch = Orchestrator()
            orch.handle_init(force=False)
        # Verify .agent/rules is a real directory, not symlink
        default_rules = project / ".agent" / "rules"
        assert not default_rules.is_symlink()
        assert default_rules.is_dir()
        assert (default_rules / "coding-standards.md").exists()
