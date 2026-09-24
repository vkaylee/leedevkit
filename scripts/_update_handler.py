#!/usr/bin/env python3
"""Self-update handler extracted from Orchestrator (SRP).

Handles: downloading and applying devkit release updates from GitHub Releases,
with automatic backup and rollback on failure.
"""

from __future__ import annotations

import shutil
import urllib.request  # noqa: F401
import uuid
from pathlib import Path

from _download import download_and_extract_tarball, latest_release_version
from _logging import log_info, log_success, log_warn


DOWNLOAD_TIMEOUT = 120


def _devkit_root() -> Path:
    """Return the directory this devkit is installed in (parent of scripts/)."""
    return Path(__file__).resolve().parent.parent


def _remove_path(path: Path) -> None:
    if path.is_symlink() or path.is_file():
        path.unlink()
    elif path.exists():
        shutil.rmtree(path)


def _restore_backup(root: Path, backup: Path) -> None:
    if not backup.exists():
        return
    backup_skills = backup / "skills.d"
    restored_skills = root / "skills.d"
    if root.exists() and not backup_skills.exists() and not backup_skills.is_symlink():
        if restored_skills.exists() or restored_skills.is_symlink():
            shutil.move(str(restored_skills), str(backup_skills))
    _remove_path(root)
    if backup.exists():
        shutil.move(str(backup), str(root))


def _latest_release_version() -> str:
    """Return latest release tag with bounded network operation."""
    return latest_release_version(
        "https://api.github.com/repos/vkaylee/leedevkit/releases/latest",
        timeout=DOWNLOAD_TIMEOUT,
    )


def handle_update(target: str | None = None) -> None:
    """Download and atomically apply release, including project-side updates."""
    root = _devkit_root()
    current = (root / "VERSION").read_text().strip()
    if target is None:
        target = _latest_release_version()
    ver = target.lstrip("v")
    if ver == current:
        log_info(f"Already on latest ({current}).")
        return

    log_info(f"Updating {current} → {target}")
    backup = root.with_name(root.name + ".bak")
    tmp_extract = root.parent / f".leedevkit-update-{uuid.uuid4().hex[:8]}"
    config_toml = root.parent / "leedevkit.toml"
    original_config = config_toml.read_bytes() if config_toml.exists() else None
    url = f"https://github.com/vkaylee/leedevkit/archive/refs/tags/{target}.tar.gz"
    try:
        download_and_extract_tarball(url, tmp_extract)
        version_file = tmp_extract / "VERSION"
        if not version_file.is_file():
            raise RuntimeError("Downloaded devkit is missing VERSION")
        new_ver = version_file.read_text().strip()
        if new_ver != ver:
            raise RuntimeError(
                f"Downloaded devkit version {new_ver!r} does not match requested {ver!r}"
            )
        if backup.exists() or backup.is_symlink():
            _remove_path(backup)
        shutil.move(str(root), str(backup))
        log_info(f"Backed up current install to {backup.name}/")
        shutil.move(str(tmp_extract), str(root))
        for preserved_name in ("skills.d", ".venv"):
            preserved = backup / preserved_name
            if preserved.exists() or preserved.is_symlink():
                restored = root / preserved_name
                _remove_path(restored)
                shutil.move(str(preserved), str(restored))

        if config_toml.exists():
            import re

            content = config_toml.read_text()
            if "version =" in content and "[devkit]" in content:
                updated, count = re.subn(
                    r'(\[devkit\].*?version\s*=\s*)"[^"]*"',
                    f'\\1"{new_ver}"',
                    content,
                    count=1,
                    flags=re.DOTALL,
                )
                if count:
                    config_toml.write_text(updated)
                    log_success(f'Updated leedevkit.toml: version = "{new_ver}"')

        log_info("Syncing rules and creating symlinks...")
        from _init_handler import InitHandler
        from _orchestrator import Orchestrator

        orch = Orchestrator.__new__(Orchestrator)
        setattr(orch, "_devkit_root", root)
        InitHandler(orch).handle_post_update_sync()
        log_success("Post-update sync complete")
    except Exception:
        if original_config is None:
            _remove_path(config_toml)
        else:
            config_toml.write_bytes(original_config)
        _restore_backup(root, backup)
        log_warn("Update failed; rolled back to previous version.")
        raise
    finally:
        if tmp_extract.exists() or tmp_extract.is_symlink():
            _remove_path(tmp_extract)

    log_success(f"Updated leedevkit {current} → {new_ver}")
    log_info(f"Previous version kept at {backup.name}/ (safe to remove).")
