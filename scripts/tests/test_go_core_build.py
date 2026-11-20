"""Tests for Compose-backed Go core builds."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from _go_core_build import build_command, build_go_core, target_binary_name  # noqa: E402
from _go_core_matrix import build_matrix  # noqa: E402


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


def test_target_binary_name_adds_windows_suffix() -> None:
    assert target_binary_name("windows", "amd64") == "leedevkit-go-windows-amd64.exe"
    assert target_binary_name("linux", "arm64") == "leedevkit-go-linux-arm64"


def test_build_go_core_rejects_missing_compose_file(tmp_path: Path) -> None:
    try:
        build_go_core(tmp_path, tmp_path / "out", "1.0.0")
    except FileNotFoundError as exc:
        assert "Compose file missing" in str(exc)
    else:
        raise AssertionError("missing compose file accepted")


def test_build_go_core_rejects_missing_source(tmp_path: Path) -> None:
    compose_file = tmp_path / "container" / "go" / "docker-compose.core.yml"
    compose_file.parent.mkdir(parents=True)
    compose_file.write_text("services: {}\n")
    try:
        build_go_core(tmp_path, tmp_path / "out", "1.0.0")
    except FileNotFoundError as exc:
        assert "source files missing" in str(exc)
    else:
        raise AssertionError("missing source accepted")


def test_build_go_core_rejects_existing_staging(tmp_path: Path) -> None:
    (tmp_path / "go.mod").write_text("module example\n")
    (tmp_path / "cmd" / "leedevkit").mkdir(parents=True)
    (tmp_path / "cmd" / "leedevkit" / "main.go").write_text("package main\n")
    compose_file = tmp_path / "container" / "go" / "docker-compose.core.yml"
    compose_file.parent.mkdir(parents=True)
    compose_file.write_text("services: {}\n")
    (tmp_path / ".leedevkit-go-build").mkdir()
    try:
        build_go_core(tmp_path, tmp_path / "out", "1.0.0")
    except FileExistsError:
        pass
    else:
        raise AssertionError("existing staging accepted")


def test_build_go_core_rejects_failed_compose(tmp_path: Path, monkeypatch) -> None:
    (tmp_path / "go.mod").write_text("module example\n")
    (tmp_path / "cmd" / "leedevkit").mkdir(parents=True)
    (tmp_path / "cmd" / "leedevkit" / "main.go").write_text("package main\n")
    compose_file = tmp_path / "container" / "go" / "docker-compose.core.yml"
    compose_file.parent.mkdir(parents=True)
    compose_file.write_text("services: {}\n")
    monkeypatch.setattr(
        "_go_core_build.subprocess.run",
        lambda *args, **kwargs: type(
            "Result", (), {"returncode": 7, "stdout": "out", "stderr": "err"}
        )(),
    )
    try:
        build_go_core(tmp_path, tmp_path / "out", "1.0.0")
    except RuntimeError as exc:
        assert "failed (7)" in str(exc)
    else:
        raise AssertionError("failed build accepted")
    assert not (tmp_path / ".leedevkit-go-build").exists()


def test_build_go_core_rejects_missing_output_binary(
    tmp_path: Path, monkeypatch
) -> None:
    (tmp_path / "go.mod").write_text("module example\n")
    (tmp_path / "cmd" / "leedevkit").mkdir(parents=True)
    (tmp_path / "cmd" / "leedevkit" / "main.go").write_text("package main\n")
    compose_file = tmp_path / "container" / "go" / "docker-compose.core.yml"
    compose_file.parent.mkdir(parents=True)
    compose_file.write_text("services: {}\n")
    monkeypatch.setattr(
        "_go_core_build.subprocess.run",
        lambda *args, **kwargs: type(
            "Result", (), {"returncode": 0, "stdout": "", "stderr": ""}
        )(),
    )
    try:
        build_go_core(tmp_path, tmp_path / "out", "1.0.0")
    except RuntimeError as exc:
        assert "produced no binary" in str(exc)
    else:
        raise AssertionError("missing output accepted")


def test_build_matrix_builds_all_default_targets(tmp_path: Path, monkeypatch) -> None:
    calls = []
    monkeypatch.setattr(
        "_go_core_matrix.build_go_core",
        lambda repo, output, version, **kwargs: (
            calls.append((output, kwargs)) or output
        ),
    )
    results = build_matrix(tmp_path, tmp_path / "dist", "1.0.0")
    assert len(results) == 5
    assert len(calls) == 5
    assert calls[0][1]["goos"] == "linux"
    assert calls[-1][1]["goarch"] == "amd64"


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
