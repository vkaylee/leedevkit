"""Tests for customer platform binary selection."""

from pathlib import Path

import pytest

from _platform import go_binary, platform_key


def test_platform_aliases() -> None:
    assert platform_key("Linux", "x86_64") == "linux-amd64"
    assert platform_key("Darwin", "arm64") == "darwin-arm64"


def test_unsupported_platform_fails_closed() -> None:
    with pytest.raises(RuntimeError, match="unsupported platform"):
        platform_key("FreeBSD", "x86_64")


def test_binary_must_be_regular_nonempty_file(tmp_path: Path) -> None:
    binary = tmp_path / "bin" / "go" / "leedevkit-go-linux-amd64"
    binary.parent.mkdir(parents=True)
    binary.write_bytes(b"go")
    assert go_binary(tmp_path, "linux-amd64") == binary
