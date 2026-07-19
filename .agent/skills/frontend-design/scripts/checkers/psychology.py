"""Psychology laws checks: Hick's, Fitts', Miller's, Von Restorff, Serial Position."""

import re

from ._base import AuditContext


class PsychologyChecker:
    """Checks compliance with core UX psychology laws."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Hick's Law
        if ctx.nav_items > 7:
            findings.append({
                "severity": "issue",
                "message": f"[Hick's Law] {ctx.filename}: {ctx.nav_items} nav items (Max 7)",
            })

        # Fitts' Law
        if re.search(r'height:\s*([0-3]\d)px', ctx.content) or re.search(
            r'h-[1-9]\b|h-10\b', ctx.content
        ):
            findings.append({
                "severity": "warning",
                "message": f"[Fitts' Law] {ctx.filename}: Small targets (< 44px)",
            })

        # Miller's Law
        form_fields = len(
            re.findall(r'<input|<select|<textarea', ctx.content, re.IGNORECASE)
        )
        if form_fields > 7 and not re.search(
            r'step|wizard|stage', ctx.content, re.IGNORECASE
        ):
            findings.append({
                "severity": "warning",
                "message": f"[Miller's Law] {ctx.filename}: Complex form ({form_fields} fields)",
            })

        # Von Restorff
        if 'button' in ctx.content.lower() and not re.search(
            r'primary|bg-primary|Button.*primary|variant=["\']primary',
            ctx.content,
            re.IGNORECASE,
        ):
            findings.append({
                "severity": "warning",
                "message": f"[Von Restorff] {ctx.filename}: No primary CTA",
            })

        # Serial Position Effect
        if ctx.nav_items > 3:
            nav_content = re.findall(
                r'<NavLink|<Link|<a\s+href[^>]*>([^<]+)</a>',
                ctx.content,
                re.IGNORECASE,
            )
            if nav_content and len(nav_content) > 2:
                last_item = nav_content[-1].lower() if nav_content else ''
                if not any(
                    x in last_item
                    for x in ['contact', 'login', 'sign', 'get started', 'cta', 'button']
                ):
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Serial Position] {ctx.filename}: Last nav item may not be "
                            "important. Place key actions at start/end."
                        ),
                    })

        return findings
