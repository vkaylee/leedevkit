#!/usr/bin/env python3
"""Build a release tarball for leedevkit.

Usage:
    python3 scripts/_release_build.py [--output dist/]

Produces:
    dist/leedevkit-v{VERSION}.tar.gz

The tarball contains the immutable devkit artifacts that get installed
into each project's .leedevkit/ directory via `leedevkit init`.
"""

from __future__ import annotations

import argparse
import sys
import tarfile
from pathlib import Path


# ── Items to include in the release tarball ──
INCLUDE_DIRS = ["scripts", "templates", "bin", ".agent", "container"]
INCLUDE_FILES = ["VERSION", "devkit.manifest.json"]

# ── Patterns to exclude — always filtered ──
EXCLUDE_PATTERNS = {"__pycache__", ".pyc", ".pyo", ".git", ".DS_Store"}


def _should_exclude(name: str) -> bool:
    """Check if a file/dir should be excluded from the tarball."""
    return any(part in EXCLUDE_PATTERNS for part in Path(name).parts)


def build_release(repo_root: Path, output_dir: Path) -> Path:
    """Build the release tarball.

    Args:
        repo_root: Root of the leedevkit repository.
        output_dir: Directory where the tarball will be written.

    Returns:
        Path to the created tarball.
    """
    version_file = repo_root / "VERSION"
    if not version_file.exists():
        raise FileNotFoundError(f"VERSION file not found in {repo_root}")
    version = version_file.read_text().strip()

    output_dir.mkdir(parents=True, exist_ok=True)
    tarball_name = f"leedevkit-{version}.tar.gz"
    tarball_path = output_dir / tarball_name

    print(f"Building {tarball_path} ...")

    with tarfile.open(tarball_path, "w:gz") as tf:
        # Add directories
        for dirname in INCLUDE_DIRS:
            dir_path = repo_root / dirname
            if dir_path.exists():
                print(f"  + {dirname}/")
                tf.add(dir_path, arcname=f"leedevkit-{version}/{dirname}",
                       filter=lambda m: None if _should_exclude(m.name) else m)

        # Add individual files
        for fname in INCLUDE_FILES:
            fpath = repo_root / fname
            if fpath.exists():
                print(f"  + {fname}")
                tf.add(fpath, arcname=f"leedevkit-{version}/{fname}")

    size_kb = tarball_path.stat().st_size // 1024
    print(f"\n✅ {tarball_path} ({size_kb} KB)")
    return tarball_path


def main() -> None:
    parser = argparse.ArgumentParser(description="Build leedevkit release tarball")
    parser.add_argument("--output", default="dist", help="Output directory (default: dist/)")
    parser.add_argument("--repo-root", default=".",
                        help="Repository root (default: current directory)")
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    output_dir = Path(args.output).resolve()

    if not (repo_root / "scripts" / "_orchestrator.py").exists():
        print(f"Error: {repo_root} does not look like a leedevkit repo "
              f"(scripts/_orchestrator.py missing)", file=sys.stderr)
        sys.exit(1)

    build_release(repo_root, output_dir)


if __name__ == "__main__":
    main()
