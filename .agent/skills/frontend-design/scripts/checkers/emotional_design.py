"""Emotional Design checks: Visceral, Behavioral, Reflective (Don Norman)."""

import re

from ._base import AuditContext


class EmotionalDesignChecker:
    """Checks compliance with Don Norman's three levels of emotional design."""

    def run(self, ctx: AuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Visceral: First impressions
        if ctx.has_hero:
            has_gradient = bool(
                re.search(r'gradient|linear-gradient|radial-gradient', ctx.content)
            )
            has_animation = bool(
                re.search(r'@keyframes|transition:|animate-', ctx.content)
            )
            has_visual_interest = has_gradient or has_animation

            if not has_visual_interest and not re.search(
                r'background:|bg-', ctx.content
            ):
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Visceral] {ctx.filename}: Hero section lacks visual appeal. "
                        "Consider gradients or subtle animations."
                    ),
                })

        # Behavioral: Instant feedback and usability
        if 'onClick' in ctx.content or '@click' in ctx.content or 'onclick' in ctx.content:
            has_feedback = re.search(
                r'transition|animate|hover:|focus:|disabled|loading|spinner',
                ctx.content,
                re.IGNORECASE,
            )
            has_state_change = re.search(
                r'setState|useState|disabled|loading', ctx.content
            )

            if not has_feedback and not has_state_change:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Behavioral] {ctx.filename}: Interactive elements lack immediate "
                        "feedback. Add hover/focus/disabled states."
                    ),
                })

        # Reflective: Brand story, values, identity
        has_reflective = bool(
            re.search(
                r'about|story|mission|values|why we|our journey|testimonials',
                ctx.content,
                re.IGNORECASE,
            )
        )
        if ctx.has_long_text and not has_reflective:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Reflective] {ctx.filename}: Long-form content without brand "
                    "story/values. Add 'About' or 'Why We Exist' section."
                ),
            })

        return findings
