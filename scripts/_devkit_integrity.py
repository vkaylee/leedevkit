#!/usr/bin/env python3
"""Devkit integrity verification — checksum + tamper detection.

Usage:
    python _devkit_integrity.py verify     # Check installed devkit integrity
    python _devkit_integrity.py checksum   # Generate checksums for current version
"""

from __future__ import annotations

import hashlib
import json
import os
import sys
from pathlib import Path

# ── Bootstrap ──────────────────────────────────────────────────────────────

try:
    from _devkit_config import _find_devkit_root
except ImportError:

    def _find_devkit_root() -> Path:
        env = os.environ.get("DEVKIT_HOME")
        if env:
            return Path(env)
        home = Path.home() / ".leedevkit" / "current"
        if home.exists():
            return home
        raise FileNotFoundError("Cannot locate devkit")


MANIFEST_FILE = "devkit.manifest.json"
RUNTIME_DIRS = {".venv", "skills.d", "__pycache__"}
RUNTIME_FILES = {"dev-state.json", ".DS_Store"}


def _read_manifest(devkit_root: Path) -> dict | None:
    path = devkit_root / MANIFEST_FILE
    if not path.exists():
        return None
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError):
        return None


def _compute_file_hash(path: Path) -> str:
    """SHA256 of a single file."""
    sha = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            sha.update(chunk)
    return sha.hexdigest()


def _walk_files(root: Path) -> list[Path]:
    """List immutable files under root, excluding generated runtime state."""
    files: list[Path] = []
    for entry in root.rglob("*"):
        if not entry.is_file() or entry.name in {MANIFEST_FILE, *RUNTIME_FILES}:
            continue
        relative_parts = entry.relative_to(root).parts
        if not any(part in RUNTIME_DIRS for part in relative_parts):
            files.append(entry)
    return sorted(files)


# ── Generate ───────────────────────────────────────────────────────────────


def generate_manifest(devkit_root: Path | None = None) -> dict:
    """Compute SHA256 hashes for all files in the current devkit."""
    if devkit_root is None:
        devkit_root = _find_devkit_root()

    files = _walk_files(devkit_root)
    hashes: dict[str, str] = {}
    for f in files:
        rel = str(f.relative_to(devkit_root))
        hashes[rel] = _compute_file_hash(f)

    version = "unknown"
    version_file = devkit_root / "VERSION"
    if version_file.exists():
        version = version_file.read_text().strip()

    return {
        "version": version,
        "generated_at": __import__("datetime")
        .datetime.now(__import__("datetime").timezone.utc)
        .isoformat()
        .replace("+00:00", "Z"),
        "algorithm": "sha256",
        "file_count": len(hashes),
        "files": hashes,
    }


def write_manifest(devkit_root: Path | None = None) -> Path:
    """Generate and persist manifest to devkit root."""
    if devkit_root is None:
        devkit_root = _find_devkit_root()
    manifest = generate_manifest(devkit_root)
    path = devkit_root / MANIFEST_FILE
    with open(path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)
    return path


# ── Verify ─────────────────────────────────────────────────────────────────


class VerificationResult:
    def __init__(self) -> None:
        self.ok: list[str] = []
        self.modified: list[tuple[str, str, str]] = []  # (path, expected, actual)
        self.missing: list[str] = []
        self.extra: list[str] = []  # files not in manifest
        self.no_manifest: bool = False
        self.invalid_manifest: list[str] = []

    @property
    def is_clean(self) -> bool:
        return not (
            self.modified
            or self.missing
            or self.extra
            or self.no_manifest
            or self.invalid_manifest
        )

    def report(self) -> str:
        lines: list[str] = []
        if self.no_manifest:
            lines.append("❌ No manifest found — devkit integrity unknown.")
            lines.append("   Run: python _devkit_integrity.py checksum")
            return "\n".join(lines)

        if self.is_clean:
            lines.append("✅ Devkit integrity verified — all files match manifest.")
            lines.append(f"   {len(self.ok)} files OK.")
            return "\n".join(lines)

        lines.append("❌ Devkit integrity FAILED:")
        if self.invalid_manifest:
            lines.append("\n  Invalid manifest:")
            for issue in self.invalid_manifest:
                lines.append(f"    {issue}")
        if self.modified:
            lines.append(f"\n  Modified ({len(self.modified)}):")
            for path, expected, actual in self.modified:
                lines.append(f"    {path}")
                lines.append(f"      expected: {expected[:16]}...")
                lines.append(f"      actual:   {actual[:16]}...")
        if self.missing:
            lines.append(f"\n  Missing ({len(self.missing)}):")
            for path in self.missing:
                lines.append(f"    {path}")
        if self.extra:
            lines.append(f"\n  Unknown files ({len(self.extra)}):")
            for path in self.extra:
                lines.append(f"    {path}")
        lines.append(f"\n  {len(self.ok)} files OK.")
        lines.append("  Reinstall with: ./devkit upgrade --force")
        return "\n".join(lines)


def verify_devkit(devkit_root: Path | None = None) -> VerificationResult:
    """Verify all files match the published manifest."""
    if devkit_root is None:
        devkit_root = _find_devkit_root()

    result = VerificationResult()
    manifest = _read_manifest(devkit_root)

    if manifest is None:
        result.no_manifest = True
        return result
    if not isinstance(manifest, dict):
        result.invalid_manifest.append("manifest root must be an object")
        return result

    files = manifest.get("files")
    if manifest.get("algorithm") != "sha256":
        result.invalid_manifest.append("algorithm must be sha256")
    if not isinstance(files, dict):
        result.invalid_manifest.append("files must be an object")
    else:
        if manifest.get("file_count") != len(files):
            result.invalid_manifest.append("file_count does not match files")

    version_file = devkit_root / "VERSION"
    actual_version = (
        version_file.read_text().strip() if version_file.exists() else "unknown"
    )
    if manifest.get("version") != actual_version:
        result.invalid_manifest.append("version does not match VERSION")

    if result.invalid_manifest:
        return result

    expected = files
    actual_files = {
        str(f.relative_to(devkit_root)): f for f in _walk_files(devkit_root)
    }

    for rel_path, expected_hash in expected.items():
        if rel_path not in actual_files:
            result.missing.append(rel_path)
            continue
        actual_hash = _compute_file_hash(actual_files[rel_path])
        if actual_hash != expected_hash:
            result.modified.append((rel_path, expected_hash, actual_hash))
        else:
            result.ok.append(rel_path)
        del actual_files[rel_path]

    result.extra = sorted(actual_files.keys())

    return result


# ── CLI ────────────────────────────────────────────────────────────────────

if __name__ == "__main__":  # pragma: no cover
    action = sys.argv[1] if len(sys.argv) > 1 else "verify"
    devkit_root = _find_devkit_root()

    if action == "verify":
        result = verify_devkit(devkit_root)
        print(result.report())
        sys.exit(0 if result.is_clean else 1)
    elif action == "checksum":
        path = write_manifest(devkit_root)
        print(f"✅ Manifest written: {path}")
    elif action == "doctor":
        print(f"Devkit root: {devkit_root}")
        result = verify_devkit(devkit_root)
        print(result.report())
        print(f"\nVersion: {_read_manifest(devkit_root) or {}}")
        sys.exit(0 if result.is_clean else 1)
