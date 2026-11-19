"""Tests for Compose-backed Go core builds."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from _go_core_build import build_command, build_go_core  # noqa: E402


def test_build_command_uses_compose_and_safe_build_argv(tmp_path: Path) -> None:
    command = build_command(
        compose_cmd=("docker", "compose"),
        compose_file=tmp_path / "docker-compose.core.yml",
        output=tmp_path / "leedevkit-go",
        version="0.10.0",
        goos="linux",
        goarch="amd64",
    )

    assert command == [
        "docker",
        "compose",
        "-f",
        str(tmp_path / "docker-compose.core.yml"),
        "--profile",
        "go-core",
        "run",
        "--rm",
        "--build",
        "-e",
        "GOOS=linux",
        "-e",
        "GOARCH=amd64",
        "leedevkit-go",
        "build",
        "-buildvcs=false",
        "-trimpath",
        "-ldflags",
        "-X=main.version=0.10.0",
        "-o",
        "/workspace/.leedevkit-go-build/leedevkit-go",
        "./cmd/leedevkit",
    ]


def test_build_go_core_copies_binary_and_cleans_staging(
    tmp_path: Path, monkeypatch
) -> None:
    (tmp_path / "go.mod").write_text("module example\n")
    (tmp_path / "cmd" / "leedevkit").mkdir(parents=True)
    (tmp_path / "cmd" / "leedevkit" / "main.go").write_text("package main\n")
    compose_file = tmp_path / "container" / "go" / "docker-compose.core.yml"
    compose_file.parent.mkdir(parents=True)
    compose_file.write_text("services: {}\n")
    output = tmp_path / "dist" / "leedevkit-go"

    def fake_run(command, **kwargs):
        built = tmp_path / ".leedevkit-go-build" / "leedevkit-go"
        built.write_bytes(b"binary")
        return type("Result", (), {"returncode": 0, "stdout": "", "stderr": ""})()

    monkeypatch.setattr("_go_core_build.subprocess.run", fake_run)
    result = build_go_core(tmp_path, output, "0.10.0")

    assert result == output.resolve()
    assert output.read_bytes() == b"binary"
    assert output.stat().st_mode & 0o111
    assert not (tmp_path / ".leedevkit-go-build").exists()
