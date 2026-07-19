"""Accessibility checks: Alt text for images."""

import re

from ._base import AuditContext


class AccessibilityChecker:
    """Basic accessibility audit."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Missing img alt text
        if re.search(r'<img(?![^>]*alt=)[^>]*>', ctx.content):
            findings.append({
                "severity": "issue",
                "message": f"[Accessibility] {ctx.filename}: Missing img alt text",
            })

        return findings
