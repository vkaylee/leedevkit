#!/usr/bin/env python3
"""
UX Audit Script - Full Frontend Design Coverage

Analyzes code for compliance with:

1. CORE PSYCHOLOGY LAWS:
   - Hick's Law (nav items, form complexity)
   - Fitts' Law (target sizes, touch targets)
   - Miller's Law (chunking, memory limits)
   - Von Restorff Effect (primary CTA visibility)
   - Serial Position Effect (important items at start/end)

2. EMOTIONAL DESIGN (Don Norman):
   - Visceral (first impressions, gradients, animations)
   - Behavioral (feedback, usability, performance)
   - Reflective (brand story, values, identity)

3. TRUST BUILDING:
   - Security signals (SSL, encryption on forms)
   - Social proof (testimonials, reviews, logos)
   - Authority indicators (certifications, awards, media)

4. COGNITIVE LOAD MANAGEMENT:
   - Progressive disclosure (accordion, tabs, "Advanced")
   - Visual noise (too many colors/borders)
   - Familiar patterns (labels, standard conventions)

5. PERSUASIVE DESIGN (Ethical):
   - Smart defaults (pre-selected options)
   - Anchoring (original vs discount price)
   - Social proof (live indicators, numbers)
   - Progress indicators (progress bars, steps)

6. TYPOGRAPHY SYSTEM (9 sections):
   - Font Pairing (max 3 families)
   - Line Length (45-75ch)
   - Line Height (proper ratios)
   - Letter Spacing (uppercase, display text)
   - Weight and Emphasis (contrast levels)
   - Responsive Typography (clamp())
   - Hierarchy (sequential headings)
   - Modular Scale (consistent ratios)
   - Readability (chunking, subheadings)

7. VISUAL EFFECTS (10 sections):
   - Glassmorphism (blur + transparency)
   - Neomorphism (dual shadows, inset)
   - Shadow Hierarchy (elevation levels)
   - Gradients (usage, overuse)
   - Border Effects (complexity check)
   - Glow Effects (text-shadow, box-shadow)
   - Overlay Techniques (image text readability)
   - GPU Acceleration (transform/opacity vs layout)
   - Performance (will-change usage)
   - Effect Selection (purpose over decoration)

8. COLOR SYSTEM (7 sections):
   - PURPLE BAN (Critical Maestro rule)
   - 60-30-10 Rule (dominant, secondary, accent)
   - Color Scheme Patterns (monochromatic, analogous)
   - Dark Mode Compliance (no pure black/white)
   - WCAG Contrast (low-contrast detection)
   - Color Psychology Context (food + blue = bad)
   - HSL-Based Palettes (recommended approach)

9. ANIMATION GUIDE (6 sections):
   - Duration Appropriateness (50ms minimum, 1s max transitions)
   - Easing Functions (ease-out for entry, ease-in for exit)
   - Micro-interactions (hover/focus feedback)
   - Loading States (skeleton, spinner, progress)
   - Page Transitions (fade/slide for routing)
   - Scroll Animation Performance (no layout properties)

10. MOTION GRAPHICS (7 sections):
   - Lottie Animations (reduced motion fallbacks)
   - GSAP Memory Leaks (kill/revert on unmount)
   - SVG Animation Performance (stroke-dashoffset sparingly)
   - 3D Transforms (perspective parent, mobile warning)
   - Particle Effects (mobile fallback)
   - Scroll-Driven Animations (throttle with rAF)
   - Motion Decision Tree (functional vs decorative)

11. ACCESSIBILITY:
   - Alt text for images
   - Reduced motion checks
   - Form labels

Total: 80+ checks across all design principles
"""

import sys
import os
import re
import json
from pathlib import Path


class UXAuditor:
    """Audits frontend code for UX / design compliance.

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
    # Populated by _init_checkers() so imports happen only when a checker
    # is actually needed (lazy, avoids circular imports).

    _CHECKERS: list[tuple[str, object]] = []

    @classmethod
    def _init_checkers(cls) -> None:
        if cls._CHECKERS:
            return  # already initialised
        # Use absolute imports so the script works both when run directly
        # (python ux_audit.py) and when imported as a module.
        from checkers.psychology import PsychologyChecker
        from checkers.emotional_design import EmotionalDesignChecker
        from checkers.trust import TrustChecker
        from checkers.cognitive_load import CognitiveLoadChecker
        from checkers.persuasion import PersuasiveChecker
        from checkers.typography import TypographyChecker
        from checkers.visual_effects import VisualEffectsChecker
        from checkers.color_system import ColorSystemChecker
        from checkers.animation import AnimationChecker
        from checkers.motion import MotionGraphicsChecker
        from checkers.accessibility import AccessibilityChecker

        cls._CHECKERS = [
            ("psychology", PsychologyChecker()),
            ("emotional_design", EmotionalDesignChecker()),
            ("trust", TrustChecker()),
            ("cognitive_load", CognitiveLoadChecker()),
            ("persuasion", PersuasiveChecker()),
            ("typography", TypographyChecker()),
            ("visual", VisualEffectsChecker()),
            ("color", ColorSystemChecker()),
            ("animation", AnimationChecker()),
            ("motion", MotionGraphicsChecker()),
            ("accessibility", AccessibilityChecker()),
        ]

    # -- Core audit methods ------------------------------------------------

    def audit_file(self, filepath: str) -> None:
        """Run all registered checkers against a single file."""
        content = self._read_file(filepath)
        if content is None:
            return

        self.files_checked += 1
        filename = os.path.basename(filepath)

        self._init_checkers()
        ctx = self._build_context(filename, content)

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
        extensions = {'.tsx', '.jsx', '.html', '.vue', '.svelte', '.css'}
        for root, dirs, files in os.walk(directory):
            dirs[:] = [d for d in dirs if d not in {
                'node_modules', '.git', 'dist', 'build', '.next', 'coverage', '.test_logs',
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
    def _build_context(filename: str, content: str):
        """Pre-calculate shared flags used by multiple checkers."""
        from checkers._base import AuditContext

        return AuditContext(
            filename=filename,
            content=content,
            has_long_text=bool(re.search(
                r'<p|<div.*class=.*text|article|<span.*text',
                content, re.IGNORECASE,
            )),
            has_form=bool(re.search(
                r'<form|<input|<select|<textarea', content, re.IGNORECASE,
            )),
            complex_elements=len(re.findall(
                r'<input|<select|<textarea|<option', content, re.IGNORECASE,
            )),
            has_hero=bool(re.search(r'hero|<h1|banner', content, re.IGNORECASE)),
            nav_items=len(re.findall(
                r'<NavLink|<Link|<a\s+href|nav-item', content, re.IGNORECASE,
            )),
        )


def main():
    if len(sys.argv) < 2:
        sys.exit(1)

    path = sys.argv[1]
    is_json = "--json" in sys.argv

    auditor = UXAuditor()
    if os.path.isfile(path):
        auditor.audit_file(path)
    else:
        auditor.audit_directory(path)

    report = auditor.get_report()

    if is_json:
        print(json.dumps(report))
    else:
        # Use ASCII-safe output for Windows console compatibility
        print(f"\n[UX AUDIT] {report['files_checked']} files checked")
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
