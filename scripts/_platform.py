"""Platform mapping for customer-facing Go entrypoint binaries."""

from __future__ import annotations

import platform
from pathlib import Path


_MACHINE_ALIASES = {
    "x86_64": "amd64",
    "amd64": "amd64",
    "aarch64": "arm64",
    "arm64": "arm64",
}
_SUPPORTED_SYSTEMS = {"linux", "darwin", "windows"}


def platform_key(system: str | None = None, machine: str | None = None) -> str:
    """Return release platform key or fail before project mutation."""
    system_name = (system or platform.system()).lower()
    machine_name = (machine or platform.machine()).lower()
    arch = _MACHINE_ALIASES.get(machine_name)
    if system_name not in _SUPPORTED_SYSTEMS or arch is None:
        raise RuntimeError(f"unsupported platform: {system_name}/{machine_name}")
    return f"{system_name}-{arch}"


def go_binary(devkit_root: Path, key: str | None = None) -> Path:
    """Return selected regular Go binary path from installed devkit."""
    selected = key or platform_key()
    path = devkit_root / "bin" / "go" / f"leedevkit-go-{selected}"
    if not path.is_file() or path.is_symlink() or not path.stat().st_size:
        raise RuntimeError(f"Go core binary missing or empty: {path}")
    return path
