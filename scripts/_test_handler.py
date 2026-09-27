#!/usr/bin/env python3
"""Test handler extracted from Orchestrator (Single Responsibility Principle).

Handles: test orchestration, phase execution, infra lint/format/verify,
and test result summary parsing. Depends on an Orchestrator-like object for
shared state and lifecycle management.
"""

import argparse
import datetime
import json
import re
import shlex
import sys
import time
import typing
import uuid
from typing import TYPE_CHECKING


from _bootstrap import DEVKIT_ROOT, PROJECT_ROOT, SCRIPTS_DIR
from _devkit_config import build_mode_map, inject_rust_version_env
from _handler_base import HandlerBase
from _lifecycle import lifecycle_down
from _lifecycle import lifecycle_up as _lifecycle_up
from _logging import log_error, log_info, log_success, log_warn
from _test_modules import (
    leedevkit_run_coverage,
    leedevkit_run_integration,
    leedevkit_run_lint,
    leedevkit_run_unit,
)
from _test_utils import (
    clear_last_phase_commands,
    get_last_phase_commands,
    get_last_phase_exit_code,
    set_quiet_output,
)

if TYPE_CHECKING:
    import argparse


class TestHandler(HandlerBase):
    """Test orchestration extracted from the Orchestrator god class.

    Inherits shared property forwarding and _execute_safe from HandlerBase.
    Manages the test pipeline: parse args → resolve targets → run phases
    (lint, unit, integration, coverage) → print summary.
    """

    # ── helpers that forward to the orchestrator ──

    @property
    def _results(self) -> dict[str, dict[str, typing.Any]]:
        return self._orch.results

    @property
    def _start_time(self) -> float:
        return self._orch.start_time

    # ── public API ──

    def handle_test(self, args: argparse.Namespace) -> None:
        """Run configured phases and emit a truthful quality-gate result."""
        nested = bool(getattr(args, "_nested_test", False))
        self._event_args = args
        machine_output = self._flag(args, "json_output") or self._flag(
            args, "json_stream"
        )
        set_quiet_output(self._flag(args, "quiet") or machine_output)
        if machine_output or self._flag(args, "quiet"):
            self._orch.env_vars["LEEDEVKIT_QUIET"] = "1"
        else:
            self._orch.env_vars.pop("LEEDEVKIT_QUIET", None)
        if not nested:
            self._run_id = uuid.uuid4().hex
            self._run_started = time.time()
            self._event_sequence = 0
            self._emit_event("run_started", target=getattr(args, "target", "all"))
        target = getattr(args, "target", None)
        if target == "infra":
            self._run_infra_target(args)
            if not nested:
                self._finish_test(args, "infra")
            return

        mode_map = build_mode_map()
        service_names = [k for k in mode_map if k not in ("all", "api", "web")]
        mode = mode_map.get(target or "all", "all")
        args.component = target if target in service_names else ""
        self._orch.active_mode = mode
        if mode != "go":
            inject_rust_version_env()

        timeout = getattr(args, "timeout", None)
        if timeout:
            import os

            for key in (
                "TIMEOUT_LINT",
                "TIMEOUT_UNIT",
                "TIMEOUT_INTEGRATION",
                "TIMEOUT_BUILD",
            ):
                os.environ[key] = str(timeout)
                self._orch.env_vars[key] = str(timeout)

        if target == "all" and not nested:
            from _devkit_config import resolve_test_targets

            sub_targets = resolve_test_targets("all")
            if any(t != "infra" for t in sub_targets) and not self._flag(
                args, "skip_build"
            ):
                self.run_phase("Prebuild", "all", args, target_name="all")
            elif any(t != "infra" for t in sub_targets):
                self._record_result("Prebuild", "all", "compose build", "skipped", None)
            for sub_target in sub_targets:
                args.target = sub_target
                args._nested_test = True
                self.handle_test(args)
            args.target = "all"
            args._nested_test = False
            self._finish_test(args, "all")
            return

        if self._flag(args, "e2e_only") and not nested:
            if self._flag(args, "skip_build"):
                self._record_result(
                    "Prebuild", target or "all", "compose build", "skipped", None
                )
            else:
                self.run_phase("Prebuild", mode, args, target_name=target or "all")

        self._orch.needs_cleanup = True
        run_lint = not self._flag(args, "skip_lint") and not (
            self._flag(args, "unit_only") or self._flag(args, "e2e_only")
        )
        run_unit = not (self._flag(args, "lint_only") or self._flag(args, "e2e_only"))
        run_e2e = not (self._flag(args, "lint_only") or self._flag(args, "unit_only"))
        if self._flag(args, "skip_e2e"):
            run_e2e = False
        if self._flag(args, "coverage"):
            run_unit = False
            run_e2e = False

        selected = [
            ("Unit Tests", run_unit),
            ("Integration Tests", run_e2e),
            ("Linting", run_lint),
        ]
        for phase, enabled in selected:
            if enabled:
                self.run_phase(phase, mode, args, target_name=target or "all")
            else:
                self._record_result(phase, target or "all", phase, "skipped", None)
        if self._flag(args, "coverage"):
            self.run_phase("Coverage", mode, args, target_name=target or "all")

        if not nested:
            self._finish_test(args, target or "all")

    def _run_infra_target(self, args: argparse.Namespace) -> None:
        phase = self._select_infra_phase(args)
        start = time.time()
        status = "passed"
        exit_code = 0
        command = (
            f"leedevkit test infra --{phase}-only"
            if phase != "full"
            else "leedevkit test infra"
        )
        try:
            if phase == "lint":
                self.handle_lint_infra()
            elif phase == "unit":
                self.handle_test_infra()
            else:
                self.handle_verify_infra()
        except SystemExit as exc:
            status = "failed"
            exit_code = int(exc.code) if isinstance(exc.code, int) else 1
        except Exception:
            status = "failed"
            exit_code = 1
        self._record_result(
            "Infrastructure",
            getattr(args, "target", "infra"),
            command,
            status,
            exit_code,
            start,
        )

    def _record_result(
        self,
        phase: str,
        target: str,
        command: str,
        status: str,
        exit_code: int | None,
        started: float | None = None,
        error: str | None = None,
    ) -> None:
        start = started or time.time()
        end = time.time()
        key = (
            phase
            if target in ("infra", "api", "web", "go") and phase not in self._results
            else f"{target}:{phase}"
        )
        result: dict[str, typing.Any] = {
            "phase": phase,
            "target": target,
            "command": command,
            "start_time": datetime.datetime.fromtimestamp(
                start, datetime.timezone.utc
            ).isoformat(),
            "end_time": datetime.datetime.fromtimestamp(
                end, datetime.timezone.utc
            ).isoformat(),
            "duration_s": round(end - start, 3),
            "exit_code": exit_code,
            "status": status,
            "required": status not in ("skipped",),
        }
        if phase in ("Linting", "Unit Tests", "Integration Tests", "Coverage"):
            commands = get_last_phase_commands()
            if commands:
                result["commands"] = {
                    name: shlex.join(cmd) for name, cmd in sorted(commands.items())
                }
        if error:
            result["error"] = error
        if status in ("failed", "blocked"):
            result["log"] = f".test_logs/{phase.replace(' ', '_')}.log"
            result["rerun"] = command
        self._results[key] = result
        if status in ("failed", "blocked"):
            log_path = PROJECT_ROOT / ".test_logs" / f"{phase.replace(' ', '_')}.log"
            try:
                log_path.parent.mkdir(parents=True, exist_ok=True)
                if not log_path.exists():
                    log_path.write_text(
                        f"phase={phase}\nstatus={status}\nexit_code={exit_code}\n"
                        f"command={command}\nerror={error or ''}\n"
                    )
            except OSError:
                pass
        self._emit_event("phase_finished", **result)

    def _finish_test(self, args: argparse.Namespace, target: str) -> None:
        statuses = [result.get("status") for result in self._results.values()]
        failed = any(status == "failed" for status in statuses)
        blocked = any(status == "blocked" for status in statuses)
        partial = any(status == "skipped" for status in statuses) or bool(
            getattr(args, "pattern", "")
        )
        focused = any(
            self._flag(args, name) for name in ("lint_only", "unit_only", "e2e_only")
        )
        partial = (
            partial
            or focused
            or self._flag(args, "skip_lint")
            or self._flag(args, "skip_e2e")
            or self._flag(args, "skip_build")
        )
        overall = (
            "failed"
            if failed
            else "blocked"
            if blocked
            else "partial"
            if partial
            else "passed"
        )
        self.print_test_summary(target, overall)
        report = self._build_report(target, overall)
        if self._flag(args, "json_output") and not self._flag(args, "json_stream"):
            print(json.dumps(report, indent=2, sort_keys=True), flush=True)
        self._emit_event("run_finished", **report)
        if overall in ("failed", "blocked"):
            sys.exit(1)

        if overall == "partial":
            log_warn(
                "Focused or incomplete verification: result is partial/unverified, not a full regression pass."
            )

    def _build_report(self, target: str, overall: str) -> dict[str, typing.Any]:
        phases = dict(sorted(self._results.items()))
        counts = {
            status: sum(
                1 for result in phases.values() if result.get("status") == status
            )
            for status in ("passed", "failed", "skipped", "blocked")
        }
        failures = [
            result for result in phases.values() if result.get("status") == "failed"
        ]
        blocked = [
            result for result in phases.values() if result.get("status") == "blocked"
        ]
        skipped = [
            result for result in phases.values() if result.get("status") == "skipped"
        ]
        next_action = self._next_action(failures, blocked, skipped)
        first_failure = failures[0] if failures else blocked[0] if blocked else None
        exit_code = (
            int(first_failure["exit_code"])
            if first_failure and first_failure.get("exit_code") is not None
            else 0
        )
        return {
            "schema_version": 1,
            "run_id": getattr(self, "_run_id", ""),
            "status": overall,
            "full_regression": overall == "passed",
            "exit_code": exit_code,
            "target": target,
            "started_at": self._iso_time(getattr(self, "_run_started", time.time())),
            "finished_at": self._iso_time(time.time()),
            "duration_s": round(
                time.time() - getattr(self, "_run_started", time.time()), 3
            ),
            "counts": counts,
            "phases": phases,
            "failed": failures,
            "blocked": blocked,
            "skipped": skipped,
            "next_action": next_action,
        }

    @staticmethod
    def _iso_time(value: float) -> str:
        return datetime.datetime.fromtimestamp(value, datetime.timezone.utc).isoformat()

    @staticmethod
    def _next_action(
        failures: list[dict[str, typing.Any]],
        blocked: list[dict[str, typing.Any]],
        skipped: list[dict[str, typing.Any]],
    ) -> str | None:
        result: dict[str, typing.Any] | None = (
            failures[0]
            if failures
            else blocked[0]
            if blocked
            else skipped[0]
            if skipped
            else None
        )
        if result is None:
            return None
        if result.get("status") == "blocked":
            return "Fix dependency, setup, or readiness failure, then rerun the phase."
        if result.get("status") == "skipped":
            return "Run without focused or skip flags for full regression verification."
        return "Inspect phase log and rerun the reported command."

    def _emit_event(self, event: str, **payload: typing.Any) -> None:
        args = getattr(self, "_event_args", None)
        if args is None or not self._flag(args, "json_stream"):
            return
        sequence = getattr(self, "_event_sequence", 0) + 1
        self._event_sequence = sequence
        record = {
            "schema_version": 1,
            "run_id": getattr(self, "_run_id", ""),
            "sequence": sequence,
            "event": event,
            "timestamp": self._iso_time(time.time()),
            **payload,
        }
        print(json.dumps(record, sort_keys=True), flush=True)

    @staticmethod
    def _flag(args: argparse.Namespace, name: str) -> bool:
        """Read parser booleans without treating missing mock attributes as true."""
        return getattr(args, name, False) is True

    def _select_infra_phase(self, args: argparse.Namespace) -> str:
        """Select infra phase and reject flags that cannot be combined."""
        lint_only = self._flag(args, "lint_only")
        unit_only = self._flag(args, "unit_only")
        e2e_only = self._flag(args, "e2e_only")
        coverage = self._flag(args, "coverage")
        skip_lint = self._flag(args, "skip_lint")

        if e2e_only:
            log_error("The infra target does not support --e2e-only")
            sys.exit(2)
        if lint_only and unit_only:
            log_error("The infra target cannot combine --lint-only and --unit-only")
            sys.exit(2)
        if coverage and (lint_only or unit_only):
            log_error(
                "The infra target cannot combine --coverage with an isolated phase"
            )
            sys.exit(2)
        if skip_lint and not unit_only:
            log_error("The infra target does not support --skip-lint")
            sys.exit(2)
        if skip_lint and unit_only:
            return "unit"
        if lint_only:
            return "lint"
        if unit_only:
            return "unit"
        return "full"

    @staticmethod
    def _phase_command(args: argparse.Namespace, phase: str, target: str) -> str:
        command = f"./leedevkit test {target}"
        if phase == "Linting":
            command += " --lint-only"
        elif phase == "Unit Tests":
            command += " --unit-only"
        elif phase == "Integration Tests":
            command += " --e2e-only"
        for name, flag in (
            ("skip_lint", "--skip-lint"),
            ("skip_e2e", "--skip-e2e"),
            ("skip_build", "--skip-build"),
        ):
            if TestHandler._flag(args, name):
                command += f" {flag}"
        pattern = getattr(args, "pattern", "") or ""
        if pattern:
            command += f" --pattern {shlex.quote(pattern)}"
        return command

    def run_phase(
        self,
        phase_name: str,
        mode: str,
        args: argparse.Namespace,
        target_name: str | None = None,
    ) -> None:
        """Execute phase, retain failure details, and always tear down its environment."""
        target = str(target_name or getattr(args, "target", "all") or "all")
        clear_last_phase_commands()
        if self._dry_run:
            self._record_result(phase_name, target, phase_name, "skipped", None)
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
            ("Linting", "go"): "lint-go",
            ("Unit Tests", "go"): "unit-go",
            ("Integration Tests", "go"): "int-go",
            ("Coverage", "go"): "go",
        }
        granular_mode = granular_map.get((phase_name, mode))
        started = time.time()
        command = self._phase_command(args, phase_name, target)
        self._emit_event(
            "phase_started", phase=phase_name, target=target, command=command
        )
        setup_ok = True
        try:
            if granular_mode:
                log_info(
                    f"🔹 Starting isolated environment for: {phase_name} ({granular_mode})"
                )
                try:
                    setup_ok = _lifecycle_up(granular_mode)
                    if (
                        setup_ok
                        and phase_name == "Integration Tests"
                        and granular_mode == "int-api"
                    ):
                        for dependency_mode in (
                            "infra-db",
                            "infra-redis",
                            "infra-pooler",
                        ):
                            setup_ok = _lifecycle_up(dependency_mode) and setup_ok
                except Exception as exc:
                    log_error(f"Phase setup failed: {exc}")
                    self._record_result(
                        phase_name,
                        target,
                        command,
                        "blocked",
                        1,
                        started,
                        error=str(exc),
                    )
                    return
                if not setup_ok:
                    self._record_result(
                        phase_name,
                        target,
                        command,
                        "blocked",
                        1,
                        started,
                        error="Lifecycle readiness failed",
                    )
                    return

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
                    getattr(args, "pattern", "") or "",
                ),
                "Database Setup": self._orch.handle_db_setup_phase,
                "Prebuild": self._orch.handle_prebuild_phase,
            }
            func = func_map.get(phase_name)
            if func is None:
                self._record_result(phase_name, target, command, "failed", 1, started)
                log_error(f"Unknown phase: {phase_name}")
                sys.exit(1)
            res = func()
            task_exit_code = get_last_phase_exit_code()
            phase_exit_code = (
                task_exit_code
                if task_exit_code is not None
                else (0 if res is not False else 1)
            )
            phase_status = (
                "failed"
                if res is False or task_exit_code not in (None, 0)
                else "passed"
            )
            self._record_result(
                phase_name, target, command, phase_status, phase_exit_code, started
            )
            if phase_status == "failed" and any(
                self._flag(args, name)
                for name in ("lint_only", "unit_only", "e2e_only")
            ):
                sys.exit(phase_exit_code or 1)
        except TimeoutError as exc:
            self._record_result(
                phase_name,
                target,
                command,
                "failed",
                124,
                started,
                error=str(exc) or "Phase timed out",
            )
        except SystemExit as exc:
            code = int(exc.code) if isinstance(exc.code, int) else 1
            self._record_result(phase_name, target, command, "failed", code, started)
            raise
        except Exception as exc:
            log_error(f"Phase {phase_name} failed: {exc}")
            self._record_result(
                phase_name, target, command, "failed", 1, started, error=str(exc)
            )
        finally:
            if granular_mode:
                log_info(
                    f"🔹 Tearing down isolated environment for: {phase_name} ({granular_mode})"
                )
                lifecycle_down("all")

    def handle_test_infra(self) -> None:
        """Run infra tests with production and non-gating test-source reports."""
        import os

        env = os.environ.copy()
        env["LEEDEVKIT_QUIET"] = "0"
        self._orch.env_vars["LEEDEVKIT_QUIET"] = "0"
        tests_dir = SCRIPTS_DIR / "tests"
        env["PYTHONPATH"] = str(SCRIPTS_DIR)
        test_files = sorted(str(p) for p in tests_dir.glob("test_*.py"))
        venv_pytest = DEVKIT_ROOT / ".venv" / "bin" / "pytest"
        production_cmd = [
            str(venv_pytest),
            "--cov=scripts",
            "--cov-config",
            str(SCRIPTS_DIR / ".coveragerc"),
            "--cov-report=term-missing",
            "--cov-report=html:.test_logs/coverage-production",
            "--cov-fail-under=80",
        ] + test_files
        self._execute_safe(production_cmd, env=env)

        test_source_cmd = [
            str(venv_pytest),
            "--cov=scripts/tests",
            "--cov-config",
            "/dev/null",
            "--cov-report=term-missing",
            "--cov-report=html:.test_logs/coverage-tests",
        ] + test_files
        self._execute_safe(test_source_cmd, env=env)

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

        log_success("✨ Infrastructure linting passed!")

    def handle_fmt_infra(self) -> None:
        """Format all infra scripts with ruff."""
        venv_bin = DEVKIT_ROOT / ".venv" / "bin"
        self._execute_safe([str(venv_bin / "ruff"), "format", str(SCRIPTS_DIR)])
        log_success("✨ Infrastructure formatting completed!")

    def handle_format_check_infra(self) -> None:
        """Check infra formatting without changing source files."""
        venv_bin = DEVKIT_ROOT / ".venv" / "bin"
        self._execute_safe(
            [str(venv_bin / "ruff"), "format", "--check", str(SCRIPTS_DIR)]
        )
        log_success("✨ Infrastructure formatting verified!")

    def handle_verify_infra(self) -> None:
        """Run the read-only infra verification pipeline."""
        self.handle_format_check_infra()
        self.handle_lint_infra()
        self.handle_test_infra()

    def print_test_summary(self, target: str, overall: str | None = None) -> None:
        """Parse test log files and print a summary of passed/total tests.

        Defense-in-depth: if any phase recorded a failure in self._results
        (e.g. clippy lint failing), print FAILED instead of the green
        "all passed" line. The old logic only counted test-log patterns, so a
        failing lint phase could still print a green summary.
        """
        failed_phases = [
            phase
            for phase, result in self._results.items()
            if result.get("status") in ("failed", "fail", "blocked")
        ]
        if failed_phases:
            log_error(
                f"❌ Phase(s) FAILED/BLOCKED: {', '.join(failed_phases)}. "
                "Not all checks passed — see per-phase logs above."
            )
            return

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
                except (OSError, UnicodeDecodeError, ValueError):
                    pass

        if overall == "partial":
            log_warn(
                f"⚠️ Partial verification for [{target}]; full regression not verified."
            )
        elif total_tests > 0:
            log_success(
                f"All selected tests for [{target}] passed successfully! ({passed_tests}/{total_tests} tests)"
            )
        elif overall not in ("failed", "blocked"):
            log_success(f"All selected tests for [{target}] passed successfully!")
