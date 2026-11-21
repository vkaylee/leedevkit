"""Shared test isolation guards for filesystem-mutating tests."""

from __future__ import annotations

import os
import tempfile
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
                if (
                    child_relative in RUNTIME_ARTIFACTS
                    or "__pycache__" in child.relative_to(REPO_ROOT).parts
                    or child.suffix in {".pyc", ".pyo"}
                ):
                    continue
                if child.is_symlink():
                    snapshot[child_relative] = f"symlink:{child.readlink()}"
                elif child.is_file():
                    snapshot[child_relative] = child.read_bytes()
    return snapshot


@pytest.fixture(autouse=True)
def provide_container_cli_paths(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> Iterator[None]:
    """Point unit tests at harmless local stubs, never host runtimes."""
    bin_dir = Path(tempfile.mkdtemp(prefix="leedevkit-container-bin-"))
    script = '#!/bin/sh\nif [ "$1" = "ps" ]; then exit 0; fi\nprintf \'%s version 0-test\\n\' "$(basename "$0")"\n'
    for command in ("docker", "podman", "podman-compose"):
        executable = bin_dir / command
        executable.write_text(script)
        executable.chmod(0o755)
    monkeypatch.setenv("PATH", f"{bin_dir}:{os.environ.get('PATH', '')}")
    yield


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
