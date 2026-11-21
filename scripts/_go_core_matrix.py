"""Build supported Go core platform binaries through Compose."""

from __future__ import annotations

import argparse
from pathlib import Path

from _go_core_build import (
    DEFAULT_TARGETS,
    build_debug_mcp,
    build_go_core,
    debug_binary_name,
    target_binary_name,
)


def build_matrix(
    repo_root: Path,
    output_dir: Path,
    version: str,
    *,
    compose_cmd: tuple[str, ...] = ("docker", "compose"),
) -> list[Path]:
    """Build default release targets into output_dir."""
    results = []
    for goos, goarch in DEFAULT_TARGETS:
        results.append(
            build_go_core(
                repo_root,
                output_dir / target_binary_name(goos, goarch),
                version,
                compose_cmd=compose_cmd,
                goos=goos,
                goarch=goarch,
            )
        )
    return results


def build_debug_matrix(
    repo_root: Path,
    output_dir: Path,
    version: str,
    *,
    compose_cmd: tuple[str, ...] = ("docker", "compose"),
) -> list[Path]:
    """Build standalone Debug MCP binaries for release targets."""
    return [
        build_debug_mcp(
            repo_root,
            output_dir / debug_binary_name(goos, goarch),
            version,
            compose_cmd=compose_cmd,
            goos=goos,
            goarch=goarch,
        )
        for goos, goarch in DEFAULT_TARGETS
    ]


def main() -> None:
    parser = argparse.ArgumentParser(description="Build Go core platform binaries")
    parser.add_argument("--repo-root", default=".")
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--version")
    args = parser.parse_args()
    repo_root = Path(args.repo_root).resolve()
    version = args.version or (repo_root / "VERSION").read_text().strip()
    build_matrix(repo_root, Path(args.output_dir), version)


if __name__ == "__main__":
    main()
