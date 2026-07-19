"""Tests for security scanner — scan_dependencies and run_full_scan."""

import os
import sys

import pytest

# Allow importing from the security scan scripts directory
_SCAN_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..",
    ".agent", "skills", "vulnerability-scanner", "scripts",
)
sys.path.insert(0, _SCAN_DIR)


class TestScanDependencies:
    """Unit tests for the dependency scanner."""

    def test_no_lock_files_triggers_finding(self, tmp_path, monkeypatch):
        """Project with package.json but no lock file should report missing locks."""
        (tmp_path / "package.json").write_text('{"name": "test"}')
        monkeypatch.chdir(tmp_path)

        from security_scan import scan_dependencies
        result = scan_dependencies(str(tmp_path))

        assert len(result["findings"]) >= 1
        assert any("Missing Lock File" in f["type"] for f in result["findings"])

    def test_no_dependency_files_is_clean(self, tmp_path, monkeypatch):
        """Project with no dependency files should pass."""
        monkeypatch.chdir(tmp_path)

        from security_scan import scan_dependencies
        result = scan_dependencies(str(tmp_path))

        assert result["status"] in ("[OK] Supply chain checks passed", "[OK] Secure")

    def test_lock_file_present(self, tmp_path, monkeypatch):
        """Project with package.json and package-lock.json has npm secured."""
        (tmp_path / "package.json").write_text('{"name": "test"}')
        (tmp_path / "package-lock.json").write_text("{}")
        monkeypatch.chdir(tmp_path)

        from security_scan import scan_dependencies
        result = scan_dependencies(str(tmp_path))

        # npm is locked; yarn/pnpm warnings are expected since those managers
        # aren't in use but the scanner flags every missing lock file.
        npm_missing = [
            f for f in result["findings"]
            if f["type"] == "Missing Lock File" and f["message"].startswith("npm:")
        ]
        assert len(npm_missing) == 0

    def test_pip_lock_files_detected(self, tmp_path, monkeypatch):
        """Project with requirements.txt AND poetry.lock is clean."""
        (tmp_path / "requirements.txt").write_text("flask==2.0")
        (tmp_path / "poetry.lock").write_text("[[package]]")
        monkeypatch.chdir(tmp_path)

        from security_scan import scan_dependencies
        result = scan_dependencies(str(tmp_path))

        assert "Missing Lock File" not in str(result["findings"])


class TestRunFullScan:
    """Integration tests for the full scan pipeline."""

    def test_full_scan_returns_valid_report(self, tmp_path, monkeypatch):
        """run_full_scan should return a properly structured report."""
        monkeypatch.chdir(tmp_path)

        from security_scan import run_full_scan
        result = run_full_scan(str(tmp_path), scan_type="deps")

        assert "project" in result
        assert "scans" in result
        assert "summary" in result
        assert result["project"] == str(tmp_path)
        assert "dependencies" in result["scans"]
        assert "total_findings" in result["summary"]

    def test_secrets_scan_runs(self, tmp_path, monkeypatch):
        """Secrets scan should run without errors on clean project."""
        monkeypatch.chdir(tmp_path)

        from security_scan import run_full_scan
        result = run_full_scan(str(tmp_path), scan_type="secrets")

        assert "secrets" in result["scans"]

    def test_scan_type_all_includes_all(self, tmp_path, monkeypatch):
        """scan_type=all should run all scanners."""
        (tmp_path / "package.json").write_text('{"name": "test"}')
        monkeypatch.chdir(tmp_path)

        from security_scan import run_full_scan
        result = run_full_scan(str(tmp_path), scan_type="all")

        assert len(result["scans"]) == 4  # deps, secrets, patterns, config
