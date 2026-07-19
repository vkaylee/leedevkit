"""Trust Building checks: Security signals, Social proof, Authority indicators."""

import re

from ._base import AuditContext


class TrustChecker:
    """Checks for trust-building elements: security, social proof, authority."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Security signals
        if ctx.has_form:
            security_signals = re.findall(
                r'ssl|secure|encrypt|lock|padlock|https', ctx.content, re.IGNORECASE
            )
            if len(security_signals) == 0 and not re.search(
                r'checkout|payment', ctx.content, re.IGNORECASE
            ):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Trust] {ctx.filename}: Form without security indicators. "
                        "Add 'SSL Secure' or lock icon."
                    ),
                })

        # Social proof elements
        social_proof = re.findall(
            r'review|testimonial|rating|star|trust|trusted by|customer|logo',
            ctx.content,
            re.IGNORECASE,
        )
        if len(social_proof) > 0:
            findings.append({"severity": "pass", "message": ""})
        else:
            if ctx.has_long_text:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Trust] {ctx.filename}: No social proof detected. Consider "
                        "adding testimonials, ratings, or 'Trusted by' logos."
                    ),
                })

        # Authority indicators
        has_footer = bool(re.search(r'footer|<footer', ctx.content, re.IGNORECASE))
        if has_footer:
            authority = re.findall(
                r'certif|award|media|press|featured|as seen in',
                ctx.content,
                re.IGNORECASE,
            )
            if len(authority) == 0:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Trust] {ctx.filename}: Footer lacks authority signals. "
                        "Add certifications, awards, or media mentions."
                    ),
                })

        return findings
