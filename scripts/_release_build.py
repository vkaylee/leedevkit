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

from _go_core_build import DEFAULT_TARGETS, target_binary_name
from _devkit_integrity import verify_devkit, write_manifest
from _runtime import enforce_project_venv


def log_info(message: str) -> None:
    print(f"ℹ️ {message}")


def log_success(message: str) -> None:
    print(f"✅ {message}")


def log_error(message: str) -> None:
    print(f"❌ {message}", file=sys.stderr)
from _sbom import write_sbom


# ── Items to include in the release tarball ──
INCLUDE_DIRS = ["scripts", "templates", "bin", ".agent", "container", "cmd", "internal"]
INCLUDE_FILES = ["VERSION", "go.mod"]
REQUIRED_FILES = [
    "cmd/debug-mcp/main.go",
    "container/debug/Dockerfile",
    "VERSION",
    "bin/leedevkit",
    "scripts/requirements.lock",
    "scripts/requirements-runtime.lock",
    "scripts/_go_core_build.py",
    "scripts/_go_core_matrix.py",
    "scripts/_platform.py",
    "go.mod",
    "cmd/leedevkit/main.go",
    "container/go/docker-compose.core.yml",
]
GO_CORE_BINARY_DIR = "bin/go"
DEBUG_MCP_BINARY_DIR = "bin/debug-mcp"
GO_CORE_TARGETS = DEFAULT_TARGETS


def _copy_debug_binaries(source_dir: Path, stage_root: Path) -> None:
    destination = stage_root / DEBUG_MCP_BINARY_DIR
    destination.mkdir(parents=True, exist_ok=True)
    from _go_core_build import debug_binary_name

    for goos, goarch in GO_CORE_TARGETS:
        name = debug_binary_name(goos, goarch)
        source = source_dir / name
        if not source.is_file():
            raise FileNotFoundError(f"Debug MCP binary missing: {source}")
        shutil.copy2(source, destination / name)
        (destination / name).chmod((destination / name).stat().st_mode | 0o111)


def _copy_go_core_binaries(source_dir: Path, stage_root: Path) -> None:
    """Copy complete platform Go core matrix into release staging."""
    destination = stage_root / GO_CORE_BINARY_DIR
    destination.mkdir(parents=True, exist_ok=True)
    for goos, goarch in GO_CORE_TARGETS:
        name = target_binary_name(goos, goarch)
        source = source_dir / name
        if not source.is_file():
            raise FileNotFoundError(f"Go core binary missing: {source}")
        shutil.copy2(source, destination / name)
        (destination / name).chmod((destination / name).stat().st_mode | 0o111)


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


def build_release(
    repo_root: Path,
    output_dir: Path,
    go_core_dir: Path | None = None,
    debug_mcp_dir: Path | None = None,
) -> Path:
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
        if go_core_dir is not None:
            _copy_go_core_binaries(go_core_dir, stage_root)
        elif (stage_root / GO_CORE_BINARY_DIR).exists():
            shutil.rmtree(stage_root / GO_CORE_BINARY_DIR)
        if debug_mcp_dir is not None:
            _copy_debug_binaries(debug_mcp_dir, stage_root)
        elif (stage_root / DEBUG_MCP_BINARY_DIR).exists():
            shutil.rmtree(stage_root / DEBUG_MCP_BINARY_DIR)
        write_sbom(
            repo_root / "scripts" / "requirements.lock",
            version,
            stage_root / "sbom.cdx.json",
        )
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
    parser.add_argument(
        "--go-core-dir",
        help="Directory containing prebuilt platform Go core binaries",
    )
    parser.add_argument(
        "--build-go-core",
        action="store_true",
        help="Build platform Go core binaries through Compose before packaging",
    )
    parser.add_argument(
        "--debug-mcp-dir",
        help="Directory containing prebuilt platform Debug MCP binaries",
    )
    parser.add_argument(
        "--build-debug-mcp",
        action="store_true",
        help="Build platform Debug MCP binaries through Compose before packaging",
    )
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    output_dir = Path(args.output).resolve()

    if not (repo_root / "cmd" / "leedevkit" / "main.go").exists():
        log_error(
            f"{repo_root} does not look like a Go leedevkit repo "
            f"(cmd/leedevkit/main.go missing)"
        )
        sys.exit(1)

    go_core_dir = Path(args.go_core_dir).resolve() if args.go_core_dir else None
    debug_mcp_dir = Path(args.debug_mcp_dir).resolve() if args.debug_mcp_dir else None
    if args.build_go_core and go_core_dir is not None:
        raise ValueError("use either --go-core-dir or --build-go-core")
    if args.build_debug_mcp and debug_mcp_dir is not None:
        raise ValueError("use either --debug-mcp-dir or --build-debug-mcp")
    version = (repo_root / "VERSION").read_text().strip()
    if args.build_go_core or args.build_debug_mcp:
        from _go_core_matrix import build_debug_matrix, build_matrix

        if args.build_go_core:
            go_core_dir = output_dir / ".go-core"
            build_matrix(repo_root, go_core_dir, version)
        if args.build_debug_mcp:
            debug_mcp_dir = output_dir / ".debug-mcp"
            build_debug_matrix(repo_root, debug_mcp_dir, version)
    build_release(repo_root, output_dir, go_core_dir, debug_mcp_dir)


if __name__ == "__main__":
    enforce_project_venv()
    main()
