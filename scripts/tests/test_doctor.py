"""Tests for _doctor — system health check module."""

from __future__ import annotations

from unittest.mock import MagicMock, patch


class TestRunDoctor:
    """Tests for run_doctor() covering all code paths."""

    @staticmethod
    def _dk(tmp_path, version="0.1.0"):
        """Create a minimal devkit directory with VERSION."""
        dk = tmp_path / ".leedevkit"
        dk.mkdir(parents=True, exist_ok=True)
        (dk / "VERSION").write_text(version)
        return dk

    @staticmethod
    def _patches(tmp_path, dk, **overrides):
        """Return a context manager stacking all patches for run_doctor."""
        from contextlib import ExitStack

        defaults = {
            "load_project_config": {"project": {}, "targets": {}},
            "get_devkit_root": dk,
            "resolve_ai_rules": [],
            "socket_connect_ex": 1,  # port free
        }
        defaults.update(overrides)

        # socket.connect_ex is an instance method — mock socket.socket()
        mock_sock = MagicMock()
        mock_sock.connect_ex.return_value = defaults["socket_connect_ex"]

        stack = ExitStack()
        stack.enter_context(patch("_doctor.PROJECT_ROOT", tmp_path))
        stack.enter_context(patch("_doctor.load_project_config", return_value=defaults["load_project_config"]))
        stack.enter_context(patch("_doctor.get_devkit_root", return_value=defaults["get_devkit_root"]))
        stack.enter_context(patch("_doctor.resolve_ai_rules", return_value=defaults["resolve_ai_rules"]))
        stack.enter_context(patch("_doctor.socket.socket", return_value=mock_sock))
        stack.enter_context(patch("subprocess.run"))
        return stack

    # ── project config ───────────────────────────────────────────────────

    def test_with_valid_config(self, tmp_path, capsys):
        """Valid config shows project name and targets."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)
        (tmp_path / ".agent" / "rules").mkdir(parents=True)
        (tmp_path / ".venv").mkdir()

        with self._patches(
            tmp_path,
            dk,
            load_project_config={
                "project": {"name": "TestProject"},
                "targets": {"api": ["apiserver"], "web": ["webdashboard"]},
            },
            resolve_ai_rules=[tmp_path / "r1.md"],
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "TestProject" in out
        assert "api" in out

    def test_config_load_failure(self, tmp_path, capsys):
        """Broken/missing config shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with (
            patch("_doctor.PROJECT_ROOT", tmp_path),
            patch("_doctor.load_project_config", side_effect=OSError("no file")),
            patch("_doctor.get_devkit_root", side_effect=OSError("nope")),
            patch("_doctor.resolve_ai_rules", side_effect=OSError("nope")),
            patch("_doctor.socket.socket", return_value=MagicMock(connect_ex=MagicMock(return_value=1))),
            patch("subprocess.run"),
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "leedevkit.toml" in out

    def test_devkit_exception(self, tmp_path, capsys):
        """DevKit resolution failure shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with (
            patch("_doctor.PROJECT_ROOT", tmp_path),
            patch("_doctor.load_project_config", return_value={"project": {}, "targets": {}}),
            patch("_doctor.get_devkit_root", side_effect=OSError("no devkit")),
            patch("_doctor.resolve_ai_rules", return_value=[]),
            patch("_doctor.socket.socket", return_value=MagicMock(connect_ex=MagicMock(return_value=1))),
            patch("subprocess.run"),
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "DevKit" in out

    def test_ai_rules_exception(self, tmp_path, capsys):
        """AI rules resolution failure shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with self._patches(tmp_path, dk):
            with patch("_doctor.resolve_ai_rules", side_effect=ValueError("bad manifest")):
                run_doctor("podman")

        out = capsys.readouterr().err
        assert "AI rules" in out

    # ── .agent directory ──────────────────────────────────────────────────

    def test_agent_is_symlink(self, tmp_path, capsys):
        """Symlinked .agent shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)
        real_dir = tmp_path / "real-agent"
        real_dir.mkdir()
        agent_dir = tmp_path / ".agent"
        agent_dir.symlink_to(real_dir)

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "symlink" in out

    def test_agent_is_directory(self, tmp_path, capsys):
        """Real .agent directory shows success with rule count."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)
        agent_dir = tmp_path / ".agent" / "rules"
        agent_dir.mkdir(parents=True)
        (agent_dir / "coding.md").write_text("# Coding")
        (agent_dir / "testing.md").write_text("# Testing")

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "real directory" in out
        assert "2 rulebooks" in out

    def test_agent_missing(self, tmp_path, capsys):
        """Missing .agent directory shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "missing" in out

    # ── devkit install ────────────────────────────────────────────────────

    def test_devkit_with_version(self, tmp_path, capsys):
        """DevKit with VERSION file shows version."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path, "0.3.0")

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "v0.3.0" in out

    def test_devkit_without_version_file(self, tmp_path, capsys):
        """DevKit without VERSION file shows '?'."""
        from _doctor import run_doctor

        dk = tmp_path / ".leedevkit"
        dk.mkdir()  # no VERSION file

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "(v?)" in out

    # ── AI rules ──────────────────────────────────────────────────────────

    def test_ai_rules_loaded(self, tmp_path, capsys):
        """AI rules count shown."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with self._patches(
            tmp_path, dk,
            resolve_ai_rules=[tmp_path / "r1.md", tmp_path / "r2.md"],
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "2 files loaded" in out

    # ── container engine ──────────────────────────────────────────────────

    def test_container_engine_shown(self, tmp_path, capsys):
        """Container engine name is shown."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with self._patches(tmp_path, dk):
            run_doctor("docker")

        out = capsys.readouterr().err
        assert "docker" in out

    # ── port scan ─────────────────────────────────────────────────────────

    def test_port_occupied(self, tmp_path, capsys):
        """Occupied port shows warning."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        mock_sock = MagicMock()
        # First call returns 0 (occupied), rest return 1 (free)
        mock_sock.connect_ex.side_effect = [0, 1, 1]

        with (
            patch("_doctor.PROJECT_ROOT", tmp_path),
            patch("_doctor.load_project_config", return_value={"project": {}, "targets": {}}),
            patch("_doctor.get_devkit_root", return_value=dk),
            patch("_doctor.resolve_ai_rules", return_value=[]),
            patch("_doctor.socket.socket", return_value=mock_sock),
            patch("subprocess.run"),
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "Port 3000 is occupied" in out

    # ── virtual environment ───────────────────────────────────────────────

    def test_venv_found(self, tmp_path, capsys):
        """Virtual environment found message."""
        from _doctor import run_doctor

        (tmp_path / ".venv").mkdir()
        dk = self._dk(tmp_path)

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "Virtual Environment: Found" in out

    def test_venv_missing(self, tmp_path, capsys):
        """Virtual environment missing message."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        with self._patches(tmp_path, dk):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "Missing" in out

    # ── running containers ────────────────────────────────────────────────

    def test_containers_running(self, tmp_path, capsys):
        """Running containers detected."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)

        mock_sock = MagicMock()
        mock_sock.connect_ex.return_value = 1
        fake_run = MagicMock()
        fake_run.return_value.stdout = "leedevkit-dev-db\nleedevkit-dev-api\nother-container\n"

        with (
            patch("_doctor.PROJECT_ROOT", tmp_path),
            patch("_doctor.load_project_config", return_value={"project": {}, "targets": {}}),
            patch("_doctor.get_devkit_root", return_value=dk),
            patch("_doctor.resolve_ai_rules", return_value=[]),
            patch("_doctor.socket.socket", return_value=mock_sock),
            patch("subprocess.run", fake_run),
        ):
            run_doctor("podman")

        out = capsys.readouterr().err
        assert "leedevkit-dev-db" in out
        assert "leedevkit-dev-api" in out

    def test_no_engine_skips_container_check(self, tmp_path, capsys):
        """Empty engine string skips container check."""
        from _doctor import run_doctor

        dk = self._dk(tmp_path)
        mock_sock = MagicMock()
        mock_sock.connect_ex.return_value = 1

        with (
            patch("_doctor.PROJECT_ROOT", tmp_path),
            patch("_doctor.load_project_config", return_value={"project": {}, "targets": {}}),
            patch("_doctor.get_devkit_root", return_value=dk),
            patch("_doctor.resolve_ai_rules", return_value=[]),
            patch("_doctor.socket.socket", return_value=mock_sock),
            patch("subprocess.run") as mock_run,
        ):
            run_doctor("")

        out = capsys.readouterr().err
        # subprocess.run should NOT be called for container check when engine is empty
        mock_run.assert_not_called()
