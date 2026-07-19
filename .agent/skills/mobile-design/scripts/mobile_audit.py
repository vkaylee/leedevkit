#!/usr/bin/env python3
"""
Mobile UX Audit Script - Full Mobile Design Coverage

Analyzes React Native / Flutter code for compliance with:

1. TOUCH PSYCHOLOGY (touch-psychology.md):
   - Touch Target Sizes (44pt iOS, 48dp Android, 44px WCAG)
   - Touch Target Spacing (8px minimum gap)
   - Thumb Zone Placement (primary CTAs at bottom)
   - Gesture Alternatives (visible buttons for swipe)
   - Haptic Feedback Patterns
   - Touch Feedback Timing (<50ms)

2. MOBILE PERFORMANCE (mobile-performance.md):
   - ScrollView vs FlatList (CRITICAL)
   - React.memo for List Items
   - useCallback for renderItem
   - Stable keyExtractor (NOT index)
   - useNativeDriver for Animations
   - Memory Leak Prevention (cleanup)
   - Console.log Detection
   - Inline Function Detection
   - Animation Performance (transform/opacity only)

3. MOBILE NAVIGATION (mobile-navigation.md):
   - Tab Bar Max Items (5)
   - Tab State Preservation
   - Proper Back Handling
   - Deep Link Support

4. MOBILE TYPOGRAPHY (mobile-typography.md):
   - System Font Usage
   - Dynamic Type Support (iOS)
   - Text Scaling Constraints
   - Mobile Line Height
   - Font Size Limits

5. MOBILE COLOR SYSTEM (mobile-color-system.md):
   - Pure Black Avoidance (#000000)
   - OLED Optimization
   - Dark Mode Support

6. PLATFORM iOS (platform-ios.md):
   - SF Symbols Usage
   - iOS Haptic Types
   - Safe Area Detection

7. PLATFORM ANDROID (platform-android.md):
   - Material Icons Usage
   - Ripple Effects
   - Hardware Back Button

8. MOBILE BACKEND (mobile-backend.md):
   - Secure Storage (NOT AsyncStorage)
   - Offline Handling
   - Push Notification Support

9. MOBILE TESTING:
   - Testing Tools Detection
   - Test Pyramid Balance
   - Accessibility Labels

10. MOBILE DEBUGGING:
    - Console.log Statements
    - Error Boundaries
    - Performance Profiling

Total: 50+ mobile-specific checks
"""

import sys
import os
import re
import json
from pathlib import Path


class MobileAuditor:
    """Audits mobile (React Native / Flutter) code for UX / platform compliance.

    Each check category is implemented in its own checker class under
    the ``checkers/`` package.  Adding a new category is a one-line
    addition to ``_CHECKERS`` plus a new checker class.
    """

    def __init__(self):
        self.issues: list[str] = []
        self.warnings: list[str] = []
        self.passed_count: int = 0
        self.files_checked: int = 0

    # -- Checker registry --------------------------------------------------

    _CHECKERS: list[tuple[str, object]] = []

    @classmethod
    def _init_checkers(cls) -> None:
        if cls._CHECKERS:
            return  # already initialised
        from mobile_checkers.touch_psychology import TouchPsychologyChecker
        from mobile_checkers.performance import MobilePerformanceChecker
        from mobile_checkers.navigation import MobileNavigationChecker
        from mobile_checkers.typography import MobileTypographyChecker
        from mobile_checkers.color import MobileColorChecker
        from mobile_checkers.platform_ios import PlatformIOSChecker
        from mobile_checkers.platform_android import PlatformAndroidChecker
        from mobile_checkers.backend import MobileBackendChecker
        from mobile_checkers.testing import MobileTestingChecker
        from mobile_checkers.debugging import MobileDebuggingChecker

        cls._CHECKERS = [
            ("touch_psychology", TouchPsychologyChecker()),
            ("performance", MobilePerformanceChecker()),
            ("navigation", MobileNavigationChecker()),
            ("typography", MobileTypographyChecker()),
            ("color", MobileColorChecker()),
            ("platform_ios", PlatformIOSChecker()),
            ("platform_android", PlatformAndroidChecker()),
            ("backend", MobileBackendChecker()),
            ("testing", MobileTestingChecker()),
            ("debugging", MobileDebuggingChecker()),
        ]

    # -- Core audit methods ------------------------------------------------

    def audit_file(self, filepath: str) -> None:
        """Run all registered checkers against a single file."""
        content = self._read_file(filepath)
        if content is None:
            return

        # Detect framework — skip non-mobile files early
        is_react_native = bool(re.search(
            r'react-native|@react-navigation|React\.Native', content,
        ))
        is_flutter = bool(re.search(
            r'import \'package:flutter|MaterialApp|Widget\.build', content,
        ))
        if not (is_react_native or is_flutter):
            return

        self.files_checked += 1
        filename = os.path.basename(filepath)

        self._init_checkers()
        ctx = self._build_context(filename, content, is_react_native, is_flutter)

        for _category, checker in self._CHECKERS:
            for finding in checker.run(ctx):
                sev = finding["severity"]
                if sev == "issue":
                    self.issues.append(finding["message"])
                elif sev == "warning":
                    self.warnings.append(finding["message"])
                elif sev == "pass":
                    self.passed_count += 1

    def audit_directory(self, directory: str) -> None:
        extensions = {'.tsx', '.ts', '.jsx', '.js', '.dart'}
        for root, dirs, files in os.walk(directory):
            dirs[:] = [d for d in dirs if d not in {
                'node_modules', '.git', 'dist', 'build', '.next',
                'ios', 'android', '.idea',
            }]
            for file in files:
                if Path(file).suffix in extensions:
                    self.audit_file(os.path.join(root, file))

    def get_report(self) -> dict:
        return {
            "files_checked": self.files_checked,
            "issues": self.issues,
            "warnings": self.warnings,
            "passed_checks": self.passed_count,
            "compliant": len(self.issues) == 0,
        }

    # -- Internal helpers --------------------------------------------------

    @staticmethod
    def _read_file(filepath: str) -> str | None:
        try:
            with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
                return f.read()
        except Exception:
            return None

    @staticmethod
    def _build_context(
        filename: str, content: str, is_react_native: bool, is_flutter: bool,
    ):
        """Build the mobile audit context with pre-computed flags."""
        from mobile_checkers._base import MobileAuditContext

        return MobileAuditContext(
            filename=filename,
            content=content,
            is_react_native=is_react_native,
            is_flutter=is_flutter,
        )


def main():
    if len(sys.argv) < 2:
        print("Usage: python mobile_audit.py <directory>")
        sys.exit(1)

    path = sys.argv[1]
    is_json = "--json" in sys.argv

    auditor = MobileAuditor()
    if os.path.isfile(path):
        auditor.audit_file(path)
    else:
        auditor.audit_directory(path)

    report = auditor.get_report()

    if is_json:
        print(json.dumps(report, indent=2))
    else:
        print(f"\n[MOBILE AUDIT] {report['files_checked']} mobile files checked")
        print("-" * 50)
        if report['issues']:
            print(f"[!] ISSUES ({len(report['issues'])}):")
            for i in report['issues'][:10]:
                print(f"  - {i}")
        if report['warnings']:
            print(f"[*] WARNINGS ({len(report['warnings'])}):")
            for w in report['warnings'][:15]:
                print(f"  - {w}")
        print(f"[+] PASSED CHECKS: {report['passed_checks']}")
        status = "PASS" if report['compliant'] else "FAIL"
        print(f"STATUS: {status}")

    sys.exit(0 if report['compliant'] else 1)


if __name__ == "__main__":
    main()
