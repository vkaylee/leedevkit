"""Mobile Color System checks: pure black avoidance, dark mode support,
OLED optimization, saturated colors, outdoor visibility, dark mode text."""

import re

from ._base import MobileAuditContext


class MobileColorChecker:
    """Color system checks for mobile platforms (sections 5 + 10)."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # -- Section 5: Base color --
        self._check_pure_black(ctx, findings)
        self._check_dark_mode(ctx, findings)

        # -- Section 10: Extended color --
        self._check_oled_optimization(ctx, findings)
        self._check_saturated_colors(ctx, findings)
        self._check_outdoor_visibility(ctx, findings)
        self._check_dark_mode_text(ctx, findings)

        return findings

    # --- 5.1 Pure Black Avoidance ---
    @staticmethod
    def _check_pure_black(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if re.search(
            r'#000000|color:\s*black|backgroundColor:\s*["\']?black', ctx.content,
        ):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: Pure black (#000000) detected. Use dark "
                    "gray (#1C1C1E iOS, #121212 Android) for better OLED/battery."
                ),
            })

    # --- 5.2 Dark Mode Support ---
    @staticmethod
    def _check_dark_mode(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_color_schemes = bool(re.search(
            r'useColorScheme|colorScheme|appearance:\s*["\']?dark', ctx.content,
        ))
        has_dark_mode_style = bool(re.search(
            r'\\\?.*dark|style:\s*.*dark|isDark', ctx.content,
        ))
        if not has_color_schemes and not has_dark_mode_style:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Color] {ctx.filename}: No dark mode support detected. "
                    "Consider useColorScheme for system dark mode."
                ),
            })

    # --- 10.1 OLED Optimization ---
    @staticmethod
    def _check_oled_optimization(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if re.search(r'#121212|#1A1A1A|#0D0D0D', ctx.content):
            findings.append({"severity": "pass", "message": ""})
        elif re.search(
            r'backgroundColor:\s*["\']?#000000', ctx.content,
        ):
            pass  # Pure black background is OK for OLED
        elif re.search(r'backgroundColor:\s*["\']?#[0-9A-Fa-f]{6}', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Mobile Color] {ctx.filename}: Consider OLED-optimized dark "
                    "backgrounds (#121212 Android, #000000 iOS) for battery savings."
                ),
            })

    # --- 10.2 Saturated Color Detection ---
    @staticmethod
    def _check_saturated_colors(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        hex_colors = re.findall(
            r'#([0-9A-Fa-f]{2})([0-9A-Fa-f]{2})([0-9A-Fa-f]{2})', ctx.content,
        )
        saturated_count = 0
        for r, g, b in hex_colors:
            try:
                r_val, g_val, b_val = int(r, 16), int(g, 16), int(b, 16)
                max_val = max(r_val, g_val, b_val)
                min_val = min(r_val, g_val, b_val)
                if max_val > 0:
                    saturation = (max_val - min_val) / max_val
                    if saturation > 0.8:
                        saturated_count += 1
            except (ValueError, ZeroDivisionError):
                pass

        if saturated_count > 10:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Mobile Color] {ctx.filename}: {saturated_count} highly "
                    "saturated colors detected. Desaturated colors save battery "
                    "on OLED screens."
                ),
            })

    # --- 10.3 Outdoor Visibility ---
    @staticmethod
    def _check_outdoor_visibility(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        potential_low_contrast = bool(re.search(
            r'#[EeEeEeEe].*#ffffff|#999999.*#ffffff|#333333.*#000000|#666666.*#000000',
            ctx.content,
        ))
        if potential_low_contrast:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Mobile Color] {ctx.filename}: Possible low contrast "
                    "combination detected. Critical for outdoor visibility. "
                    "Ensure WCAG AAA (7:1) for mobile."
                ),
            })

    # --- 10.4 Dark Mode Text Color ---
    @staticmethod
    def _check_dark_mode_text(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_dark_mode = bool(re.search(
            r'dark:\s*|isDark|useColorScheme|colorScheme:\s*["\']?dark', ctx.content,
        ))
        if has_dark_mode:
            has_pure_white_text = bool(re.search(
                r'color:\s*["\']?#ffffff|#fff["\']?\}|textColor:\s*["\']?white',
                ctx.content,
            ))
            if has_pure_white_text:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Mobile Color] {ctx.filename}: Pure white text (#FFFFFF) in "
                        "dark mode. Use #E8E8E8 or light gray for better readability."
                    ),
                })
