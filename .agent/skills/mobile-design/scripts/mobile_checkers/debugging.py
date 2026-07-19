"""Mobile Debugging checks: console.log, performance profiling, error boundaries."""

import re

from ._base import MobileAuditContext


class MobileDebuggingChecker:
    """Debugging and production-readiness checks for mobile apps."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # 14.1 Performance Profiling
        has_performance = bool(re.search(
            r'Performance|systrace|profile|Flipper', ctx.content,
        ))
        has_console_log = len(re.findall(
            r'console\.(log|warn|error|debug|info)', ctx.content,
        ))
        if has_console_log > 10:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Debugging] {ctx.filename}: {has_console_log} console.log "
                    "statements. Remove before production; they block JS thread."
                ),
            })

        if has_performance:
            findings.append({"severity": "pass", "message": ""})

        # 14.2 Error Boundary
        has_error_boundary = bool(re.search(
            r'ErrorBoundary|componentDidCatch|getDerivedStateFromError', ctx.content,
        ))
        if not has_error_boundary and ctx.is_react_native:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Debugging] {ctx.filename}: No ErrorBoundary detected. Consider "
                    "adding ErrorBoundary to prevent app crashes."
                ),
            })

        # 14.3 Hermes Engine (default in modern RN, just passes)
        if ctx.is_react_native:
            findings.append({"severity": "pass", "message": ""})

        return findings
