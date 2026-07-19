#!/usr/bin/env python3
"""Database handler extracted from Orchestrator (Single Responsibility Principle).

Handles: DB setup phase, DB queries, diesel commands, compose file resolution,
and prebuild phase. Depends on an Orchestrator-like object for engine/compose
resolution, env vars, and safe command execution.
"""

from __future__ import annotations

import subprocess
import time
from pathlib import Path
from typing import TYPE_CHECKING

from _bootstrap import PROJECT_ROOT
from _handler_base import HandlerBase
from _lifecycle import lifecycle_up as _lifecycle_up
from _logging import log_error, log_info, log_success

if TYPE_CHECKING:
    from typing import Any

    import argparse


class DbHandler(HandlerBase):
    """Database operations extracted from the Orchestrator god class.

    Inherits shared property forwarding and _execute_safe from HandlerBase.
    """

    # ── public API ──

    def _exec_psql(
        self,
        db_container: str,
        *,
        sql: str | None = None,
        input_text: str | None = None,
        tuples_only: bool = False,
        capture: bool = False,
    ) -> "subprocess.CompletedProcess[str]":
        """Run a psql command inside a database container.

        Provides a single canonical form for the repeated pattern of
        ``[engine, exec, -i, <container>, psql, -U, test_user, -d, test_database, ...]``
        that appears throughout handle_db_setup_phase().

        Args:
            db_container: Name of the running Postgres container.
            sql: SQL to pass via ``-c``.
            input_text: SQL to feed via stdin (mutually exclusive with *sql*).
            tuples_only: Add ``-t`` flag for tuples-only output.
            capture: If True, capture stdout/stderr as text; otherwise discard.
        """
        cmd: list[str] = [
            self._engine,
            "exec",
            "-i",
            db_container,
            "psql",
            "-U",
            "test_user",
            "-d",
            "test_database",
        ]
        if tuples_only:
            cmd.append("-t")
        if sql is not None:
            cmd.extend(["-c", sql])

        kwargs: dict[str, Any] = {"check": False}
        if capture:
            kwargs["capture_output"] = True
            kwargs["text"] = True
        else:
            kwargs["stdout"] = subprocess.DEVNULL
            kwargs["stderr"] = subprocess.DEVNULL
        if input_text is not None:
            kwargs["input"] = input_text
            kwargs["text"] = True

        return subprocess.run(cmd, **kwargs)

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

    def handle_prebuild_phase(self) -> bool:
        """Build test Docker images ahead of the test suite."""
        log_info("Building test Docker images...")
        project_name = self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        build_cmd = self._compose_engine + [
            "-p",
            project_name,
            "-f",
            str(PROJECT_ROOT / ".compose" / "docker-compose.test.yml"),
            "build",
        ]
        self._execute_safe(build_cmd)
        log_success("✅ Prebuild complete.")
        return True

    def handle_db_setup_phase(self) -> bool:
        """Bring up DB containers, run migrations, and prepare a template DB.

        Steps:
          1. Start int-api, infra-db, infra-redis containers.
          2. Wait for pg_isready (60 × 1 s).
          3. Drop zombie test_% databases left from crashed runs.
          4. Create a leedevkit_test_template database.
          5. Run migrations on main and template databases.
          6. Revoke CONNECT on the template to prevent accidental use.
        """
        log_info("Bringing up database and apiserver containers...")
        _lifecycle_up("int-api")
        _lifecycle_up("infra-db")
        _lifecycle_up("infra-redis")

        project_name = self._env_vars.get("COMPOSE_PROJECT_NAME", "leedevkit-test")
        db_container = f"{project_name}_db_system_1"

        # ── Wait for database container to be ready ──
        log_info("Waiting for database container to be ready...")
        db_ready = False
        for _ in range(60):
            res = subprocess.run(
                [
                    self._engine,
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

        # ── Drop zombie test databases ──
        log_info("Cleaning up old test databases...")
        cleanup_sql = (
            "SELECT 'DROP DATABASE IF EXISTS \"' || datname || '\";' "
            "FROM pg_database WHERE datname LIKE 'test_db_%';"
        )
        cleanup_res = self._exec_psql(
            db_container, sql=cleanup_sql, tuples_only=True, capture=True
        )
        if cleanup_res.returncode == 0 and cleanup_res.stdout.strip():
            drop_commands = cleanup_res.stdout.strip()
            self._exec_psql(db_container, input_text=drop_commands)

        # ── Create template database ──
        log_info("Creating template database...")
        create_sql = (
            "DROP DATABASE IF EXISTS leedevkit_test_template; "
            "CREATE DATABASE leedevkit_test_template;"
        )
        self._exec_psql(db_container, sql=create_sql)

        # ── Run migrations on main database ──
        log_info("Running migrations on main database...")
        main_db_url = (
            "postgres://test_user:test_password@db_system:5432/"
            "test_database?sslmode=disable"
        )
        main_cmd = self._compose_engine + [
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
        self._execute_safe(main_cmd)

        # ── Run migrations on template database ──
        log_info("Running migrations on template database...")
        template_db_url = (
            "postgres://test_user:test_password@db_system:5432/"
            "leedevkit_test_template?sslmode=disable"
        )
        template_cmd = self._compose_engine + [
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
        self._execute_safe(template_cmd)

        # ── Revoke connect on template to prevent hangs ──
        revoke_sql = "REVOKE CONNECT ON DATABASE leedevkit_test_template FROM public;"
        terminate_sql = (
            "SELECT pg_terminate_backend(pid) FROM pg_stat_activity "
            "WHERE datname = 'leedevkit_test_template' AND pid <> pg_backend_pid();"
        )
        self._exec_psql(db_container, sql=revoke_sql)
        self._exec_psql(db_container, sql=terminate_sql)

        log_success("✅ Database Setup complete.")
        return True
