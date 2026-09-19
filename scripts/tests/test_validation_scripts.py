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
