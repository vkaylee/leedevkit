#!/usr/bin/env python3
"""Database query, Diesel, and Compose helpers extracted from Orchestrator.

Depends on an Orchestrator-like object for engine/compose resolution, env vars,
and safe command execution.
"""

from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING

from _bootstrap import PROJECT_ROOT
from _handler_base import HandlerBase
from _logging import log_info

if TYPE_CHECKING:
    import argparse


class DbHandler(HandlerBase):
    """Database operations extracted from the Orchestrator god class.

    Inherits shared property forwarding and _execute_safe from HandlerBase.
    """

    # ── public API ──

    def get_compose_files(self, env: str) -> list[str]:
        """Return compose-file arguments for the given environment."""
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
            project_name = self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
            return ["-p", project_name, "-f", ".compose/docker-compose.test.yml"]
        return ["-p", "leedevkit", "-f", "docker-compose.yml"]

    def handle_db_query(self, args: argparse.Namespace) -> None:
        """Execute an arbitrary SQL query against the dev database."""
        sql = args.sql
        log_info(f"🗄️ Executing SQL: {sql}")
        cmd = [
            self._engine,
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
        self._execute_safe(cmd)

    def handle_diesel(self, args: list[str]) -> None:
        """Wrapper for diesel CLI commands inside the apiserver container."""
        import contextlib as _ctxlib

        caller_dir = Path.cwd()
        rel_dir = ""
        with _ctxlib.suppress(ValueError):
            rel_dir = str(caller_dir.relative_to(PROJECT_ROOT))

        workdir = f"/workspace/{rel_dir}" if rel_dir else "/workspace"
        cmd = (
            self._compose_engine
            + [
                "-p",
                self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test"),
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
        self._execute_safe(cmd)
