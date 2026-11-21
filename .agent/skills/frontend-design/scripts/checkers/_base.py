"""Shared base types for composable UX audit checkers."""

from dataclasses import dataclass
from enum import Enum
from typing import Protocol


class Severity(str, Enum):
    """Type-safe severity levels for check findings."""
    ISSUE = "issue"
    WARNING = "warning"
    PASS = "pass"


@dataclass
class AuditContext:
    """Holds pre-computed flags and file metadata for all checks."""

    filename: str
    content: str
    has_long_text: bool = False
    has_form: bool = False
    complex_elements: int = 0
    has_hero: bool = False
    nav_items: int = 0


class Checker(Protocol):
    """Protocol for individual audit checkers.

    Each checker is independent — it reads the AuditContext and returns
    a list of findings dicts.  Checks must NOT mutate shared state.
    """

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        """Run checks and return list of findings.

        Each finding: {"severity": "issue"|"warning"|"pass", "message": str}
        """
        ...


# ── Helper utilities (reduces boilerplate in checker run() methods) ──

def match_finding(
    findings: list[dict[str, str]],
    ctx: AuditContext,
    severity: Severity,
    category: str,
    pattern: str,
    message: str,
    *,
    negate: bool = False,
    condition: bool = True,
) -> bool:
    """Append a finding dict when a regex pattern matches in ctx.content.

    Set negate=True to emit when pattern is NOT found (require semantics).
    Returns True if a finding was appended.
    """
    import re

    if not condition:
        return False
    matched = bool(re.search(pattern, ctx.content))
    if negate:
        matched = not matched
    if matched:
        findings.append({
            "severity": str(severity),
            "message": f"[{category}] {ctx.filename}: {message}",
        })
        return True
    return False


def require_finding(
    findings: list[dict[str, str]],
    ctx: AuditContext,
    category: str,
    pattern: str,
    message: str,
    *,
    condition: bool = True,
) -> bool:
    """Append an ISSUE when a required pattern is MISSING."""
    return match_finding(
        findings, ctx, Severity.ISSUE, category, pattern, message,
        negate=True, condition=condition,
    )


def count_above(
    findings: list[dict[str, str]],
    ctx: AuditContext,
    severity: Severity,
    category: str,
    pattern: str,
    threshold: int,
    message: str,
    *,
    condition: bool = True,
) -> int:
    """Count pattern occurrences; emit finding when count > threshold.

    Returns the total count.
    """
    import re

    if not condition:
        return 0
    count = len(re.findall(pattern, ctx.content))
    if count > threshold:
        findings.append({
            "severity": str(severity),
            "message": f"[{category}] {ctx.filename}: {count} {message}",
        })
    return count
