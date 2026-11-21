"""Animation Guide checks: Duration, Easing, Micro-interactions, Loading states,
Page transitions, Scroll animation performance."""

import re

from ._base import AuditContext


class AnimationChecker:
    """Animation/motion audit covering 6 dimensions."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        self._check_duration(ctx, findings)
        self._check_easing(ctx, findings)
        self._check_micro_interactions(ctx, findings)
        self._check_loading_states(ctx, findings)
        self._check_page_transitions(ctx, findings)
        self._check_scroll_animation(ctx, findings)

        return findings

    # --- 5.1 Duration Appropriateness ---
    @staticmethod
    def _check_duration(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        durations = re.findall(
            r'(?:duration|animation-duration|transition-duration):\s*([\d.]+)(s|ms)',
            ctx.content,
        )
        for duration, unit in durations:
            duration_ms = float(duration) * (1000 if unit == 's' else 1)
            if duration_ms < 50:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Animation] {ctx.filename}: Very fast animation "
                        f"({duration}{unit}). Minimum 50ms for visibility."
                    ),
                })
            elif duration_ms > 1000 and 'transition' in ctx.content.lower():
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Animation] {ctx.filename}: Long transition "
                        f"({duration}{unit}). Transitions should be 100-300ms for "
                        "responsiveness."
                    ),
                })

    # --- 5.2 Easing Functions ---
    @staticmethod
    def _check_easing(ctx: AuditContext, findings: list[dict[str, str]]) -> None:
        if re.search(r'ease-in\s+.*entry|fade-in.*ease-in', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Animation] {ctx.filename}: Entry animation with ease-in. "
                    "Entry should use ease-out for snappy feel."
                ),
            })
        if re.search(r'ease-out\s+.*exit|fade-out.*ease-out', ctx.content):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Animation] {ctx.filename}: Exit animation with ease-out. "
                    "Exit should use ease-in for natural feel."
                ),
            })

    # --- 5.3 Micro-interactions ---
    @staticmethod
    def _check_micro_interactions(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        interactive_elements = len(re.findall(
            r'<button|<a\s+href|onClick|@click', ctx.content,
        ))
        has_hover_focus = bool(re.search(
            r'hover:|focus:|:hover|:focus', ctx.content,
        ))
        if interactive_elements > 2 and not has_hover_focus:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Animation] {ctx.filename}: Interactive elements without "
                    "hover/focus states. Add micro-interactions for feedback."
                ),
            })

    # --- 5.4 Loading States ---
    @staticmethod
    def _check_loading_states(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_async = bool(re.search(
            r'async|await|fetch|axios|loading|isLoading', ctx.content,
        ))
        has_loading_indicator = bool(re.search(
            r'skeleton|spinner|progress|loading|<circle.*animate', ctx.content,
        ))
        if has_async and not has_loading_indicator:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Animation] {ctx.filename}: Async operations without loading "
                    "indicator. Add skeleton or spinner for perceived performance."
                ),
            })

    # --- 5.5 Page Transitions ---
    @staticmethod
    def _check_page_transitions(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_routing = bool(re.search(
            r'router|navigate|Link.*to|useHistory', ctx.content,
        ))
        has_page_transition = bool(re.search(
            r'AnimatePresence|motion\.|transition.*page|fade.*route', ctx.content,
        ))
        if has_routing and not has_page_transition:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Animation] {ctx.filename}: Routing detected without page "
                    "transitions. Consider fade/slide for context continuity."
                ),
            })

    # --- 5.6 Scroll Animation Performance ---
    @staticmethod
    def _check_scroll_animation(
        ctx: AuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_scroll_anim = bool(re.search(
            r'onScroll|scroll.*trigger|IntersectionObserver', ctx.content,
        ))
        if has_scroll_anim:
            if re.search(r'onScroll.*[^\w](width|height|top|left)', ctx.content):
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Animation] {ctx.filename}: Scroll handler animating layout "
                        "properties. Use transform/opacity for 60fps."
                    ),
                })
