"""Visual Effects checks: Glassmorphism, Neomorphism, Shadow hierarchy, Gradients,
Borders, Glow, Overlays, Performance/will-change, Effect selection."""

import re

from ._base import AuditContext

_LAYOUT_PROPS = {'width', 'height', 'top', 'left', 'right', 'bottom', 'margin', 'padding'}


class VisualEffectsChecker:
    """Comprehensive visual effects audit."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        self._check_glassmorphism(ctx, findings)
        self._check_gpu_performance(ctx, findings)
        self._check_shadows(ctx, findings)
        self._check_neomorphism(ctx, findings)
        self._check_shadow_hierarchy(ctx, findings)
        self._check_gradients(ctx, findings)
        self._check_borders(ctx, findings)
        self._check_glow(ctx, findings)
        self._check_overlays(ctx, findings)
        self._check_will_change(ctx, findings)
        self._check_effect_selection(ctx, findings)

        return findings

    # --- Glassmorphism ---
    @staticmethod
    def _check_glassmorphism(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if 'backdrop-filter' in ctx.content or 'blur(' in ctx.content:
            if not re.search(
                r'background:\s*rgba|bg-opacity|bg-[a-z0-9]+\/\d+', ctx.content,
            ):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Blur used without semi-transparent "
                        "background (Glassmorphism fail)"
                    ),
                })

    # --- GPU Acceleration / Performance ---
    @staticmethod
    def _check_gpu_performance(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if re.search(r'@keyframes|transition:', ctx.content):
            expensive_props = re.findall(
                r'width|height|top|left|right|bottom|margin|padding', ctx.content,
            )
            if expensive_props:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: Animating expensive properties "
                        f"({', '.join(set(expensive_props))}). Use transform/opacity "
                        "where possible."
                    ),
                })

            if not re.search(r'prefers-reduced-motion', ctx.content):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Accessibility] {ctx.filename}: Animations found without "
                        "prefers-reduced-motion check"
                    ),
                })

    # --- Natural Shadows ---
    @staticmethod
    def _check_shadows(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        shadows = re.findall(r'box-shadow:\s*([^;]+)', ctx.content)
        for shadow in shadows:
            if ',' not in shadow and not re.search(
                r'\d+px\s+[1-9]\d*px', shadow,
            ):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Simple/Unnatural shadow detected. "
                        "Consider multiple layers or Y > X offset for realism."
                    ),
                })

    # --- Neomorphism ---
    @staticmethod
    def _check_neomorphism(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        neo_shadows = re.findall(r'box-shadow:\s*([^;]+)', ctx.content)
        for shadow in neo_shadows:
            if ',' in shadow and '-' in shadow:
                if 'inset' in shadow:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Visual] {ctx.filename}: Neomorphism inset detected. "
                            "Ensure adequate contrast for accessibility."
                        ),
                    })

    # --- Shadow Hierarchy ---
    @staticmethod
    def _check_shadow_hierarchy(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        shadows = re.findall(r'box-shadow:\s*([^;]+)', ctx.content)
        shadow_count = len(shadows)
        if shadow_count > 0:
            opacities = re.findall(r'rgba?\([^)]+,\s*([\d.]+)\)', ctx.content)
            shadow_opacities = [float(o) for o in opacities if float(o) < 0.5]
            if shadow_count >= 3 and len(shadow_opacities) > 0:
                unique_opacities = len(set(shadow_opacities))
                if unique_opacities < 2:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Visual] {ctx.filename}: All shadows at same opacity "
                            "level. Vary shadow intensity for elevation hierarchy."
                        ),
                    })

    # --- Gradient Checks ---
    @staticmethod
    def _check_gradients(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_gradient = bool(re.search(
            r'gradient|linear-gradient|radial-gradient|conic-gradient', ctx.content,
        ))
        if has_gradient:
            gradient_count = len(re.findall(r'gradient', ctx.content, re.IGNORECASE))
            if gradient_count > 5:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Many gradients detected "
                        f"({gradient_count}). Ensure this serves purpose, not decoration."
                    ),
                })
        else:
            if ctx.has_hero and not re.search(r'background:|bg-', ctx.content):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Hero section without visual interest. "
                        "Consider gradient for depth."
                    ),
                })

    # --- Border Effects ---
    @staticmethod
    def _check_borders(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_border = bool(re.search(r'border:|border-', ctx.content))
        if has_border:
            border_count = len(re.findall(r'border:', ctx.content))
            if border_count > 8:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Many border declarations "
                        f"({border_count}). Simplify for cleaner look."
                    ),
                })

    # --- Glow Effects ---
    @staticmethod
    def _check_glow(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        text_shadows = re.findall(r'text-shadow:', ctx.content)
        for ts in text_shadows:
            if ',' in ts:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Text glow effect detected. Ensure "
                        "readability is maintained."
                    ),
                })

        glow_shadows = re.findall(r'box-shadow:\s*[^;]*0\s+0\s+', ctx.content)
        if len(glow_shadows) > 2:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Visual] {ctx.filename}: Multiple glow effects detected. Use "
                    "sparingly for emphasis only."
                ),
            })

    # --- Overlay Techniques ---
    @staticmethod
    def _check_overlays(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_images = bool(re.search(
            r'<img|background-image:|bg-\[url', ctx.content,
        ))
        if has_images and ctx.has_long_text:
            has_overlay = bool(re.search(
                r'overlay|rgba\(0|gradient.*transparent|::after|::before', ctx.content,
            ))
            if not has_overlay:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visual] {ctx.filename}: Text over image without overlay. "
                        "Add gradient overlay for readability."
                    ),
                })

    # --- Performance: will-change ---
    @staticmethod
    def _check_will_change(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if re.search(r'will-change:', ctx.content):
            will_change_props = re.findall(r'will-change:\s*([^;]+)', ctx.content)
            for prop in will_change_props:
                prop = prop.strip().lower()
                if prop in _LAYOUT_PROPS:
                    findings.append({
                        "severity": "issue",
                        "message": (
                            f"[Performance] {ctx.filename}: will-change on '{prop}' "
                            "(layout property). Use only for transform/opacity."
                        ),
                    })

        will_change_count = len(re.findall(r'will-change:', ctx.content))
        if will_change_count > 3:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Performance] {ctx.filename}: Many will-change declarations "
                    f"({will_change_count}). Use sparingly, only for heavy animations."
                ),
            })

    # --- Effect Selection ---
    @staticmethod
    def _check_effect_selection(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        shadows = re.findall(r'box-shadow:\s*([^;]+)', ctx.content)
        shadow_count = len(shadows)
        has_gradient = bool(re.search(
            r'gradient|linear-gradient|radial-gradient|conic-gradient', ctx.content,
        ))

        effect_count = (
            (1 if has_gradient else 0)
            + shadow_count
            + len(re.findall(r'backdrop-filter|blur\(', ctx.content))
            + len(re.findall(r'text-shadow:', ctx.content))
        )
        if effect_count > 10:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Visual] {ctx.filename}: Many visual effects ({effect_count}). "
                    "Ensure effects serve purpose, not decoration."
                ),
            })

        if ctx.has_long_text and effect_count == 0:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Visual] {ctx.filename}: Flat design with no depth. Consider "
                    "shadows or subtle gradients for hierarchy."
                ),
            })
