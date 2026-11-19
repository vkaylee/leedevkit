"""Tests for Go core platform target naming and matrix orchestration."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from _go_core_build import target_binary_name  # noqa: E402
from _go_core_matrix import build_matrix  # noqa: E402


def test_target_binary_name_adds_windows_suffix() -> None:
    assert target_binary_name("linux", "amd64") == "leedevkit-go-linux-amd64"
    assert target_binary_name("windows", "amd64") == "leedevkit-go-windows-amd64.exe"


def test_build_matrix_builds_each_default_target(tmp_path: Path, monkeypatch) -> None:
    calls = []

    def fake_build(repo_root, output, version, **kwargs):
        calls.append((repo_root, output, version, kwargs))
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_bytes(b"binary")
        return output

    monkeypatch.setattr("_go_core_matrix.build_go_core", fake_build)
    results = build_matrix(tmp_path, tmp_path / "dist", "0.10.0")

    assert len(results) == 5
    assert [
        {key: value for key, value in call[3].items() if key != "compose_cmd"}
        for call in calls
    ] == [
        {"goos": "linux", "goarch": "amd64"},
        {"goos": "linux", "goarch": "arm64"},
        {"goos": "darwin", "goarch": "amd64"},
        {"goos": "darwin", "goarch": "arm64"},
        {"goos": "windows", "goarch": "amd64"},
    ]
