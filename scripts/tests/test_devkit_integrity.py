"""Tests for _devkit_integrity — checksum manifest generation & verification."""

import json
import os
import sys


sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _devkit_integrity import (
    _compute_file_hash,
    _read_manifest,
    _walk_files,
    generate_manifest,
    verify_devkit,
    write_manifest,
    VerificationResult,
)


class TestComputeFileHash:
    def test_deterministic(self, tmp_path):
        file_path = tmp_path / "test.txt"
        file_path.write_text("hello world")
        h1 = _compute_file_hash(file_path)
        h2 = _compute_file_hash(file_path)
        assert h1 == h2
        assert len(h1) == 64  # SHA256 hex

    def test_different_content_different_hash(self, tmp_path):
        a = tmp_path / "a.txt"
        b = tmp_path / "b.txt"
        a.write_text("hello")
        b.write_text("world")
        assert _compute_file_hash(a) != _compute_file_hash(b)

    def test_empty_file(self, tmp_path):
        f = tmp_path / "empty.txt"
        f.write_text("")
        h = _compute_file_hash(f)
        assert len(h) == 64


class TestWalkFiles:
    def test_excludes_manifest(self, tmp_path, monkeypatch):
        (tmp_path / "file.txt").write_text("data")
        (tmp_path / "devkit.manifest.json").write_text("{}")
        (tmp_path / "VERSION").write_text("1.0.0")

        class FakeDevkitRoot:
            @staticmethod
            def _find_devkit_root():
                return tmp_path

        files = _walk_files(tmp_path)
        names = {f.name for f in files}
        assert "devkit.manifest.json" not in names
        assert "file.txt" in names
        assert "VERSION" in names

    def test_excludes_pycache(self, tmp_path):
        pycache = tmp_path / "__pycache__"
        pycache.mkdir()
        (pycache / "cached.pyc").write_text("binary")
        (tmp_path / "real.py").write_text("code")

        files = _walk_files(tmp_path)
        paths = [str(f.relative_to(tmp_path)) for f in files]
        assert all("__pycache__" not in p for p in paths)
        assert "real.py" in paths


class TestGenerateManifest:
    def test_has_required_keys(self, tmp_path):
        (tmp_path / "VERSION").write_text("0.2.0")
        (tmp_path / "file.txt").write_text("content")
        manifest = generate_manifest(tmp_path)
        assert manifest["version"] == "0.2.0"
        assert manifest["algorithm"] == "sha256"
        assert manifest["file_count"] > 0
        assert "files" in manifest
        assert "file.txt" in manifest["files"]

    def test_version_unknown_when_missing(self, tmp_path):
        (tmp_path / "file.txt").write_text("x")
        manifest = generate_manifest(tmp_path)
        assert manifest["version"] == "unknown"


class TestWriteManifest:
    def test_writes_json_file(self, tmp_path):
        (tmp_path / "VERSION").write_text("0.1.0")
        (tmp_path / "a.py").write_text("print(1)")
        path = write_manifest(tmp_path)
        assert path.exists()
        assert path.name == "devkit.manifest.json"
        data = json.loads(path.read_text())
        assert "files" in data
        assert "a.py" in data["files"]


class TestReadManifest:
    def test_returns_none_for_missing(self, tmp_path):
        assert _read_manifest(tmp_path) is None

    def test_reads_valid_manifest(self, tmp_path):
        manifest = tmp_path / "devkit.manifest.json"
        manifest.write_text(json.dumps({"version": "1.0", "files": {}}))
        result = _read_manifest(tmp_path)
        assert result == {"version": "1.0", "files": {}}

    def test_returns_none_for_invalid_json(self, tmp_path):
        manifest = tmp_path / "devkit.manifest.json"
        manifest.write_text("not json")
        assert _read_manifest(tmp_path) is None


class TestVerificationResult:
    def test_is_clean_when_no_issues(self):
        r = VerificationResult()
        r.ok = ["file1.md", "file2.py"]
        assert r.is_clean is True

    def test_not_clean_when_modified(self):
        r = VerificationResult()
        r.modified.append(("file.md", "abc", "def"))
        assert r.is_clean is False

    def test_not_clean_when_missing(self):
        r = VerificationResult()
        r.missing.append("gone.md")
        assert r.is_clean is False

    def test_not_clean_when_no_manifest(self):
        r = VerificationResult()
        r.no_manifest = True
        assert r.is_clean is False

    def test_report_clean(self):
        r = VerificationResult()
        r.ok = ["a.md", "b.md"]
        report = r.report()
        assert "verified" in report
        assert "2 files OK" in report

    def test_report_modified(self):
        r = VerificationResult()
        r.ok = ["good.md"]
        r.modified.append(("bad.md", "abc123def", "999aaa"))
        report = r.report()
        assert "FAILED" in report
        assert "bad.md" in report


class TestVerifyDevkit:
    def test_no_manifest_returns_flag(self, tmp_path):
        result = verify_devkit(tmp_path)
        assert result.no_manifest is True

    def test_clean_manifest(self, tmp_path):
        (tmp_path / "a.txt").write_text("hello")
        write_manifest(tmp_path)
        result = verify_devkit(tmp_path)
        assert result.is_clean is True
        assert len(result.ok) > 0

    def test_detects_modified_file(self, tmp_path):
        (tmp_path / "a.txt").write_text("original")
        write_manifest(tmp_path)
        (tmp_path / "a.txt").write_text("modified")
        result = verify_devkit(tmp_path)
        assert "a.txt" in [m[0] for m in result.modified]
