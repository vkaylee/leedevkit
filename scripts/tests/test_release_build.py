"""Tests for _release_build — release tarball construction."""

import json
import os
import subprocess
import sys
from pathlib import Path
import tarfile


sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _release_build import (
    INCLUDE_DIRS,
    INCLUDE_FILES,
    _should_exclude,
    build_release,
    main,
)


class TestShouldExclude:
    def test_excludes_pycache(self):
        assert _should_exclude("__pycache__/foo.pyc") is True
        assert _should_exclude("scripts/__pycache__/something.pyc") is True

    def test_excludes_hidden_pyc(self):
        """Literal .pyc files (hidden files, not extensions) are excluded."""
        assert _should_exclude(".pyc") is True

    def test_excludes_hidden_pyo(self):
        """Literal .pyo files (hidden files, not extensions) are excluded."""
        assert _should_exclude(".pyo") is True

    def test_excludes_git(self):
        assert _should_exclude(".git/config") is True
        assert _should_exclude(".git/HEAD") is True

    def test_excludes_ds_store(self):
        assert _should_exclude(".DS_Store") is True
        assert _should_exclude("dir/.DS_Store") is True

    def test_allows_normal_files(self):
        assert _should_exclude("scripts/_orchestrator.py") is False
        assert _should_exclude("templates/CLAUDE.base.md") is False
        assert _should_exclude("bin/leedevkit") is False

    def test_allows_version_file(self):
        assert _should_exclude("VERSION") is False


class TestAgentTestingGuidance:
    """Guard the mandatory post-implementation test decision in shipped context."""

    repo_root = Path(__file__).resolve().parents[2]

    def test_base_prompt_requires_explicit_test_impact_outcome(self):
        prompt = (self.repo_root / "templates" / "CLAUDE.base.md").read_text()

        assert "Post-Implementation Test Gate" in prompt
        assert "tests added/updated" in prompt
        assert "reason no test change is needed" in prompt
        assert "./leedevkit test <target>" in prompt
        assert "scripts/_release_acceptance.py" in prompt
        assert "source tests alone are NOT enough" in prompt

    def test_testing_standard_defines_test_gate_and_exceptions(self):
        rules = (
            self.repo_root / ".agent" / "rules" / "testing-standards.md"
        ).read_text()
        internals = (
            self.repo_root / ".agent" / "rules" / "devkit-internals.md"
        ).read_text()

        assert "Decide autonomously whether tests must be added or updated" in rules
        assert "No test change needed" in rules
        assert "Bug fixes and regressions" in rules
        assert "documentation-only" in rules
        assert "./leedevkit test <target>" in rules
        assert "./test.sh" not in rules
        assert "scripts/_release_acceptance.py" in rules
        assert "Release Packaging Gate" in internals
        assert "LEEDEVKIT_RELEASE_BASE_URL" in internals


class TestBuildRelease:
    def test_shell_builder_uses_canonical_root_and_verified_manifest(self, tmp_path):
        """Run the shipped shell builder against a local tag, without network."""
        repo_root = Path(__file__).resolve().parents[2]
        output = tmp_path / "dist"
        result = subprocess.run(
            [
                "bash",
                str(repo_root / "scripts" / "build-release.sh"),
                "v0.3.15",
                str(output),
            ],
            cwd=repo_root,
            env={**os.environ, "LEEDEVKIT_REPO": str(repo_root)},
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )

        assert result.returncode == 0, result.stderr or result.stdout
        artifact = output / "leedevkit-0.3.15.tar.gz"
        with tarfile.open(artifact, "r:gz") as archive:
            roots = {Path(name).parts[0] for name in archive.getnames() if name}
            assert roots == {"leedevkit-0.3.15"}
            manifest = archive.extractfile("leedevkit-0.3.15/devkit.manifest.json")
            assert manifest is not None
            assert json.load(manifest)["version"] == "0.3.15"

    def test_happy_path_creates_tarball(self, tmp_path):
        """build_release should create a valid .tar.gz with expected contents."""
        repo = tmp_path / "repo"
        repo.mkdir()
        (repo / "VERSION").write_text("0.4.0")

        # Create minimal runtime payload required by the release contract.
        for d in INCLUDE_DIRS:
            (repo / d).mkdir(parents=True, exist_ok=True)
        for f in INCLUDE_FILES:
            fpath = repo / f
            if not fpath.exists():
                fpath.write_text("dummy")
        (repo / "bin" / "leedevkit").write_text("#!/bin/bash\n")
        (repo / "scripts" / "_orchestrator.py").write_text("# orchestrator\n")
        (repo / "scripts" / "_devkit_integrity.py").write_text("# integrity\n")

        out_dir = tmp_path / "dist"
        result = build_release(repo, out_dir)

        assert result.exists()
        assert result.name == "leedevkit-0.4.0.tar.gz"
        assert result.stat().st_size > 0

        # Verify it's a valid tar.gz
        with tarfile.open(result, "r:gz") as tf:
            names = tf.getnames()
            assert any("leedevkit-0.4.0/scripts" in n for n in names)
            assert any("leedevkit-0.4.0/VERSION" in n for n in names)
            manifest_file = tf.extractfile("leedevkit-0.4.0/devkit.manifest.json")
            assert manifest_file is not None
            manifest = json.load(manifest_file)
            assert manifest["version"] == "0.4.0"
            assert manifest["file_count"] == len(manifest["files"])
            assert "bin/leedevkit" in manifest["files"]

    def test_missing_version_file_raises(self, tmp_path):
        """build_release should raise FileNotFoundError when VERSION is missing."""
        repo = tmp_path / "repo"
        repo.mkdir()
        out_dir = tmp_path / "dist"

        import pytest

        with pytest.raises(FileNotFoundError, match="VERSION file not found"):
            build_release(repo, out_dir)

    def test_creates_output_dir_if_missing(self, tmp_path):
        """build_release should create the output directory if it doesn't exist."""
        repo = tmp_path / "repo"
        repo.mkdir()
        (repo / "VERSION").write_text("1.0.0")
        for d in INCLUDE_DIRS:
            (repo / d).mkdir(parents=True, exist_ok=True)
        for f in INCLUDE_FILES:
            fpath = repo / f
            if not fpath.exists():
                fpath.write_text("dummy")
        (repo / "bin" / "leedevkit").write_text("#!/bin/bash\n")
        (repo / "scripts" / "_orchestrator.py").write_text("# orchestrator\n")
        (repo / "scripts" / "_devkit_integrity.py").write_text("# integrity\n")

        out_dir = tmp_path / "nested" / "dist"
        result = build_release(repo, out_dir)
        assert result.exists()
        assert out_dir.exists()

    def test_excludes_pycache_from_tarball(self, tmp_path):
        """PyCache files should not appear in the tarball."""
        repo = tmp_path / "repo"
        repo.mkdir()
        (repo / "VERSION").write_text("2.0.0")
        for d in INCLUDE_DIRS:
            (repo / d).mkdir(parents=True, exist_ok=True)
        for f in INCLUDE_FILES:
            fpath = repo / f
            if not fpath.exists():
                fpath.write_text("dummy")
        (repo / "bin" / "leedevkit").write_text("#!/bin/bash\n")
        (repo / "scripts" / "_orchestrator.py").write_text("# orchestrator\n")
        (repo / "scripts" / "_devkit_integrity.py").write_text("# integrity\n")

        # Create a pycache file that should be excluded
        pycache_dir = repo / "scripts" / "__pycache__"
        pycache_dir.mkdir(parents=True, exist_ok=True)
        (pycache_dir / "cached.pyc").write_text("cache")

        out_dir = tmp_path / "dist"
        result = build_release(repo, out_dir)

        with tarfile.open(result, "r:gz") as tf:
            names = tf.getnames()
            assert not any("__pycache__" in n for n in names)


class TestMain:
    def test_invalid_repo_root_exits(self, tmp_path):
        """main() should sys.exit(1) when repo_root doesn't look like leedevkit."""
        import pytest

        fake_root = tmp_path / "not-a-repo"
        fake_root.mkdir()

        with pytest.raises(SystemExit) as exc_info:
            # Simulate: --repo-root pointing to empty dir
            main_args = [
                "prog",
                "--repo-root",
                str(fake_root),
            ]
            old_argv = sys.argv
            try:
                sys.argv = main_args
                main()
            finally:
                sys.argv = old_argv
        assert exc_info.value.code == 1

    def test_main_with_output_flag(self, tmp_path, monkeypatch):
        """main() should accept --output flag."""
        repo = tmp_path / "repo"
        repo.mkdir()
        (repo / "VERSION").write_text("3.0.0")
        # Need scripts/_orchestrator.py for repo-root validation
        scripts_dir = repo / "scripts"
        scripts_dir.mkdir()
        (scripts_dir / "_orchestrator.py").write_text("# orchestrator")
        for d in INCLUDE_DIRS:
            p = repo / d
            if not p.exists():
                p.mkdir(parents=True, exist_ok=True)
        for f in INCLUDE_FILES:
            fpath = repo / f
            if not fpath.exists():
                fpath.write_text("dummy")
        (repo / "bin" / "leedevkit").write_text("#!/bin/bash\n")
        (repo / "scripts" / "_devkit_integrity.py").write_text("# integrity\n")

        out_dir = tmp_path / "custom-dist"
        main_args = [
            "prog",
            "--repo-root",
            str(repo),
            "--output",
            str(out_dir),
        ]
        old_argv = sys.argv
        try:
            sys.argv = main_args
            main()
        finally:
            sys.argv = old_argv

        tarballs = list(out_dir.glob("leedevkit-*.tar.gz"))
        assert len(tarballs) == 1
