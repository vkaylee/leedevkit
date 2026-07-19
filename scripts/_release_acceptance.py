#!/usr/bin/env python3
"""Black-box acceptance gate for a built LeeDevKit release artifact."""

from __future__ import annotations

import argparse
import os
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path, PurePosixPath

from _devkit_integrity import verify_devkit
from _release_build import build_release

REQUIRED_PATHS = {
    "VERSION",
    "devkit.manifest.json",
    "bin/leedevkit",
    "scripts/_orchestrator.py",
    "scripts/_devkit_integrity.py",
}
FORBIDDEN_PARTS = {".git", "__pycache__", ".venv", "skills.d"}


def _archive_members(artifact: Path, expected_root: str) -> list[tarfile.TarInfo]:
    with tarfile.open(artifact, "r:gz") as archive:
        members = archive.getmembers()

    if not members:
        raise RuntimeError("release archive is empty")

    for member in members:
        path = PurePosixPath(member.name)
        if path.is_absolute() or ".." in path.parts:
            raise RuntimeError(f"unsafe archive path: {member.name}")
        if path.parts[0] != expected_root:
            raise RuntimeError(f"unexpected archive root: {member.name}")
        if not (member.isfile() or member.isdir()):
            raise RuntimeError(f"unsupported archive member: {member.name}")
        if member.mode & (stat.S_ISUID | stat.S_ISGID):
            raise RuntimeError(f"unsafe archive mode: {member.name}")
        if FORBIDDEN_PARTS.intersection(path.parts):
            raise RuntimeError(f"forbidden release content: {member.name}")
    return members


def _extract_artifact(artifact: Path, destination: Path, expected_root: str) -> Path:
    members = _archive_members(artifact, expected_root)
    with tarfile.open(artifact, "r:gz") as archive:
        archive.extractall(destination, members=members)  # noqa: S202
    return destination / expected_root


def _run(
    command: list[str], cwd: Path, env: dict[str, str]
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        command,
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"command failed ({result.returncode}): {' '.join(command)}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result


def _assert_artifact_contract(artifact: Path, version: str, workspace: Path) -> Path:
    expected_root = f"leedevkit-{version}"
    members = _archive_members(artifact, expected_root)
    names = {member.name for member in members}
    missing = {
        f"{expected_root}/{relative}"
        for relative in REQUIRED_PATHS
        if f"{expected_root}/{relative}" not in names
    }
    if missing:
        raise RuntimeError(f"release archive missing: {', '.join(sorted(missing))}")

    extracted = _extract_artifact(artifact, workspace, expected_root)
    if (extracted / "VERSION").read_text().strip() != version:
        raise RuntimeError("artifact VERSION does not match filename")
    verification = verify_devkit(extracted)
    if not verification.is_clean:
        raise RuntimeError(verification.report())
    return extracted


def _assert_offline_fast_paths(
    extracted: Path, version: str, env: dict[str, str]
) -> None:
    cli = extracted / "bin" / "leedevkit"
    for arguments in (["version"], ["--version"], ["--help"], []):
        result = _run([str(cli), *arguments], extracted, env)
        if arguments in (["version"], ["--version"]):
            if result.stdout.strip() != version:
                raise RuntimeError(f"unexpected version output: {result.stdout!r}")
        elif "Usage: leedevkit" not in result.stdout:
            raise RuntimeError(f"usage output missing for {arguments}")
    if (extracted / ".venv").exists():
        raise RuntimeError("fast-path commands unexpectedly created .venv")


def _assert_bootstrap(
    repo_root: Path, artifact: Path, version: str, workspace: Path, env: dict[str, str]
) -> None:
    mirror = workspace / "mirror"
    release_dir = mirror / "download" / f"v{version}"
    release_dir.mkdir(parents=True)
    shutil.copy2(artifact, release_dir / artifact.name)

    project = workspace / "project"
    project.mkdir()
    (project / ".git").mkdir()
    bootstrap_env = {
        **env,
        "LEEDEVKIT_RELEASE_BASE_URL": mirror.as_uri(),
    }
    _run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"], project, bootstrap_env
    )

    installed = project / ".leedevkit"
    verification = verify_devkit(installed)
    if not verification.is_clean:
        raise RuntimeError(verification.report())
    result = _run([str(project / "leedevkit"), "version"], project, bootstrap_env)
    if result.stdout.strip() != version:
        raise RuntimeError("bootstrapped wrapper reports the wrong version")
    if (installed / ".venv").exists():
        raise RuntimeError("bootstrapped version command unexpectedly created .venv")
    if ".leedevkit/" not in (project / ".gitignore").read_text():
        raise RuntimeError("bootstrap did not add .leedevkit to .gitignore")

    sentinel = installed / "SENTINEL"
    sentinel.write_text("preserve")
    broken_env = {
        **bootstrap_env,
        "LEEDEVKIT_RELEASE_BASE_URL": (workspace / "missing-mirror").as_uri(),
    }
    failure = subprocess.run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"],
        cwd=project,
        env=broken_env,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    if failure.returncode == 0:
        raise RuntimeError("bootstrap unexpectedly accepted a missing artifact")
    if sentinel.read_text() != "preserve":
        raise RuntimeError("failed bootstrap replaced the existing installation")


def run_acceptance(repo_root: Path, artifact: Path | None = None) -> Path:
    """Build when needed, then validate and exercise the release artifact."""
    version = (repo_root / "VERSION").read_text().strip()
    owned_workspace: tempfile.TemporaryDirectory[str] | None = None
    if artifact is None:
        owned_workspace = tempfile.TemporaryDirectory(
            prefix="leedevkit-acceptance-artifact-"
        )
        artifact = build_release(repo_root, Path(owned_workspace.name) / "dist")
    artifact = artifact.resolve()

    try:
        with tempfile.TemporaryDirectory(prefix="leedevkit-acceptance-") as temp_dir:
            workspace = Path(temp_dir)
            expected_name = f"leedevkit-{version}.tar.gz"
            if artifact.name != expected_name:
                raise RuntimeError(
                    f"expected artifact name {expected_name}, got {artifact.name}"
                )

            home = workspace / "home"
            temp = workspace / "tmp"
            home.mkdir()
            temp.mkdir()
            env = {
                **os.environ,
                "HOME": str(home),
                "TMPDIR": str(temp),
                "HTTP_PROXY": "http://127.0.0.1:9",
                "HTTPS_PROXY": "http://127.0.0.1:9",
                "NO_PROXY": "",
            }
            extracted_parent = workspace / "extracted"
            extracted_parent.mkdir()
            extracted = _assert_artifact_contract(artifact, version, extracted_parent)
            _assert_offline_fast_paths(extracted, version, env)
            _assert_bootstrap(repo_root, artifact, version, workspace, env)
    finally:
        if owned_workspace is not None:
            owned_workspace.cleanup()

    return artifact


def main() -> None:
    parser = argparse.ArgumentParser(description="Accept a LeeDevKit release artifact")
    parser.add_argument("--repo-root", default=".")
    parser.add_argument("--artifact")
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    artifact = Path(args.artifact).resolve() if args.artifact else None
    accepted = run_acceptance(repo_root, artifact)
    print(f"✅ Release acceptance passed: {accepted}")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, tarfile.TarError) as error:
        print(f"❌ Release acceptance failed: {error}", file=sys.stderr)
        raise SystemExit(1) from error
