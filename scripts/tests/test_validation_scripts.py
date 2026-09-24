"""Regression tests for fail-closed validation orchestration."""

import importlib.util
from pathlib import Path


ROOT = Path(__file__).parents[2]


def load_script(name, relative_path):
    spec = importlib.util.spec_from_file_location(name, ROOT / relative_path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


checklist = load_script("checklist", ".agent/scripts/checklist.py")
verify_all = load_script("verify_all", ".agent/scripts/verify_all.py")


def test_checklist_missing_required_fails(tmp_path):
    result = checklist.run_script(
        "required", tmp_path / "missing.py", str(tmp_path), required=True
    )
    assert result["passed"] is False
    assert result["skipped"] is False


def test_checklist_missing_optional_is_explicit_skip(tmp_path):
    result = checklist.run_script(
        "optional", tmp_path / "missing.py", str(tmp_path), required=False
    )
    assert result["passed"] is True
    assert result["skipped"] is True


def test_checklist_empty_summary_fails():
    assert checklist.print_summary([]) is False


def test_checklist_success_summary_passes():
    assert (
        checklist.print_summary([{"name": "ok", "passed": True, "skipped": False}])
        is True
    )


def test_verifier_missing_required_fails(tmp_path):
    result = verify_all.run_script(
        "required", tmp_path / "missing.py", str(tmp_path), required=True
    )
    assert result["passed"] is False
    assert result["skipped"] is False


def test_verifier_missing_optional_is_explicit_skip(tmp_path):
    result = verify_all.run_script(
        "optional", tmp_path / "missing.py", str(tmp_path), required=False
    )
    assert result["passed"] is True
    assert result["skipped"] is True


def test_verifier_empty_report_fails():
    assert verify_all.print_final_report([], verify_all.datetime.now()) is False


def test_verifier_success_report_passes():
    result = {
        "name": "ok",
        "passed": True,
        "skipped": False,
        "duration": 0,
        "category": "Test",
    }
    assert verify_all.print_final_report([result], verify_all.datetime.now()) is True


def test_checklist_checker_failure_fails_summary():
    assert (
        checklist.print_summary([{"name": "broken", "passed": False, "skipped": False}])
        is False
    )


def test_verifier_checker_failure_fails_report():
    assert (
        verify_all.print_final_report(
            [
                {
                    "name": "broken",
                    "passed": False,
                    "skipped": False,
                    "duration": 0,
                    "category": "Test",
                    "error": "checker failed",
                }
            ],
            verify_all.datetime.now(),
        )
        is False
    )


def test_verifier_no_url_is_visible_skip():
    suite = {
        "category": "Performance",
        "requires_url": True,
        "checks": [("Lighthouse", "missing.py", True)],
    }
    reason = verify_all.suite_skip_reason(suite, Path("."), None)
    assert reason == "URL not provided"


def test_verifier_no_e2e_is_visible_skip():
    suite = {
        "category": "E2E Testing",
        "checks": [("E2E", "missing.py", False)],
    }
    reason = verify_all.suite_skip_reason(suite, Path("."), "http://example.test", True)
    assert reason == "disabled by --no-e2e"


def test_cli_project_skips_inapplicable_ux_and_seo(tmp_path, monkeypatch):
    security = tmp_path / "security.py"
    lint = tmp_path / "lint.py"
    security.write_text("raise SystemExit(0)\n", encoding="utf-8")
    lint.write_text("raise SystemExit(0)\n", encoding="utf-8")
    monkeypatch.setattr(
        checklist,
        "CORE_CHECKS",
        [
            ("Security Scan", "security.py", True),
            ("Lint Check", "lint.py", True),
            ("UX Audit", "ux.py", False),
            ("SEO Check", "seo.py", False),
        ],
    )
    monkeypatch.setattr(checklist, "PERFORMANCE_CHECKS", [])
    monkeypatch.setattr("sys.argv", ["checklist.py", str(tmp_path)])

    try:
        checklist.main()
    except SystemExit as exc:
        assert exc.code == 0


def test_cli_project_is_not_ux_or_seo_applicable(tmp_path):
    assert verify_all.is_frontend_project(tmp_path) is False
    assert verify_all.is_seo_project(tmp_path) is False


def test_web_project_keeps_ux_and_seo_applicable(tmp_path):
    page_dir = tmp_path / "pages"
    page_dir.mkdir()
    (page_dir / "index.html").write_text("<html></html>", encoding="utf-8")
    assert checklist.check_skip_reason("UX Audit", tmp_path) is None
    assert checklist.check_skip_reason("SEO Check", tmp_path) is None
    assert verify_all.is_frontend_project(tmp_path) is True
    assert verify_all.is_seo_project(tmp_path) is True
