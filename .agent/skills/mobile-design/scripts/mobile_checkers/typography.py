"""Mobile Typography checks: system fonts, text scaling, line height, font size limits,
extended type scales (iOS HIG, Android Material), modular scale, line length, font weights."""

import re

from ._base import MobileAuditContext

_COMMON_RATIOS = {1.125, 1.2, 1.25, 1.333, 1.5}
_IOS_SCALE_SIZES = [34, 28, 22, 20, 17, 16, 15, 13, 12, 11]
_WEIGHT_MAP = {'normal': '400', 'light': '300', 'medium': '500', 'bold': '700'}


class MobileTypographyChecker:
    """Typography checks specific to mobile platforms (sections 4 + 9)."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # -- Section 4: Base typography --
        self._check_system_font(ctx, findings)
        self._check_text_scaling(ctx, findings)
        self._check_line_height(ctx, findings)
        self._check_font_size_limits(ctx, findings)

        # -- Section 9: Extended typography --
        self._check_ios_type_scale(ctx, findings)
        self._check_android_material_scale(ctx, findings)
        self._check_modular_scale(ctx, findings)
        self._check_line_length(ctx, findings)
        self._check_font_weight_pattern(ctx, findings)

        return findings

    # --- 4.1 System Font ---
    @staticmethod
    def _check_system_font(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        has_custom_font = bool(re.search(r"fontFamily:\s*[\"'][^\"']+", ctx.content))
        has_system_font = bool(re.search(
            r"fontFamily:\s*[\"']?(?:System|San Francisco|Roboto|-apple-system)",
            ctx.content,
        ))
        if has_custom_font and not has_system_font:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: Custom font detected. Consider "
                    "system fonts (iOS: SF Pro, Android: Roboto) for native feel."
                ),
            })

    # --- 4.2 Text Scaling ---
    @staticmethod
    def _check_text_scaling(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        has_font_sizes = bool(re.search(r'fontSize:', ctx.content))
        has_scaling = bool(re.search(
            r'allowFontScaling:\s*true|responsiveFontSize|useWindowDimensions',
            ctx.content,
        ))
        if has_font_sizes and not has_scaling:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: Fixed font sizes without scaling "
                    "support. Consider allowFontScaling for accessibility."
                ),
            })

    # --- 4.3 Line Height ---
    @staticmethod
    def _check_line_height(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        line_heights = re.findall(r'lineHeight:\s*([\d.]+)', ctx.content)
        for lh in line_heights:
            if float(lh) > 1.8:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: lineHeight {lh} too high for "
                        "mobile. Mobile text needs tighter spacing (1.3-1.5)."
                    ),
                })

    # --- 4.4 Font Size Limits ---
    @staticmethod
    def _check_font_size_limits(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        font_sizes = re.findall(r'fontSize:\s*([\d.]+)', ctx.content)
        for fs in font_sizes:
            size = float(fs)
            if size < 12:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: fontSize {size}px below 12px "
                        "minimum readability."
                    ),
                })
            elif size > 32:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: fontSize {size}px very large. "
                        "Consider using responsive scaling."
                    ),
                })

    # --- 9.1 iOS Type Scale ---
    @staticmethod
    def _check_ios_type_scale(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        font_sizes = re.findall(r'fontSize:\s*([\d.]+)', ctx.content)
        if len(font_sizes) <= 3:
            return
        matching_ios = sum(
            1 for size in font_sizes
            if any(abs(float(size) - ios_size) < 1 for ios_size in _IOS_SCALE_SIZES)
        )
        if matching_ios < len(font_sizes) / 2:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS Typography] {ctx.filename}: Font sizes don't match iOS "
                    "type scale. Consider iOS text styles for native feel."
                ),
            })

    # --- 9.2 Android Material Type Scale ---
    @staticmethod
    def _check_android_material_scale(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        has_display = bool(re.search(
            r'fontSize:\s*[456][0-9]|display', ctx.content,
        ))
        has_headline_material = bool(re.search(
            r'fontSize:\s*[23][0-9]|headline', ctx.content,
        ))
        uses_sp = bool(re.search(r'\d+\s*sp\b', ctx.content))
        if (has_display or has_headline_material) and not uses_sp:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android Typography] {ctx.filename}: Material typography "
                    "detected without sp units. Use sp for text to respect user "
                    "font size preferences."
                ),
            })

    # --- 9.3 Modular Scale ---
    @staticmethod
    def _check_modular_scale(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        font_sizes = re.findall(r'fontSize:\s*(\d+(?:\.\d+)?)', ctx.content)
        if len(font_sizes) <= 3:
            return
        sorted_sizes = sorted(set(float(s) for s in font_sizes))
        ratios = []
        for i in range(1, len(sorted_sizes)):
            if sorted_sizes[i - 1] > 0:
                ratios.append(sorted_sizes[i] / sorted_sizes[i - 1])

        for ratio in ratios[:3]:
            if not any(abs(ratio - cr) < 0.03 for cr in _COMMON_RATIOS):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Font sizes may not follow "
                        f"modular scale (ratio: {ratio:.2f}). Consider consistent ratio."
                    ),
                })
                break

    # --- 9.4 Line Length (Mobile) ---
    @staticmethod
    def _check_line_length(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        has_long_text = bool(re.search(r'<Text[^>]*>[^<]{40,}', ctx.content))
        has_max_width = bool(re.search(
            r'maxWidth|max-w-\d+|width:\s*["\']?\d+', ctx.content,
        ))
        if has_long_text and not has_max_width:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Mobile Typography] {ctx.filename}: Text without max-width "
                    "constraint. Mobile text should be 40-60 characters per line "
                    "for readability."
                ),
            })

    # --- 9.5 Font Weight Pattern ---
    @staticmethod
    def _check_font_weight_pattern(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        if not ctx.is_react_native:
            return
        font_weights = re.findall(
            r'fontWeight:\s*["\']?(\d+|normal|bold|medium|light)', ctx.content,
        )
        numeric_weights = []
        for w in font_weights:
            val = _WEIGHT_MAP.get(w.lower(), w)
            try:
                numeric_weights.append(int(val))
            except (ValueError, TypeError):
                pass

        bold_count = sum(1 for w in numeric_weights if w >= 700)
        regular_count = sum(1 for w in numeric_weights if 400 <= w < 500)
        if bold_count > regular_count:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Mobile Typography] {ctx.filename}: More bold weights than "
                    "regular. Mobile typography should be regular-dominant for "
                    "readability."
                ),
            })
