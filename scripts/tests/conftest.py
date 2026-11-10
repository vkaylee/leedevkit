"""Shared test isolation guards for filesystem-mutating tests."""

from __future__ import annotations

from pathlib import Path
from typing import Iterator

import pytest


REPO_ROOT = Path(__file__).resolve().parents[2]
PROTECTED_PATHS = (
    "CLAUDE.md",
    "VERSION",
    "leedevkit.toml",
    ".gitignore",
    ".mcp.json",
    ".agent",
    ".claude",
    ".codex",
    ".cursor",
    ".gemini",
    ".omp",
    ".pi",
)
RUNTIME_ARTIFACTS = {".claude/scheduled_tasks.lock"}


def _snapshot(paths: tuple[str, ...]) -> dict[str, bytes | str]:
    """Capture protected files and symlink targets without following links."""
    snapshot: dict[str, bytes | str] = {}
    for relative in paths:
        if relative in RUNTIME_ARTIFACTS:
            continue
        path = REPO_ROOT / relative
        if path.is_symlink():
            snapshot[relative] = f"symlink:{path.readlink()}"
        elif path.is_file():
            snapshot[relative] = path.read_bytes()
        elif path.is_dir():
            for child in sorted(path.rglob("*")):
                child_relative = child.relative_to(REPO_ROOT).as_posix()
                if child_relative in RUNTIME_ARTIFACTS:
                    continue
                if child.is_symlink():
                    snapshot[child_relative] = f"symlink:{child.readlink()}"
                elif child.is_file():
                    snapshot[child_relative] = child.read_bytes()
    return snapshot


@pytest.fixture(autouse=True)
def protect_checkout_and_reset_runtime_cache() -> Iterator[None]:
    """Prevent tests from persisting project mutations in the source checkout."""
    before = _snapshot(PROTECTED_PATHS)
    try:
        yield
    finally:
        after = _snapshot(PROTECTED_PATHS)
        try:
            assert after == before, "test modified protected checkout files"
        finally:
            try:
                from _devkit_config import reset_devkit_root_cache

                reset_devkit_root_cache()
            except ImportError:
                pass
