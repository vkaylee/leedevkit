#!/usr/bin/env python3
"""Build a release tarball for leedevkit.

Usage:
    python3 scripts/_release_build.py [--output dist/]

Produces:
    dist/leedevkit-{VERSION}.tar.gz

The tarball contains the immutable devkit artifacts that get installed
into each project's .leedevkit/ directory via `leedevkit init`.
"""

from __future__ import annotations

import argparse
import shutil
import sys
import tarfile
import tempfile
from pathlib import Path

from _devkit_integrity import verify_devkit, write_manifest
from _logging import log_error, log_info, log_success


# ── Items to include in the release tarball ──
INCLUDE_DIRS = ["scripts", "templates", "bin", ".agent", "container"]
INCLUDE_FILES = ["VERSION"]
REQUIRED_FILES = [
    "VERSION",
    "bin/leedevkit",
    "scripts/_orchestrator.py",
    "scripts/_devkit_integrity.py",
]

# ── Patterns to exclude — always filtered ──
EXCLUDE_PATTERNS = {"__pycache__", ".pyc", ".pyo", ".git", ".DS_Store"}


def _should_exclude(name: str) -> bool:
    """Check if a file/dir should be excluded from the tarball."""
    return any(part in EXCLUDE_PATTERNS for part in Path(name).parts)


def _copy_release_payload(repo_root: Path, stage_root: Path) -> None:
    """Copy the immutable release allowlist into a clean staging root."""
    for dirname in INCLUDE_DIRS:
        source = repo_root / dirname
        if source.exists():
            log_info(f"  + {dirname}/")
            shutil.copytree(
                source,
                stage_root / dirname,
                ignore=lambda _directory, names: [
                    name for name in names if _should_exclude(name)
                ],
            )

    for filename in INCLUDE_FILES:
        source = repo_root / filename
        if source.exists():
            log_info(f"  + {filename}")
            shutil.copy2(source, stage_root / filename)


def build_release(repo_root: Path, output_dir: Path) -> Path:
    """Stage, validate, and archive the release payload."""
    version_file = repo_root / "VERSION"
    if not version_file.exists():
        raise FileNotFoundError(f"VERSION file not found in {repo_root}")
    version = version_file.read_text().strip()
    if not version:
        raise ValueError("VERSION file is empty")

    missing = [path for path in REQUIRED_FILES if not (repo_root / path).is_file()]
    if missing:
        raise FileNotFoundError(f"Required release files missing: {', '.join(missing)}")

    output_dir.mkdir(parents=True, exist_ok=True)
    tarball_path = output_dir / f"leedevkit-{version}.tar.gz"
    log_info(f"Building {tarball_path} ...")

    with tempfile.TemporaryDirectory(prefix="leedevkit-release-") as temp_dir:
        stage_root = Path(temp_dir) / f"leedevkit-{version}"
        stage_root.mkdir()
        _copy_release_payload(repo_root, stage_root)
        write_manifest(stage_root)

        verification = verify_devkit(stage_root)
        if not verification.is_clean:
            raise RuntimeError(verification.report())

        with tarfile.open(tarball_path, "w:gz") as tf:
            tf.add(
                stage_root,
                arcname=stage_root.name,
                filter=lambda member: None if _should_exclude(member.name) else member,
            )

    size_kb = tarball_path.stat().st_size // 1024
    log_success(f"✅ {tarball_path} ({size_kb} KB)")
    return tarball_path


def main() -> None:
    parser = argparse.ArgumentParser(description="Build leedevkit release tarball")
    parser.add_argument(
        "--output", default="dist", help="Output directory (default: dist/)"
    )
    parser.add_argument(
        "--repo-root", default=".", help="Repository root (default: current directory)"
    )
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    output_dir = Path(args.output).resolve()

    if not (repo_root / "scripts" / "_orchestrator.py").exists():
        log_error(
            f"{repo_root} does not look like a leedevkit repo "
            f"(scripts/_orchestrator.py missing)"
        )
        sys.exit(1)

    build_release(repo_root, output_dir)


if __name__ == "__main__":
    main()
