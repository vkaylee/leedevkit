"""Tests for Rust container mode — service auto-detection, init, compose resolution.

Covers:
  - _resolve_rust_service / _resolve_pkg_flag
  - _detect_rust_service / _build_mode_map
  - handle_init auto-detection of Cargo.toml
  - Compose file priority resolution
"""

from __future__ import annotations

from pathlib import Path
from unittest.mock import patch

# ── Helpers ──────────────────────────────────────────────────────────────────


def _write_toml(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def _make_project_with_rust_service(tmp_path: Path, service_name: str = "rust") -> Path:
    """Create a project with leedevkit.toml that has a single Rust service."""
    proj = tmp_path / "project"
    proj.mkdir(parents=True)
    (proj / ".git").mkdir()
    _write_toml(
        proj / "leedevkit.toml",
        f"""[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["rust"]

[services.{service_name}]
lang = "rust"
cargo = true

[targets]
all = ["{service_name}"]
""",
    )
    return proj


def _make_project_with_multi_services(tmp_path: Path) -> Path:
    """Create a project with both Rust and TypeScript services (backward compat)."""
    proj = tmp_path / "project"
    proj.mkdir(parents=True)
    (proj / ".git").mkdir()
    _write_toml(
        proj / "leedevkit.toml",
        """[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["rust", "typescript"]

[services.apiserver]
lang = "rust"
cargo = true
ports = [3000]

[services.webdashboard]
lang = "typescript"
ports = [5173]

[targets]
api = ["apiserver"]
web = ["webdashboard"]
all = ["apiserver", "webdashboard"]
""",
    )
    return proj


def _setup_bootstrap_paths(
    monkeypatch, project_root: Path, devkit_root: Path | None = None
):
    """Override _bootstrap module-level paths to point at temp directories.

    _bootstrap computes PROJECT_ROOT, SCRIPTS_DIR, DEVKIT_ROOT at import time
    and caches them. This helper re-points them for isolated tests.
    """
    import _bootstrap

    monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", project_root)
    if devkit_root is not None:
        monkeypatch.setattr(_bootstrap, "SCRIPTS_DIR", devkit_root / "scripts")
        monkeypatch.setattr(_bootstrap, "DEVKIT_ROOT", devkit_root)


# ── _resolve_rust_service ──────────────────────────────────────────────────


class TestResolveRustService:
    def test_no_config_returns_apiserver(self, tmp_path, monkeypatch):
        """Without leedevkit.toml, fall back to 'apiserver'."""
        proj = tmp_path / "empty"
        proj.mkdir()
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service() == "apiserver"

    def test_rust_service_detected(self, tmp_path, monkeypatch):
        """With [services.rust] → returns 'rust'."""
        proj = _make_project_with_rust_service(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service() == "rust"

    def test_custom_service_name(self, tmp_path, monkeypatch):
        """With [services.mybackend] lang=rust → returns 'mybackend'."""
        proj = _make_project_with_rust_service(tmp_path, "mybackend")
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service() == "mybackend"

    def test_skips_non_cargo_service(self, tmp_path, monkeypatch):
        """Services without cargo=true are ignored."""
        proj = tmp_path / "project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        _write_toml(
            proj / "leedevkit.toml",
            """[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["python"]

[services.myservice]
lang = "python"
""",
        )
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service() == "apiserver"

    def test_component_filter_matches(self, tmp_path, monkeypatch):
        """When component_filter is a valid Rust service, return it."""
        proj = _make_project_with_rust_service(tmp_path, "mybackend")
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service("mybackend") == "mybackend"

    def test_component_filter_no_match_returns_first(self, tmp_path, monkeypatch):
        """When component_filter doesn't match, return first Rust service."""
        proj = _make_project_with_rust_service(tmp_path, "mybackend")
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service("nonexistent") == "mybackend"

    def test_backward_compat_apiserver(self, tmp_path, monkeypatch):
        """Old project with [services.apiserver] → returns 'apiserver'."""
        proj = _make_project_with_multi_services(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service() == "apiserver"

    def test_component_filter_apiserver(self, tmp_path, monkeypatch):
        """Old project with component_filter='apiserver' → returns 'apiserver'."""
        proj = _make_project_with_multi_services(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _test_modules import _resolve_rust_service

        assert _resolve_rust_service("apiserver") == "apiserver"


# ── _resolve_pkg_flag ──────────────────────────────────────────────────────


class TestResolvePkgFlag:
    def test_known_apiserver(self):
        from _test_modules import _resolve_pkg_flag

        assert _resolve_pkg_flag("apiserver") == "--package apiserver"

    def test_known_agent_main(self):
        from _test_modules import _resolve_pkg_flag

        assert _resolve_pkg_flag("agent-main") == "--package agent"

    def test_unknown_service_returns_workspace(self):
        from _test_modules import _resolve_pkg_flag

        assert _resolve_pkg_flag("rust") == "--workspace"

    def test_empty_returns_workspace(self):
        from _test_modules import _resolve_pkg_flag

        assert _resolve_pkg_flag("") == "--workspace"


# ── Orchestrator: _detect_rust_service / _build_mode_map ───────────────────


class TestOrchestratorRustDetection:
    def test_detect_rust_service_delegates(self, tmp_path, monkeypatch):
        """_detect_rust_service delegates to _resolve_rust_service."""
        proj = _make_project_with_rust_service(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch._detect_rust_service() == "rust"

    def test_detect_rust_service_fallback(self, tmp_path, monkeypatch):
        """No config → returns 'apiserver'."""
        proj = tmp_path / "empty"
        proj.mkdir()
        _setup_bootstrap_paths(monkeypatch, proj)
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch._detect_rust_service() == "apiserver"

    def test_tool_map_uses_detected_service(self, tmp_path, monkeypatch):
        """tool_map['cargo'] returns the detected Rust service."""
        proj = _make_project_with_rust_service(tmp_path, "mybackend")
        _setup_bootstrap_paths(monkeypatch, proj)
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch.tool_map["cargo"] == "mybackend"
            assert orch.tool_map["diesel"] == "mybackend"
            assert orch.tool_map["npm"] == "webdashboard"  # unchanged

    def test_tool_map_backward_compat(self, tmp_path, monkeypatch):
        """Old project: tool_map['cargo'] returns 'apiserver'."""
        proj = _make_project_with_multi_services(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            assert orch.tool_map["cargo"] == "apiserver"

    def test_build_mode_map_with_rust_service(self, tmp_path, monkeypatch):
        """build_mode_map includes services from config."""
        proj = _make_project_with_rust_service(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _devkit_config import build_mode_map

        with patch("_devkit_config._find_project_root", return_value=proj):
            mode_map = build_mode_map()
        assert mode_map["rust"] == "api"
        assert mode_map["all"] == "all"

    def test_build_mode_map_multi_service(self, tmp_path, monkeypatch):
        """build_mode_map handles Rust + TS services."""
        proj = _make_project_with_multi_services(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        from _devkit_config import build_mode_map

        with patch("_devkit_config._find_project_root", return_value=proj):
            mode_map = build_mode_map()
        assert mode_map["apiserver"] == "api"
        assert mode_map["webdashboard"] == "web"

    def test_build_mode_map_fallback(self, tmp_path, monkeypatch):
        """No config → fallback services present."""
        proj = tmp_path / "empty"
        proj.mkdir()
        _setup_bootstrap_paths(monkeypatch, proj)
        from _devkit_config import build_mode_map

        with patch("_devkit_config._find_project_root", return_value=proj):
            mode_map = build_mode_map()
        assert mode_map["apiserver"] == "api"
        assert mode_map["agent-main"] == "api"
        assert mode_map["webdashboard"] == "web"


# ── handle_init: auto-detect Cargo.toml ────────────────────────────────────


class TestInitRustAutoDetect:
    def test_init_detects_cargo_toml(self, tmp_path, monkeypatch):
        """When Cargo.toml exists, init uses leedevkit.rust.toml template."""
        proj = tmp_path / "rust-project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        (proj / "Cargo.toml").write_text('[package]\nname = "mycrate"\n')
        source = _make_devkit_source_with_rust_template(tmp_path)
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(proj)

        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=False)

        toml_content = (proj / "leedevkit.toml").read_text()
        # Rust template has single service 'rust', no multi-service defaults
        assert "[services.rust]" in toml_content
        assert 'languages = ["rust"]' in toml_content
        assert 'all = ["rust"]' in toml_content
        assert "[services.apiserver]" not in toml_content
        assert "[services.webdashboard]" not in toml_content

    def test_init_without_cargo_toml_uses_default(self, tmp_path, monkeypatch):
        """Without Cargo.toml, init uses leedevkit.default.toml template."""
        proj = tmp_path / "default-project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        source = _make_devkit_source_with_rust_template(tmp_path)
        monkeypatch.setenv("DEVKIT_LOCAL_PATH", str(source))
        monkeypatch.chdir(proj)

        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch.handle_init(force=False)

        toml_content = (proj / "leedevkit.toml").read_text()
        # Default template has multi-service setup
        assert "[services.apiserver]" in toml_content
        assert "[services.webdashboard]" in toml_content


def _make_devkit_source_with_rust_template(tmp_path: Path) -> Path:
    """Create a fake devkit source with both default and rust templates."""
    src = tmp_path / "devkit-src"
    src.mkdir(parents=True, exist_ok=True)
    (src / "VERSION").write_text("0.1.0")
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
    templates = src / "templates"
    templates.mkdir()
    (templates / "leedevkit.default.toml").write_text(
        """[devkit]
version = "latest"

[project]
name = "DefaultProject"
languages = ["rust", "typescript"]

[services.apiserver]
lang = "rust"
cargo = true

[services.webdashboard]
lang = "typescript"

[targets]
all = ["apiserver", "webdashboard"]
"""
    )
    (templates / "leedevkit.rust.toml").write_text(
        """[devkit]
version = "latest"

[project]
name = "RustProject"
languages = ["rust"]

[services.rust]
lang = "rust"
cargo = true

[targets]
all = ["rust"]
"""
    )
    agent = src / ".agent"
    agent.mkdir()
    rules = agent / "rules"
    rules.mkdir()
    (rules / "coding-standards.md").write_text("# Coding Standards\n")
    (rules / "testing-standards.md").write_text("# Testing Standards\n")
    (agent / "skills-catalog.toml").write_text("")
    return src


# ── Compose file resolution ─────────────────────────────────────────────────


class TestComposeFileResolution:
    def test_project_compose_takes_priority(self, tmp_path, monkeypatch):
        """.compose/docker-compose.test.yml takes priority if it exists."""
        proj = _make_project_with_rust_service(tmp_path)
        leedevkit = proj / ".leedevkit"

        # Create project-level compose file
        compose_dir = proj / ".compose"
        compose_dir.mkdir()
        project_compose = compose_dir / "docker-compose.test.yml"
        project_compose.write_text("services:\n  custom:\n    image: alpine\n")

        # Create devkit-level compose file (under container/rust/)
        container_dir = leedevkit / "container" / "rust"
        container_dir.mkdir(parents=True)
        devkit_compose = container_dir / "docker-compose.test.yml"
        devkit_compose.write_text("services:\n  rust:\n    build: .\n")

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)

        from _bootstrap import bootstrap_env

        env = bootstrap_env()
        compose_cmd = env.get("DOCKER_COMPOSE_CMD", "")
        assert ".compose/docker-compose.test.yml" in compose_cmd
        assert "container/rust/docker-compose.test.yml" not in compose_cmd

    def test_fallback_to_devkit_container(self, tmp_path, monkeypatch):
        """When no .compose/ exists, use .leedevkit/container/rust/."""
        proj = _make_project_with_rust_service(tmp_path)
        leedevkit = proj / ".leedevkit"

        # Create only devkit-level compose file (no .compose/ in project)
        container_dir = leedevkit / "container" / "rust"
        container_dir.mkdir(parents=True)
        devkit_compose = container_dir / "docker-compose.test.yml"
        devkit_compose.write_text("services:\n  rust:\n    build: .\n")

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)

        from _bootstrap import bootstrap_env

        env = bootstrap_env()
        compose_cmd = env.get("DOCKER_COMPOSE_CMD", "")
        assert "container/rust/docker-compose.test.yml" in compose_cmd

    def test_no_compose_file_anywhere_still_works(self, tmp_path, monkeypatch):
        """Even without any compose file, the path is set (runtime will fail later)."""
        proj = tmp_path / "empty"
        proj.mkdir()
        _setup_bootstrap_paths(monkeypatch, proj)

        from _bootstrap import bootstrap_env

        env = bootstrap_env()
        compose_cmd = env.get("DOCKER_COMPOSE_CMD", "")
        # Should reference the devkit container path as fallback (or project .compose/)
        assert (
            "container/" in compose_cmd
            or ".compose/docker-compose.test.yml" in compose_cmd
        )


# ── Rust version injection ─────────────────────────────────────────────────


class TestRustVersionInjection:
    def test_default_version_is_1_85(self, tmp_path, monkeypatch):
        """Without config, RUST_VERSION defaults to '1.85'."""
        proj = _make_project_with_rust_service(tmp_path)
        _setup_bootstrap_paths(monkeypatch, proj)
        # Clear any pre-existing RUST_VERSION
        monkeypatch.delenv("RUST_VERSION", raising=False)

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._inject_rust_version_env()
            import os

            assert os.environ.get("RUST_VERSION") == "1.85"

    def test_reads_from_config(self, tmp_path, monkeypatch):
        """Reads rust_version from leedevkit.toml [services.rust]."""
        proj = tmp_path / "project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        _write_toml(
            proj / "leedevkit.toml",
            """[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["rust"]

[services.rust]
lang = "rust"
cargo = true
rust_version = "1.83"
""",
        )
        _setup_bootstrap_paths(monkeypatch, proj)
        monkeypatch.delenv("RUST_VERSION", raising=False)

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._inject_rust_version_env()
            import os

            assert os.environ.get("RUST_VERSION") == "1.83"

    def test_env_var_takes_priority(self, tmp_path, monkeypatch):
        """Explicit RUST_VERSION env var wins over config."""
        proj = tmp_path / "project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        _write_toml(
            proj / "leedevkit.toml",
            """[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["rust"]

[services.rust]
lang = "rust"
cargo = true
rust_version = "1.83"
""",
        )
        _setup_bootstrap_paths(monkeypatch, proj)
        monkeypatch.setenv("RUST_VERSION", "1.80")

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._inject_rust_version_env()
            import os

            assert os.environ.get("RUST_VERSION") == "1.80"

    def test_reads_from_any_rust_service(self, tmp_path, monkeypatch):
        """Finds rust_version on any service with lang=rust, cargo=true."""
        proj = tmp_path / "project"
        proj.mkdir(parents=True)
        (proj / ".git").mkdir()
        _write_toml(
            proj / "leedevkit.toml",
            """[devkit]
version = "0.1.0"

[project]
name = "test"
languages = ["rust"]

[services.mybackend]
lang = "rust"
cargo = true
rust_version = "1.82"

[services.rust]
lang = "rust"
cargo = true
rust_version = "1.84"
""",
        )
        _setup_bootstrap_paths(monkeypatch, proj)
        monkeypatch.delenv("RUST_VERSION", raising=False)

        from _orchestrator import Orchestrator

        with patch.object(Orchestrator, "register_traps", return_value=None):
            orch = Orchestrator()
            orch._inject_rust_version_env()
            import os

            # Should pick the first one found (mybackend)
            assert os.environ.get("RUST_VERSION") == "1.82"


# ── _find_container_compose auto-discovery ─────────────────────────────────


class TestFindContainerCompose:
    def test_finds_rust_subdirectory(self, tmp_path, monkeypatch):
        """Finds docker-compose.test.yml under container/rust/."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"

        # Create container/rust/docker-compose.test.yml
        rust_dir = leedevkit / "container" / "rust"
        rust_dir.mkdir(parents=True)
        (rust_dir / "docker-compose.test.yml").write_text(
            "services:\n  rust:\n    build: .\n"
        )

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose()
        assert result is not None
        assert result.name == "docker-compose.test.yml"
        assert "container/rust" in str(result)

    def test_returns_none_when_no_container_dir(self, tmp_path, monkeypatch):
        """Returns None when container/ directory doesn't exist."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"
        leedevkit.mkdir()
        # No container/ directory at all

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        assert _find_container_compose() is None

    def test_returns_none_when_no_compose_in_subdirs(self, tmp_path, monkeypatch):
        """Returns None when container/ exists but has no compose files."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"

        # container/rust/ exists but no compose file
        rust_dir = leedevkit / "container" / "rust"
        rust_dir.mkdir(parents=True)

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        assert _find_container_compose() is None

    def test_skips_pycache_directory(self, tmp_path, monkeypatch):
        """Skips __pycache__ subdirectories."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"

        container_dir = leedevkit / "container"
        container_dir.mkdir(parents=True)

        # __pycache__ should be skipped
        pycache = container_dir / "__pycache__"
        pycache.mkdir()
        (pycache / "docker-compose.test.yml").write_text("should be ignored\n")

        # No real language directories — should return None
        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        assert _find_container_compose() is None

    def test_skips_files_in_container_dir(self, tmp_path, monkeypatch):
        """Non-directory entries in container/ are skipped."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"

        container_dir = leedevkit / "container"
        container_dir.mkdir(parents=True)
        (container_dir / "README.md").write_text("# docs\n")
        (container_dir / "some_random_file").write_text("ignored\n")

        # No real language directories with compose files
        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        assert _find_container_compose() is None

    def test_picks_first_alphabetically(self, tmp_path, monkeypatch):
        """When multiple language dirs exist, picks first sorted alphabetically."""
        proj = tmp_path / "project"
        proj.mkdir()
        leedevkit = proj / ".leedevkit"

        # Create both rust/ and go/ — rust comes before go alphabetically
        for lang in ["rust", "go"]:
            lang_dir = leedevkit / "container" / lang
            lang_dir.mkdir(parents=True)
            (lang_dir / "docker-compose.test.yml").write_text(
                f"services:\n  {lang}:\n    build: .\n"
            )

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import _find_container_compose

        result = _find_container_compose()
        assert result is not None
        # "go" < "rust" alphabetically, so go/ should be picked first
        assert "container/go" in str(result)

    def test_integration_bootstrap_env_uses_container_rust(self, tmp_path, monkeypatch):
        """bootstrap_env() returns compose path under container/rust/ when no project override."""
        proj = _make_project_with_rust_service(tmp_path)
        leedevkit = proj / ".leedevkit"

        # Create container/rust/compose (no .compose/ in project)
        rust_dir = leedevkit / "container" / "rust"
        rust_dir.mkdir(parents=True)
        (rust_dir / "docker-compose.test.yml").write_text(
            "services:\n  rust:\n    build: .\n"
        )

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import bootstrap_env

        env = bootstrap_env()
        compose_cmd = env.get("DOCKER_COMPOSE_CMD", "")
        assert "container/rust/docker-compose.test.yml" in compose_cmd

    def test_integration_bootstrap_env_falls_back_to_compose(
        self, tmp_path, monkeypatch
    ):
        """bootstrap_env() still uses .compose/ when both exist (project priority)."""
        proj = _make_project_with_rust_service(tmp_path)
        leedevkit = proj / ".leedevkit"

        # Create project-level compose (Priority 1)
        compose_dir = proj / ".compose"
        compose_dir.mkdir()
        (compose_dir / "docker-compose.test.yml").write_text(
            "services:\n  custom:\n    image: alpine\n"
        )

        # Create devkit-level compose (Priority 2)
        rust_dir = leedevkit / "container" / "rust"
        rust_dir.mkdir(parents=True)
        (rust_dir / "docker-compose.test.yml").write_text(
            "services:\n  rust:\n    build: .\n"
        )

        _setup_bootstrap_paths(monkeypatch, proj, leedevkit)
        from _bootstrap import bootstrap_env

        env = bootstrap_env()
        compose_cmd = env.get("DOCKER_COMPOSE_CMD", "")
        assert ".compose/docker-compose.test.yml" in compose_cmd
        assert "container" not in compose_cmd
