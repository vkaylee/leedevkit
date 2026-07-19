"""Platform iOS checks: SF Symbols, haptic types, safe area,
SF Pro font, semantic colors, accent colors, navigation, components (sections 6 + 11)."""

import re

from ._base import MobileAuditContext


class PlatformIOSChecker:
    """iOS-specific platform checks (sections 6 + 11)."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        if not ctx.is_react_native:
            return findings

        # -- Section 6: Base iOS --
        self._check_sf_symbols(ctx, findings)
        self._check_haptic_types(ctx, findings)
        self._check_safe_area(ctx, findings)

        # -- Section 11: Extended iOS --
        self._check_sf_pro_font(ctx, findings)
        self._check_system_colors(ctx, findings)
        self._check_accent_colors(ctx, findings)
        self._check_navigation(ctx, findings)
        self._check_components(ctx, findings)

        return findings

    # --- 6.1 SF Symbols ---
    @staticmethod
    def _check_sf_symbols(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_ios_icons = bool(re.search(r'@expo/vector-icons|ionicons', ctx.content))
        has_sf_symbols = bool(re.search(r'sf-symbol|SF Symbols', ctx.content))
        if has_ios_icons and not has_sf_symbols:
            findings.append({"severity": "pass", "message": ""})

    # --- 6.2 iOS Haptic Types ---
    @staticmethod
    def _check_haptic_types(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_haptic_import = bool(re.search(
            r'expo-haptics|react-native-haptic-feedback', ctx.content,
        ))
        has_haptic_types = bool(re.search(
            r'ImpactFeedback|NotificationFeedback|SelectionFeedback', ctx.content,
        ))
        if has_haptic_import and not has_haptic_types:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS Haptics] {ctx.filename}: Haptic library imported but not "
                    "using typed haptics (Impact/Notification/Selection)."
                ),
            })

    # --- 6.3 Safe Area ---
    @staticmethod
    def _check_safe_area(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_safe_area = bool(re.search(
            r'SafeAreaView|useSafeAreaInsets|safeArea', ctx.content,
        ))
        if not has_safe_area:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS] {ctx.filename}: No SafeArea detected. Content may be "
                    "hidden by notch/home indicator."
                ),
            })

    # --- 11.1 SF Pro Font ---
    @staticmethod
    def _check_sf_pro_font(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_sf_pro = bool(re.search(
            r'SF Pro|SFPro|fontFamily:\s*["\']?[-\s]*SF', ctx.content,
        ))
        has_custom_font = bool(re.search(
            r'fontFamily:\s*["\'][^"\']+', ctx.content,
        ))
        if has_custom_font and not has_sf_pro:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS] {ctx.filename}: Custom font without SF Pro fallback. "
                    "Consider SF Pro Text for body, SF Pro Display for headings."
                ),
            })

    # --- 11.2 iOS System Colors ---
    @staticmethod
    def _check_system_colors(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_label = bool(re.search(
            r'color:\s*["\']?label|\.label', ctx.content,
        ))
        has_secondary_label = bool(re.search(
            r'secondaryLabel|\.secondaryLabel', ctx.content,
        ))
        has_hardcoded_gray = bool(re.search(r'#[78]0{4}', ctx.content))
        if has_hardcoded_gray and not (has_label or has_secondary_label):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS] {ctx.filename}: Hardcoded gray colors detected. Consider "
                    "iOS semantic colors (label, secondaryLabel) for automatic dark mode."
                ),
            })

    # --- 11.3 iOS Accent Colors ---
    @staticmethod
    def _check_accent_colors(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        ios_blue = bool(re.search(r'#007AFF|#0A84FF|systemBlue', ctx.content))
        ios_green = bool(re.search(r'#34C759|#30D158|systemGreen', ctx.content))
        ios_red = bool(re.search(r'#FF3B30|#FF453A|systemRed', ctx.content))
        has_custom_primary = bool(re.search(
            r'primaryColor|theme.*primary|colors\.primary', ctx.content,
        ))
        if has_custom_primary and not (ios_blue or ios_green or ios_red):
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS] {ctx.filename}: Custom primary color without iOS system "
                    "color fallback. Consider systemBlue for consistent iOS feel."
                ),
            })

    # --- 11.4 iOS Navigation ---
    @staticmethod
    def _check_navigation(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_navigation_bar = bool(re.search(
            r'navigationOptions|headerStyle|cardStyle', ctx.content,
        ))
        has_header_title = bool(re.search(
            r'title:\s*["\']|headerTitle|navigation\.setOptions', ctx.content,
        ))
        if has_navigation_bar and not has_header_title:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[iOS] {ctx.filename}: Navigation bar detected without title. "
                    "iOS apps should have clear context in nav bar."
                ),
            })

    # --- 11.5 iOS Components ---
    @staticmethod
    def _check_components(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_alert = bool(re.search(r'Alert\.alert|showAlert', ctx.content))
        has_action_sheet = bool(re.search(
            r'ActionSheet|ActionSheetIOS|showActionSheetWithOptions', ctx.content,
        ))
        has_activity_indicator = bool(re.search(
            r'ActivityIndicator|ActivityIndic', ctx.content,
        ))
        if has_alert or has_action_sheet or has_activity_indicator:
            findings.append({"severity": "pass", "message": ""})
