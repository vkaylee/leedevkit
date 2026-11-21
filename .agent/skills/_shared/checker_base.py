"""Shared BaseChecker framework — composable audit checks without boilerplate.

Replaces the procedural regex-check-and-append pattern repeated across
all 21 checker classes in frontend-design and mobile-design skills.

Usage (declarative style):
    class MyChecker(BaseChecker):
        def checks(self) -> list[Check]:
            return [
                Check("scrollview_map", Severity.ISSUE, "Performance CRITICAL",
                      r"ScrollView.*\.map\(|ScrollView.*\{.*\.map",
                      "ScrollView with .map() detected. Use FlatList instead."),
                Check("missing_memo", Severity.WARNING, "Performance",
                      r"FlatList|FlashList",
                      "FlatList without React.memo on list items."),
            ]

Usage (programmatic style):
    class MyChecker(BaseChecker):
        def run(self, ctx: AuditContext) -> list[dict[str, str]]:
            findings: list[dict[str, str]] = []
            self.check_findings(ctx, [
                Check("id", Severity.ISSUE, "Tag", r"pattern", "message"),
            ], findings)
            return findings
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Protocol


# ── Type-safe severity ──


class Severity(str, Enum):
    """Severity levels matching the existing dict convention."""
    ISSUE = "issue"
    WARNING = "warning"
    PASS = "pass"


# ── Structured finding (replaces raw dict) ──


@dataclass(slots=True)
class CheckResult:
    """A single finding produced by a checker.

    Drop-in replacement for the `{"severity": "...", "message": "..."}` dict
    pattern used across all existing checkers.  Backward-compatible via
    `.as_dict()`.
    """
    severity: Severity
    message: str

    def as_dict(self) -> dict[str, str]:
        return {"severity": str(self.severity), "message": self.message}

    def is_issue(self) -> bool:
        return self.severity == Severity.ISSUE

    def is_warning(self) -> bool:
        return self.severity == Severity.WARNING

    def is_pass(self) -> bool:
        return self.severity == Severity.PASS


# ── Declarative check definition ──


@dataclass(slots=True)
class Check:
    """Single audit rule — regex pattern → finding when matched.

    Attributes:
        check_id: Short machine-readable identifier (e.g. "scrollview_map").
        severity: Result severity if the pattern is found.
        category: Human-readable label prepended to the message (e.g. "Performance CRITICAL").
        pattern: Regex pattern to search for in file content.
        message: Human-readable explanation of the finding.
        negate: If True, emit finding when pattern is NOT found.
        condition: Optional callable — check only runs when this returns True.
        precompute: Optional callable that returns a pre-check boolean.
    """
    check_id: str
    severity: Severity
    category: str
    pattern: str
    message: str
    negate: bool = False
    condition: Any | None = None    # callable(ctx) -> bool
    precompute: Any | None = None   # callable(ctx) -> bool, pre-checks before regex


# ── Core helper ──


def check_pattern(
    ctx: Any,   # AuditContext | MobileAuditContext
    check: Check,
) -> CheckResult | None:
    """Run one declarative check against a file context.

    Returns a CheckResult when the pattern matches (or fails to match when
    negate=True), otherwise None.
    """
    # Precondition guard
    if check.condition is not None and not check.condition(ctx):
        return None

    matched = bool(re.search(check.pattern, ctx.content))
    if check.precompute is not None:
        matched = check.precompute(ctx)

    if check.negate:
        matched = not matched

    if matched:
        return CheckResult(
            severity=check.severity,
            message=f"[{check.category}] {ctx.filename}: {check.message}",
        )
    return None


def run_checks(
    ctx: Any,
    checks: list[Check],
    findings: list[dict[str, str]] | None = None,
) -> list[dict[str, str]]:
    """Run a list of declarative Checks and return findings as dicts.

    This is the main entry point for declarative checkers. Pass your list of
    Check definitions and get back a list of finding dicts ready to return
    from `run()`.
    """
    if findings is None:
        findings = []
    for check in checks:
        result = check_pattern(ctx, check)
        if result is not None:
            findings.append(result.as_dict())
    return findings


# ── Base class (optional, backward-compatible) ──


class BaseChecker:
    """Optional base class for individual audit checkers.

    Provides a default `run()` that delegates to `checks()` for declarative
    checkers, plus helper methods for programmatic checkers.

    Declarative mode — override `checks()`:
        class MyChecker(BaseChecker):
            def checks(self) -> list[Check]:
                return [Check(...), Check(...)]

    Programmatic mode — override `run()`:
        class MyChecker(BaseChecker):
            def run(self, ctx: AuditContext) -> list[dict[str, str]]:
                findings: list[dict[str, str]] = []
                # ... use self.match(), self.require(), self.count() helpers ...
                return findings
    """

    # ── Subclass overrides ──

    def checks(self) -> list[Check]:
        """Override to define declarative checks. Default: empty."""
        return []

    def run(self, ctx: Any) -> list[dict[str, str]]:
        """Run all checks. Override for programmatic style, or use default."""
        return run_checks(ctx, self.checks())

    # ── Programmatic helpers (for manual `run()` overrides) ──

    @staticmethod
    def match(
        findings: list[dict[str, str]],
        ctx: Any,
        severity: Severity,
        category: str,
        pattern: str,
        message: str,
        *,
        negate: bool = False,
        condition: bool = True,
    ) -> bool:
        """Run a regex check and append a finding dict if matched.

        Returns True if a finding was appended, False otherwise.
        """
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
        return matched

    @staticmethod
    def require(
        findings: list[dict[str, str]],
        ctx: Any,
        category: str,
        pattern: str,
        message: str,
        *,
        condition: bool = True,
    ) -> bool:
        """Emit an ISSUE when a required pattern is MISSING.

        Returns True if a finding was appended (pattern missing), False if OK.
        """
        return BaseChecker.match(
            findings, ctx, Severity.ISSUE, category, pattern, message,
            negate=True, condition=condition,
        )

    @staticmethod
    def count_above(
        findings: list[dict[str, str]],
        ctx: Any,
        severity: Severity,
        category: str,
        pattern: str,
        threshold: int,
        message: str,
        *,
        condition: bool = True,
    ) -> int:
        """Count pattern occurrences; emit finding if count > threshold.

        Returns the count of pattern matches.
        """
        if not condition:
            return 0
        count = len(re.findall(pattern, ctx.content))
        if count > threshold:
            findings.append({
                "severity": str(severity),
                "message": f"[{category}] {ctx.filename}: {count} {message}",
            })
        return count
