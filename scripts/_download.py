#!/usr/bin/env python3
"""Shared tarball download & extract utility.

Canonical single source for downloading and extracting release tarballs.
Used by both init (project bootstrap) and update (self-update) workflows
to eliminate the DRY violation where the same ~25-line function lived in
both _init_handler.py and _update_handler.py.
"""

from __future__ import annotations

import shutil
import tarfile
import tempfile
import urllib.request
from pathlib import Path


def download_and_extract_tarball(url: str, target_dir: Path) -> None:
    """Download a release tarball from *url* and extract into *target_dir*.

    Handles the common GitHub-release convention where the tarball
    contains a single top-level directory (e.g. ``leedevkit-v0.3.0/``).
    If detected, that inner directory is moved into *target_dir*;
    otherwise the entire extracted tree is moved.

    The temporary download directory is always cleaned up, even on error.
    """
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
            shutil.rmtree(target_dir)
        target_dir.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(source), str(target_dir))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
