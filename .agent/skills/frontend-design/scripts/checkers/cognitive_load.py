"""Cognitive Load checks: Progressive disclosure, Visual noise, Familiar patterns."""

import re

from ._base import AuditContext


class CognitiveLoadChecker:
    """Checks for cognitive load management patterns."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Progressive disclosure
        if ctx.complex_elements > 5:
            has_progressive = re.search(
                r'step|wizard|stage|accordion|collapsible|tab|more\.\.\.|advanced|show more',
                ctx.content,
                re.IGNORECASE,
            )
            if not has_progressive:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Cognitive Load] {ctx.filename}: Many form elements without "
                        "progressive disclosure. Consider accordion, tabs, or 'Advanced' toggle."
                    ),
                })

        # Visual noise check
        has_many_colors = (
            len(re.findall(r'#[0-9a-fA-F]{3,6}|rgb|hsl', ctx.content)) > 15
        )
        has_many_borders = len(re.findall(r'border:|border-', ctx.content)) > 10
        if has_many_colors and has_many_borders:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Cognitive Load] {ctx.filename}: High visual noise detected. "
                    "Many colors and borders increase cognitive load."
                ),
            })

        # Familiar patterns
        if ctx.has_form:
            has_standard_labels = bool(
                re.search(r'<label|placeholder|aria-label', ctx.content, re.IGNORECASE)
            )
            if not has_standard_labels:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Cognitive Load] {ctx.filename}: Form inputs without labels. "
                        "Use <label> for accessibility and clarity."
                    ),
                })

        return findings
