"""Test function modules — Python equivalents of _test-lint.sh, _test-unit.sh,
_test-integration.sh, _test-coverage.sh.

Provides leedevkit_run_lint(), leedevkit_run_unit(), leedevkit_run_integration(),
and leedevkit_run_coverage() called directly by the orchestrator.

All user-provided strings interpolated into shell commands are escaped via
shlex.quote() to prevent shell injection through --pattern args.
"""

import shlex

from _test_utils import (
    build_compose_exec,
    run_parallel_ordered,
)


def _safe_pattern(pattern: str) -> str:
    """Shell-escape a test pattern for safe interpolation into bash -c."""
    if pattern:
        return shlex.quote(pattern)
    return ""


def _safe_pattern_quoted(pattern: str) -> str:
    """Shell-escape a test pattern that goes inside double quotes (-g \"...\")."""
    if pattern:
        # shlex.quote adds outer quotes, but we need inner escaped value
        return shlex.quote(pattern)
    return ""


def leedevkit_run_lint(
    component_filter: str = "", mode: str = "all", fix: bool = False
) -> bool:
    """Run linting (fmt check/clippy/eslint). Use fix=True to auto-format."""
    tasks: list[tuple[str, str, list[str]]] = []

    # ── Fast local pre-check (no Docker, instant feedback) ──
    # Runs first so AI agents get immediate results without waiting for Docker.
    import shutil as _shutil

    if mode in ("all", "api", "integration") and _shutil.which("cargo"):
        fmt_cmd = ["cargo", "fmt", "--all"]  # pragma: no cover - host-dependent
        if not fix:  # pragma: no cover - host-dependent
            fmt_cmd.extend(["--", "--check"])
        tasks.append(("rust-backend-fmt-check", "host", fmt_cmd))  # pragma: no cover

    if mode in ("all", "api", "integration"):
        pkg_name = "agent" if component_filter == "agent-main" else component_filter
        pkg_flag = (
            f"--package {pkg_name}"
            if component_filter in ["apiserver", "agent-main"]
            else "--workspace"
        )
        fmt_flag = "" if fix else "-- --check"
        backend_cmd = build_compose_exec(
            "apiserver",
            f"cargo fmt --all {fmt_flag} && cargo clippy {pkg_flag} -- -D warnings",
            workdir="/workspace",
            mode=mode,
        )
        task_name = (
            f"rust-backend-clippy-{component_filter}"
            if component_filter
            else "rust-backend-clippy"
        )
        tasks.append((task_name, "apiserver", backend_cmd))

        # Add custom Python linters (run on host as apiserver container lacks Python)
        import sys

        from _bootstrap import SCRIPTS_DIR

        clean_code_cmd = [sys.executable, str(SCRIPTS_DIR / "lint_clean_code.py")]
        tasks.append(("rust-backend-clean-code-linter", "apiserver", clean_code_cmd))

        tenant_isolation_cmd = [
            sys.executable,
            str(SCRIPTS_DIR / "lint_tenant_isolation.py"),
        ]
        tasks.append(
            ("rust-backend-tenant-isolation-linter", "apiserver", tenant_isolation_cmd)
        )

    if mode in ("all", "web"):
        lint_cmd = build_compose_exec(
            "webdashboard",
            "bun run lint:fix" if fix else "bun run lint",
            mode=mode,
        )
        tasks.append(("webdashboard-lint", "webdashboard", lint_cmd))

        typecheck_cmd = build_compose_exec(
            "webdashboard", "bun run type-check", mode=mode
        )
        tasks.append(("webdashboard-typecheck", "webdashboard", typecheck_cmd))

        i18n_cmd = build_compose_exec("webdashboard", "bun run check-i18n", mode=mode)
        tasks.append(("webdashboard-i18n", "webdashboard", i18n_cmd))

    if mode == "all":
        sync_cmd = build_compose_exec(
            "apiserver",
            "cargo run -- gen-openapi > /workspace/shared/openapi.json",
            mode=mode,
        )
        sync_cmd2 = build_compose_exec(
            "webdashboard",
            "npx openapi-typescript /workspace/shared/openapi.json -o ./src/types/api-generated.ts",
            mode=mode,
        )
        import shlex

        cmd1_str = " ".join(shlex.quote(arg) for arg in sync_cmd)
        cmd2_str = " ".join(shlex.quote(arg) for arg in sync_cmd2)
        combined = ["bash", "-c", f"{cmd1_str} && {cmd2_str}"]
        tasks.append(("api-sync", "webdashboard", combined))

    return run_parallel_ordered("Linting", component_filter, tasks)


def leedevkit_run_unit(
    component_filter: str = "",
    mode: str = "all",
    test_pattern: str = "",
    shard_n: str = "",
    shard_m: str = "",
) -> bool:
    """Run unit tests (cargo nextest, bun test)."""
    tasks: list[tuple[str, str, list[str]]] = []

    shard_flag = f"--shard={shard_n}/{shard_m}" if shard_n and shard_m else ""

    if mode in ("all", "api", "integration"):
        pkg_name = "agent" if component_filter == "agent-main" else component_filter
        pkg_flag = (
            f"--package {pkg_name}"
            if component_filter in ["apiserver", "agent-main"]
            else "--workspace"
        )
        db_url = "postgres://test_user:test_password@localhost:5432/test_database?sslmode=disable"
        backend = f"DATABASE_URL={db_url} cargo nextest run {pkg_flag} --lib {_safe_pattern(test_pattern)} {shard_flag} --no-tests=pass"
        backend_cmd = build_compose_exec(
            "apiserver", backend, workdir="/workspace", mode=mode
        )
        task_name = (
            f"rust-backend-{component_filter}" if component_filter else "rust-backend"
        )
        tasks.append((task_name, "apiserver", backend_cmd))

    if mode in ("all", "web"):
        web = f"bun run test -- {_safe_pattern(test_pattern)} {shard_flag} --passWithNoTests"
        web_cmd = build_compose_exec("webdashboard", web, mode=mode)
        tasks.append(("webdashboard", "webdashboard", web_cmd))

    return run_parallel_ordered("Unit Tests", component_filter, tasks)


def leedevkit_run_integration(
    component_filter: str = "", mode: str = "all", test_pattern: str = ""
) -> bool:
    """Run integration and E2E tests (cargo nextest --test, playwright)."""
    tasks: list[tuple[str, str, list[str]]] = []

    no_tests_flag = "pass"
    workdir = "/workspace"

    if mode in ("all", "api", "integration") and "web" not in component_filter:
        # API server startup is handled by the lifecycle/container health check
        # Integration tests use the already-running apiserver container
        db_url = (
            "postgres://test_user:test_password@db_system:5432/leedevkit_test_template"
        )
        pkg_name = "agent" if component_filter == "agent-main" else component_filter
        pkg_flag = (
            f"--package {pkg_name}"
            if component_filter in ["apiserver", "agent-main"]
            else "--workspace"
        )
        import os

        jobs = os.environ.get("CARGO_BUILD_JOBS", "2")
        backend = (
            f"DATABASE_URL={db_url} "
            f"CARGO_BUILD_JOBS={jobs} cargo nextest run {pkg_flag} --test '*' {_safe_pattern(test_pattern)} "
            f"--no-tests={no_tests_flag}"
        )
        backend_cmd = build_compose_exec(
            "apiserver", backend, workdir=workdir, mode=mode
        )
        task_name = (
            f"rust-backend-int-{component_filter}"
            if component_filter
            else "rust-backend-int"
        )
        tasks.append((task_name, "apiserver", backend_cmd))

    if mode in ("all", "web"):
        pw_args = f"-g {_safe_pattern(test_pattern)}" if test_pattern else ""
        pw = f"bunx playwright test {pw_args} --workers 2"
        pw_cmd = build_compose_exec("webdashboard", pw, mode=mode)
        tasks.append(("playwright-e2e", "webdashboard", pw_cmd))

    return run_parallel_ordered("Integration & E2E", component_filter, tasks)


def leedevkit_run_coverage(
    component_filter: str = "", mode: str = "all", unit_only: bool = False
) -> bool:
    """Run test coverage (cargo llvm-cov, vitest coverage)."""
    tasks: list[tuple[str, str, list[str]]] = []

    coverage_env = "RUSTC_WRAPPER='' DATABASE_URL='postgres://test_user:test_password@db_system:5432/leedevkit_test_template'"

    if mode in ("all", "api", "integration"):
        pkg_name = "agent" if component_filter == "agent-main" else component_filter
        pkg_flag = (
            f"--package {pkg_name}"
            if component_filter in ["apiserver", "agent-main"]
            else "--workspace"
        )
        test_flags = "--lib" if unit_only else "--lib --test integration"
        ignore_regex = "--ignore-filename-regex 'app_builder\\.rs|handlers/debug\\.rs|worker/runner\\.rs|worker/main\\.rs|main\\.rs|report_service_test\\.rs|handlers/custom_fields\\.rs|handlers/document_.*\\.rs|handlers/files\\.rs|handlers/signatures\\.rs|handlers/storage_config\\.rs|models/custom_field\\.rs|models/diesel/file\\.rs|models/diesel/signature\\.rs|models/signature\\.rs|models/file\\.rs|services/storage/.*|diesel_file_repository\\.rs|storage_config_repo\\.rs|diesel_custom_field_repository\\.rs|diesel_signature_repository\\.rs|custom_field_service\\.rs|signature_service\\.rs|file_scan_scheduler\\.rs|employee/mod\\.rs|crypto\\.rs|lifecycle_checklist.*|employment_phases\\.rs|tenant_manager\\.rs|job_queue\\.rs'"
        apiserver_cov = (
            f"mkdir -p /workspace/.test_logs && "
            f"{coverage_env} cargo llvm-cov nextest {test_flags} {pkg_flag} {ignore_regex} "
            f"--lcov --output-path /workspace/.test_logs/coverage-apiserver.lcov "
            f"--no-tests=pass 2>&1 </dev/null && "
            f"{coverage_env} cargo llvm-cov report {ignore_regex} --html "
            f"--output-dir /workspace/.test_logs/coverage-apiserver-html 2>&1 </dev/null && "
            f"{coverage_env} cargo llvm-cov report {ignore_regex} --summary-only 2>&1 </dev/null && "
            f"{coverage_env} cargo llvm-cov report {ignore_regex} --json > /workspace/.test_logs/coverage.json 2>/dev/null </dev/null && "
            f'python3 -c "import json\n'
            f"cov=json.load(open('/workspace/.test_logs/coverage.json'))\n"
            f"print('\\n\\n' + '='*60 + '\\n🤖 AI COVERAGE SUMMARY (Target: 50.0%)\\n' + '='*60)\n"
            f"files=[f for f in cov['data'][0]['files'] if f['summary']['lines']['count'] > 0 and (f['summary']['lines']['covered'] / f['summary']['lines']['count']) < 0.5]\n"
            f"files.sort(key=lambda x: x['summary']['lines']['covered'] / x['summary']['lines']['count'])\n"
            f"for f in files: print('❌ %s (%.1f%%)' % (f['filename'], (f['summary']['lines']['covered']/f['summary']['lines']['count'])*100))\n"
            f"if not files: print('✅ Tất cả các file đều đã đạt >= 50% coverage!')\n"
            f"print('='*60 + '\\n')\""
        )
        task_name = (
            f"rust-backend-coverage-{component_filter}"
            if component_filter
            else "rust-backend-coverage"
        )
        tasks.append(
            (
                task_name,
                "apiserver",
                build_compose_exec("apiserver", apiserver_cov, mode=mode),
            )
        )

    if mode in ("all", "web"):
        tasks.append(
            (
                "webdashboard-coverage",
                "webdashboard",
                build_compose_exec("webdashboard", "bun run test:coverage", mode=mode),
            )
        )

    return run_parallel_ordered("Coverage", component_filter, tasks)
