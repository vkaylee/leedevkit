"""Tests for opt-in Go launcher binary discovery."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_go_launcher_uses_platform_binary(tmp_path: Path) -> None:
    binary = tmp_path / "leedevkit-go-linux-amd64"
    binary.write_text("#!/bin/sh\nprintf '%s\\n' go-version\n")
    binary.chmod(0o755)

    result = subprocess.run(
        [str(ROOT / "bin" / "leedevkit"), "version"],
        cwd=ROOT,
        env={**os.environ, "LEEDEVKIT_CORE": "go", "LEEDEVKIT_GO_BINARY": str(binary)},
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0
    assert result.stdout.strip() == "go-version"


def test_go_launcher_routes_run_to_go_binary(tmp_path: Path) -> None:
    binary = tmp_path / "leedevkit-go-linux-amd64"
    binary.write_text("#!/bin/sh\nprintf '%s\\n' go-run\n")
    binary.chmod(0o755)

    result = subprocess.run(
        [str(ROOT / "bin" / "leedevkit"), "run", "go", "version"],
        cwd=ROOT,
        env={**os.environ, "LEEDEVKIT_CORE": "go", "LEEDEVKIT_GO_BINARY": str(binary)},
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 0
    assert result.stdout.strip() == "go-run"


def test_go_launcher_missing_binary_fails_closed() -> None:
    result = subprocess.run(
        [str(ROOT / "bin" / "leedevkit"), "version"],
        cwd=ROOT,
        env={
            **os.environ,
            "LEEDEVKIT_CORE": "go",
            "LEEDEVKIT_GO_BINARY": "/tmp/no-such-leedevkit-go",
        },
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 2
    assert "Go core enabled but leedevkit-go binary missing" in result.stderr
