#!/usr/bin/env python3
"""CLI argument parser extracted from Orchestrator (Single Responsibility Principle).

Builds the complete argparse.ArgumentParser for the leedevkit CLI.
Orchestrator calls CliParser(tool_map).build() instead of defining
~180 lines of parser setup inline.
"""

from __future__ import annotations

import argparse
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
            epilog="Examples:\n  leedevkit test all\n  leedevkit dev up\n  leedevkit dev down",
        )
        parent_parser = argparse.ArgumentParser(add_help=False)
        parent_parser.add_argument("--dry-run", action="store_true")

        subparsers = parser.add_subparsers(dest="command", required=True)
        self._setup_test_parser(subparsers, parent_parser)
        self._setup_dev_parser(subparsers, parent_parser)
        self._setup_top_level_commands(subparsers, parent_parser)
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
            "  leedevkit test all                         # Required full regression gate\n"
            "  leedevkit test all --json                  # Final machine-readable report\n"
            "  leedevkit test all --json-stream --quiet   # JSONL events, compact output\n"
            "  leedevkit test infra --lint-only           # Focused fast feedback (partial)\n"
            "  leedevkit test infra --lint-only --fix    # Auto-fix formatting\n"
            "\n"
            "Normal startup runs `compose up -d --build` for active services and reuses Compose/BuildKit cache; --skip-build uses existing images and reports partial/unverified. Focused flags and --pattern never prove full regression.",
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
            help="Skip linting; result is partial/unverified",
        )
        test_parser.add_argument(
            "--skip-e2e",
            action="store_true",
            help="Skip integration/E2E; result is partial/unverified",
        )
        test_parser.add_argument(
            "--skip-build",
            action="store_true",
            help="Skip automatic image build (--no-build); missing/stale images fail; result is partial/unverified",
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
            help="Output one machine-readable JSON summary at the end",
        )
        test_parser.add_argument(
            "--json-stream",
            action="store_true",
            help="Output machine-readable JSONL lifecycle events",
        )
        test_parser.add_argument(
            "--quiet",
            action="store_true",
            help="Suppress task logs; keep compact failures and summary",
        )

    def _setup_dev_parser(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        dev_parser = subparsers.add_parser(
            "dev",
            parents=[parent_parser],
            description="LeeDevKit development environment",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="Examples:\n  leedevkit dev up         # Start development environment\n  leedevkit dev down       # Stop development environment\n  leedevkit dev logs       # Follow development logs",
        )
        dev_cmd_sub = dev_parser.add_subparsers(dest="subcommand", required=True)

        for cmd in ["up", "down", "ps", "clean"]:
            dev_cmd_sub.add_parser(cmd, parents=[parent_parser])

        logs_p = dev_cmd_sub.add_parser("logs", parents=[parent_parser])
        logs_p.add_argument("service", nargs="?")

        exec_p = dev_cmd_sub.add_parser("exec", parents=[parent_parser])
        exec_p.add_argument("service")
        exec_p.add_argument("args", nargs=argparse.REMAINDER)

    def _setup_top_level_commands(
        self,
        subparsers: typing.Any,  # noqa: ANN401
        parent_parser: argparse.ArgumentParser,
    ) -> None:
        init_p = subparsers.add_parser("init", parents=[parent_parser])
        init_p.set_defaults(subcommand="init")
        init_p.add_argument(
            "--force", action="store_true", help="Overwrite existing files"
        )

        doctor_p = subparsers.add_parser("doctor", parents=[parent_parser])
        doctor_p.set_defaults(subcommand="doctor")
        doctor_p.add_argument(
            "--fix",
            action="store_true",
            help="Repair missing worktree runtime, venv, and AI rules",
        )

        skills_p = subparsers.add_parser("skills", parents=[parent_parser])
        skills_p.set_defaults(subcommand="skills")
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

        for command in [
            "sync:api",
            "test:infra",
            "migrate:run",
            "migrate:revert",
            "migrate:status",
            "db:setup",
            "fmt:infra",
            "verify:infra",
        ]:
            command_parser = subparsers.add_parser(command, parents=[parent_parser])
            command_parser.set_defaults(subcommand=command)

        db_query_p = subparsers.add_parser("db:query", parents=[parent_parser])
        db_query_p.set_defaults(subcommand="db:query")
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
            "--project",
            default=None,
            metavar="PROJECT",
            help=(
                "Compose project to attach to: 'dev' maps to leedevkit-dev; "
                "an explicit name attaches to that test project; omitted creates an isolated one"
            ),
        )
        run_p.add_argument(
            "--pooler", action="store_true", help="Enable connection pooler"
        )
        run_p.add_argument(
            "tool", choices=list(self._tool_map.keys()), help="Tool to run"
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
