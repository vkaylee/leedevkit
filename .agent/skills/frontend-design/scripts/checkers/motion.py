"""Motion Graphics checks: Lottie, GSAP, SVG animation, 3D transforms,
Particle effects, Scroll-driven animation, Motion decision tree."""

import re

from ._base import AuditContext


class MotionGraphicsChecker:
    """Motion graphics audit covering 7 dimensions."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        self._check_lottie(ctx, findings)
        self._check_gsap(ctx, findings)
        self._check_svg_animation(ctx, findings)
        self._check_3d_transform(ctx, findings)
        self._check_particles(ctx, findings)
        self._check_scroll_driven(ctx, findings)
        self._check_decision_tree(ctx, findings)

        return findings

    # --- 6.1 Lottie Animations ---
    @staticmethod
    def _check_lottie(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_lottie = bool(re.search(r'lottie|Lottie|@lottie-react', ctx.content))
        if has_lottie:
            has_lottie_fallback = bool(re.search(
                r'prefers-reduced-motion.*lottie|lottie.*isPaused|lottie.*stop',
                ctx.content,
            ))
            if not has_lottie_fallback:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Motion] {ctx.filename}: Lottie animation without "
                        "reduced-motion fallback. Add pause/stop for accessibility."
                    ),
                })

    # --- 6.2 GSAP Memory Leaks ---
    @staticmethod
    def _check_gsap(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_gsap = bool(re.search(r'gsap|ScrollTrigger|from\(.*gsap', ctx.content))
        if has_gsap:
            has_gsap_cleanup = bool(re.search(
                r'kill\(|revert\(|useEffect.*return.*gsap', ctx.content,
            ))
            if not has_gsap_cleanup:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Motion] {ctx.filename}: GSAP animation without cleanup "
                        "(kill/revert). Memory leak risk on unmount."
                    ),
                })

    # --- 6.3 SVG Animation Performance ---
    @staticmethod
    def _check_svg_animation(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        svg_animations = re.findall(
            r'<animate|<animateTransform|stroke-dasharray|stroke-dashoffset',
            ctx.content,
        )
        if len(svg_animations) > 3:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Motion] {ctx.filename}: Multiple SVG animations detected. "
                    "Ensure stroke-dashoffset is used sparingly for mobile performance."
                ),
            })

    # --- 6.4 3D Transform Performance ---
    @staticmethod
    def _check_3d_transform(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_3d_transform = bool(re.search(
            r'transform3d|perspective\(|rotate3d|translate3d', ctx.content,
        ))
        if has_3d_transform:
            has_perspective_parent = bool(re.search(
                r'perspective:\s*\d+px|perspective\s*\(', ctx.content,
            ))
            if not has_perspective_parent:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Motion] {ctx.filename}: 3D transform without perspective "
                        "parent. Add perspective: 1000px for realistic depth."
                    ),
                })

            findings.append({
                "severity": "warning",
                "message": (
                    f"[Motion] {ctx.filename}: 3D transforms detected. Test on mobile; "
                    "can impact performance on low-end devices."
                ),
            })

    # --- 6.5 Particle Effects ---
    @staticmethod
    def _check_particles(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        has_particles = bool(re.search(
            r'particle|canvas.*loop|requestAnimationFrame.*draw|Three\.js', ctx.content,
        ))
        if has_particles:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Motion] {ctx.filename}: Particle effects detected. Ensure "
                    "fallback or reduced-quality option for mobile devices."
                ),
            })

    # --- 6.6 Scroll-Driven Animations ---
    @staticmethod
    def _check_scroll_driven(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_scroll_driven = bool(re.search(
            r'IntersectionObserver.*animate|scroll.*progress|view-timeline', ctx.content,
        ))
        if has_scroll_driven:
            has_throttle = bool(re.search(
                r'throttle|debounce|requestAnimationFrame', ctx.content,
            ))
            if not has_throttle:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Motion] {ctx.filename}: Scroll-driven animation without "
                        "throttling. Add requestAnimationFrame for 60fps."
                    ),
                })

    # --- 6.7 Motion Decision Tree ---
    @staticmethod
    def _check_decision_tree(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_lottie = bool(re.search(r'lottie|Lottie|@lottie-react', ctx.content))
        has_gsap = bool(re.search(r'gsap|ScrollTrigger|from\(.*gsap', ctx.content))

        total_animations = (
            len(re.findall(r'@keyframes|transition:|animate-', ctx.content))
            + (1 if has_lottie else 0)
            + (1 if has_gsap else 0)
        )
        if total_animations > 5:
            functional_animations = len(re.findall(
                r'hover:|focus:|disabled|loading|error|success', ctx.content,
            ))
            if functional_animations < total_animations / 2:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Motion] {ctx.filename}: Many animations ({total_animations})."
                        " Ensure majority serve functional purpose (feedback, guidance),"
                        " not decoration."
                    ),
                })
