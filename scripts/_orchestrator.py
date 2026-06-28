#!/usr/bin/env python3
import argparse
import atexit
import contextlib
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import time
import typing
from datetime import datetime
from pathlib import Path
from types import FrameType

from _arg_sanitizer import sanitize
from _bootstrap import (
    PROJECT_ROOT,
    SCRIPTS_DIR,
    detect_compose_cmd,
    detect_engine,
)
from _lifecycle import lifecycle_down
from _lifecycle import lifecycle_up as _lifecycle_up
from _test_modules import (
    leedevkit_run_coverage,
    leedevkit_run_integration,
    leedevkit_run_lint,
    leedevkit_run_unit,
)

# LeeDevKit Enterprise Orchestrator Core
# ==============================================================================


class Colors:
    GREEN = "\033[0;32m"
    RED = "\033[0;31m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    BOLD = "\033[1m"
    NC = "\033[0m"


def log_info(msg: str) -> None:
    print(f"{Colors.BLUE}{Colors.BOLD}ℹ️ {msg}{Colors.NC}", file=sys.stderr, flush=True)


def log_success(msg: str) -> None:
    print(
        f"{Colors.GREEN}{Colors.BOLD}✅ {msg}{Colors.NC}", file=sys.stderr, flush=True
    )


def log_warn(msg: str) -> None:
    print(
        f"{Colors.YELLOW}{Colors.BOLD}⚠️ {msg}{Colors.NC}", file=sys.stderr, flush=True
    )


def log_error(msg: str, file: typing.TextIO = sys.stderr) -> None:
    print(f"{Colors.RED}{Colors.BOLD}❌ {msg}{Colors.NC}", file=file, flush=True)


def _resolve_targets() -> list[str]:
    """Resolve valid test targets from leedevkit.toml, falling back to defaults."""
    try:
        from _devkit_config import load_project_config

        cfg = load_project_config()
        targets = cfg.get("targets", {})
        if targets:
            return list(targets.keys())
    except Exception:
        pass
    return ["all", "api", "web", "apiserver", "agent-main", "webdashboard", "infra"]


class Orchestrator:
    def __init__(self) -> None:
        self.results: dict[
            str, dict[str, typing.Any]
        ] = {}  # phase -> {status, duration, details}
        prog_name = "leedevkit"
        self.parser = argparse.ArgumentParser(
            prog=prog_name,
            description="LeeDevKit Enterprise Orchestrator",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples:\n  leedevkit test all\n  leedevkit manage up dev\n  leedevkit manage sync:api",
        )
        # Mapping tool names to their target service in docker-compose.test.yml
        self.tool_map = {
            "npm": "webdashboard",
            "cargo": "apiserver",
            "diesel": "apiserver",
        }
        self.setup_parser()
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
        self.register_traps()

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
        if self.lock_fd is not None:
            import fcntl
            import tempfile
            from pathlib import Path

            try:
                fcntl.flock(self.lock_fd, fcntl.LOCK_UN)
                os.close(self.lock_fd)
                lock_path = (
                    Path(tempfile.gettempdir())
                    / f"{os.environ.get('COMPOSE_PROJECT_NAME')}.lock"
                )
                with contextlib.suppress(OSError):
                    lock_path.unlink(missing_ok=True)
            except OSError:
                pass
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

    def setup_parser(self) -> None:
        """Setup command line argument parser."""
        parent_parser = argparse.ArgumentParser(add_help=False)
        parent_parser.add_argument("--dry-run", action="store_true")

        subparsers = self.parser.add_subparsers(dest="command", required=True)
        self._setup_test_parser(subparsers, parent_parser)
        self._setup_manage_parser(subparsers, parent_parser)
        self._setup_run_parser(subparsers, parent_parser)

    def _setup_test_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        test_prog = (
None
        )
        test_parser = subparsers.add_parser(
            "test",
            prog=test_prog,
            parents=[parent_parser],
            description="LeeDevKit Enterprise Test Orchestrator - Automatically handles environments, mocking, and parallel execution.",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples for AI Agents:\n"
            "  leedevkit test all                 # Full suite\n"
            "  leedevkit test infra --lint-only   # Quick format + lint\n"
            "  leedevkit test infra --lint-only --fix # Auto-fix formatting\n"
            "  leedevkit test all --json          # Machine-readable output\n"
            "\n"
            "Tips: prefer specific targets for faster feedback.",
        )
        # Dynamically resolve valid targets from leedevkit.toml (or fall back to defaults)
        targets = _resolve_targets()
        test_parser.add_argument(
            "target",
            choices=targets,
            help=f"Target: {', '.join(targets)}",
        )
        test_parser.add_argument(
            "--lint-only",
            action="store_true",
            help="Run only code formatting and linting checks",
        )
        test_parser.add_argument(
            "--unit-only",
            action="store_true",
            help="Run only unit tests (skips e2e and linting)",
        )
        test_parser.add_argument(
            "--e2e-only", action="store_true", help="Run only integration/e2e tests"
        )
        test_parser.add_argument(
            "--skip-lint",
            action="store_true",
            help="Skip linting phase (useful for faster iterative TDD)",
        )
        test_parser.add_argument(
            "--pattern",
            dest="pattern",
            help="Regex pattern to filter tests by name (e.g., 'auth' or 'user_login')",
        )
        test_parser.add_argument(
            "--coverage",
            action="store_true",
            help="Run tests with coverage reporting (enforces 100%% threshold)",
        )
        test_parser.add_argument(
            "--timeout",
            type=int,
            default=1800,
            help="Test execution timeout in seconds",
        )
        test_parser.add_argument(
            "--fix",
            action="store_true",
            help="Auto-fix formatting issues (cargo fmt, eslint --fix) instead of just checking",
        )
        test_parser.add_argument(
            "--json",
            action="store_true",
            dest="json_output",
            help="Output machine-readable JSON summary at the end (for AI agents)",
        )

    def _setup_manage_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        manage_prog = (
            "manage.sh" if "manage" in sys.argv and "test" not in sys.argv else None
        )
        manage_parser = subparsers.add_parser(
            "manage",
            prog=manage_prog,
            parents=[parent_parser],
            description="LeeDevKit Infrastructure & Environment Manager",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples for AI Agents:\n  leedevkit manage up dev         # Start development environment\n  leedevkit manage db:setup       # Initialize database and run migrations\n  leedevkit manage sync:api       # Sync OpenAPI schema from backend to frontend\n  leedevkit manage logs apiserver # View backend logs",
        )
        manage_cmd_sub = manage_parser.add_subparsers(dest="subcommand", required=True)

        for cmd in ["up", "down", "ps", "clean"]:
            p = manage_cmd_sub.add_parser(cmd, parents=[parent_parser])
            p.add_argument(
                "env", choices=["dev", "test", "prod"], default="dev", nargs="?"
            )

        init_p = manage_cmd_sub.add_parser("init", parents=[parent_parser])
        init_p.add_argument(
            "--force", action="store_true", help="Overwrite existing files"
        )

        skills_p = manage_cmd_sub.add_parser("skills", parents=[parent_parser])
        skills_sub = skills_p.add_subparsers(dest="skills_action", required=True)
        skills_sub.add_parser("list", parents=[parent_parser])
        skills_sub.add_parser("update", parents=[parent_parser])
        skills_install = skills_sub.add_parser("install", parents=[parent_parser])
        skills_install.add_argument("name", nargs="?", help="Skill name from catalog (or leave empty to install from leedevkit.toml)")
        skills_add = skills_sub.add_parser("add", parents=[parent_parser])
        skills_add.add_argument("url", help="Git repo URL to clone")
        skills_add.add_argument(
            "--version", default="main", help="Branch/tag to checkout (default: main)"
        )
        skills_rm = skills_sub.add_parser("remove", parents=[parent_parser])
        skills_rm.add_argument("name", help="Skill repo name to remove")

        logs_p = manage_cmd_sub.add_parser("logs", parents=[parent_parser])
        logs_p.add_argument(
            "env", choices=["dev", "test", "prod"], default="dev", nargs="?"
        )
        logs_p.add_argument("service", nargs="?")

        manage_cmds = [
            "sync:api",
            "test:infra",
            "migrate:run",
            "migrate:revert",
            "migrate:status",
            "db:setup",
            "prebuild",
            "fmt:infra",
            "doctor",
            "verify:infra",
        ]
        for cmd in manage_cmds:
            manage_cmd_sub.add_parser(cmd, parents=[parent_parser])

        exec_p = manage_cmd_sub.add_parser("exec", parents=[parent_parser])
        exec_p.add_argument("service")
        exec_p.add_argument("args", nargs=argparse.REMAINDER)

        db_query_p = manage_cmd_sub.add_parser("db:query", parents=[parent_parser])
        db_query_p.add_argument("sql", help="SQL query to execute")
        db_query_p.add_argument(
            "--json", action="store_true", help="Output in JSON format"
        )

    def _setup_run_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        run_p = subparsers.add_parser(
            "run", help="Run toolbox command", parents=[parent_parser]
        )
        run_p.add_argument(
            "tool", choices=list(self.tool_map.keys()), help="Tool to run"
        )
        run_p.add_argument(
            "--pooler", action="store_true", help="Enable connection pooler"
        )
        run_p.add_argument(
            "args", nargs=argparse.REMAINDER, help="Arguments for the tool"
        )

    def run(self) -> None:
        args = self.parser.parse_args()
        self.dry_run = args.dry_run
        if not args.command:
            self.parser.print_help()
            return
        if args.command in ("test", "run"):
            import fcntl
            import uuid

            # Use dynamic project name for perfect run isolation
            suffix = uuid.uuid4().hex[:8]
            project_name = f"leedevkit-test-{suffix}"
            self.env_vars["COMPOSE_PROJECT_NAME"] = project_name
            self.env_vars["PODMAN_COMPOSE_PROJECT_NAME"] = project_name
            os.environ["COMPOSE_PROJECT_NAME"] = project_name
            os.environ["PODMAN_COMPOSE_PROJECT_NAME"] = project_name

            # Acquire an OS-level file lock to prevent garbage collection
            import tempfile
            from pathlib import Path

            lock_path = Path(tempfile.gettempdir()) / f"{project_name}.lock"
            try:
                self.lock_fd = os.open(str(lock_path), os.O_CREAT | os.O_RDWR)
                fcntl.flock(self.lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except OSError:
                log_warn(
                    f"Failed to lock {lock_path}, garbage collector might sweep this run!"
                )

        if args.command == "test":
            self.handle_test(args)
        elif args.command == "manage":
            self.handle_manage(args)
        elif args.command == "run":
            self.handle_run(args)

    def handle_test(self, args: argparse.Namespace) -> None:  # noqa: PLR0915
        target = getattr(args, "target", None)

        if target == "infra":
            if args.lint_only:
                self.handle_lint_infra()
            else:
                self.handle_verify_infra()
            return

        mode_map = {
            "all": "all",
            "api": "api",
            "web": "web",
            "apiserver": "api",
            "agent-main": "api",
            "webdashboard": "web",
        }

        mode = mode_map.get(target or "all", "all")
        component_filter = (
            target if target in ["apiserver", "agent-main", "webdashboard"] else ""
        )
        args.component = component_filter

        self.active_mode = mode
        log_info(f"🚀 Starting LeeDevKit Test Suite for [{target}] in mode [{mode}]")

        if getattr(args, "timeout", None):
            timeout_str = str(args.timeout)
            os.environ["TIMEOUT_LINT"] = timeout_str
            os.environ["TIMEOUT_UNIT"] = timeout_str
            os.environ["TIMEOUT_INTEGRATION"] = timeout_str
            os.environ["TIMEOUT_BUILD"] = timeout_str
            self.env_vars["TIMEOUT_LINT"] = timeout_str
            self.env_vars["TIMEOUT_UNIT"] = timeout_str
            self.env_vars["TIMEOUT_INTEGRATION"] = timeout_str
            self.env_vars["TIMEOUT_BUILD"] = timeout_str

        if target == "all" and not (args.lint_only or args.unit_only or args.e2e_only):
            # Resolve sub-targets from config: targets.all defines what "all" means.
            # Fall back to infra-only — every project has format+lint infrastructure.
            resolved = _resolve_targets()
            sub_targets = [t for t in resolved if t != "all"] or ["infra"]
            for sub_target in sub_targets:
                args.target = sub_target
                self.handle_test(args)
                if self.needs_cleanup and not self.dry_run:
                    from _lifecycle import lifecycle_down

                    lifecycle_down("all")
                    self.needs_cleanup = False
            args.target = "all"
            self.print_test_summary("all")
            return

        self.needs_cleanup = True

        run_lint = not args.skip_lint and not (args.unit_only or args.e2e_only)
        run_unit = not (args.lint_only or args.e2e_only)
        run_e2e = not (args.lint_only or args.unit_only)

        if args.lint_only:
            run_lint = True

        if args.coverage:
            run_unit = False
            run_e2e = False

        if run_unit:
            self.run_phase("Unit Tests", mode, args)

        if run_e2e:
            self.run_phase("Integration Tests", mode, args)

        if run_lint:
            self.run_phase("Linting", mode, args)

        if args.coverage:
            self.run_phase("Coverage", mode, args)

        self.print_test_summary(target or "all")

        if getattr(args, "json_output", False) and self.results:
            import json

            print(json.dumps(self.results, indent=2), file=sys.stderr, flush=True)

        is_single_phase = (
            getattr(args, "lint_only", False)
            or getattr(args, "unit_only", False)
            or getattr(args, "e2e_only", False)
        )
        if is_single_phase:
            target_name = target or "all"
            log_success(
                "\n💡 Tip: Isolated phase completed successfully! Run the full suite to verify everything:"
            )
            log_success(f"   leedevkit test {target_name}\n")

    def run_phase(self, phase_name: str, mode: str, args: argparse.Namespace) -> None:
        if self.dry_run:
            log_info(f"🔍 Dry-run: Phase [{phase_name}] for [{mode}]")
            return

        granular_map = {
            ("Linting", "api"): "lint-api",
            ("Unit Tests", "api"): "unit-api",
            ("Integration Tests", "api"): "int-api",
            ("Coverage", "api"): "api",
            ("Linting", "web"): "lint-web",
            ("Unit Tests", "web"): "unit-web",
            ("Integration Tests", "web"): "e2e-web",
            ("Coverage", "web"): "web",
        }
        granular_mode = granular_map.get((phase_name, mode))

        if granular_mode:
            log_info(
                f"🔹 Starting isolated environment for: {phase_name} ({granular_mode})"
            )
            _lifecycle_up(granular_mode)
            if phase_name == "Integration Tests" and granular_mode in (
                "int-api",
                "api",
            ):
                _lifecycle_up("infra-db")
                _lifecycle_up("infra-redis")
                _lifecycle_up("infra-pooler")

        log_info(f"🔹 Running {phase_name}...")
        func_map: dict[str, typing.Callable[..., typing.Any]] = {
            "Startup": lambda: _lifecycle_up(mode),
            "Linting": lambda: leedevkit_run_lint(
                getattr(args, "component", "") or "",
                mode,
                fix=getattr(args, "fix", False),
            ),
            "Unit Tests": lambda: leedevkit_run_unit(
                getattr(args, "component", "") or "",
                mode,
                test_pattern=getattr(args, "pattern", "") or "",
            ),
            "Integration Tests": lambda: leedevkit_run_integration(
                getattr(args, "component", "") or "",
                mode,
                test_pattern=getattr(args, "pattern", "") or "",
            ),
            "Coverage": lambda: leedevkit_run_coverage(
                getattr(args, "component", "") or "",
                mode,
                getattr(args, "unit_only", False),
            ),
            "Database Setup": self.handle_db_setup_phase,
            "Prebuild": self.handle_prebuild_phase,
        }

        func = func_map.get(phase_name)
        if func is None:
            log_error(f"Unknown phase: {phase_name}")
            sys.exit(1)

        start = time.time()
        res = func()
        elapsed = time.time() - start

        self.results[phase_name] = {
            "status": "pass" if res else "fail",
            "duration_s": round(elapsed, 1),
        }

        if granular_mode:
            log_info(
                f"🔹 Tearing down isolated environment for: {phase_name} ({granular_mode})"
            )
            lifecycle_down("all")

        if res is False:
            is_single_phase = (
                getattr(args, "lint_only", False)
                or getattr(args, "unit_only", False)
                or getattr(args, "e2e_only", False)
            )
            target = getattr(args, "target", "api")
            if not is_single_phase:
                if phase_name == "Linting":
                    log_warn(
                        f"\n💡 Tip: To quickly verify only linting/formatting fixes, run:\n   leedevkit test {target} --lint-only\n"
                    )
                elif phase_name == "Unit Tests":
                    log_warn(
                        f"\n💡 Tip: To focus on unit tests and skip linting/e2e, run:\n   leedevkit test {target} --unit-only\n"
                    )
                elif phase_name == "Integration Tests":
                    log_warn(
                        f"\n💡 Tip: To focus on integration/E2E tests only, run:\n   leedevkit test {target} --e2e-only\n"
                    )
            # Continue to next phase instead of immediate exit — AI needs all results
            if phase_name != "Linting":  # lint failures are non-blocking
                sys.exit(1)

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
        env = os.environ.copy()
        env["PYTHONPATH"] = str(SCRIPTS_DIR)
        tests_dir = SCRIPTS_DIR / "tests"
        test_files = sorted(str(p) for p in tests_dir.glob("test_*.py"))
        venv_pytest = PROJECT_ROOT / ".venv" / "bin" / "pytest"
        cmd = [
            str(venv_pytest),
            "--cov=scripts",
            "--cov-report=term-missing",
            "--cov-fail-under=80",
        ] + test_files
        self.execute_safe(cmd, env=env)

    def handle_run(self, args: argparse.Namespace) -> None:  # noqa: PLR0915
        self.needs_cleanup = True
        tool = args.tool
        tool_args = args.args

        if getattr(args, "pooler", False):
            pooler_host = os.environ.get("SUPAVISOR_HOST", "pgbouncer_tx")
            pooler_port = os.environ.get("SUPAVISOR_PORT", "6543")
            os.environ["DATABASE_POOLER_URL"] = (
                f"postgres://test_user:test_password@{pooler_host}:{pooler_port}/test_database"
            )
            self.env_vars["DATABASE_POOLER_URL"] = os.environ["DATABASE_POOLER_URL"]

        # Resolve target service dynamically based on context
        caller_dir = Path.cwd()
        rel_dir = ""
        with contextlib.suppress(ValueError):
            rel_dir = str(caller_dir.relative_to(PROJECT_ROOT))

        service = self.tool_map.get(tool, "apiserver")
        if "webdashboard" in rel_dir or any("webdashboard" in arg for arg in tool_args):
            service = "webdashboard"
        elif "agent-main" in rel_dir or any("agent-main" in arg for arg in tool_args):
            service = "agent-main"
        elif "apiserver" in rel_dir or any("apiserver" in arg for arg in tool_args):
            service = "apiserver"

        if tool_args and tool_args[0] == "--":
            tool_args = tool_args[1:]

        # Sanitize AI-provided arguments BEFORE any processing
        tool_args = sanitize(tool_args, die_on_error=True)

        log_info(f"🛠️ Running {tool} inside container environment...")

        compose_cmd = self.compose_engine + [
            "-p",
            self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test"),
        ]

        if tool in ["cargo", "diesel"]:
            needs_db = tool == "diesel"
            if tool == "cargo" and tool_args:
                first_arg = tool_args[0]
                if first_arg in ["run", "test", "nextest"]:
                    needs_db = True

            if needs_db:
                log_info(f"🔹 Bringing up backend dependencies for {tool}...")
                _lifecycle_up("infra-db")

                log_info(
                    "ℹ️ 🔹 Bringing up backend dependencies for connection pooler..."
                )
                _lifecycle_up("infra-pooler")
            compose_cmd.extend(["--profile", "int-api"])
            if needs_db:
                compose_cmd.extend(
                    ["--profile", "infra-db", "--profile", "infra-redis"]
                )
                if "DATABASE_POOLER_URL" in os.environ:
                    _lifecycle_up("infra-pooler")  # pragma: no cover
                    compose_cmd.extend(
                        ["--profile", "infra-pooler"]
                    )  # pragma: no cover
        else:
            compose_cmd.extend(["--profile", "frontend"])

        compose_cmd.extend(
            [
                "-f",
                str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
            ]
        )

        if tool == "npm":
            first = tool_args[0] if tool_args else ""
            is_standalone = tool_args and first in ("--version", "--help", "-v", "-h")
            if is_standalone:
                # standalone: compose run --no-deps --entrypoint bun (no prefix needed)
                compose_cmd.extend(
                    ["run", "-T", "--rm", "--no-deps", "--entrypoint", "bun", service]
                )
                if tool_args:
                    compose_cmd.extend(tool_args)
            else:
                is_running = self._is_service_running(service)
                if is_running:
                    compose_cmd.extend(["exec", "-T", service])
                else:
                    compose_cmd.extend(["run", "-T", "--rm"])
                self._handle_run_npm(compose_cmd, tool_args, service, is_running)
        elif tool == "cargo":
            if not needs_db:
                compose_cmd.extend(["run", "-T", "--rm", "--no-deps"])
            else:
                compose_cmd.extend(["run", "-T", "--rm"])
            self._handle_run_cargo(compose_cmd, tool_args, service)
        else:  # diesel
            compose_cmd.extend(
                ["run", "-T", "--rm", "--entrypoint", tool, service]
            )  # pragma: no cover
            if tool_args:  # pragma: no cover
                compose_cmd.extend(tool_args)  # pragma: no cover

        self.execute_safe(compose_cmd)

    def _is_service_running(self, service: str) -> bool:
        res = subprocess.run(
            [self.engine, "ps", "--format", "{{.Names}}"],
            capture_output=True,
            text=True,
            stdin=subprocess.DEVNULL,
            check=False,
        )
        project_name = self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        return any(project_name in n and service in n for n in res.stdout.splitlines())

    def _handle_run_npm(
        self,
        compose_cmd: list[str],
        tool_args: list[str],
        service: str,
        is_running: bool = True,
    ) -> None:
        """Run npm/bun commands via compose exec/run into container.

        Auto-detects if container is running. If not, uses compose run with custom entrypoint.
        """
        npm_min_args = 2
        is_typecheck = (
            len(tool_args) >= npm_min_args
            and tool_args[0] == "run"
            and tool_args[1] == "typecheck"
        )
        if is_typecheck:
            tool_args[1] = "type-check"

        first_arg = tool_args[0] if tool_args else ""

        executable = "bun"
        if first_arg in ("bun", "npm", "bash", "sh"):
            executable = first_arg
            tool_args = tool_args[1:]

        if is_running:
            compose_cmd.extend([executable])
        else:
            compose_cmd.extend(["--entrypoint", executable, service])

        if tool_args:
            compose_cmd.extend(tool_args)

    def _handle_run_cargo(
        self, compose_cmd: list[str], tool_args: list[str], service: str
    ) -> None:
        caller_dir = Path.cwd()
        rel_dir = ""
        with contextlib.suppress(ValueError):
            rel_dir = str(caller_dir.relative_to(PROJECT_ROOT))
            if rel_dir == ".":
                rel_dir = ""

        workdir = f"/workspace/{rel_dir}" if rel_dir else "/workspace"
        compose_cmd.extend(
            [
                "--workdir",
                workdir,
                "--entrypoint",
                "cargo",
                service,
            ]
        )
        if tool_args:
            # AI Agent behavior correction: silently convert "cargo test" to "cargo nextest run"
            if tool_args[0] == "test":
                tool_args = ["nextest", "run", "--test-threads", "4"] + tool_args[1:]
            compose_cmd.extend(tool_args)

    def handle_lint_infra(self) -> None:
        log_info("🎨 Linting infrastructure scripts...")
        venv_bin = PROJECT_ROOT / ".venv" / "bin"
        self.execute_safe([str(venv_bin / "ruff"), "check", str(SCRIPTS_DIR)])
        self.execute_safe([str(venv_bin / "mypy"), str(SCRIPTS_DIR)])

        sh_files = [str(f) for f in PROJECT_ROOT.glob("*.sh")]
        sh_files += [str(f) for f in SCRIPTS_DIR.glob("*.sh")]
        if shutil.which("shellcheck"):
            self.execute_safe(["shellcheck"] + sh_files)

        log_success("✨ Infrastructure linting passed!")

    def handle_fmt_infra(self) -> None:
        log_info("💅 Formatting infrastructure scripts...")
        venv_bin = PROJECT_ROOT / ".venv" / "bin"
        self.execute_safe([str(venv_bin / "ruff"), "format", str(SCRIPTS_DIR)])
        log_success("✨ Infrastructure formatting completed!")

    def handle_verify_infra(self) -> None:
        log_info("🚀 Starting full infrastructure verification...")
        self.handle_fmt_infra()
        self.handle_lint_infra()
        self.handle_test_infra()
        log_success("✨ Infrastructure is Premium Grade!")

    def handle_init(self, force: bool = False) -> None:
        """Set up project: leedevkit wrapper, leedevkit.toml, .agent symlinks."""
        from pathlib import Path

        from _devkit_config import _find_devkit_root

        root = Path.cwd()
        devkit = _find_devkit_root()

        # 1. Create leedevkit wrapper
        for script in ["leedevkit"]:
            path = root / script
            if not path.exists() or force:
                path.write_text(
                    "#!/bin/bash\n"
                    "# Delegates to leedevkit orchestrator\n"
                    'DEVKIT_ROOT="${DEVKIT_HOME:-$HOME/.leedevkit/current}"\n'
                    f'exec "$DEVKIT_ROOT/bin/{script}" "$@"\n'  # noqa: S608
                )
                path.chmod(0o755)
                log_success(f"Created {script}")

        # 2. Create leedevkit.toml if missing
        config_toml = root / "leedevkit.toml"
        if not config_toml.exists() or force:
            template = devkit / "templates" / "leedevkit.default.toml"
            if template.exists():
                config_toml.write_text(template.read_text())
                log_success("Created leedevkit.toml (edit it for your project)")

        # 3. Create .agent directory with symlinks to devkit shared dirs
        agent_dir = root / ".agent"
        if agent_dir.is_symlink():
            # Replace old symlink with directory
            agent_dir.unlink()
            agent_dir.mkdir(exist_ok=True)
        else:
            agent_dir.mkdir(exist_ok=True)
        devkit_agent = devkit / ".agent"

        symlinks = {
            "skills": devkit_agent / "skills",
            "workflows": devkit_agent / "workflows",
            ".shared": devkit_agent / ".shared",
            "scripts": devkit_agent / "scripts",
            "agents": devkit_agent / "agents",
        }
        for name, target in symlinks.items():
            link = agent_dir / name
            if target.exists():
                if link.is_symlink():
                    link.unlink()
                elif link.exists():
                    continue  # don't overwrite real dirs
                link.symlink_to(target)
                log_success(f"  .agent/{name} → leedevkit")

        # 4. Validate: verify all symlinks resolve
        broken = []
        for name in symlinks:
            link = agent_dir / name
            if not link.exists() or (link.is_symlink() and not link.resolve().exists()):
                broken.append(name)
        if broken:
            log_warn(f"Broken symlinks: {', '.join(broken)}")
        else:
            log_success("All .agent symlinks validated")

        # 4b. Populate project rules directory from devkit base
        try:
            from _devkit_config import _load_toml
            cfg = _load_toml(config_toml)
            rules_rel = cfg.get("ai", {}).get("rules_dir", ".agent/rules")
            project_rules = root / rules_rel
            devkit_rules = devkit_agent / "rules"
            if devkit_rules.exists() and not project_rules.exists():
                project_rules.mkdir(parents=True, exist_ok=True)
                for rule_file in devkit_rules.glob("*.md"):
                    target = project_rules / rule_file.name
                    if not target.exists():
                        target.write_text(rule_file.read_text())
                log_success(f"Populated {rules_rel}/ with {len(list(project_rules.glob('*.md')))} rulebooks")
            # Also copy overrides.yaml if project doesn't have one
            override_manifest = cfg.get("ai", {}).get("override_manifest", ".agent/overrides.yaml")
            override_path = root / override_manifest
            if not override_path.exists():
                devkit_override = devkit_agent / "overrides.yaml"
                if devkit_override.exists():
                    override_path.parent.mkdir(parents=True, exist_ok=True)
                    override_path.write_text(devkit_override.read_text())
                    log_success(f"Created {override_manifest}")
        except Exception:
            pass  # Don't fail init if rule population fails

        # 5. Pin devkit version in leedevkit.toml
        try:
            from _devkit_config import _load_toml
            cfg = _load_toml(config_toml)
            current = cfg.get("devkit", {}).get("version", "")
            devkit_version = (devkit / "VERSION").read_text().strip()
            if current != devkit_version:
                content = config_toml.read_text()
                if "version =" in content and "[devkit]" in content:
                    import re
                    content = re.sub(
                        r'(\[devkit\].*?version\s*=\s*)"[^"]*"',
                        f'\\1"{devkit_version}"',
                        content, flags=re.DOTALL
                    )
                    config_toml.write_text(content)
                log_success(f"Pinned devkit version: {devkit_version}")
        except Exception:
            pass

        # Auto-install community skills from leedevkit.toml
        self.handle_skills(argparse.Namespace(skills_action="install"))

        log_success("Project initialized. Run ./leedevkit --help to start.")

    def _load_skills_catalog(self) -> dict:
        """Load the curated skills catalog from devkit."""
        from _devkit_config import _find_devkit_root

        devkit = _find_devkit_root()
        catalog_path = devkit / ".agent" / "skills-catalog.toml"
        if not catalog_path.exists():
            return {}
        try:
            from _devkit_config import _load_toml

            return _load_toml(catalog_path).get("skills", {})
        except Exception:
            return {}

    def handle_skills(self, args: argparse.Namespace) -> None:
        """Manage community add-on skills in the project's .leedevkit/skills.d/."""
        from _devkit_config import _find_devkit_root

        devkit = _find_devkit_root()
        skills_d = PROJECT_ROOT / ".leedevkit" / "skills.d"
        skills_d.mkdir(parents=True, exist_ok=True)
        catalog = self._load_skills_catalog()

        action = getattr(args, "skills_action", "list")

        if action == "list":
            # Built-in skills (shipped with devkit, always available)
            builtin_dir = devkit / ".agent" / "skills"
            builtins = set()
            if builtin_dir.exists():
                for d in builtin_dir.iterdir():
                    if d.is_dir() and not d.name.startswith("."):
                        builtins.add(d.name)

            # Installed community skills (per-project)
            installed = set()
            if skills_d.exists():
                for repo in skills_d.iterdir():
                    if repo.is_dir() and not repo.name.startswith("."):
                        installed.add(repo.name)

            log_info("Skills")
            log_info("")

            # Built-in
            log_info("Built-in (always available, shipped with devkit):")
            for name in sorted(builtins):
                log_info(f"  ▸ {name}")
            log_info(f"  ({len(builtins)} skills)")
            log_info("")

            # Catalog (available for install)
            if catalog:
                log_info("Community catalog (leedevkit skills install <name>):")
                for key, skill in sorted(catalog.items()):
                    status = "● installed" if key in installed else "○ available"
                    log_info(f"  {status}  {key}")
                    log_info(f"          {skill.get('description', '')}")
                log_info("")

            # External (installed but not in catalog)
            for name in sorted(installed):
                if name not in catalog:
                    log_info(f"  ● {name} [external — not in catalog]")

            if not installed and not catalog:
                log_info("No community skills. Add one:")
                log_info("  leedevkit skills add <git-url>")

        elif action == "install":
            name = getattr(args, "name", None)
            if name:
                # Install by name from catalog
                if name not in catalog:
                    log_error(f"'{name}' not found in catalog. Use 'skills list' to see available skills.")
                    log_error("Or install from URL: leedevkit skills add <git-url>")
                    return
                skill = catalog[name]
                url = skill["url"]
                version = skill.get("version", "main")
                target = skills_d / name
                if target.exists():
                    log_warn(f"'{name}' already installed. Use 'skills update' to refresh.")
                    return
                log_info(f"Installing {skill['name']} from catalog...")
                subprocess.run(
                    ["git", "clone", "--depth", "1", "--branch", version, url, str(target)],
                    check=False,
                    stdin=subprocess.DEVNULL,
                )
                log_success(f"Installed {skill['name']} @ {version}")
                self._write_lock(skills_d)
            else:
                # No name given: install from leedevkit.toml [addons.skills]
                self._skills_install_from_toml(skills_d)

        elif action == "update":
            self._skills_update_and_lock(skills_d)

        elif action == "add":
            url = getattr(args, "url", "")
            if not url:
                log_error("Usage: leedevkit skills add <git-url> [--version main]")
                return
            if not url.startswith(("http://", "https://", "git@")):
                catalog = self._load_skills_catalog()
                if url in catalog:
                    log_error(f"'{url}' is in the skills catalog. Use: leedevkit skills install {url}")
                else:
                    log_error(f"'{url}' is not a valid URL. Provide a git URL or use: leedevkit skills install <name>")
                return
            version = getattr(args, "version", "main")
            name = url.rstrip("/").split("/")[-1].replace(".git", "")
            target = skills_d / name
            if target.exists():
                log_warn(f"{name} already exists. Use 'skills update' to refresh.")
                return
            subprocess.run(
                ["git", "clone", "--depth", "1", "--branch", version, url, str(target)],
                check=False,
                stdin=subprocess.DEVNULL,
            )
            log_success(f"Installed {name} @ {version}")
            self._write_lock(skills_d)

        elif action == "remove":
            name = getattr(args, "name", "")
            if not name:
                log_error("Usage: leedevkit skills remove <name>")
                return
            target = skills_d / name
            if not target.exists():
                log_warn(f"{name} not found in skills.d/")
                return
            import shutil

            shutil.rmtree(str(target))
            log_success(f"Removed {name}")
            self._write_lock(skills_d)

    def _lock_path(self) -> Path:
        return PROJECT_ROOT / "leedevkit.lock"

    def _read_lock(self) -> dict:
        path = self._lock_path()
        if path.exists():
            try:
                import tomllib

                with open(path, "rb") as f:
                    return tomllib.load(f)
            except Exception:
                pass
            try:
                import json

                return json.loads(path.read_text())
            except Exception:
                pass
        return {}

    def _write_lock(self, skills_d: Path) -> None:
        lock = {}
        for repo in sorted(skills_d.iterdir()):
            if (repo / ".git").exists():
                r = subprocess.run(
                    ["git", "-C", str(repo), "rev-parse", "HEAD"],
                    capture_output=True,
                    text=True,
                    check=False,
                )
                sha = r.stdout.strip()
                if sha:
                    lock[repo.name] = sha
        path = self._lock_path()
        path.parent.mkdir(exist_ok=True)
        try:
            import tomli_w

            with open(path, "wb") as f:
                tomli_w.dump(lock, f)
        except ImportError:
            import json

            path.write_text(json.dumps(lock, indent=2))
        log_success(f"Updated leedevkit.lock ({len(lock)} entries)")

    def _skills_install_from_toml(self, skills_d: Path) -> None:
        """Install skills from leedevkit.toml, preferring lock file SHAs."""
        from _devkit_config import load_project_config

        try:
            cfg = load_project_config()
            entries = cfg.get("addons", {}).get("skills", [])
        except Exception:
            entries = []
        lock = self._read_lock()
        installed = 0
        for entry in entries:
            if isinstance(entry, str):
                url, version = entry, "main"
            else:
                url = entry.get("url", "")
                version = entry.get("version", "main")
            name = url.rstrip("/").split("/")[-1].replace(".git", "")
            target = skills_d / name
            if target.exists():
                # Checkout pinned version if lock exists
                if name in lock:
                    subprocess.run(
                        ["git", "-C", str(target), "checkout", "--detach", lock[name]],
                        check=False,
                        stdin=subprocess.DEVNULL,
                    )
                    log_success(f"  {name} @ {lock[name][:8]} (locked)")
                continue
            log_info(f"Installing {name} @ {version}...")
            subprocess.run(
                ["git", "clone", "--depth", "1", "--branch", version, url, str(target)],
                check=False,
                stdin=subprocess.DEVNULL,
            )
            # If lock has exact SHA, checkout that instead
            if name in lock:
                subprocess.run(
                    [
                        "git",
                        "-C",
                        str(target),
                        "fetch",
                        "--depth",
                        "1",
                        "origin",
                        lock[name],
                    ],
                    check=False,
                    stdin=subprocess.DEVNULL,
                )
                subprocess.run(
                    ["git", "-C", str(target), "checkout", "--detach", lock[name]],
                    check=False,
                    stdin=subprocess.DEVNULL,
                )
            installed += 1
        log_success(f"Installed {installed} new skill repo(s)")
        self._write_lock(skills_d)

    def _skills_update_and_lock(self, skills_d: Path) -> None:
        """Pull latest, update lock file."""
        updated = 0
        for repo in skills_d.iterdir():
            if (repo / ".git").exists():
                subprocess.run(
                    ["git", "-C", str(repo), "pull", "--ff-only"],
                    check=False,
                    stdin=subprocess.DEVNULL,
                )
                updated += 1
        log_success(f"Updated {updated} skill repo(s)")
        self._write_lock(skills_d)

    def handle_doctor(self) -> None:
        from _devkit_config import load_project_config, resolve_ai_rules

        log_info("🩺 Running LeeDevKit System Doctor...")

        # ── Project config ──
        try:
            cfg = load_project_config()
            name = cfg.get("project", {}).get("name", "unknown")
            targets = list(cfg.get("targets", {}).keys())
            log_success(f"✅ Project: {name} (targets: {', '.join(targets)})")
        except Exception as e:
            log_warn(f"⚠️  leedevkit.toml: {e}")

        # ── .agent symlinks ──
        agent_dir = PROJECT_ROOT / ".agent"
        if agent_dir.is_symlink():
            log_warn(
                "⚠️  .agent is a bare symlink — expected directory with sub-symlinks"
            )
        elif agent_dir.is_dir():
            expected = ["skills", "workflows", "agents", "scripts", ".shared"]
            for name in expected:
                link = agent_dir / name
                if link.is_symlink():
                    target = link.resolve()
                    if target.exists():
                        log_success(f"✅ .agent/{name} → {target}")
                    else:
                        log_warn(f"⚠️  .agent/{name} → broken (target missing)")
                elif link.exists():
                    log_success(f"✅ .agent/{name} (custom directory)")
                else:
                    log_warn(f"⚠️  .agent/{name} — missing (run: leedevkit init)")
        else:
            log_warn("⚠️  .agent directory missing (run: leedevkit init)")

        # ── AI rules ──
        try:
            rules = resolve_ai_rules()
            log_success(f"✅ AI rules: {len(rules)} files loaded")
        except Exception as e:
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
        sql = args.sql
        log_info(f"🗄️ Executing SQL: {sql}")
        cmd = [
            self.engine,
            "exec",
            "-t",
            "leedevkit-dev-db",
            "psql",
            "-U",
            "lee",
            "-d",
            "leedevkit",
            "-c",
            sql,
        ]
        if args.json:
            cmd += ["-A", "-t", "--json"]
        self.execute_safe(cmd)

    def handle_diesel(self, args: list[str]) -> None:
        """Wrapper for diesel commands."""
        caller_dir = Path.cwd()
        rel_dir = ""
        with contextlib.suppress(ValueError):
            rel_dir = str(caller_dir.relative_to(PROJECT_ROOT))

        workdir = f"/workspace/{rel_dir}" if rel_dir else "/workspace"
        cmd = (
            self.compose_engine
            + [
                "-p",
                self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test"),
                "-f",
                str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
                "run",
                "-T",
                "--rm",
                "--workdir",
                workdir,
                "--entrypoint",
                "diesel",
                "apiserver",
            ]
            + args
        )
        self.execute_safe(cmd)

    def get_compose_files(self, env: str) -> list[str]:
        if env == "dev":
            return [
                "-p",
                "leedevkit-dev",
                "-f",
                "docker-compose.yml",
                "-f",
                ".compose/docker-compose.dev.yml",
            ]
        if env == "test":
            project_name = self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
            return ["-p", project_name, "-f", ".compose/docker-compose.test.yml"]
        return ["-p", "leedevkit", "-f", "docker-compose.yml"]

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
        total_tests = 0
        passed_tests = 0

        nextest_pattern = re.compile(r"Summary \[.*?\] (\d+) tests? run: (\d+) passed")
        vitest_pattern = re.compile(r"Tests\s+(\d+)\s+passed\s+\((\d+)\)")
        pw_pattern = re.compile(r"^\s*(\d+)\s+passed\s+\(.*?\)", re.MULTILINE)

        log_dir = PROJECT_ROOT / ".test_logs"
        if log_dir.exists():
            for log_file in log_dir.glob("*.log"):
                try:
                    if log_file.stat().st_mtime < self.start_time:
                        continue

                    content = log_file.read_text(errors="replace")
                    ansi_escape = re.compile(r"\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])")
                    clean_content = ansi_escape.sub("", content)

                    # nextest
                    matches = list(nextest_pattern.finditer(clean_content))
                    if matches:
                        last_match = matches[-1]
                        total_tests += int(last_match.group(1))
                        passed_tests += int(last_match.group(2))
                        continue

                    # vitest
                    matches = list(vitest_pattern.finditer(clean_content))
                    if matches:
                        last_match = matches[-1]
                        passed_tests += int(last_match.group(1))
                        total_tests += int(last_match.group(2))
                        continue

                    # playwright
                    if "playwright" in log_file.name.lower():
                        matches = list(pw_pattern.finditer(clean_content))
                        if matches:
                            last_match = matches[-1]
                            passed = int(last_match.group(1))
                            passed_tests += passed
                            total_tests += passed
                except Exception:
                    pass  # Ignore log parsing errors

        if total_tests > 0:
            msg = f"All selected tests for [{target}] passed successfully! ({passed_tests}/{total_tests} tests)"
            log_success(msg)
        else:
            log_success(f"All selected tests for [{target}] passed successfully!")

    def handle_db_setup_phase(self) -> bool:
        log_info("Bringing up database and apiserver containers...")
        _lifecycle_up("int-api")
        _lifecycle_up("infra-db")
        _lifecycle_up("infra-redis")

        project_name = self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        db_container = f"{project_name}_db_system_1"

        # Wait for database container to be ready
        log_info("Waiting for database container to be ready...")
        db_ready = False
        for _ in range(60):
            res = subprocess.run(
                [
                    self.engine,
                    "exec",
                    "-i",
                    db_container,
                    "pg_isready",
                    "-U",
                    "test_user",
                    "-d",
                    "test_database",
                ],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            if res.returncode == 0:
                db_ready = True
                break
            time.sleep(1)

        if not db_ready:
            log_error("❌ Database container failed to become ready in time.")
            return False

        # Drop zombie test databases (from nextest crashed runs)
        log_info("Cleaning up old test databases...")
        cleanup_sql = "SELECT 'DROP DATABASE IF EXISTS \"' || datname || '\";' FROM pg_database WHERE datname LIKE 'test_db_%';"
        cleanup_res = subprocess.run(
            [
                self.engine,
                "exec",
                "-i",
                db_container,
                "psql",
                "-U",
                "test_user",
                "-d",
                "test_database",
                "-t",
                "-c",
                cleanup_sql,
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        if cleanup_res.returncode == 0 and cleanup_res.stdout.strip():
            drop_commands = cleanup_res.stdout.strip()
            subprocess.run(
                [
                    self.engine,
                    "exec",
                    "-i",
                    db_container,
                    "psql",
                    "-U",
                    "test_user",
                    "-d",
                    "test_database",
                ],
                input=drop_commands,
                text=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )

        # Create template database
        log_info("Creating template database...")
        create_sql = "DROP DATABASE IF EXISTS leedevkit_test_template; CREATE DATABASE leedevkit_test_template;"
        subprocess.run(
            [
                self.engine,
                "exec",
                "-i",
                db_container,
                "psql",
                "-U",
                "test_user",
                "-d",
                "test_database",
                "-c",
                create_sql,
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )

        # Run migrations on main database
        log_info("Running migrations on main database...")
        main_db_url = "postgres://test_user:test_password@db_system:5432/test_database?sslmode=disable"
        main_cmd = self.compose_engine + [
            "-p",
            project_name,
            "-f",
            str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
            "run",
            "-T",
            "--rm",
            "-e",
            f"DATABASE_URL={main_db_url}",
            "--entrypoint",
            "cargo",
            "apiserver",
            "test",
            "--test",
            "setup_migrations",
            "--",
            "run_migrations",
            "--ignored",
            "--exact",
        ]
        self.execute_safe(main_cmd)

        # Run migrations on template database
        log_info("Running migrations on template database...")
        template_db_url = "postgres://test_user:test_password@db_system:5432/leedevkit_test_template?sslmode=disable"
        template_cmd = self.compose_engine + [
            "-p",
            project_name,
            "-f",
            str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
            "run",
            "-T",
            "--rm",
            "-e",
            f"DATABASE_URL={template_db_url}",
            "--entrypoint",
            "cargo",
            "apiserver",
            "test",
            "--test",
            "setup_migrations",
            "--",
            "run_migrations",
            "--ignored",
            "--exact",
        ]
        self.execute_safe(template_cmd)

        # Revoke connect on template to prevent hangs
        revoke_sql = "REVOKE CONNECT ON DATABASE leedevkit_test_template FROM public;"
        terminate_sql = "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'leedevkit_test_template' AND pid <> pg_backend_pid();"
        subprocess.run(
            [
                self.engine,
                "exec",
                "-i",
                db_container,
                "psql",
                "-U",
                "test_user",
                "-d",
                "test_database",
                "-c",
                revoke_sql,
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        subprocess.run(
            [
                self.engine,
                "exec",
                "-i",
                db_container,
                "psql",
                "-U",
                "test_user",
                "-d",
                "test_database",
                "-c",
                terminate_sql,
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )

        log_success("✅ Database Setup complete.")
        return True

    def handle_prebuild_phase(self) -> bool:
        log_info("Building test Docker images...")
        project_name = self.env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        build_cmd = self.compose_engine + [
            "-p",
            project_name,
            "-f",
            str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
            "build",
        ]
        self.execute_safe(build_cmd)
        log_success("✅ Prebuild complete.")
        return True


if __name__ == "__main__":  # pragma: no cover
    Orchestrator().run()
