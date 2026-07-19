"""Persuasive Design checks: Smart defaults, Anchoring, Social proof, Progress."""

import re

from ._base import AuditContext


class PersuasiveChecker:
    """Checks for ethical persuasive design patterns."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Smart defaults
        if ctx.has_form:
            has_defaults = bool(
                re.search(r'checked|selected|default|value=["\'].*["\']', ctx.content)
            )
            radio_inputs = len(
                re.findall(r'type=["\']radio', ctx.content, re.IGNORECASE)
            )
            if radio_inputs > 0 and not has_defaults:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Persuasion] {ctx.filename}: Radio buttons without default "
                        "selection. Pre-select recommended option."
                    ),
                })

        # Anchoring (showing original price)
        if re.search(r'price|pricing|cost|\$\d+', ctx.content, re.IGNORECASE):
            has_anchor = bool(
                re.search(
                    r'original|was|strike|del|save \d+%', ctx.content, re.IGNORECASE
                )
            )
            if not has_anchor:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Persuasion] {ctx.filename}: Prices without anchoring. "
                        "Show original price to frame discount value."
                    ),
                })

        # Social proof live indicators
        has_social = bool(
            re.search(r'join|subscriber|member|user', ctx.content, re.IGNORECASE)
        )
        if has_social:
            has_count = bool(re.findall(r'\d+[+kmb]|\d+,\d+', ctx.content))
            if not has_count:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Persuasion] {ctx.filename}: Social proof without specific "
                        "numbers. Use 'Join 10,000+' format."
                    ),
                })

        # Progress indicators
        if ctx.has_form:
            has_progress = bool(
                re.search(
                    r'progress|step \d+|complete|%|bar', ctx.content, re.IGNORECASE
                )
            )
            if ctx.complex_elements > 5 and not has_progress:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Persuasion] {ctx.filename}: Long form without progress "
                        "indicator. Add progress bar or 'Step X of Y'."
                    ),
                })

        return findings
