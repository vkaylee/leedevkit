"""Typography System checks: Font pairing, Line length, Line height, Letter spacing,
Weight emphasis, Responsive typography, Hierarchy, Modular scale, Readability."""

import re

from ._base import AuditContext

# Generic font families that don't count toward the font-family limit
_SYSTEM_FONTS = {
    'sans-serif', 'serif', 'monospace', 'cursive', 'fantasy', 'system-ui',
    'inherit', 'arial', 'georgia', 'times new roman', 'courier new',
    'verdana', 'helvetica', 'tahoma',
}

_WEIGHT_MAP = {
    'thin': '100', 'extralight': '200', 'light': '300', 'normal': '400',
    'medium': '500', 'semibold': '600', 'bold': '700', 'extrabold': '800',
    'black': '900',
}

_COMMON_RATIOS = {1.067, 1.125, 1.2, 1.25, 1.333, 1.5, 1.618}


class TypographyChecker:
    """Comprehensive typography system audit."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        self._check_font_pairing(ctx, findings)
        self._check_line_length(ctx, findings)
        self._check_line_height(ctx, findings)
        self._check_letter_spacing(ctx, findings)
        self._check_weight_emphasis(ctx, findings)
        self._check_responsive_typography(ctx, findings)
        self._check_hierarchy(ctx, findings)
        self._check_modular_scale(ctx, findings)
        self._check_readability(ctx, findings)

        return findings

    # --- 2.1 Font Pairing ---
    @staticmethod
    def _check_font_pairing(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        font_families: set[str] = set()

        font_faces = re.findall(
            r'@font-face\s*\{[^}]*family:\s*["\']?([^;"\'\s}]+)',
            ctx.content, re.IGNORECASE,
        )
        google_fonts = re.findall(
            r'fonts\.googleapis\.com[^"\']*family=([^"&]+)', ctx.content, re.IGNORECASE,
        )
        font_family_css = re.findall(r'font-family:\s*([^;]+)', ctx.content, re.IGNORECASE)

        for font in font_faces:
            font_families.add(font.strip().lower())
        for font in google_fonts:
            for f in font.replace('+', ' ').split('|'):
                font_families.add(f.split(':')[0].strip().lower())
        for family in font_family_css:
            first_font = family.split(',')[0].strip().strip('"\'')
            if first_font.lower() not in _SYSTEM_FONTS:
                font_families.add(first_font.lower())

        if len(font_families) > 3:
            findings.append({
                "severity": "issue",
                "message": (
                    f"[Typography] {ctx.filename}: {len(font_families)} font families "
                    "detected. Limit to 2-3 for cohesion."
                ),
            })

    # --- 2.2 Line Length ---
    @staticmethod
    def _check_line_length(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if ctx.has_long_text and not re.search(
            r'max-w-(?:prose|[\[\\]?\d+ch[\]\\]?)|max-width:\s*\d+ch', ctx.content
        ):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: No line length constraint (45-75ch). "
                    "Use max-w-prose or max-w-[65ch]."
                ),
            })

    # --- 2.3 Line Height ---
    @staticmethod
    def _check_line_height(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        text_elements = len(re.findall(
            r'<p|<span|<div.*text|<h[1-6]', ctx.content, re.IGNORECASE,
        ))
        if text_elements > 0 and not re.search(r'leading-|line-height:', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: Text elements found without "
                    "line-height. Body: 1.4-1.6, Headings: 1.1-1.3"
                ),
            })

        if re.search(
            r'<h[1-6]|text-(?:xl|2xl|3xl|4xl|5xl|6xl)', ctx.content, re.IGNORECASE,
        ):
            line_heights = re.findall(
                r'(?:leading-|line-height:\s*)([\d.]+)', ctx.content,
            )
            for lh in line_heights:
                if float(lh) > 1.5:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Typography] {ctx.filename}: Heading has line-height "
                            f"{lh} (>1.3). Headings should be tighter (1.1-1.3)."
                        ),
                    })

    # --- 2.4 Letter Spacing ---
    @staticmethod
    def _check_letter_spacing(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if re.search(
            r'uppercase|text-transform:\s*uppercase', ctx.content, re.IGNORECASE,
        ):
            if not re.search(r'tracking-|letter-spacing:', ctx.content):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Uppercase text without tracking. "
                        "ALL CAPS needs +5-10% spacing."
                    ),
                })

        if re.search(
            r'text-(?:4xl|5xl|6xl|7xl|8xl|9xl)|font-size:\s*[3-9]\dpx', ctx.content,
        ):
            if not re.search(r'tracking-tight|letter-spacing:\s*-[0-9]', ctx.content):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Large display text without "
                        "tracking-tight. Big text needs -1% to -4% spacing."
                    ),
                })

    # --- 2.5 Weight and Emphasis ---
    @staticmethod
    def _check_weight_emphasis(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        weights = re.findall(
            r'font-weight:\s*(\d+)|font-(?:thin|extralight|light|normal|medium|semibold|bold|extrabold|black)|fw-(\d+)',
            ctx.content, re.IGNORECASE,
        )
        weight_values: list[int] = []
        for w in weights:
            val = w[0] or w[1]
            if val:
                val = _WEIGHT_MAP.get(val.lower(), val)
                try:
                    weight_values.append(int(val))
                except (ValueError, TypeError):
                    pass

        # Check for adjacent weights (400/500, 500/600, etc.)
        for i in range(len(weight_values) - 1):
            diff = abs(weight_values[i] - weight_values[i + 1])
            if diff == 100:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Adjacent font weights "
                        f"({weight_values[i]}/{weight_values[i+1]}). Skip at least "
                        "2 levels for contrast."
                    ),
                })

        unique_weights = set(weight_values)
        if len(unique_weights) > 4:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: {len(unique_weights)} font weights. "
                    "Limit to 3-4 per page."
                ),
            })

    # --- 2.6 Responsive Typography ---
    @staticmethod
    def _check_responsive_typography(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_font_sizes = bool(re.search(
            r'font-size:|text-(?:xs|sm|base|lg|xl|2xl)', ctx.content,
        ))
        if has_font_sizes and not re.search(r'clamp\(|responsive:', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Typography] {ctx.filename}: Fixed font sizes without clamp(). "
                    "Consider fluid typography: clamp(MIN, PREFERRED, MAX)"
                ),
            })

    # --- 2.7 Hierarchy ---
    @staticmethod
    def _check_hierarchy(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        headings = re.findall(r'<(h[1-6])', ctx.content, re.IGNORECASE)
        if headings:
            for i in range(len(headings) - 1):
                curr = int(headings[i][1])
                next_h = int(headings[i + 1][1])
                if next_h > curr + 1:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Typography] {ctx.filename}: Skipped heading level "
                            f"(h{curr} -> h{next_h}). Maintain sequential hierarchy."
                        ),
                    })

            if 'h1' not in [h.lower() for h in headings] and ctx.has_long_text:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: No h1 found. Each page should "
                        "have one primary heading."
                    ),
                })

    # --- 2.8 Modular Scale ---
    @staticmethod
    def _check_modular_scale(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        font_sizes = re.findall(
            r'font-size:\s*(\d+(?:\.\d+)?)(px|rem|em)', ctx.content,
        )
        size_values: list[float] = []
        for size, unit in font_sizes:
            if unit in ('rem', 'em'):
                size_values.append(float(size))
            elif unit == 'px':
                size_values.append(float(size) / 16)

        if len(size_values) > 2:
            sorted_sizes = sorted(set(size_values))
            ratios = []
            for i in range(1, len(sorted_sizes)):
                if sorted_sizes[i - 1] > 0:
                    ratios.append(sorted_sizes[i] / sorted_sizes[i - 1])

            for ratio in ratios[:3]:
                if not any(abs(ratio - cr) < 0.05 for cr in _COMMON_RATIOS):
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Typography] {ctx.filename}: Font sizes may not follow "
                            f"modular scale (ratio: {ratio:.2f}). Consider consistent "
                            "ratio like 1.25 (Major Third)."
                        ),
                    })
                    break

    # --- 2.9 Readability ---
    @staticmethod
    def _check_readability(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        paragraphs = re.findall(
            r'<p[^>]*>([^<]+)</p>', ctx.content, re.IGNORECASE,
        )
        for p in paragraphs:
            word_count = len(p.split())
            if word_count > 100:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Long paragraph detected "
                        f"({word_count} words). Break into 3-4 line chunks for readability."
                    ),
                })

        if len(paragraphs) > 5:
            subheadings = len(re.findall(r'<h[2-6]', ctx.content, re.IGNORECASE))
            if subheadings == 0:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Typography] {ctx.filename}: Long content without subheadings. "
                        "Add h2/h3 to break up text."
                    ),
                })
