"""Color System checks: Purple ban, 60-30-10 rule, Monochromatic detection,
Dark mode, WCAG contrast, Color psychology, HSL palette."""

import re

from ._base import AuditContext

_PURPLE_HEXES = [
    '#8B5CF6', '#A855F7', '#9333EA', '#7C3AED', '#6D28D9',
    '#8B5CF6', '#A78BFA', '#C4B5FD', '#DDD6FE', '#EDE9FE',
    '#8b5cf6', '#a855f7', '#9333ea', '#7c3aed', '#6d28d9',
    'purple', 'violet', 'fuchsia', 'magenta', 'lavender',
]


class ColorSystemChecker:
    """Color system audit covering 7 dimensions."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        self._check_purple_ban(ctx, findings)
        self._check_60_30_10_rule(ctx, findings)
        self._check_monochromatic(ctx, findings)
        self._check_dark_mode(ctx, findings)
        self._check_wcag_contrast(ctx, findings)
        self._check_color_psychology(ctx, findings)
        self._check_hsl_palette(ctx, findings)

        return findings

    # --- 4.1 PURPLE BAN ---
    @staticmethod
    def _check_purple_ban(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        for purple in _PURPLE_HEXES:
            if purple.lower() in ctx.content.lower():
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Color] {ctx.filename}: PURPLE DETECTED ('{purple}'). "
                        "Banned by Maestro rules. Use Teal/Cyan/Emerald instead."
                    ),
                })
                break

    # --- 4.2 60-30-10 Rule ---
    @staticmethod
    def _check_60_30_10_rule(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        color_hex_count = len(re.findall(r'#[0-9a-fA-F]{3,6}', ctx.content))
        hsl_count = len(re.findall(r'hsl\(', ctx.content))
        total_colors = color_hex_count + hsl_count
        if total_colors > 3:
            bg_declarations = re.findall(
                r'(?:background|bg-|bg\[)([^;}\s]+)', ctx.content,
            )
            text_declarations = re.findall(
                r'(?:color|text-)([^;}\s]+)', ctx.content,
            )
            if len(bg_declarations) > 0 and len(text_declarations) > 0:
                unique_hexes = set(re.findall(r'#[0-9a-fA-F]{6}', ctx.content))
                if len(unique_hexes) > 5:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Color] {ctx.filename}: {len(unique_hexes)} distinct "
                            "colors. Consider 60-30-10 rule: dominant (60%), "
                            "secondary (30%), accent (10%)."
                        ),
                    })

    # --- 4.3 Monochromatic Detection ---
    @staticmethod
    def _check_monochromatic(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        hsl_matches = re.findall(
            r'hsl\((\d+),\s*\d+%,\s*\d+%\)', ctx.content,
        )
        if len(hsl_matches) >= 3:
            hues = [int(h) for h in hsl_matches]
            hue_range = max(hues) - min(hues)
            if hue_range < 10:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Color] {ctx.filename}: Monochromatic palette detected "
                        f"(hue variance: {hue_range}deg). Ensure adequate contrast."
                    ),
                })

    # --- 4.4 Dark Mode Compliance ---
    @staticmethod
    def _check_dark_mode(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if re.search(r'color:\s*#000000|#000\b', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Pure black (#000000) detected. Use "
                    "#1a1a1a or darker grays for better dark mode."
                ),
            })
        if re.search(r'background:\s*#ffffff|#fff\b', ctx.content) and re.search(
            r'dark:\s*|dark:', ctx.content,
        ):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Pure white background in dark mode "
                    "context. Use slight off-white (#f9fafb) for reduced eye strain."
                ),
            })

    # --- 4.5 WCAG Contrast ---
    @staticmethod
    def _check_wcag_contrast(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        light_bg_light_text = bool(re.search(
            r'bg-(?:gray|slate|zinc)-50|bg-white.*text-(?:gray|slate)-[12]',
            ctx.content,
        ))
        dark_bg_dark_text = bool(re.search(
            r'bg-(?:gray|slate|zinct)-9|bg-black.*text-(?:gray|slate)-[89]',
            ctx.content,
        ))
        if light_bg_light_text or dark_bg_dark_text:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Possible low-contrast combination "
                    "detected. Verify WCAG AA (4.5:1 for text)."
                ),
            })

    # --- 4.6 Color Psychology ---
    @staticmethod
    def _check_color_psychology(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_blue = bool(re.search(
            r'bg-blue|text-blue|from-blue|#[0-9a-fA-F]*00[0-9A-Fa-f]{2}|#[0-9a-fA-F]*1[0-9A-Fa-f]{2}',
            ctx.content,
        ))
        has_food_context = bool(re.search(
            r'restaurant|food|cooking|recipe|menu|dish|meal',
            ctx.content, re.IGNORECASE,
        ))
        if has_blue and has_food_context:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Blue color in food context. Blue "
                    "suppresses appetite; consider warm colors (red, orange, yellow)."
                ),
            })

    # --- 4.7 HSL Palette ---
    @staticmethod
    def _check_hsl_palette(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_color_vars = bool(re.search(
            r'--color-|color-|primary-|secondary-', ctx.content,
        ))
        if has_color_vars and not re.search(r'hsl\(', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Color variables without HSL. Consider "
                    "HSL for easier palette adjustment (Hue, Saturation, Lightness)."
                ),
            })
