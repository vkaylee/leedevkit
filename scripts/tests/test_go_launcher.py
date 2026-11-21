"""Tests for Go-first launcher binary discovery and routing."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def _binary(tmp_path: Path, output: str) -> Path:
    binary = tmp_path / "leedevkit-go-linux-amd64"
    binary.write_text(f"#!/bin/sh\nprintf '%s\\n' {output}\n")
    binary.chmod(0o755)
    return binary


def test_go_launcher_uses_explicit_binary(tmp_path: Path) -> None:
    binary = _binary(tmp_path, "go-version")
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


def test_go_launcher_routes_migrated_commands(tmp_path: Path) -> None:
    binary = _binary(tmp_path, "go-command")
    for command in (
        ["run", "go", "version"],
        ["doctor"],
        ["skills", "list"],
        ["dev", "up"],
    ):
        result = subprocess.run(
            [str(ROOT / "bin" / "leedevkit"), *command],
            cwd=ROOT,
            env={
                **os.environ,
                "LEEDEVKIT_CORE": "go",
                "LEEDEVKIT_GO_BINARY": str(binary),
            },
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0
        assert result.stdout.strip() == "go-command"


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
