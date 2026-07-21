#!/usr/bin/env python3
import argparse
import atexit
import os
import signal
import subprocess
import sys
import time
import typing
import uuid
from types import FrameType

from _bootstrap import (
    PROJECT_ROOT,
    SCRIPTS_DIR,
    detect_compose_cmd,
    detect_engine,
)
from _cli_parser import CliParser
from _db_handler import DbHandler
from _devkit_config import inject_rust_version_env
from _init_handler import InitHandler
from _lifecycle import lifecycle_down
from _lock_manager import LockManager
from _logging import log_error, log_info, log_warn
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

    def _inject_rust_version_env(self) -> None:
        """Set RUST_VERSION env var from leedevkit.toml (delegates to _devkit_config)."""
        from _bootstrap import PROJECT_ROOT as _prj_root

        inject_rust_version_env(_prj_root)

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
                now = time.strftime("%Y-%m-%d %H:%M:%S")
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
            from _update_handler import handle_update as _do_update

            _do_update(args.version)

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
            "test:infra": lambda: self._test_handler.handle_test_infra(),
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

    # ── CLI Facade — public API methods (called from dispatch map + tests) ───

    def handle_lint_infra(self) -> None:
        self._test_handler.handle_lint_infra()

    def handle_fmt_infra(self) -> None:
        self._test_handler.handle_fmt_infra()

    def handle_verify_infra(self) -> None:
        self._test_handler.handle_verify_infra()

    def handle_init(self, force: bool = False) -> None:
        """Set up project with per-project devkit install."""
        self._init_handler.handle_init(force=force)

    def handle_skills(self, args: argparse.Namespace) -> None:
        """Manage community add-on skills (delegated to SkillsManager)."""
        from _skills_manager import SkillsManager

        SkillsManager().dispatch(args)

    def handle_doctor(self) -> None:
        """Run system health check (delegated to _doctor module)."""
        from _doctor import run_doctor

        run_doctor(self.engine)

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
