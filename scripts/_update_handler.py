#!/usr/bin/env python3
"""Self-update handler extracted from Orchestrator (SRP).

Handles: downloading and applying devkit release updates from GitHub Releases,
with automatic backup and rollback on failure.
"""

from __future__ import annotations

import json
import shutil
import urllib.request
import uuid
from pathlib import Path

from _logging import log_info, log_success, log_warn


def _devkit_root() -> Path:
    """Return the directory this devkit is installed in (parent of scripts/)."""
    return Path(__file__).resolve().parent.parent


def _latest_release_version() -> str:
    """Return the latest release tag (e.g. 'v0.2.0') from GitHub Releases."""
    api = "https://api.github.com/repos/vkaylee/leedevkit/releases/latest"
    req = urllib.request.Request(
        api, headers={"Accept": "application/vnd.github+json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            data = json.load(r)
    except (urllib.error.URLError, OSError, json.JSONDecodeError) as e:
        raise RuntimeError(f"Could not reach GitHub Releases: {e}") from e
    tag = data.get("tag_name")
    if not tag:
        raise RuntimeError("GitHub releases/latest returned no tag_name")
    return tag


def _download_and_extract(url: str, target_dir: Path) -> None:
    """Download a release tarball and extract into target_dir."""
    import shutil as _shutil
    import tarfile
    import tempfile

    tmp = Path(tempfile.mkdtemp())
    try:
        tarball = tmp / "release.tar.gz"
        urllib.request.urlretrieve(url, tarball)
        extract_dir = tmp / "extracted"
        extract_dir.mkdir()
        with tarfile.open(tarball, "r:gz") as tf:
            tf.extractall(extract_dir)  # noqa: S202
        # The tarball may contain a single root dir (leedevkit-vX.Y.Z/)
        contents = list(extract_dir.iterdir())
        if len(contents) == 1 and contents[0].is_dir():
            source = contents[0]
        else:
            source = extract_dir
        if target_dir.exists():
            _shutil.rmtree(target_dir)
        target_dir.parent.mkdir(parents=True, exist_ok=True)
        _shutil.move(str(source), str(target_dir))
    finally:
        _shutil.rmtree(tmp, ignore_errors=True)


def handle_update(target: str | None = None) -> None:
    """Download a release tarball and overlay it onto this devkit install.

    Args:
        target: Specific version tag (e.g. 'v0.2.0'), or None for latest.
    """
    root = _devkit_root()
    current = (root / "VERSION").read_text().strip()  # e.g. "0.1.0"

    if target is None:
        target = _latest_release_version()  # e.g. "v0.2.0"

    ver = target.lstrip("v")  # tarball version, no "v"
    if ver == current:
        log_info(f"Already on latest ({current}).")
        return

    log_info(f"Updating {current} → {target}")

    # Backup current install to <root>.bak before overwriting.
    backup = root.with_name(root.name + ".bak")
    if backup.exists():
        shutil.rmtree(backup)
    shutil.move(str(root), str(backup))
    log_info(f"Backed up current install to {backup.name}/")

    # Download into a temp dir, then move the extracted tree onto root.
    tmp_extract = root.parent / f".leedevkit-update-{uuid.uuid4().hex[:8]}"
    url = (
        f"https://github.com/vkaylee/leedevkit/releases/download/"
        f"{target}/leedevkit-{ver}.tar.gz"
    )
    try:
        log_info(f"Downloading {url} ...")
        _download_and_extract(url, tmp_extract)
        if root.exists():
            shutil.rmtree(root)
        shutil.move(str(tmp_extract), str(root))
    except Exception:
        # Roll back on any failure — broad catch is intentional here
        # to guarantee rollback regardless of the error type.
        if root.exists():
            shutil.rmtree(root)
        shutil.move(str(backup), str(root))
        log_warn("Update failed; rolled back to previous version.")
        raise

    new_ver = (root / "VERSION").read_text().strip()
    log_success(f"Updated leedevkit {current} → {new_ver}")
    log_info(f"Previous version kept at {backup.name}/ (safe to remove).")
