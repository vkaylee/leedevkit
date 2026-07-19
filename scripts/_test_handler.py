#!/usr/bin/env python3
"""Test handler extracted from Orchestrator (Single Responsibility Principle).

Handles: test orchestration, phase execution, infra lint/format/verify,
and test result summary parsing. Depends on an Orchestrator-like object for
shared state and lifecycle management.
"""

from __future__ import annotations

import re
import sys
import time
import typing
from typing import TYPE_CHECKING

from _bootstrap import DEVKIT_ROOT, PROJECT_ROOT, SCRIPTS_DIR
from _lifecycle import lifecycle_down
from _lifecycle import lifecycle_up as _lifecycle_up
from _test_modules import (
    leedevkit_run_coverage,
    leedevkit_run_integration,
    leedevkit_run_lint,
    leedevkit_run_unit,
)

if TYPE_CHECKING:
    import argparse
    from typing import Any


def _log_info(msg: str) -> None:
    import sys as _sys
    print(f"\033[0;34m\033[1mℹ️ {msg}\033[0m", file=_sys.stderr, flush=True)


def _log_success(msg: str) -> None:
    import sys as _sys
    print(f"\033[0;32m\033[1m✅ {msg}\033[0m", file=_sys.stderr, flush=True)


def _log_warn(msg: str) -> None:
    import sys as _sys
    print(f"\033[1;33m\033[1m⚠️ {msg}\033[0m", file=_sys.stderr, flush=True)


def _log_error(msg: str) -> None:
    import sys as _sys
    print(f"\033[0;31m\033[1m❌ {msg}\033[0m", file=_sys.stderr, flush=True)


class TestHandler:
    """Test orchestration extracted from the Orchestrator god class.

    Manages the test pipeline: parse args → resolve targets → run phases
    (lint, unit, integration, coverage) → print summary.
    """

    __slots__ = ("_orch",)

    def __init__(self, orchestrator: Any) -> None:
        self._orch = orchestrator

    # ── helpers that forward to the orchestrator ──

    @property
    def _dry_run(self) -> bool:
        return self._orch.dry_run

    @property
    def _results(self) -> dict[str, dict[str, typing.Any]]:
        return self._orch.results

    @property
    def _start_time(self) -> float:
        return self._orch.start_time

    def _execute_safe(self, cmd: list[str], env: dict[str, str] | None = None, timeout: int = 1800) -> None:
        self._orch.execute_safe(cmd, env=env, timeout=timeout)

    def _build_mode_map(self) -> dict[str, str]:
        return self._orch._build_mode_map()

    def _inject_rust_version_env(self) -> None:
        self._orch._inject_rust_version_env()

    # ── public API ──

    def handle_test(self, args: argparse.Namespace) -> None:
        """Orchestrate a full test run: resolve target → run phases → summarize."""
        target = getattr(args, "target", None)

        if target == "infra":
            if args.lint_only:
                self.handle_lint_infra()
            else:
                self.handle_verify_infra()
            return

        mode_map = self._build_mode_map()
        service_names = [k for k in mode_map if k not in ("all", "api", "web")]

        mode = mode_map.get(target or "all", "all")
        component_filter = target if target in service_names else ""
        args.component = component_filter

        self._orch.active_mode = mode
        self._inject_rust_version_env()
        _log_info(f"🚀 Starting LeeDevKit Test Suite for [{target}] in mode [{mode}]")

        if getattr(args, "timeout", None):
            import os
            timeout_str = str(args.timeout)
            for key in ("TIMEOUT_LINT", "TIMEOUT_UNIT", "TIMEOUT_INTEGRATION", "TIMEOUT_BUILD"):
                os.environ[key] = timeout_str
                self._orch.env_vars[key] = timeout_str

        if target == "all" and not (args.lint_only or args.unit_only or args.e2e_only):
            from _orchestrator import _resolve_targets

            resolved = _resolve_targets()
            sub_targets = [t for t in resolved if t != "all"] or ["infra"]
            for sub_target in sub_targets:
                args.target = sub_target
                self.handle_test(args)
                if self._orch.needs_cleanup and not self._dry_run:
                    lifecycle_down("all")
                    self._orch.needs_cleanup = False
            args.target = "all"
            self.print_test_summary("all")
            return

        self._orch.needs_cleanup = True

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

        if getattr(args, "json_output", False) and self._results:
            import json
            print(json.dumps(self._results, indent=2), file=sys.stderr, flush=True)

        is_single_phase = (
            getattr(args, "lint_only", False)
            or getattr(args, "unit_only", False)
            or getattr(args, "e2e_only", False)
        )
        if is_single_phase:
            target_name = target or "all"
            _log_success(
                "\n💡 Tip: Isolated phase completed successfully! Run the full suite to verify everything:"
            )
            _log_success(f"   leedevkit test {target_name}\n")

    def run_phase(self, phase_name: str, mode: str, args: argparse.Namespace) -> None:
        """Execute a single test phase (lint, unit, integration, coverage, db setup)."""
        if self._dry_run:
            _log_info(f"🔍 Dry-run: Phase [{phase_name}] for [{mode}]")
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
            _log_info(
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

        _log_info(f"🔹 Running {phase_name}...")
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
            "Database Setup": self._orch.handle_db_setup_phase,
            "Prebuild": self._orch.handle_prebuild_phase,
        }

        func = func_map.get(phase_name)
        if func is None:
            _log_error(f"Unknown phase: {phase_name}")
            sys.exit(1)

        start = time.time()
        res = func()
        elapsed = time.time() - start

        self._results[phase_name] = {
            "status": "pass" if res else "fail",
            "duration_s": round(elapsed, 1),
        }

        if granular_mode:
            _log_info(
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
                    _log_warn(
                        f"\n💡 Tip: To quickly verify only linting/formatting fixes, run:\n   leedevkit test {target} --lint-only\n"
                    )
                elif phase_name == "Unit Tests":
                    _log_warn(
                        f"\n💡 Tip: To focus on unit tests and skip linting/e2e, run:\n   leedevkit test {target} --unit-only\n"
                    )
                elif phase_name == "Integration Tests":
                    _log_warn(
                        f"\n💡 Tip: To focus on integration/E2E tests only, run:\n   leedevkit test {target} --e2e-only\n"
                    )
            if phase_name != "Linting":
                sys.exit(1)

    def handle_test_infra(self) -> None:
        """Run all test files with coverage enforcement."""
        import os
        env = os.environ.copy()
        env["PYTHONPATH"] = str(SCRIPTS_DIR)
        tests_dir = SCRIPTS_DIR / "tests"
        test_files = sorted(str(p) for p in tests_dir.glob("test_*.py"))
        venv_pytest = DEVKIT_ROOT / ".venv" / "bin" / "pytest"
        cmd = [
            str(venv_pytest),
            "--cov=scripts",
            "--cov-report=term-missing",
            "--cov-fail-under=80",
        ] + test_files
        self._execute_safe(cmd, env=env)

    def handle_lint_infra(self) -> None:
        """Run ruff + mypy on infra scripts, plus shellcheck on shell scripts."""
        import shutil as _shutil
        venv_bin = DEVKIT_ROOT / ".venv" / "bin"
        self._execute_safe([str(venv_bin / "ruff"), "check", str(SCRIPTS_DIR)])
        self._execute_safe([str(venv_bin / "mypy"), str(SCRIPTS_DIR)])

        sh_files = [str(f) for f in PROJECT_ROOT.glob("*.sh")]
        sh_files += [str(f) for f in SCRIPTS_DIR.glob("*.sh")]
        if _shutil.which("shellcheck"):
            self._execute_safe(["shellcheck"] + sh_files)

        _log_success("✨ Infrastructure linting passed!")

    def handle_fmt_infra(self) -> None:
        """Format all infra scripts with ruff."""
        venv_bin = DEVKIT_ROOT / ".venv" / "bin"
        self._execute_safe([str(venv_bin / "ruff"), "format", str(SCRIPTS_DIR)])
        _log_success("✨ Infrastructure formatting completed!")

    def handle_verify_infra(self) -> None:
        """Run the full infra verification pipeline: fmt → lint → test."""
        self.handle_fmt_infra()
        self.handle_lint_infra()
        self.handle_test_infra()

    def print_test_summary(self, target: str) -> None:
        """Parse test log files and print a summary of passed/total tests."""
        total_tests = 0
        passed_tests = 0

        nextest_pattern = re.compile(r"Summary \[.*?\] (\d+) tests? run: (\d+) passed")
        vitest_pattern = re.compile(r"Tests\s+(\d+)\s+passed\s+\((\d+)\)")
        pw_pattern = re.compile(r"^\s*(\d+)\s+passed\s+\(.*?\)", re.MULTILINE)

        log_dir = PROJECT_ROOT / ".test_logs"
        if log_dir.exists():
            for log_file in log_dir.glob("*.log"):
                try:
                    if log_file.stat().st_mtime < self._start_time:
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
                    pass

        if total_tests > 0:
            msg = f"All selected tests for [{target}] passed successfully! ({passed_tests}/{total_tests} tests)"
            _log_success(msg)
        else:
            _log_success(f"All selected tests for [{target}] passed successfully!")
