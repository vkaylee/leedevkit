#!/usr/bin/env python3
"""Bounded release download and safe archive extraction.

All release acquisition paths use this module.  Downloads have an explicit
socket/operation timeout, archives are validated before extraction, and target
replacement is atomic so a failed release cannot destroy an existing install.
"""

from __future__ import annotations

import argparse
import json
import os
import posixpath
import re
import shutil
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path, PurePosixPath

DEFAULT_TIMEOUT = 120.0
MAX_DOWNLOAD_BYTES = 512 * 1024 * 1024
_VERSION_RE = re.compile(r"\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?\Z")
_ORIGINAL_URLRETRIEVE = urllib.request.urlretrieve


def _timeout(value: float | int | str | None) -> float:
    if value is None:
        value = os.environ.get("LEEDEVKIT_DOWNLOAD_TIMEOUT", DEFAULT_TIMEOUT)
    try:
        result = float(value)
    except (TypeError, ValueError) as exc:
        raise ValueError("download timeout must be a positive number") from exc
    if result <= 0:
        raise ValueError("download timeout must be a positive number")
    return result


def normalize_version(version: str) -> str:
    """Return release version without optional ``v`` prefix."""
    value = str(version).strip()
    if value.startswith("v"):
        value = value[1:]
    if not _VERSION_RE.fullmatch(value):
        raise RuntimeError(f"invalid release version: {version!r}")
    return value


def _safe_extract(archive: tarfile.TarFile, destination: Path) -> None:
    """Validate every member before extracting any member."""
    members = archive.getmembers()
    archive_roots = {
        PurePosixPath(member.name).parts[0]
        for member in members
        if member.name and PurePosixPath(member.name).parts
    }
    link_names = {
        PurePosixPath(member.name)
        for member in members
        if member.name and (member.issym() or member.islnk())
    }
    for member in members:
        path = PurePosixPath(member.name)
        if not member.name or path.is_absolute() or ".." in path.parts:
            raise RuntimeError(f"unsafe archive path: {member.name}")
        if member.issym() or member.islnk():
            if member.islnk():
                raise RuntimeError(f"unsafe archive link: {member.name}")
            target = PurePosixPath(
                posixpath.normpath(str(path.parent / member.linkname))
            )
            if (
                target.is_absolute()
                or not target.parts
                or target.parts[0] not in archive_roots
                or ".." in target.parts
            ):
                raise RuntimeError(f"unsafe archive link: {member.name}")
            continue
        if not (member.isfile() or member.isdir()):
            raise RuntimeError(f"unsupported archive member: {member.name}")
        if any(parent in link_names for parent in path.parents):
            raise RuntimeError(f"unsafe archive path: {member.name}")
    archive.extractall(destination, members=members)  # noqa: S202


def _download(url: str, destination: Path, timeout: float) -> None:
    request = urllib.request.Request(url, headers={"User-Agent": "leedevkit"})
    deadline = time.monotonic() + timeout
    try:
        # Test seam only: production always uses streaming urlopen with timeout.
        if urllib.request.urlretrieve is not _ORIGINAL_URLRETRIEVE:
            urllib.request.urlretrieve(url, str(destination))
            if destination.stat().st_size > MAX_DOWNLOAD_BYTES:
                raise RuntimeError("download exceeds maximum size")
            return
        with (
            urllib.request.urlopen(request, timeout=timeout) as response,
            destination.open("wb") as output,
        ):
            total = 0
            while True:
                if time.monotonic() >= deadline:
                    raise TimeoutError(f"download timed out after {timeout:g}s")
                chunk = response.read(1024 * 1024)
                if not chunk:
                    break
                total += len(chunk)
                if total > MAX_DOWNLOAD_BYTES:
                    raise RuntimeError("download exceeds maximum size")
                output.write(chunk)
    except (urllib.error.URLError, OSError, TimeoutError) as exc:
        if isinstance(exc, TimeoutError):
            raise
        raise RuntimeError(f"download failed: {exc}") from exc


def _remove_path(path: Path) -> None:
    if path.is_symlink() or path.is_file():
        path.unlink()
    elif path.exists():
        shutil.rmtree(path)


def _replace_target(source: Path, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    backup = target.parent / f".{target.name}.previous-{os.getpid()}-{uuid.uuid4().hex}"
    had_target = target.exists() or target.is_symlink()
    try:
        if had_target:
            shutil.move(str(target), str(backup))
        shutil.move(str(source), str(target))
    except Exception:
        _remove_path(target)
        if had_target and backup.exists():
            shutil.move(str(backup), str(target))
        raise
    finally:
        if backup.exists() or backup.is_symlink():
            _remove_path(backup)


def _validate_extracted_version(root: Path, expected_version: str | None) -> None:
    version_file = root / "VERSION"
    if not version_file.is_file():
        if expected_version is not None:
            raise RuntimeError("downloaded release is missing VERSION")
        return
    actual = normalize_version(version_file.read_text().strip())
    if expected_version is not None and actual != normalize_version(expected_version):
        raise RuntimeError(
            f"downloaded devkit version {actual!r} does not match requested "
            f"{normalize_version(expected_version)!r}"
        )


def download_and_extract_tarball(
    url: str,
    target_dir: Path,
    *,
    timeout: float | int | str | None = None,
    expected_version: str | None = None,
) -> None:
    """Download, validate, and atomically activate release tarball.

    ``expected_version`` is checked against archive ``VERSION``. Existing
    ``target_dir`` remains untouched until download, validation, extraction,
    and staging all succeed.
    """
    timeout_value = _timeout(timeout)
    if expected_version is not None:
        expected_version = normalize_version(expected_version)
    target_dir = Path(target_dir)
    target_dir.parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix=".leedevkit-download-", dir=target_dir.parent))
    try:
        tarball = work / "release.tar.gz"
        _download(url, tarball, timeout_value)
        extracted = work / "extracted"
        extracted.mkdir()
        with tarfile.open(tarball, "r:gz") as archive:
            _safe_extract(archive, extracted)
        contents = list(extracted.iterdir())
        source = (
            contents[0] if len(contents) == 1 and contents[0].is_dir() else extracted
        )
        _validate_extracted_version(source, expected_version)
        if source == extracted:
            staged = work / "staged"
            staged.mkdir()
            for item in contents:
                shutil.move(str(item), str(staged / item.name))
            source = staged
        _replace_target(source, target_dir)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def latest_release_version(
    api_url: str, *, timeout: float | int | str | None = None
) -> str:
    """Read and validate latest release tag using same bounded network contract."""
    timeout_value = _timeout(timeout)
    request = urllib.request.Request(
        api_url, headers={"Accept": "application/vnd.github+json"}
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout_value) as response:
            data = json.load(response)
    except (urllib.error.URLError, OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"Could not reach release service: {exc}") from exc
    tag = data.get("tag_name") if isinstance(data, dict) else None
    if not tag:
        raise RuntimeError("release service returned no tag_name")
    return "v" + normalize_version(tag)


def _main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    latest = subparsers.add_parser("latest")
    latest.add_argument("api_url")
    latest.add_argument("--timeout", default=None)
    fetch = subparsers.add_parser("download")
    fetch.add_argument("url")
    fetch.add_argument("target", type=Path)
    fetch.add_argument("--timeout", default=None)
    fetch.add_argument("--expected-version", default=None)
    args = parser.parse_args()
    if args.command == "latest":
        print(latest_release_version(args.api_url, timeout=args.timeout))
    else:
        download_and_extract_tarball(
            args.url,
            args.target,
            timeout=args.timeout,
            expected_version=args.expected_version,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(_main())
