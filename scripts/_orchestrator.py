#!/usr/bin/env python3
import argparse
import atexit
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import time
import typing
import urllib.request
import uuid
from datetime import datetime
from pathlib import Path
from types import FrameType

from _bootstrap import (
    PROJECT_ROOT,
    SCRIPTS_DIR,
    detect_compose_cmd,
    detect_engine,
)
from _cli_parser import CliParser
from _db_handler import DbHandler
from _init_handler import InitHandler
from _lifecycle import lifecycle_down
from _lock_manager import LockManager
from _logging import log_error, log_info, log_success, log_warn
from _run_handler import RunHandler
from _test_handler import TestHandler
from _test_modules import (
    _resolve_rust_service,
)


class Orchestrator:
    def __init__(self) -> None:
        self.results: dict[
            str, dict[str, typing.Any]
        ] = {}  # phase -> {status, duration, details}
        # Mapping tool names to their target service in docker-compose.test.yml
        rust_svc = self._detect_rust_service()
        self.tool_map = {
            "npm": "webdashboard",
            "cargo": rust_svc,
            "diesel": rust_svc,
        }
        self.parser = CliParser(self.tool_map).build()
        self.dry_run = False
        self.env_vars: dict[str, str] = {
            "USE_DOCKER": "true",
            "COMPOSE_PROJECT_NAME": "leedevkit-test",
            "PODMAN_COMPOSE_PROJECT_NAME": "leedevkit-test",
            "PROJECT_ROOT": str(PROJECT_ROOT),
            "CONTAINER_ENGINE": self.engine,
            "DOCKER_COMPOSE_CMD": " ".join(self.compose_engine),
        }
        self.needs_cleanup = False
        self.active_mode = "all"
        self.start_time = time.time()
        self.lock_fd: int | None = None
        self._db_handler = DbHandler(self)
        self._run_handler = RunHandler(self)
        self._test_handler = TestHandler(self)
        self._init_handler = InitHandler(self)
        self.register_traps()

    def _detect_rust_service(self) -> str:
        """Return the service name for Rust/cargo operations.

        Delegates to _resolve_rust_service() in _test_modules for a single
        source of truth. Falls back to 'apiserver' for backward compatibility
        with projects that pre-date auto-detection.
        """
        return _resolve_rust_service()

    def _build_mode_map(self) -> dict[str, str]:
        """Build mode_map from leedevkit.toml services.

        Maps each service to its test mode: Rust → 'api', TypeScript → 'web'.
        Falls back to hardcoded defaults for backward compatibility.
        """
        mode_map: dict[str, str] = {"all": "all"}
        try:
            from _bootstrap import PROJECT_ROOT as _project_root
            from _devkit_config import _load_toml

            config_toml = _project_root / "leedevkit.toml"
            if config_toml.exists():
                cfg = _load_toml(config_toml)
                services = cfg.get("services", {})
                for name, svc in services.items():
                    if isinstance(svc, dict):
                        lang = svc.get("lang", "")
                        if lang == "rust":
                            mode_map[name] = "api"
                        elif lang in ("typescript", "javascript"):
                            mode_map[name] = "web"
        except (OSError, ValueError, KeyError):
            pass
        # Backward-compat fallbacks
        mode_map.setdefault("apiserver", "api")
        mode_map.setdefault("agent-main", "api")
        mode_map.setdefault("webdashboard", "web")
        mode_map.setdefault("api", "api")
        mode_map.setdefault("web", "web")
        return mode_map

    def _inject_rust_version_env(self) -> None:
        """Read rust_version from leedevkit.toml and set RUST_VERSION env var.

        Defaults to '1.85' if not configured. Users can override with:
          [services.<name>]
          rust_version = "1.83"
        or via environment: RUST_VERSION=1.83 ./leedevkit test all
        """
        if "RUST_VERSION" in os.environ:
            return  # Explicit env override wins
        try:
            from _bootstrap import PROJECT_ROOT as _project_root
            from _devkit_config import _load_toml

            config_toml = _project_root / "leedevkit.toml"
            if config_toml.exists():
                cfg = _load_toml(config_toml)
                services = cfg.get("services", {})
                for _svc in services.values():
                    if isinstance(_svc, dict) and "rust_version" in _svc:
                        os.environ["RUST_VERSION"] = str(_svc["rust_version"])
                        return
        except (OSError, ValueError, KeyError):
            pass
        os.environ.setdefault("RUST_VERSION", "1.85")

    def register_traps(self) -> None:
        """Register signal handlers and exit hooks for cleanup."""

        atexit.register(self.cleanup)

        def signal_handler(sig: int, frame: FrameType | None) -> None:
            log_warn(f"Interrupted by signal {sig}. Triggering cleanup...")
            sys.exit(128 + sig)

        signal.signal(signal.SIGINT, signal_handler)
        signal.signal(signal.SIGTERM, signal_handler)
        if hasattr(signal, "SIGHUP"):
            signal.signal(signal.SIGHUP, signal_handler)

    def cleanup(self) -> None:
        """Final cleanup of infrastructure."""
        if not self.needs_cleanup or self.dry_run:
            return

        self.needs_cleanup = False
        os.environ["MODE"] = self.active_mode

        try:
            cleanup_log = PROJECT_ROOT / ".test_logs" / "cleanup.log"
            cleanup_log.parent.mkdir(exist_ok=True)
            with cleanup_log.open("a") as f:
                now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                f.write(f"\n--- Cleanup started at {now} ---\n")
                lifecycle_down(self.active_mode)
        except (OSError, ValueError):
            pass  # stderr may be closed during test teardown
        except Exception as e:
            log_error(f"❌ Cleanup failed with error: {e}")

        # Release and remove OS-level lock
        LockManager.release(
            self.lock_fd,
            os.environ.get("COMPOSE_PROJECT_NAME"),
        )
        self.lock_fd = None

        # Fix PTY hang by restoring sane terminal state at exit
        if sys.stdout.isatty():  # pragma: no cover
            subprocess.run(
                ["/bin/stty", "sane"],
                stderr=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                check=False,
            )

    @property
    def engine(self) -> str:
        """Detect container engine (podman or docker)."""
        return detect_engine()

    @property
    def compose_engine(self) -> list[str]:
        """Detect compose tool."""
        return detect_compose_cmd()

    @property
    def compose_engine_cmd(self) -> str:
        """Get compose tool as a single string for shell scripts."""
        return " ".join(self.compose_engine)

    def run(self) -> None:
        args = self.parser.parse_args()
        self.dry_run = args.dry_run
        if not args.command:
            self.parser.print_help()
            return
        if args.command in ("test", "run"):
            # Use dynamic project name for perfect run isolation
            suffix = uuid.uuid4().hex[:8]
            project_name = f"leedevkit-test-{suffix}"
            self.env_vars["COMPOSE_PROJECT_NAME"] = project_name
            self.env_vars["PODMAN_COMPOSE_PROJECT_NAME"] = project_name
            os.environ["COMPOSE_PROJECT_NAME"] = project_name
            os.environ["PODMAN_COMPOSE_PROJECT_NAME"] = project_name

            # Acquire an OS-level file lock to prevent garbage collection
            self.lock_fd = LockManager.acquire(project_name)
            if self.lock_fd is None:
                log_warn(
                    f"Failed to lock project {project_name}, "
                    "garbage collector might sweep this run!"
                )

        if args.command == "test":
            self.handle_test(args)
        elif args.command == "manage":
            self.handle_manage(args)
        elif args.command == "run":
            self.handle_run(args)
        elif args.command == "update":
            self.handle_update(args)

    def handle_test(self, args: argparse.Namespace) -> None:
        self._test_handler.handle_test(args)

    def run_phase(self, phase_name: str, mode: str, args: argparse.Namespace) -> None:
        self._test_handler.run_phase(phase_name, mode, args)

    def handle_manage(self, args: argparse.Namespace) -> None:
        """Handle 'manage' commands using a dispatch map for simplicity."""
        sub = args.subcommand

        simple_dispatch: dict[str, typing.Callable[[], typing.Any]] = {
            "init": lambda: self.handle_init(getattr(args, "force", False)),
            "sync:api": lambda: self.execute_safe(
                ["/bin/bash", "-c", f'source "{SCRIPTS_DIR}/_sync-api.sh"']
            ),
            "test:infra": self.handle_test_infra,
            "migrate:run": lambda: self.handle_diesel(["migration", "run"]),
            "migrate:revert": lambda: self.handle_diesel(["migration", "revert"]),
            "migrate:status": lambda: self.handle_diesel(["migration", "list"]),
            "db:setup": lambda: self.run_phase("Database Setup", "all", args),
            "prebuild": lambda: self.run_phase("Prebuild", "all", args),
            "fmt:infra": self.handle_fmt_infra,
            "doctor": self.handle_doctor,
            "verify:infra": self.handle_verify_infra,
        }

        if sub == "skills":
            self.handle_skills(args)
            return

        if sub in simple_dispatch:
            simple_dispatch[sub]()
            return

        if sub == "db:query":
            self.handle_db_query(args)
        elif sub in ["up", "down", "ps", "clean", "logs"]:
            target_env: str = getattr(args, "env", "dev")
            files = self.get_compose_files(target_env)
            if sub == "clean":
                cmd = self.compose_engine + files + ["down", "-v"]
            else:
                cmd = self.compose_engine + files + [sub]
                if sub == "up":
                    cmd.append("-d")
                if sub == "logs":
                    cmd.append("-f")
                    if args.service:
                        cmd.append(args.service)
            self.execute_safe(cmd)
        elif sub == "exec":
            files = self.get_compose_files("dev")
            cmd = (
                self.compose_engine
                + files
                + ["exec", args.service]
                + (args.args if args.args else ["bash"])
            )
            self.execute_safe(cmd)
        else:  # pragma: no cover
            self.parser.error(f"Unknown subcommand: {sub}")

    def handle_test_infra(self) -> None:
        """Run all test files with coverage enforcement."""
        self._test_handler.handle_test_infra()

    def handle_run(self, args: argparse.Namespace) -> None:
        self._run_handler.handle_run(args)

    def _is_service_running(self, service: str) -> bool:
        return self._run_handler.is_service_running(service)

    # ── Self-update ────────────────────────────────────────────────────────────

    def _devkit_root(self) -> Path:
        """Return the directory this devkit is installed in (parent of scripts/)."""
        return Path(__file__).resolve().parent.parent

    def _latest_release_version(self) -> str:
        """Return the latest release tag (e.g. 'v0.2.0') from GitHub Releases."""
        api = "https://api.github.com/repos/vkaylee/leedevkit/releases/latest"
        req = urllib.request.Request(
            api, headers={"Accept": "application/vnd.github+json"}
        )
        try:
            with urllib.request.urlopen(req, timeout=15) as r:
                data = json.load(r)
        except (urllib.error.URLError, OSError, json.JSONDecodeError) as e:
            raise RuntimeError(f"Could not reach GitHub Releases: {e}") from e
        tag = data.get("tag_name")
        if not tag:
            raise RuntimeError("GitHub releases/latest returned no tag_name")
        return tag

    def handle_update(self, args: argparse.Namespace) -> None:
        """Download a release tarball and overlay it onto this devkit install."""
        root = self._devkit_root()
        current = (root / "VERSION").read_text().strip()  # e.g. "0.1.0"
        target = args.version  # None → latest

        if target is None:
            target = self._latest_release_version()  # e.g. "v0.2.0"

        ver = target.lstrip("v")  # tarball version, no "v"
        if ver == current:
            log_info(f"Already on latest ({current}).")
            return

        log_info(f"Updating {current} → {target}")

        # Backup current install to <root>.bak before overwriting.
        backup = root.with_name(root.name + ".bak")
        if backup.exists():
            shutil.rmtree(backup)
        shutil.move(str(root), str(backup))
        log_info(f"Backed up current install to {backup.name}/")

        # Download into a temp dir, then move the extracted tree onto root.
        tmp_extract = root.parent / f".leedevkit-update-{uuid.uuid4().hex[:8]}"
        url = (
            f"https://github.com/vkaylee/leedevkit/releases/download/"
            f"{target}/leedevkit-{ver}.tar.gz"
        )
        try:
            log_info(f"Downloading {url} ...")
            self._download_and_extract(url, tmp_extract)
            if root.exists():
                shutil.rmtree(root)
            shutil.move(str(tmp_extract), str(root))
        except Exception:
            # Roll back on any failure — broad catch is intentional here
            # to guarantee rollback regardless of the error type.
            if root.exists():
                shutil.rmtree(root)
            shutil.move(str(backup), str(root))
            log_warn("Update failed; rolled back to previous version.")
            raise

        new_ver = (root / "VERSION").read_text().strip()
        log_success(f"Updated leedevkit {current} → {new_ver}")
        log_info(f"Previous version kept at {backup.name}/ (safe to remove).")

    def _handle_run_npm(
        self,
        compose_cmd: list[str],
        tool_args: list[str],
        service: str,
        is_running: bool = True,
    ) -> None:
        """Run npm/bun commands via compose exec/run into container."""
        self._run_handler._handle_run_npm(compose_cmd, tool_args, service, is_running)

    def _handle_run_cargo(
        self, compose_cmd: list[str], tool_args: list[str], service: str
    ) -> None:
        """Run cargo commands via compose inside the Rust service container."""
        self._run_handler._handle_run_cargo(compose_cmd, tool_args, service)

    def handle_lint_infra(self) -> None:
        self._test_handler.handle_lint_infra()

    def handle_fmt_infra(self) -> None:
        self._test_handler.handle_fmt_infra()

    def handle_verify_infra(self) -> None:
        self._test_handler.handle_verify_infra()

    def handle_init(self, force: bool = False) -> None:
        """Set up project with per-project devkit install.

        Flow:
          1. Ensure .leedevkit/ exists (download release tarball if needed)
          2. Create project-local .venv inside .leedevkit/
          3. Copy base AI rules from devkit → project (not symlinks)
          4. Create ./leedevkit wrapper → .leedevkit/bin/leedevkit
          5. Install community skills from catalog/TOML
          6. Pin devkit version in leedevkit.toml
        """
        self._init_handler.handle_init(force=force)

    def _detect_legacy_symlinks(self, agent_dir: Path) -> list[tuple[str, str]]:
        """Detect symlinks in .agent/ that point to the old global install."""
        return self._init_handler._detect_legacy_symlinks(agent_dir)

    def _read_installed_version(self, leedevkit_dir: Path) -> str | None:
        """Read the version installed in .leedevkit/, or None if not installed."""
        return self._init_handler._read_installed_version(leedevkit_dir)

    def _install_devkit(
        self, project_root: Path, target_dir: Path, version: str, force: bool = False
    ) -> None:
        """Download and install devkit into target_dir (.leedevkit/)."""
        self._init_handler._install_devkit(project_root, target_dir, version, force=force)

    def _extract_from_source(self, source_root: Path, target_dir: Path) -> None:
        """Copy devkit artifacts from a source directory into target_dir."""
        self._init_handler._extract_from_source(source_root, target_dir)

    def _download_and_extract(self, url: str, target_dir: Path) -> None:
        """Download a release tarball and extract into target_dir."""
        self._init_handler._download_and_extract(url, target_dir)

    def handle_skills(self, args: argparse.Namespace) -> None:
        """Manage community add-on skills (delegated to SkillsManager)."""
        from _skills_manager import SkillsManager

        SkillsManager().dispatch(args)

    def handle_doctor(self) -> None:
        from _devkit_config import load_project_config, resolve_ai_rules

        log_info("🩺 Running LeeDevKit System Doctor...")

        # ── Project config ──
        try:
            cfg = load_project_config()
            name = cfg.get("project", {}).get("name", "unknown")
            targets = list(cfg.get("targets", {}).keys())
            log_success(f"✅ Project: {name} (targets: {', '.join(targets)})")
        except (OSError, ValueError, KeyError) as e:
            log_warn(f"⚠️  leedevkit.toml: {e}")

        # ── .agent directory (per-project, real dir not symlink) ──
        agent_dir = PROJECT_ROOT / ".agent"
        if agent_dir.is_symlink():
            log_warn(
                "⚠️  .agent is a symlink — expected real directory (run: leedevkit init)"
            )
        elif agent_dir.is_dir():
            rules_dir = agent_dir / "rules"
            rule_count = len(list(rules_dir.glob("*.md"))) if rules_dir.exists() else 0
            log_success(f"✅ .agent/ (real directory, {rule_count} rulebooks)")
        else:
            log_warn("⚠️  .agent directory missing (run: leedevkit init)")

        # ── DevKit install location ──
        from _devkit_config import get_devkit_root

        try:
            dk = get_devkit_root()
            dk_version = (
                (dk / "VERSION").read_text().strip()
                if (dk / "VERSION").exists()
                else "?"
            )
            log_success(f"✅ DevKit: {dk} (v{dk_version})")
        except (OSError, ValueError) as e:
            log_warn(f"⚠️  DevKit: {e}")

        # ── AI rules ──
        try:
            rules = resolve_ai_rules()
            log_success(f"✅ AI rules: {len(rules)} files loaded")
        except (OSError, ValueError, KeyError) as e:
            log_warn(f"⚠️  AI rules: {e}")

        engine = self.engine
        log_success(f"✅ Container Engine: {engine}")

        default_ports = [3000, 8000, 5432]
        connection_timeout = 0.5
        for port in default_ports:
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s.settimeout(connection_timeout)
            if s.connect_ex(("127.0.0.1", port)) == 0:
                log_info(f"⚠️  Port {port} is occupied")
            s.close()

        if (PROJECT_ROOT / ".venv").exists():
            log_success("✅ Virtual Environment: Found")
        else:
            log_info("💡 Virtual Environment: Missing (will be created on next run)")

        if engine:
            res = subprocess.run(
                [engine, "ps", "--format", "{{.Names}}"],
                capture_output=True,
                text=True,
                stdin=subprocess.DEVNULL,
                check=False,
            )
            names = res.stdout.splitlines()
            for r in ["leedevkit-dev-db", "leedevkit-dev-api"]:
                if any(r in n for n in names):
                    log_success(f"✅ Container {r}: Running")

    def handle_db_query(self, args: argparse.Namespace) -> None:
        self._db_handler.handle_db_query(args)

    def handle_diesel(self, args: list[str]) -> None:
        """Wrapper for diesel commands."""
        self._db_handler.handle_diesel(args)

    def get_compose_files(self, env: str) -> list[str]:
        return self._db_handler.get_compose_files(env)

    def execute_safe(
        self, cmd: list[str], env: dict[str, str] | None = None, timeout: int = 1800
    ) -> None:
        if self.dry_run:
            log_info(f"🔍 Dry-run: Executing {' '.join(cmd)}")
            return

        safe_run = SCRIPTS_DIR / "_safe_run.py"
        full_cmd = [sys.executable, str(safe_run), str(timeout)] + cmd
        current_env = env if env else os.environ.copy()
        current_env.update(self.env_vars)

        result = subprocess.run(
            full_cmd, cwd=PROJECT_ROOT, env=current_env, check=False
        )
        if result.returncode != 0:
            log_error(f"❌ Command failed: {' '.join(cmd)}")
            sys.exit(result.returncode)

    def print_test_summary(self, target: str) -> None:
        self._test_handler.print_test_summary(target)

    def handle_db_setup_phase(self) -> bool:
        return self._db_handler.handle_db_setup_phase()

    def handle_prebuild_phase(self) -> bool:
        return self._db_handler.handle_prebuild_phase()


if __name__ == "__main__":  # pragma: no cover
    Orchestrator().run()
