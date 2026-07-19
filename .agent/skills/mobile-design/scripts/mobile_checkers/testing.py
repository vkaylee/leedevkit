"""Mobile Testing checks: testing tools, test pyramid, accessibility labels, error boundaries."""

import re

from ._base import MobileAuditContext


class MobileTestingChecker:
    """Testing infrastructure and debugging checks for mobile apps."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # Testing tool detection
        has_rntl = bool(re.search(
            r'react-native-testing-library|@testing-library', ctx.content,
        ))
        has_detox = bool(re.search(r'detox|element\(|by\.text|by\.id', ctx.content))
        has_maestro = bool(re.search(r'maestro|\.yaml$', ctx.content))
        has_jest = bool(re.search(r'jest|describe\(|test\(|it\(', ctx.content))

        testing_tools = []
        if has_jest:
            testing_tools.append('Jest')
        if has_rntl:
            testing_tools.append('RNTL')
        if has_detox:
            testing_tools.append('Detox')
        if has_maestro:
            testing_tools.append('Maestro')

        if len(testing_tools) == 0:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Testing] {ctx.filename}: No testing framework detected. "
                    "Consider Jest (unit) + Detox/Maestro (E2E) for mobile."
                ),
            })

        # 13.2 Test Pyramid Balance
        test_files = len(re.findall(
            r'\.test\.(tsx|ts|js|jsx)|\.spec\.', ctx.content,
        ))
        e2e_tests = len(re.findall(
            r'detox|maestro|e2e|spec\.e2e', ctx.content.lower(),
        ))

        if test_files > 0 and e2e_tests == 0:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Testing] {ctx.filename}: Unit tests found but no E2E tests. "
                    "Mobile needs E2E on real devices for complete coverage."
                ),
            })

        # 13.3 Accessibility Labels
        if ctx.is_react_native:
            has_pressable = bool(re.search(
                r'Pressable|TouchableOpacity|TouchableHighlight', ctx.content,
            ))
            has_a11y_label = bool(re.search(
                r'accessibilityLabel|aria-label|testID', ctx.content,
            ))
            if has_pressable and not has_a11y_label:
                findings.append({
                    "severity": "warning",
                    "message": (
                        f"[A11y Mobile] {ctx.filename}: Touchable element without "
                        "accessibilityLabel. Screen readers need labels."
                    ),
                })

        return findings
