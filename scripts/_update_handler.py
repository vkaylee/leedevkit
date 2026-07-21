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

from _download import download_and_extract_tarball
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
    url = f"https://github.com/vkaylee/leedevkit/archive/refs/tags/{target}.tar.gz"
    try:
        log_info(f"Downloading {url} ...")
        download_and_extract_tarball(url, tmp_extract)
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

    # Update version pin in leedevkit.toml (project root)
    project_root = root.parent  # .leedevkit/ is inside project root
    config_toml = project_root / "leedevkit.toml"
    if config_toml.exists():
        try:
            content = config_toml.read_text()
            if "version =" in content and "[devkit]" in content:
                import re

                content = re.sub(
                    r'(\[devkit\].*?version\s*=\s*)"[^"]*"',
                    f'\\1"{new_ver}"',
                    content,
                    flags=re.DOTALL,
                )
                config_toml.write_text(content)
                log_success(f"Updated leedevkit.toml: version = \"{new_ver}\"")
        except Exception as e:
            log_warn(f"Could not update leedevkit.toml: {e}")

    # Sync rules and create symlinks (lightweight, no network/subprocess)
    log_info("Syncing rules and creating symlinks...")
    try:
        from _init_handler import InitHandler
        from _orchestrator import Orchestrator

        # Create a minimal orchestrator instance for InitHandler
        orch = Orchestrator.__new__(Orchestrator)
        orch._devkit_root = root
        init_handler = InitHandler(orch)
        init_handler.handle_post_update_sync()
        log_success("Post-update sync complete")
    except Exception as e:
        log_warn(f"Post-update sync failed: {e}")
        log_info("You may need to run './leedevkit init' manually")
