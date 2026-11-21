"""Touch Psychology checks: target size, spacing, thumb zone, gestures, haptics, feedback."""

import re

from ._base import MobileAuditContext


class TouchPsychologyChecker:
    """Checks touch interaction patterns for mobile UX."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # 1.1 Touch Target Size
        small_sizes = re.findall(r'(?:width|height|size):\s*([0-3]\d)', ctx.content)
        for size in small_sizes:
            if int(size) < 44:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Touch Target] {ctx.filename}: Touch target size {size}px "
                        "< 44px minimum (iOS: 44pt, Android: 48dp)"
                    ),
                })

        # 1.2 Touch Target Spacing
        small_gaps = re.findall(r'(?:margin|gap):\s*([0-7])\s*(?:px|dp)', ctx.content)
        for gap in small_gaps:
            if int(gap) < 8:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Touch Spacing] {ctx.filename}: Touch target spacing {gap}px "
                        "< 8px minimum. Accidental taps risk."
                    ),
                })

        # 1.3 Thumb Zone Placement
        primary_buttons = re.findall(
            r'(?:testID|id):\s*["\'](?:.*(?:primary|cta|submit|confirm)[^"\']*)["\']',
            ctx.content, re.IGNORECASE,
        )
        has_bottom_placement = bool(re.search(
            r'position:\s*["\']?absolute["\']?|bottom:\s*\d+|style.*bottom|justifyContent:\s*["\']?flex-end',
            ctx.content,
        ))
        if primary_buttons and not has_bottom_placement:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Thumb Zone] {ctx.filename}: Primary CTA may not be in thumb "
                    "zone (bottom). Place primary actions at bottom for easy reach."
                ),
            })

        # 1.4 Gesture Alternatives
        has_swipe_gestures = bool(re.search(
            r'Swipeable|onSwipe|PanGestureHandler|swipe', ctx.content,
        ))
        has_visible_buttons = bool(re.search(
            r'Button.*(?:delete|archive|more)|TouchableOpacity|Pressable', ctx.content,
        ))
        if has_swipe_gestures and not has_visible_buttons:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Gestures] {ctx.filename}: Swipe gestures detected without "
                    "visible button alternatives. Motor impaired users need alternatives."
                ),
            })

        # 1.5 Haptic Feedback
        has_important_actions = bool(re.search(
            r'(?:onPress|onSubmit|delete|remove|confirm|purchase)', ctx.content,
        ))
        has_haptics = bool(re.search(
            r'Haptics|Vibration|react-native-haptic-feedback|FeedbackManager',
            ctx.content,
        ))
        if has_important_actions and not has_haptics:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Haptics] {ctx.filename}: Important actions without haptic "
                    "feedback. Consider adding haptic confirmation."
                ),
            })

        # 1.6 Touch Feedback Timing (React Native only)
        if ctx.is_react_native:
            has_pressable = bool(re.search(r'Pressable|TouchableOpacity', ctx.content))
            has_feedback_state = bool(re.search(
                r'pressed|style.*opacity|underlay', ctx.content,
            ))
            if has_pressable and not has_feedback_state:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Touch Feedback] {ctx.filename}: Pressable without visual "
                        "feedback state. Add opacity/scale change for tap confirmation."
                    ),
                })

        return findings
