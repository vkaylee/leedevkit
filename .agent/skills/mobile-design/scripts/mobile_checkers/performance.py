"""Mobile Performance checks: FlatList, React.memo, useCallback, keyExtractor,
useNativeDriver, memory leaks, console.log, inline functions, animation props."""

import re

from ._base import MobileAuditContext


class MobilePerformanceChecker:
    """Performance checks specific to React Native / Flutter."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # 2.1 ScrollView vs FlatList (CRITICAL)
        has_scrollview = bool(re.search(r'<ScrollView|ScrollView\.', ctx.content))
        has_map_in_scrollview = bool(re.search(
            r'ScrollView.*\.map\(|ScrollView.*\{.*\.map', ctx.content,
        ))
        if has_scrollview and has_map_in_scrollview:
            findings.append({
                "severity": "issue",
                "message": (
                    f"[Performance CRITICAL] {ctx.filename}: ScrollView with .map() "
                    "detected. Use FlatList for lists to prevent memory explosion."
                ),
            })

        # 2.2 React.memo
        if ctx.is_react_native:
            has_list = bool(re.search(r'FlatList|FlashList|SectionList', ctx.content))
            has_react_memo = bool(re.search(r'React\.memo|memo\(', ctx.content))
            if has_list and not has_react_memo:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: FlatList without React.memo "
                        "on list items. Items will re-render on every parent update."
                    ),
                })

        # 2.3 useCallback
        if ctx.is_react_native:
            has_flatlist = bool(re.search(r'FlatList|FlashList', ctx.content))
            has_use_callback = bool(re.search(r'useCallback', ctx.content))
            if has_flatlist and not has_use_callback:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: FlatList renderItem without "
                        "useCallback. New function created every render."
                    ),
                })

        # 2.4 keyExtractor (CRITICAL)
        if ctx.is_react_native:
            has_flatlist = bool(re.search(r'FlatList', ctx.content))
            has_key_extractor = bool(re.search(r'keyExtractor', ctx.content))
            uses_index_key = bool(re.search(
                r'key=\{.*index.*\}|key:\s*index', ctx.content,
            ))
            if has_flatlist and not has_key_extractor:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Performance CRITICAL] {ctx.filename}: FlatList without "
                        "keyExtractor. Index-based keys cause bugs on reorder/delete."
                    ),
                })
            if uses_index_key:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Performance CRITICAL] {ctx.filename}: Using index as key. "
                        "This causes bugs when list changes. Use unique ID from data."
                    ),
                })

        # 2.5 useNativeDriver
        if ctx.is_react_native:
            has_animated = bool(re.search(r'Animated\.', ctx.content))
            has_native_driver = bool(re.search(
                r'useNativeDriver:\s*true', ctx.content,
            ))
            has_native_driver_false = bool(re.search(
                r'useNativeDriver:\s*false', ctx.content,
            ))
            if has_animated and has_native_driver_false:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: Animation with "
                        "useNativeDriver: false. Use true for 60fps."
                    ),
                })
            if has_animated and not has_native_driver:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: Animated component without "
                        "useNativeDriver. Add useNativeDriver: true for 60fps."
                    ),
                })

        # 2.6 Memory Leak
        if ctx.is_react_native:
            has_effect = bool(re.search(r'useEffect', ctx.content))
            has_cleanup = bool(re.search(
                r'return\s*\(\)\s*=>|return\s+function', ctx.content,
            ))
            has_subscriptions = bool(re.search(
                r'addEventListener|subscribe|\.focus\(\)|\.off\(', ctx.content,
            ))
            if has_effect and has_subscriptions and not has_cleanup:
                findings.append({
                    "severity": "issue",
                    "message": (
                        f"[Memory Leak] {ctx.filename}: useEffect with subscriptions "
                        "but no cleanup function. Memory leak on unmount."
                    ),
                })

        # 2.7 Console.log Detection
        console_logs = len(re.findall(
            r'console\.log|console\.warn|console\.error|console\.debug', ctx.content,
        ))
        if console_logs > 5:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Performance] {ctx.filename}: {console_logs} console.log "
                    "statements detected. Remove before production (blocks JS thread)."
                ),
            })

        # 2.8 Inline Functions
        if ctx.is_react_native:
            inline_functions = re.findall(
                r'(?:onPress|onPressIn|onPressOut|renderItem):\s*\([^)]*\)\s*=>',
                ctx.content,
            )
            if len(inline_functions) > 3:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Performance] {ctx.filename}: {len(inline_functions)} "
                        "inline arrow functions in props. Creates new function every "
                        "render. Use useCallback."
                    ),
                })

        # 2.9 Animation Properties
        animating_layout = bool(re.search(
            r'Animated\.timing.*(?:width|height|margin|padding)', ctx.content,
        ))
        if animating_layout:
            findings.append({
                "severity": "issue",
                "message": (
                    f"[Performance] {ctx.filename}: Animating layout properties "
                    "(width/height/margin). Use transform/opacity for 60fps."
                ),
            })

        return findings
