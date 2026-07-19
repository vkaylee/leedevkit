"""Mobile Performance checks: FlatList, React.memo, useCallback, keyExtractor,
useNativeDriver, memory leaks, console.log, inline functions, animation props."""
import re
from ._base import Checker, MobileAuditContext, Severity, match_finding, count_above


class MobilePerformanceChecker:
    """Performance checks specific to React Native / Flutter."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []
        rn = ctx.is_react_native

        # 2.1 ScrollView + .map() → FlatList (CRITICAL, multi-pattern)
        if (
            re.search(r'<ScrollView|ScrollView\.', ctx.content)
            and re.search(r'ScrollView.*\.map\(|ScrollView.*\{.*\.map', ctx.content)
        ):
            findings.append({
                "severity": "issue",
                "message": (
                    f"[Performance CRITICAL] {ctx.filename}: ScrollView with .map() "
                    "detected. Use FlatList for lists to prevent memory explosion."
                ),
            })

        # 2.2 FlatList without React.memo
        if rn:
            has_list = bool(re.search(r'FlatList|FlashList|SectionList', ctx.content))
            has_memo = bool(re.search(r'React\.memo|memo\(', ctx.content))
            if has_list and not has_memo:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: FlatList without React.memo "
                        "on list items. Items will re-render on every parent update."
                    ),
                })

        # 2.3 FlatList without useCallback
        if rn:
            has_list = bool(re.search(r'FlatList|FlashList', ctx.content))
            has_cb = bool(re.search(r'useCallback', ctx.content))
            if has_list and not has_cb:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: FlatList renderItem without "
                        "useCallback. New function created every render."
                    ),
                })

        # 2.4 keyExtractor (CRITICAL)
        if rn:
            has_flat = bool(re.search(r'FlatList', ctx.content))
            has_key = bool(re.search(r'keyExtractor', ctx.content))
            uses_idx = bool(re.search(r'key=\{.*index.*\}|key:\s*index', ctx.content))
            if has_flat and not has_key:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Performance CRITICAL] {ctx.filename}: FlatList without "
                        "keyExtractor. Index-based keys cause bugs on reorder/delete."
                    ),
                })
            if uses_idx:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Performance CRITICAL] {ctx.filename}: Using index as key. "
                        "This causes bugs when list changes. Use unique ID from data."
                    ),
                })

        # 2.5 useNativeDriver
        if rn:
            has_anim = bool(re.search(r'Animated\.', ctx.content))
            if has_anim:
                has_true = bool(re.search(r'useNativeDriver:\s*true', ctx.content))
                has_false = bool(re.search(r'useNativeDriver:\s*false', ctx.content))
                if has_false:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Performance] {ctx.filename}: Animation with "
                            "useNativeDriver: false. Use true for 60fps."
                        ),
                    })
                if not has_true and not has_false:
                    findings.append({
                        "severity": "warning",
                        "message": (
                            f"[Performance] {ctx.filename}: Animated component without "
                            "useNativeDriver. Add useNativeDriver: true for 60fps."
                        ),
                    })

        # 2.6 Memory Leak (multi-condition)
        if rn:
            has_eff = bool(re.search(r'useEffect', ctx.content))
            has_clean = bool(re.search(
                r'return\s*\(\)\s*=>|return\s+function', ctx.content,
            ))
            has_sub = bool(re.search(
                r'addEventListener|subscribe|\.focus\(\)|\.off\(', ctx.content,
            ))
            if has_eff and has_sub and not has_clean:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Memory Leak] {ctx.filename}: useEffect with subscriptions "
                        "but no cleanup function. Memory leak on unmount."
                    ),
                })

        # 2.7 Console.log count (threshold-based)
        count_above(findings, ctx, Severity.WARNING, "Performance",
                    r'console\.log|console\.warn|console\.error|console\.debug',
                    5, "console.log statements detected. Remove before production (blocks JS thread).")

        # 2.8 Inline functions count (threshold-based)
        count_above(findings, ctx, Severity.WARNING, "Performance",
                    r'(?:onPress|onPressIn|onPressOut|renderItem):\s*\([^)]*\)\s*=>',
                    3, "inline arrow functions in props. Creates new function every render. Use useCallback.",
                    condition=rn)

        # 2.9 Animated.timing with layout props
        match_finding(findings, ctx, Severity.ISSUE, "Performance",
                      r'Animated\.timing.*(?:width|height|margin|padding)',
                      "Animating layout properties (width/height/margin). Use transform/opacity for 60fps.")

        return findings
