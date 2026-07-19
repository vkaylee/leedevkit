"""Platform Android checks: Material icons, ripple effect, back button,
Roboto font, Material 3 dynamic color, elevation, components, navigation (sections 7 + 12)."""

import re

from ._base import MobileAuditContext


class PlatformAndroidChecker:
    """Android-specific platform checks (sections 7 + 12)."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        if not ctx.is_react_native:
            return findings

        # -- Section 7: Base Android --
        self._check_material_icons(ctx, findings)
        self._check_ripple(ctx, findings)
        self._check_back_button(ctx, findings)

        # -- Section 12: Extended Android --
        self._check_roboto_font(ctx, findings)
        self._check_material3_color(ctx, findings)
        self._check_elevation(ctx, findings)
        self._check_components(ctx, findings)
        self._check_navigation(ctx, findings)

        return findings

    # --- 7.1 Material Icons ---
    @staticmethod
    def _check_material_icons(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_material_icons = bool(re.search(
            r'@expo/vector-icons|MaterialIcons', ctx.content,
        ))
        if has_material_icons:
            findings.append({"severity": "pass", "message": ""})

    # --- 7.2 Ripple Effect ---
    @staticmethod
    def _check_ripple(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_ripple = bool(re.search(
            r'ripple|android_ripple|foregroundRipple', ctx.content,
        ))
        has_pressable = bool(re.search(r'Pressable|Touchable', ctx.content))
        if has_pressable and not has_ripple:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android] {ctx.filename}: Touchable without ripple effect. "
                    "Android users expect ripple feedback."
                ),
            })

    # --- 7.3 Hardware Back Button ---
    @staticmethod
    def _check_back_button(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_back_button = bool(re.search(
            r'BackHandler|useBackHandler', ctx.content,
        ))
        has_navigation = bool(re.search(r'@react-navigation', ctx.content))
        if has_navigation and not has_back_button:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android] {ctx.filename}: React Navigation detected without "
                    "BackHandler listener. Android hardware back may not work correctly."
                ),
            })

    # --- 12.1 Roboto Font ---
    @staticmethod
    def _check_roboto_font(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_roboto = bool(re.search(
            r'Roboto|fontFamily:\s*["\']?[-\s]*Roboto', ctx.content,
        ))
        has_custom_font = bool(re.search(
            r'fontFamily:\s*["\'][^"\']+', ctx.content,
        ))
        if has_custom_font and not has_roboto:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android] {ctx.filename}: Custom font without Roboto fallback. "
                    "Roboto is optimized for Android displays."
                ),
            })

    # --- 12.2 Material 3 Dynamic Color ---
    @staticmethod
    def _check_material3_color(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_material_colors = bool(re.search(
            r'MD3|MaterialYou|dynamicColor|useColorScheme', ctx.content,
        ))
        has_theme_provider = bool(re.search(
            r'MaterialTheme|ThemeProvider|PaperProvider|ThemeProvider', ctx.content,
        ))
        if not has_material_colors and not has_theme_provider:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android] {ctx.filename}: No Material 3 dynamic color detected. "
                    "Consider Material 3 theming for personalized feel."
                ),
            })

    # --- 12.3 Material Elevation ---
    @staticmethod
    def _check_elevation(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_elevation = bool(re.search(
            r'elevation:\s*\d+|shadowOpacity|shadowRadius|android:elevation',
            ctx.content,
        ))
        has_box_shadow = bool(re.search(r'boxShadow:', ctx.content))
        if has_box_shadow and not has_elevation:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Android] {ctx.filename}: CSS box-shadow detected without "
                    "elevation. Consider Material elevation system for consistent depth."
                ),
            })

    # --- 12.4 Material Components ---
    @staticmethod
    def _check_components(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_ripple = bool(re.search(
            r'ripple|android_ripple|foregroundRipple', ctx.content,
        ))
        has_card = bool(re.search(r'Card|Paper|elevation.*\d+', ctx.content))
        has_fab = bool(re.search(r'FAB|FloatingActionButton|fab', ctx.content))
        has_snackbar = bool(re.search(r'Snackbar|showSnackBar|Toast', ctx.content))

        material_component_count = sum([has_ripple, has_card, has_fab, has_snackbar])
        if material_component_count >= 2:
            findings.append({"severity": "pass", "message": ""})

    # --- 12.5 Android Navigation Patterns ---
    @staticmethod
    def _check_navigation(
        ctx: MobileAuditContext, findings: list[dict[str, str]],
    ) -> None:
        has_bottom_nav = bool(re.search(
            r'BottomNavigation|BottomNav', ctx.content,
        ))
        if has_bottom_nav:
            findings.append({"severity": "pass", "message": ""})
        else:
            has_top_app_bar = bool(re.search(
                r'TopAppBar|AppBar|CollapsingToolbar', ctx.content,
            ))
            has_navigation_rail = bool(re.search(
                r'NavigationRail', ctx.content,
            ))
            if has_top_app_bar and not has_navigation_rail:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[Android] {ctx.filename}: TopAppBar without bottom "
                        "navigation. Consider BottomNavigation for thumb-friendly access."
                    ),
                })
