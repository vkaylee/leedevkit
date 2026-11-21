#!/usr/bin/env python3
"""
Master Checklist Runner - LeeDevKit
==========================================

Orchestrates all validation scripts in priority order.
Use this for incremental validation during development.

Usage:
    python scripts/checklist.py .                    # Run core checks
    python scripts/checklist.py . --url <URL>        # Include performance checks

Priority Order:
    P0: Security Scan (vulnerabilities, secrets)
    P1: Lint & Type Check (code quality)
    P2: Schema Validation (if database exists)
    P3: Test Runner (unit/integration tests)
    P4: UX Audit (psychology laws, accessibility)
    P5: SEO Check (meta tags, structure)
    P6: Performance (lighthouse - requires URL)
"""

import os
import sys
import subprocess
import argparse
from pathlib import Path
from typing import List, Optional, Iterator


_IGNORED_PROJECT_DIRS = {
    ".git",
    ".venv",
    "venv",
    "node_modules",
    "dist",
    "build",
    "coverage",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
}
_FRONTEND_SUFFIXES = {".html", ".htm", ".jsx", ".tsx", ".vue", ".svelte", ".css"}
_SEO_SUFFIXES = {".html", ".htm", ".jsx", ".tsx"}
_SEO_PAGE_DIRS = {"pages", "app", "routes", "views", "screens"}
_SEO_PAGE_STEMS = {
    "page",
    "index",
    "home",
    "about",
    "contact",
    "blog",
    "post",
    "article",
    "product",
    "landing",
    "layout",
}


def _project_files(project_path: Path) -> Iterator[Path]:
    for root, dirs, files in os.walk(project_path):
        dirs[:] = [
            directory for directory in dirs if directory not in _IGNORED_PROJECT_DIRS
        ]
        for filename in files:
            yield Path(root) / filename


def is_frontend_project(project_path: Path) -> bool:
    return any(
        path.suffix.lower() in _FRONTEND_SUFFIXES
        for path in _project_files(project_path)
    )


def is_seo_project(project_path: Path) -> bool:
    for path in _project_files(project_path):
        if path.suffix.lower() not in _SEO_SUFFIXES:
            continue
        if path.suffix.lower() in {".html", ".htm"}:
            return True
        parts = {part.lower() for part in path.parts}
        if parts & _SEO_PAGE_DIRS or path.stem.lower() in _SEO_PAGE_STEMS:
            return True
    return False


def make_skip_result(name: str, reason: str) -> dict:
    return {
        "name": name,
        "passed": True,
        "skipped": True,
        "status": "skipped",
        "reason": reason,
    }


def check_skip_reason(name: str, project_path: Path) -> Optional[str]:
    if name == "UX Audit" and not is_frontend_project(project_path):
        return "not applicable: no frontend source files"
    if name == "SEO Check" and not is_seo_project(project_path):
        return "not applicable: no public page files"
    return None


# ANSI colors for terminal output
class Colors:
    HEADER = "\033[95m"
    BLUE = "\033[94m"
    CYAN = "\033[96m"
    GREEN = "\033[92m"
    YELLOW = "\033[93m"
    RED = "\033[91m"
    ENDC = "\033[0m"
    BOLD = "\033[1m"


def print_header(text: str):
    print(f"\n{Colors.BOLD}{Colors.CYAN}{'=' * 60}{Colors.ENDC}")
    print(f"{Colors.BOLD}{Colors.CYAN}{text.center(60)}{Colors.ENDC}")
    print(f"{Colors.BOLD}{Colors.CYAN}{'=' * 60}{Colors.ENDC}\n")


def print_step(text: str):
    print(f"{Colors.BOLD}{Colors.BLUE}🔄 {text}{Colors.ENDC}")


def print_success(text: str):
    print(f"{Colors.GREEN}✅ {text}{Colors.ENDC}")


def print_warning(text: str):
    print(f"{Colors.YELLOW}⚠️  {text}{Colors.ENDC}")


def print_error(text: str):
    print(f"{Colors.RED}❌ {text}{Colors.ENDC}")


# Define priority-ordered checks
CORE_CHECKS = [
    (
        "Security Scan",
        ".agent/skills/vulnerability-scanner/scripts/security_scan.py",
        True,
    ),
    ("Lint Check", ".agent/skills/lint-and-validate/scripts/lint_runner.py", True),
    (
        "Schema Validation",
        ".agent/skills/database-design/scripts/schema_validator.py",
        False,
    ),
    ("Test Runner", ".agent/skills/testing-patterns/scripts/test_runner.py", False),
    ("UX Audit", ".agent/skills/frontend-design/scripts/ux_audit.py", False),
    ("SEO Check", ".agent/skills/seo-fundamentals/scripts/seo_checker.py", False),
]

PERFORMANCE_CHECKS = [
    (
        "Lighthouse Audit",
        ".agent/skills/performance-profiling/scripts/lighthouse_audit.py",
        True,
    ),
    (
        "Playwright E2E",
        ".agent/skills/webapp-testing/scripts/playwright_runner.py",
        False,
    ),
]


def check_script_exists(script_path: Path) -> bool:
    """Check if script file exists"""
    return script_path.exists() and script_path.is_file()


def run_script(
    name: str,
    script_path: Path,
    project_path: str,
    url: Optional[str] = None,
    required: bool = False,
) -> dict:
    """Run a check, failing closed when a required script is missing."""
    if not check_script_exists(script_path):
        if required:
            print_error(f"{name}: Required script not found")
            return {
                "name": name,
                "passed": False,
                "output": "",
                "error": "Required script not found",
                "skipped": False,
                "status": "failed",
            }
        print_warning(f"{name}: Optional script not found, skipping")
        return {
            "name": name,
            "passed": True,
            "output": "",
            "error": "",
            "skipped": True,
            "status": "skipped",
            "reason": "optional script not found",
        }
    print_step(f"Running: {name}")
    cmd = ["python3", str(script_path), project_path]
    if url and (
        "lighthouse" in script_path.name.lower()
        or "playwright" in script_path.name.lower()
    ):
        cmd.append(url)
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
        passed = result.returncode == 0
        if passed:
            print_success(f"{name}: PASSED")
        else:
            print_error(f"{name}: FAILED")
            if result.stderr:
                print(f"  Error: {result.stderr[:200]}")
        return {
            "name": name,
            "passed": passed,
            "output": result.stdout,
            "error": result.stderr,
            "skipped": False,
            "status": "passed" if passed else "failed",
        }
    except subprocess.TimeoutExpired:
        print_error(f"{name}: TIMEOUT (>5 minutes)")
        return {
            "name": name,
            "passed": False,
            "output": "",
            "error": "Timeout",
            "skipped": False,
            "status": "failed",
        }
    except Exception as e:
        print_error(f"{name}: ERROR - {str(e)}")
        return {
            "name": name,
            "passed": False,
            "output": "",
            "error": str(e),
            "skipped": False,
            "status": "failed",
        }


def print_summary(results: List[dict]):
    """Print final summary report"""
    print_header("📊 CHECKLIST SUMMARY")

    passed_count = sum(1 for r in results if r["passed"] and not r.get("skipped"))
    failed_count = sum(1 for r in results if not r["passed"] and not r.get("skipped"))
    skipped_count = sum(1 for r in results if r.get("skipped"))

    print(f"Total Checks: {len(results)}")
    print(f"{Colors.GREEN}✅ Passed: {passed_count}{Colors.ENDC}")
    print(f"{Colors.RED}❌ Failed: {failed_count}{Colors.ENDC}")
    print(f"{Colors.YELLOW}⏭️  Skipped: {skipped_count}{Colors.ENDC}")
    print()

    # Detailed results
    for r in results:
        if r.get("skipped"):
            status = f"{Colors.YELLOW}⏭️ {Colors.ENDC}"
        elif r["passed"]:
            status = f"{Colors.GREEN}✅{Colors.ENDC}"
        else:
            status = f"{Colors.RED}❌{Colors.ENDC}"

        reason = f" — {r['reason']}" if r.get("reason") else ""
        print(f"{status} {r['name']}{reason}")

    print()

    executed_count = passed_count + failed_count
    if failed_count > 0 or executed_count == 0:
        if executed_count == 0:
            print_error(
                "No checks executed - deployment readiness cannot be established"
            )
            return False
        print_error(f"{failed_count} check(s) FAILED - Please fix before proceeding")
        return False
    print_success("All checks PASSED ✨")
    return True


def main():
    parser = argparse.ArgumentParser(
        description="Run LeeDevKit validation checklist",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python scripts/checklist.py .                      # Core checks only
  python scripts/checklist.py . --url http://localhost:3000  # Include performance
        """,
    )
    parser.add_argument("project", help="Project path to validate")
    parser.add_argument(
        "--url", help="URL for performance checks (lighthouse, playwright)"
    )
    parser.add_argument(
        "--skip-performance",
        action="store_true",
        help="Skip performance checks even if URL provided",
    )
    args = parser.parse_args()
    project_path = Path(args.project).resolve()
    if not project_path.exists():
        print_error(f"Project path does not exist: {project_path}")
        sys.exit(1)
    print_header("🚀 LEEDEVKIT - MASTER CHECKLIST")
    print(f"Project: {project_path}")
    print(
        f"URL: {args.url if args.url else 'Not provided (performance checks skipped)'}"
    )
    results = []
    print_header("📋 CORE CHECKS")
    for name, script_path, required in CORE_CHECKS:
        reason = check_skip_reason(name, project_path)
        if reason:
            result = make_skip_result(name, reason)
        else:
            script = project_path / script_path
            result = run_script(name, script, str(project_path), required=required)
        results.append(result)
        if required and not result["passed"] and not result.get("skipped"):
            print_error(f"CRITICAL: {name} failed. Stopping checklist.")
            print_summary(results)
            sys.exit(1)

    performance_reason = (
        "performance disabled by --skip-performance"
        if args.skip_performance
        else "URL not provided"
        if not args.url
        else None
    )
    print_header("⚡ PERFORMANCE CHECKS")
    for name, script_path, required in PERFORMANCE_CHECKS:
        if performance_reason:
            result = make_skip_result(name, performance_reason)
        else:
            script = project_path / script_path
            result = run_script(
                name, script, str(project_path), args.url, required=required
            )
        results.append(result)
    all_passed = print_summary(results)
    sys.exit(0 if all_passed else 1)


if __name__ == "__main__":
    main()
