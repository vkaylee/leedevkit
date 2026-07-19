"""Tests for UX audit checkers — integration and unit tests."""

import os
import sys

import pytest

# Allow importing from the UX audit scripts directory
_UX_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..",
    ".agent", "skills", "frontend-design", "scripts",
)
sys.path.insert(0, _UX_DIR)


# ---------------------------------------------------------------------------
# Test fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def compliant_html():
    return """<!DOCTYPE html>
<html lang="en">
<head><title>My Page</title></head>
<body>
  <header class="hero" style="background: linear-gradient(to right, #fff, #eee)">
    <h1>Welcome to Our Site</h1>
    <p class="text-lg leading-relaxed max-w-prose">Great content here.</p>
  </header>
  <nav>
    <a href="/">Home</a>
    <a href="/about">About</a>
    <a href="/contact" class="primary">Contact</a>
  </nav>
  <main>
    <h2>About Us</h2>
    <p>Our mission is to create great products. We value quality and trust.</p>
    <p>Our journey started in 2020 with a simple vision.</p>
    <img src="team.jpg" alt="Our team">
    <button class="bg-primary">Get Started</button>
  </main>
  <footer>
    <p>Award-winning design. Featured in TechCrunch.</p>
  </footer>
</body>
</html>"""


@pytest.fixture
def problematic_html():
    return """<!DOCTYPE html>
<html>
<head><title>Bad Page</title></head>
<body>
  <nav>
    <a href="/">Home</a>
    <a href="/about">About</a>
    <a href="/services">Services</a>
    <a href="/products">Products</a>
    <a href="/blog">Blog</a>
    <a href="/faq">FAQ</a>
    <a href="/support">Support</a>
    <a href="/contact">Contact</a>
  </nav>
  <h1>Welcome</h1>
  <img src="logo.png">
  <form>
    <input type="text" name="email">
    <input type="password" name="pass">
    <button>Submit</button>
  </form>
  <p style="color: #8B5CF6">Purple text</p>
  <div style="height: 20px">Small</div>
</body>
</html>"""


@pytest.fixture
def empty_html():
    return "<html></html>"


# ---------------------------------------------------------------------------
# Integration tests
# ---------------------------------------------------------------------------

class TestUXAuditorIntegration:
    """Full pipeline tests on known HTML fixtures."""

    def test_compliant_html_passes(self, tmp_path, compliant_html):
        """Compliant HTML should produce zero issues."""
        f = tmp_path / "good.html"
        f.write_text(compliant_html, encoding="utf-8")

        from ux_audit import UXAuditor
        auditor = UXAuditor()
        auditor.audit_file(str(f))
        report = auditor.get_report()

        assert report["files_checked"] == 1
        assert report["compliant"] is True, f"Issues found: {report['issues']}"
        assert len(report["issues"]) == 0

    def test_problematic_html_fails(self, tmp_path, problematic_html):
        """Problematic HTML should produce issues."""
        f = tmp_path / "bad.html"
        f.write_text(problematic_html, encoding="utf-8")

        from ux_audit import UXAuditor
        auditor = UXAuditor()
        auditor.audit_file(str(f))
        report = auditor.get_report()

        assert report["files_checked"] == 1
        assert report["compliant"] is False
        assert len(report["issues"]) >= 2  # purple, img alt, form labels
        assert len(report["warnings"]) >= 1

    def test_audit_directory(self, tmp_path, problematic_html):
        """audit_directory walks and audits all matching files."""
        d = tmp_path / "src"
        d.mkdir()
        (d / "page1.html").write_text(problematic_html, encoding="utf-8")
        (d / "page2.tsx").write_text("<div>Hello</div>", encoding="utf-8")
        (d / "README.md").write_text("# Readme", encoding="utf-8")

        from ux_audit import UXAuditor
        auditor = UXAuditor()
        auditor.audit_directory(str(d))
        report = auditor.get_report()

        assert report["files_checked"] == 2

    def test_empty_html(self, tmp_path, empty_html):
        """Empty HTML with no content should pass."""
        f = tmp_path / "empty.html"
        f.write_text(empty_html, encoding="utf-8")

        from ux_audit import UXAuditor
        auditor = UXAuditor()
        auditor.audit_file(str(f))
        report = auditor.get_report()

        assert report["compliant"] is True


# ---------------------------------------------------------------------------
# Psychology checker unit tests
# ---------------------------------------------------------------------------

class TestPsychologyChecker:
    @staticmethod
    def _ctx(filename="test.html", content="", **kw):
        from checkers._base import AuditContext
        return AuditContext(filename=filename, content=content, **kw)

    def test_hicks_law_too_many_nav(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(nav_items=10)
        findings = PsychologyChecker().run(ctx)
        assert any("Hick's Law" in f["message"] for f in findings)

    def test_hicks_law_ok(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(nav_items=5)
        findings = PsychologyChecker().run(ctx)
        assert not any("Hick's Law" in f["message"] for f in findings)

    def test_fitts_law_small_target(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(content='<div style="height: 20px">x</div>')
        findings = PsychologyChecker().run(ctx)
        assert any("Fitts' Law" in f["message"] for f in findings)

    def test_fitts_law_large_target_ok(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(content='<div style="height: 50px">x</div>')
        findings = PsychologyChecker().run(ctx)
        assert not any("Fitts' Law" in f["message"] for f in findings)

    def test_millers_law_complex_form(self):
        from checkers.psychology import PsychologyChecker
        # 8 form fields without wizard/step
        ctx = self._ctx(content='<input><input><input><input><input><input><input><input>')
        findings = PsychologyChecker().run(ctx)
        assert any("Miller's Law" in f["message"] for f in findings)

    def test_millers_law_chunked_form_ok(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(content='<input><input><input><input><input><input><input><input> wizard step-1')
        findings = PsychologyChecker().run(ctx)
        assert not any("Miller's Law" in f["message"] for f in findings)

    def test_von_restorff_no_primary_cta(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(content='<button>Click</button>')
        findings = PsychologyChecker().run(ctx)
        assert any("Von Restorff" in f["message"] for f in findings)

    def test_von_restorff_has_primary(self):
        from checkers.psychology import PsychologyChecker
        ctx = self._ctx(content='<button class="primary">Click</button>')
        findings = PsychologyChecker().run(ctx)
        assert not any("Von Restorff" in f["message"] for f in findings)

    def test_serial_position_missing_important_last(self):
        from checkers.psychology import PsychologyChecker
        content = '<a href="/">Home</a><a href="/about">About</a><a href="/faq">FAQ</a>'
        ctx = self._ctx(content=content, nav_items=4)
        findings = PsychologyChecker().run(ctx)
        assert any("Serial Position" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Color system checker unit tests
# ---------------------------------------------------------------------------

class TestColorSystemChecker:
    @staticmethod
    def _ctx(content="", filename="test.html"):
        from checkers._base import AuditContext
        return AuditContext(filename=filename, content=content)

    def test_purple_hex_banned(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<p style="color: #8B5CF6">text</p>')
        findings = ColorSystemChecker().run(ctx)
        assert any("PURPLE DETECTED" in f["message"] for f in findings)

    def test_purple_name_banned(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<p class="text-purple">text</p>')
        findings = ColorSystemChecker().run(ctx)
        assert any("PURPLE DETECTED" in f["message"] for f in findings)

    def test_no_purple_ok(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<p style="color: #10B981">text</p>')
        findings = ColorSystemChecker().run(ctx)
        assert not any("PURPLE DETECTED" in f["message"] for f in findings)

    def test_pure_black_warns(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<p style="color: #000000">text</p>')
        findings = ColorSystemChecker().run(ctx)
        assert any("Pure black" in f["message"] for f in findings)

    def test_low_contrast_combination(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<div class="bg-gray-50 text-gray-100">low contrast</div>')
        findings = ColorSystemChecker().run(ctx)
        assert any("low-contrast" in f["message"] for f in findings)

    def test_blue_in_food_context(self):
        from checkers.color_system import ColorSystemChecker
        ctx = self._ctx('<div class="bg-blue-500 restaurant-menu">Food</div>')
        findings = ColorSystemChecker().run(ctx)
        assert any("food context" in f["message"].lower() for f in findings)


# ---------------------------------------------------------------------------
# Typography checker unit tests
# ---------------------------------------------------------------------------

class TestTypographyChecker:
    @staticmethod
    def _ctx(content="", filename="test.html", **kw):
        from checkers._base import AuditContext
        return AuditContext(filename=filename, content=content, **kw)

    def test_too_many_font_families(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(
            content='<style>'
            '@font-face{font-family:"CustomA"}'
            '@font-face{font-family:"CustomB"}'
            '@font-face{font-family:"CustomC"}'
            '@font-face{font-family:"CustomD"}'
            '</style>'
        )
        findings = TypographyChecker().run(ctx)
        assert any("font families" in f["message"] for f in findings)

    def test_no_line_length_constraint(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(
            content='<p>Long text content</p>',
            has_long_text=True,
        )
        findings = TypographyChecker().run(ctx)
        assert any("line length" in f["message"] for f in findings)

    def test_has_max_w_prose_ok(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(
            content='<p class="max-w-prose">Long text</p>',
            has_long_text=True,
        )
        findings = TypographyChecker().run(ctx)
        assert not any("line length" in f["message"] for f in findings)

    def test_no_line_height(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(content='<p>text</p><span>more</span>')
        findings = TypographyChecker().run(ctx)
        assert any("line-height" in f["message"] for f in findings)

    def test_uppercase_without_tracking(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(content='<div class="uppercase">ALL CAPS</div>')
        findings = TypographyChecker().run(ctx)
        assert any("tracking" in f["message"] for f in findings)

    def test_skipped_heading_level(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(content='<h1>A</h1><h3>C</h3>')
        findings = TypographyChecker().run(ctx)
        assert any("Skipped heading" in f["message"] for f in findings)

    def test_no_h1_found(self):
        from checkers.typography import TypographyChecker
        ctx = self._ctx(content='<h2>Title</h2><p>text text text</p>', has_long_text=True)
        findings = TypographyChecker().run(ctx)
        assert any("No h1 found" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Accessibility checker unit tests
# ---------------------------------------------------------------------------

class TestAccessibilityChecker:
    def test_missing_alt_text(self):
        from checkers._base import AuditContext
        from checkers.accessibility import AccessibilityChecker
        ctx = AuditContext(filename="test.html", content='<img src="x.png">')
        findings = AccessibilityChecker().run(ctx)
        assert any("Missing img alt" in f["message"] for f in findings)

    def test_has_alt_text_ok(self):
        from checkers._base import AuditContext
        from checkers.accessibility import AccessibilityChecker
        ctx = AuditContext(filename="test.html", content='<img src="x.png" alt="X">')
        findings = AccessibilityChecker().run(ctx)
        assert not any("Missing img alt" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Emotional Design checker unit tests
# ---------------------------------------------------------------------------

class TestEmotionalDesignChecker:
    @staticmethod
    def _ctx(filename="test.html", content="", **kw):
        from checkers._base import AuditContext
        return AuditContext(filename=filename, content=content, **kw)

    def test_visceral_hero_no_appeal(self):
        from checkers.emotional_design import EmotionalDesignChecker
        ctx = self._ctx(content='<div class="hero">Plain hero</div>', has_hero=True)
        findings = EmotionalDesignChecker().run(ctx)
        assert any("Visceral" in f["message"] for f in findings)

    def test_behavioral_no_feedback(self):
        from checkers.emotional_design import EmotionalDesignChecker
        ctx = self._ctx(content='<button onClick="do()">Click</button>')
        findings = EmotionalDesignChecker().run(ctx)
        assert any("Behavioral" in f["message"] for f in findings)

    def test_reflective_no_brand(self):
        from checkers.emotional_design import EmotionalDesignChecker
        ctx = self._ctx(content='<p>Long text content</p>', has_long_text=True)
        findings = EmotionalDesignChecker().run(ctx)
        assert any("Reflective" in f["message"] for f in findings)
