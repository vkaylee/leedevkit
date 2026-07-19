"""Tests for _release_build — release tarball construction."""

import os
import sys
import tarfile


sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _release_build import (
    EXCLUDE_PATTERNS,
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


class TestBuildRelease:
    def test_happy_path_creates_tarball(self, tmp_path):
        """build_release should create a valid .tar.gz with expected contents."""
        repo = tmp_path / "repo"
        repo.mkdir()
        (repo / "VERSION").write_text("0.4.0")

        # Create minimal dirs and files that INCLUDE_DIRS/FILES expect
        for d in INCLUDE_DIRS:
            (repo / d).mkdir(parents=True, exist_ok=True)
        for f in INCLUDE_FILES:
            fpath = repo / f
            if not fpath.exists():
                fpath.write_text("dummy")

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
            import argparse
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

        out_dir = tmp_path / "custom-dist"
        main_args = [
            "prog",
            "--repo-root",
            str(repo),
            "--output",
            str(out_dir),
        ]
        import argparse
        old_argv = sys.argv
        try:
            sys.argv = main_args
            main()
        finally:
            sys.argv = old_argv

        tarballs = list(out_dir.glob("leedevkit-*.tar.gz"))
        assert len(tarballs) == 1
