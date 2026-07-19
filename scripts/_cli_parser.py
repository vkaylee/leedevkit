#!/usr/bin/env python3
"""CLI argument parser extracted from Orchestrator (Single Responsibility Principle).

Builds the complete argparse.ArgumentParser for the leedevkit CLI.
Orchestrator calls CliParser(tool_map).build() instead of defining
~180 lines of parser setup inline.
"""

from __future__ import annotations

import argparse
import sys
import typing

from _devkit_config import resolve_targets


class CliParser:
    """Build the leedevkit CLI argument parser.

    Extracted from Orchestrator.setup_parser() and its four sub-methods
    to keep the Orchestrator focused on orchestration, not CLI plumbing.
    """

    __slots__ = ("_tool_map",)

    def __init__(self, tool_map: dict[str, str]) -> None:
        self._tool_map = tool_map

    def build(self) -> argparse.ArgumentParser:
        """Build and return the complete argument parser."""
        parser = argparse.ArgumentParser(
            prog="leedevkit",
            description="LeeDevKit Enterprise Orchestrator",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples:\n  leedevkit test all\n  leedevkit manage up dev\n  leedevkit manage sync:api",
        )
        parent_parser = argparse.ArgumentParser(add_help=False)
        parent_parser.add_argument("--dry-run", action="store_true")

        subparsers = parser.add_subparsers(dest="command", required=True)
        self._setup_test_parser(subparsers, parent_parser)
        self._setup_manage_parser(subparsers, parent_parser)
        self._setup_run_parser(subparsers, parent_parser)
        self._setup_update_parser(subparsers, parent_parser)

        return parser

    def _setup_test_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        test_parser = subparsers.add_parser(
            "test",
            prog=None,
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
        targets = resolve_targets()
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
        skills_install.add_argument(
            "name",
            nargs="?",
            help="Skill name from catalog (or leave empty to install from leedevkit.toml)",
        )
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
            "tool", choices=list(self._tool_map.keys()), help="Tool to run"
        )
        run_p.add_argument(
            "--pooler", action="store_true", help="Enable connection pooler"
        )
        run_p.add_argument(
            "args", nargs=argparse.REMAINDER, help="Arguments for the tool"
        )

    def _setup_update_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        update_p = subparsers.add_parser(
            "update",
            parents=[parent_parser],
            help="Update devkit to the latest (or pinned) release",
            description="Download a release tarball from GitHub and overlay it onto this devkit install.",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples:\n  leedevkit update               # Update to the latest release\n  leedevkit update --version v0.2.0   # Pin to a specific release",
        )
        update_p.add_argument(
            "--version",
            default=None,
            help="Pin to a specific release tag (e.g. v0.2.0). Default: latest",
        )
