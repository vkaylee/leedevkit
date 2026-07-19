#!/usr/bin/env python3
"""Run handler extracted from Orchestrator (Single Responsibility Principle).

Handles: compose-based command execution for npm/bun, cargo, and diesel tools
inside container environments. Depends on an Orchestrator-like object for
engine/compose resolution, env vars, and safe command execution.
"""

from __future__ import annotations

import contextlib
import os
import subprocess
from pathlib import Path
from typing import TYPE_CHECKING

from _arg_sanitizer import sanitize
from _bootstrap import PROJECT_ROOT
from _lifecycle import lifecycle_up as _lifecycle_up

if TYPE_CHECKING:
    import argparse
    from typing import Any


def _log_info(msg: str) -> None:
    import sys
    print(f"\033[0;34m\033[1mℹ️ {msg}\033[0m", file=sys.stderr, flush=True)


class RunHandler:
    """Container command execution extracted from the Orchestrator god class.

    Receives a reference to the orchestrator for shared state (engine,
    compose_engine, env_vars, dry_run, execute_safe, tool_map).
    """

    __slots__ = ("_orch",)

    def __init__(self, orchestrator: Any) -> None:
        self._orch = orchestrator

    # ── helpers that forward to the orchestrator ──

    @property
    def _engine(self) -> str:
        return self._orch.engine

    @property
    def _compose_engine(self) -> list[str]:
        return self._orch.compose_engine

    @property
    def _env_vars(self) -> dict[str, str]:
        return self._orch.env_vars

    @property
    def _dry_run(self) -> bool:
        return self._orch.dry_run

    @property
    def _tool_map(self) -> dict[str, str]:
        return self._orch.tool_map

    def _execute_safe(self, cmd: list[str], env: dict[str, str] | None = None, timeout: int = 1800) -> None:
        self._orch.execute_safe(cmd, env=env, timeout=timeout)

    # ── public API ──

    def is_service_running(self, service: str) -> bool:
        """Check whether a compose service is currently running."""
        res = subprocess.run(
            [self._engine, "ps", "--format", "{{.Names}}"],
            capture_output=True,
            text=True,
            stdin=subprocess.DEVNULL,
            check=False,
        )
        project_name = self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        return any(project_name in n and service in n for n in res.stdout.splitlines())

    def handle_run(self, args: argparse.Namespace) -> None:
        """Execute a tool (npm/bun, cargo, or diesel) inside a compose container.

        Resolves the correct service, brings up dependencies (DB, pooler),
        sanitizes AI-provided arguments, builds the compose command, and
        executes it safely.
        """
        self._orch.needs_cleanup = True
        self._orch._inject_rust_version_env()
        tool = args.tool
        tool_args = args.args

        if getattr(args, "pooler", False):
            pooler_host = os.environ.get("SUPAVISOR_HOST", "pgbouncer_tx")
            pooler_port = os.environ.get("SUPAVISOR_PORT", "6543")
            os.environ["DATABASE_POOLER_URL"] = (
                f"postgres://test_user:test_password@{pooler_host}:{pooler_port}/test_database"
            )
            self._env_vars["DATABASE_POOLER_URL"] = os.environ["DATABASE_POOLER_URL"]

        # Resolve target service dynamically based on context
        caller_dir = Path.cwd()
        rel_dir = ""
        with contextlib.suppress(ValueError):
            rel_dir = str(caller_dir.relative_to(PROJECT_ROOT))

        service = self._tool_map.get(tool, "apiserver")
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

        _log_info(f"🛠️ Running {tool} inside container environment...")

        compose_cmd = self._compose_engine + [
            "-p",
            self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test"),
        ]

        needs_db = False
        if tool in ["cargo", "diesel"]:
            needs_db = tool == "diesel"
            if tool == "cargo" and tool_args:
                first_arg = tool_args[0]
                if first_arg in ["run", "test", "nextest"]:
                    needs_db = True

            if needs_db:
                _log_info(f"🔹 Bringing up backend dependencies for {tool}...")
                _lifecycle_up("infra-db")

                _log_info(
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
                compose_cmd.extend(
                    ["run", "-T", "--rm", "--no-deps", "--entrypoint", "bun", service]
                )
                if tool_args:
                    compose_cmd.extend(tool_args)
            else:
                is_running = self.is_service_running(service)
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

        self._execute_safe(compose_cmd)

    def _handle_run_npm(
        self,
        compose_cmd: list[str],
        tool_args: list[str],
        service: str,
        is_running: bool = True,
    ) -> None:
        """Run npm/bun commands via compose exec/run into container.

        Auto-detects if container is running. If not, uses compose run with
        custom entrypoint. Transparently rewrites 'typecheck' → 'type-check'
        for compatibility.
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
        """Run cargo commands via compose inside the Rust service container.

        Sets the working directory based on the caller's relative path and
        transparently rewrites 'cargo test' → 'cargo nextest run' for the
        project's preferred test runner.
        """
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
