"""Mobile Navigation checks: tab bar limits, state preservation, back handling, deep links."""

import re

from ._base import MobileAuditContext


class MobileNavigationChecker:
    """Navigation pattern checks for mobile apps."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # 3.1 Tab Bar Max Items
        tab_bar_items = len(re.findall(
            r'Tab\.Screen|createBottomTabNavigator|BottomTab', ctx.content,
        ))
        if tab_bar_items > 5:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Navigation] {ctx.filename}: {tab_bar_items} tab bar items "
                    "(max 5 recommended). More than 5 becomes hard to tap."
                ),
            })

        # 3.2 Tab State Preservation
        has_tab_nav = bool(re.search(
            r'createBottomTabNavigator|Tab\.Navigator', ctx.content,
        ))
        if has_tab_nav:
            has_lazy_false = bool(re.search(r'lazy:\s*false', ctx.content))
            if not has_lazy_false:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Navigation] {ctx.filename}: Tab navigation without "
                        "lazy: false. Tabs may lose state on switch."
                    ),
                })

        # 3.3 Back Handling
        has_back_listener = bool(re.search(
            r'BackHandler|useFocusEffect|navigation\.addListener', ctx.content,
        ))
        has_custom_back = bool(re.search(
            r'onBackPress|handleBackPress', ctx.content,
        ))
        if has_custom_back and not has_back_listener:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Navigation] {ctx.filename}: Custom back handling without "
                    "BackHandler listener. May not work correctly."
                ),
            })

        # 3.4 Deep Link Support
        has_linking = bool(re.search(
            r'Linking\.|Linking\.openURL|deepLink|universalLink', ctx.content,
        ))
        has_config = bool(re.search(
            r'apollo-link|react-native-screens|navigation\.link', ctx.content,
        ))
        if not has_linking and not has_config:
            findings.append({"severity": "pass", "message": ""})
        else:
            if has_linking and not has_config:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Navigation] {ctx.filename}: Deep linking detected but may "
                        "lack proper configuration. Test notification/share flows."
                    ),
                })

        return findings
