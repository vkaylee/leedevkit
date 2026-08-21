"""Tests for _bootstrap.py — env setup, engine detection, profile resolution."""

import os
import sys
from pathlib import Path
from unittest.mock import patch

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))

from _bootstrap import (  # noqa: E402
    PROJECT_ROOT as BOOTSTRAP_ROOT,
)
from _bootstrap import (  # noqa: E402
    SCRIPTS_DIR,
    bootstrap_env,
    detect_compose_cmd,
    detect_engine,
    resolve_lifecycle_profiles,
    resolve_profiles,
)


class TestPaths:
    def test_project_root_exists(self) -> None:
        assert BOOTSTRAP_ROOT.is_dir()

    def test_scripts_dir_exists(self) -> None:
        assert SCRIPTS_DIR.is_dir()

    def test_project_root_is_parent_of_scripts(self) -> None:
        assert SCRIPTS_DIR.parent == BOOTSTRAP_ROOT


class TestDetectEngine:
    def test_podman_preferred(self) -> None:
        with patch("shutil.which", side_effect=lambda cmd: cmd in ("podman",)):
            assert detect_engine() == "podman"

    def test_docker_fallback(self) -> None:
        with patch("shutil.which", side_effect=lambda cmd: cmd in ("docker",)):
            assert detect_engine() == "docker"

    def test_default_podman_when_none(self) -> None:
        with patch("shutil.which", return_value=None):
            assert detect_engine() == "podman"

    def test_podman_over_docker(self) -> None:
        """podman should win even if docker is also available."""
        with patch("shutil.which", return_value="/usr/bin/podman"):
            assert detect_engine() == "podman"


class TestDetectCompose:
    def test_podman_compose_when_podman(self) -> None:
        with patch("shutil.which", side_effect=lambda cmd: cmd in ("podman",)):
            assert detect_compose_cmd() == ["podman-compose"]

    def test_docker_compose_when_docker_only(self) -> None:
        with patch("shutil.which", side_effect=lambda cmd: cmd in ("docker",)):
            assert detect_compose_cmd() == ["docker", "compose"]

    def test_default_podman_compose(self) -> None:
        with patch("shutil.which", return_value=None):
            assert detect_compose_cmd() == ["podman-compose"]


class TestResolveProfiles:
    @staticmethod
    def _has_profile(profiles: list[str], name: str) -> bool:
        """Check if a --profile flag with given name exists in the list."""
        for i, item in enumerate(profiles):
            if (
                item == "--profile"
                and i + 1 < len(profiles)
                and profiles[i + 1] == name
            ):
                return True
        return False

    def test_api_mode(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("api")
            assert self._has_profile(profiles, "api")
            assert not self._has_profile(profiles, "web")

    def test_integration_mode(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("integration")
            assert self._has_profile(profiles, "api")
            assert not self._has_profile(profiles, "web")

    def test_web_mode(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("web")
            assert self._has_profile(profiles, "web")
            assert not self._has_profile(profiles, "api")

    def test_all_mode(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("all")
            assert self._has_profile(profiles, "api")
            assert self._has_profile(profiles, "web")

    def test_unknown_mode_returns_mode(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("unknown")
            assert profiles == ["--profile", "unknown"]

    def test_custom_profiles_from_env(self) -> None:
        with patch.dict(
            os.environ, {"PROFILES": "--profile custom1 --profile custom2"}
        ):
            profiles = resolve_profiles("api")
            assert profiles == ["--profile", "custom1", "--profile", "custom2"]

    def test_empty_custom_profiles(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            profiles = resolve_profiles("api")
            assert len(profiles) > 0  # falls back to default


class TestResolveLifecycleProfiles:
    def test_api_mode_backend_only(self) -> None:
        profiles = resolve_lifecycle_profiles("api")
        assert profiles == [
            "--profile",
            "api",
            "--profile",
            "infra-db",
            "--profile",
            "infra-redis",
            "--profile",
            "infra-pooler",
        ]

    def test_integration_mode_backend_only(self) -> None:
        profiles = resolve_lifecycle_profiles("integration")
        assert profiles == [
            "--profile",
            "api",
            "--profile",
            "infra-db",
            "--profile",
            "infra-redis",
            "--profile",
            "infra-pooler",
        ]

    def test_web_mode_frontend_only(self) -> None:
        profiles = resolve_lifecycle_profiles("web")
        assert profiles == ["--profile", "web"]

    def test_all_mode_both(self) -> None:
        profiles = resolve_lifecycle_profiles("all")
        assert "--profile" in profiles
        assert "api" in profiles
        assert "web" in profiles

    def test_unknown_returns_mode(self) -> None:
        profiles = resolve_lifecycle_profiles("unknown")
        assert profiles == ["--profile", "unknown"]


class TestBootstrapEnv:
    def test_default_mode_all(self) -> None:
        with patch.dict(os.environ, {}, clear=True):
            env = bootstrap_env()
            assert env["COMPOSE_PROJECT_NAME"].startswith("leeattend-test")
        assert env["PODMAN_COMPOSE_PROJECT_NAME"].startswith("leeattend-test")
        assert env["USE_DOCKER"] == "true"
        assert env["MODE"] == "all"
        assert "CONTAINER_ENGINE" in env
        assert "DOCKER_COMPOSE_CMD" in env
        assert "DOCKER_COMPOSE_BASE" in env
        assert "DOCKER_COMPOSE_TOOL" in env
        assert "PROJECT_ROOT" in env

    def test_mode_injection(self) -> None:
        with patch.dict(os.environ, {"PROFILES": ""}):
            env = bootstrap_env(mode="api")
            assert env["MODE"] == "api"
            assert "--profile api" in env["DOCKER_COMPOSE_CMD"]
            assert "--profile web" not in env["DOCKER_COMPOSE_CMD"]

    def test_project_name_from_env(self) -> None:
        with patch.dict(os.environ, {"COMPOSE_PROJECT_NAME": "my-custom-project"}):
            env = bootstrap_env()
            assert env["COMPOSE_PROJECT_NAME"] == "my-custom-project"
            assert env["PODMAN_COMPOSE_PROJECT_NAME"] == "my-custom-project"

    def test_compose_base_has_no_profiles(self) -> None:
        with patch.dict(os.environ, {}, clear=True):
            env = bootstrap_env(mode="api")
            base = env["DOCKER_COMPOSE_BASE"]
        assert "--profile" not in base
        assert "leeattend-test" in base
        assert "docker-compose.test.yml" in base

    def test_compose_too_is_space_separated(self) -> None:
        env = bootstrap_env()
        tool = env["DOCKER_COMPOSE_TOOL"]
        assert isinstance(tool, str)
        # Could be "podman-compose" or "docker compose"
        assert len(tool) > 0

    def test_env_contains_project_root(self) -> None:
        env = bootstrap_env()
        assert env["PROJECT_ROOT"] == str(BOOTSTRAP_ROOT)
        assert (BOOTSTRAP_ROOT / "scripts").as_posix() == SCRIPTS_DIR.as_posix()


class TestProjectRootFallback:
    def test_fallback_to_script_parent(self, tmp_path, monkeypatch):
        """When no leedevkit.toml or .git, falls back to script parent dir."""
        monkeypatch.chdir(tmp_path)
        from _bootstrap import _find_project_root

        root = _find_project_root()
        # Falls back to the scripts directory's parent
        assert root is not None


# ── Go support: mode-aware compose selection and version injection ───────────


def _write_container_compose(devkit_root: Path, language: str) -> Path:
    """Create a built-in compose file under container/<language>/."""
    lang_dir = devkit_root / "container" / language
    lang_dir.mkdir(parents=True, exist_ok=True)
    compose = lang_dir / "docker-compose.test.yml"
    compose.write_text(f"services:\n  {language}:\n    image: {language}:latest\n")
    return compose


class TestFindContainerComposeModeAware:
    """_find_container_compose must select by mode, never alphabetically."""

    def test_go_mode_picks_go(self, tmp_path, monkeypatch) -> None:
        """mode='go' selects container/go/ even when container/rust/ exists."""
        proj = tmp_path / "project"
        proj.mkdir()
        devkit = proj / ".leedevkit"
        _write_container_compose(devkit, "go")
        _write_container_compose(devkit, "rust")

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose("go")
        assert result is not None
        assert "container/go" in str(result)

    def test_api_mode_picks_rust(self, tmp_path, monkeypatch) -> None:
        """mode='api' selects container/rust/ even when go/ sorts first."""
        proj = tmp_path / "project"
        proj.mkdir()
        devkit = proj / ".leedevkit"
        _write_container_compose(devkit, "go")
        _write_container_compose(devkit, "rust")

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose("api")
        assert result is not None
        assert "container/rust" in str(result)

    def test_all_mode_go_only(self, tmp_path, monkeypatch) -> None:
        """mode='all' with only go.mod picks Go compose (no arbitrary fallback)."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "go.mod").write_text("module example\n")
        devkit = proj / ".leedevkit"
        _write_container_compose(devkit, "go")
        _write_container_compose(devkit, "rust")

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose("all")
        assert result is not None
        assert "container/go" in str(result)

    def test_all_mode_rust_only(self, tmp_path, monkeypatch) -> None:
        """mode='all' with only Cargo.toml picks Rust compose."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "Cargo.toml").write_text("[package]\nname = 'x'\n")
        devkit = proj / ".leedevkit"
        _write_container_compose(devkit, "go")
        _write_container_compose(devkit, "rust")

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose("all")
        assert result is not None
        assert "container/rust" in str(result)

    def test_all_mode_mixed_returns_none(self, tmp_path, monkeypatch) -> None:
        """mode='all' with both go.mod and Cargo.toml must not guess a language."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "go.mod").write_text("module example\n")
        (proj / "Cargo.toml").write_text("[package]\nname = 'x'\n")
        devkit = proj / ".leedevkit"
        _write_container_compose(devkit, "go")
        _write_container_compose(devkit, "rust")

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit)
        from _bootstrap import _find_container_compose

        assert _find_container_compose("all") is None


class TestGoVersionInjection:
    """GO_VERSION must load through the shared _load_toml loader."""

    def test_go_version_read_from_config(self, tmp_path, monkeypatch) -> None:
        """[services.go] go_version sets GO_VERSION env var."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.go]\nlang = "go"\ngo = true\ngo_version = "1.23"\n'
        )

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.delenv("GO_VERSION", raising=False)
        from _bootstrap import _inject_go_version_env

        _inject_go_version_env()
        assert os.environ.get("GO_VERSION") == "1.23"

    def test_go_version_env_takes_priority(self, tmp_path, monkeypatch) -> None:
        """Explicit GO_VERSION env var wins over config."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.go]\nlang = "go"\ngo = true\ngo_version = "1.23"\n'
        )

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.setenv("GO_VERSION", "1.22")
        from _bootstrap import _inject_go_version_env

        _inject_go_version_env()
        assert os.environ.get("GO_VERSION") == "1.22"

    def test_go_version_does_not_override_rust(self, tmp_path, monkeypatch) -> None:
        """Go version injection must not touch RUST_VERSION."""
        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.rust]\nlang = "rust"\ncargo = true\nrust_version = "1.84"\n'
        )

        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        monkeypatch.delenv("GO_VERSION", raising=False)
        from _bootstrap import _inject_go_version_env

        _inject_go_version_env()
        assert os.environ.get("GO_VERSION") is None
