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


from _go_core_build import DEFAULT_TARGETS, target_binary_name
from _go_core_matrix import build_matrix
from _release_build import build_release
from _platform import platform_key

REQUIRED_PATHS = {
    "cmd/debug-mcp/main.go",
    "container/debug/Dockerfile",
    "VERSION",
    "devkit.manifest.json",
    "sbom.cdx.json",
    "bin/leedevkit",
    "scripts/requirements-runtime.lock",
    "scripts/_go_core_build.py",
    "scripts/_go_core_matrix.py",
    "scripts/_platform.py",
    "go.mod",
    "cmd/leedevkit/main.go",
    "container/go/docker-compose.core.yml",
    "bin/debug-mcp/leedevkit-debug-linux-amd64",
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
    command = [
        "go",
        "run",
        "./cmd/release-manifest",
        "--root",
        "/workspace",
        "--verify",
    ]
    if shutil.which("go"):
        _run(command, extracted, os.environ.copy())
    else:
        repo = Path(__file__).resolve().parents[1]
        module_cache = Path.home() / "go" / "pkg" / "mod"
        _run(
            [
                "docker",
                "run",
                "--rm",
                "-e",
                "GOPROXY=off",
                "-v",
                f"{extracted}:/workspace",
                "-v",
                f"{repo}:/source:ro",
                "-v",
                f"{module_cache}:/go/pkg/mod",
                "-w",
                "/workspace",
                "golang:1.26.11-bookworm",
                *command,
            ],
            extracted,
            os.environ.copy(),
        )
    return extracted


def _assert_go_core_source_contract(extracted: Path, version: str) -> None:
    """Verify Go source and optional platform binaries in release."""
    compose = extracted / "container" / "go" / "docker-compose.core.yml"
    if "leedevkit-go" not in compose.read_text():
        raise RuntimeError("Go core Compose service missing from release")
    source = extracted / "cmd" / "leedevkit" / "main.go"
    if "package main" not in source.read_text():
        raise RuntimeError("Go core source missing from release")
    if version not in source.read_text() and "var version" not in source.read_text():
        raise RuntimeError("Go core version contract missing from source")
    debug_dir = extracted / "bin" / "debug-mcp"
    if not debug_dir.exists():
        raise RuntimeError("release missing platform Debug MCP binaries")
    for goos, goarch in DEFAULT_TARGETS:
        from _go_core_build import debug_binary_name

        binary = debug_dir / debug_binary_name(goos, goarch)
        if not binary.is_file() or not binary.stat().st_size:
            raise RuntimeError(f"Debug MCP binary missing or empty: {binary.name}")
    binary_dir = extracted / "bin" / "go"
    if not binary_dir.exists():
        raise RuntimeError("release missing platform Go binaries")
    for goos, goarch in DEFAULT_TARGETS:
        binary = binary_dir / target_binary_name(goos, goarch)
        if not binary.is_file() or not binary.stat().st_size:
            raise RuntimeError(
                f"Go core platform binary missing or empty: {binary.name}"
            )


def _assert_offline_fast_paths(
    extracted: Path, version: str, env: dict[str, str]
) -> None:
    cli = extracted / "bin" / "go" / f"leedevkit-go-{platform_key()}"
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
    repo_root: Path,
    artifact: Path,
    extracted: Path,
    version: str,
    workspace: Path,
    env: dict[str, str],
) -> None:
    mirror = workspace / "mirror"
    release_dir = mirror / "download" / f"v{version}"
    release_dir.mkdir(parents=True)
    shutil.copy2(artifact, release_dir / artifact.name)

    project = workspace / "project"
    project.mkdir()
    (project / "leedevkit.toml").write_text(
        '[service]\nversion = "9.8.7"\n\n'
        '[devkit]\nversion = "0.0.1"\n\n'
        '[project]\nname = "acceptance"\n'
    )
    bootstrap_env = {
        **env,
        "LEEDEVKIT_RELEASE_BASE_URL": mirror.as_uri(),
    }
    install_root = workspace / "global-runtime"
    install_root.mkdir(parents=True, exist_ok=True)
    shutil.copytree(extracted, install_root, dirs_exist_ok=True)
    native = install_root / "bin" / "go" / f"leedevkit-go-{platform_key()}"
    shutil.copy2(native, install_root / "bin" / "leedevkit")
    (install_root / "bin" / "leedevkit").chmod(0o755)
    bootstrap_env["DEVKIT_HOME"] = str(install_root)
    _run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"], project, bootstrap_env
    )

    installed = project / ".leedevkit"
    if not (installed / "scripts" / "requirements-runtime.lock").is_file():
        raise RuntimeError("bootstrap did not install the runtime dependency lock")
    manifest_command = [
        "go",
        "run",
        "./cmd/release-manifest",
        "--root",
        "/workspace",
        "--verify",
    ]
    if shutil.which("go"):
        _run(manifest_command, installed, os.environ.copy())
    else:
        repo = Path(__file__).resolve().parents[1]
        module_cache = Path.home() / "go" / "pkg" / "mod"
        _run(
            [
                "docker",
                "run",
                "--rm",
                "-e",
                "GOPROXY=off",
                "-v",
                f"{installed}:/workspace",
                "-v",
                f"{repo}:/source:ro",
                "-v",
                f"{module_cache}:/go/pkg/mod",
                "-w",
                "/workspace",
                "golang:1.26.11-bookworm",
                *manifest_command,
            ],
            installed,
            os.environ.copy(),
        )
    binary = Path(bootstrap_env["DEVKIT_HOME"]) / "bin" / "leedevkit"
    result = _run([str(binary), "version"], project, bootstrap_env)
    if result.stdout.strip() != version:
        raise RuntimeError("global binary reports the wrong version after bootstrap")
    if (installed / ".venv").exists():
        raise RuntimeError("bootstrapped version command unexpectedly created .venv")
    if ".leedevkit/" not in (project / ".gitignore").read_text():
        raise RuntimeError("bootstrap did not add .leedevkit to .gitignore")
    config = (project / "leedevkit.toml").read_text()
    if f'version = "{version}"' not in config:
        raise RuntimeError("bootstrap did not update the existing version pin")
    if '[service]\nversion = "9.8.7"' not in config:
        raise RuntimeError("bootstrap changed a version outside [devkit]")

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
    if (project / "leedevkit.toml").read_text() != config:
        raise RuntimeError("failed bootstrap changed the existing version pin")

    fresh_project = workspace / "fresh-project"
    fresh_project.mkdir()
    (fresh_project / ".git").mkdir()
    _run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"],
        fresh_project,
        bootstrap_env,
    )
    fresh_config = (fresh_project / "leedevkit.toml").read_text()
    if f'version = "{version}"' not in fresh_config:
        raise RuntimeError("bootstrap wrote a v-prefixed version into a new config")

    pinned_project = workspace / "pinned-project"
    pinned_project.mkdir()
    (pinned_project / ".git").mkdir()
    (pinned_project / "leedevkit.toml").write_text(f'[devkit]\nversion = "{version}"\n')
    _run(["bash", str(repo_root / "bootstrap.sh")], pinned_project, bootstrap_env)
    if not (pinned_project / ".leedevkit" / "VERSION").is_file():
        raise RuntimeError(
            "bootstrap failed to install an unprefixed configured version"
        )


def _assert_global_install(
    repo_root: Path,
    artifact: Path,
    version: str,
    workspace: Path,
    env: dict[str, str],
) -> None:
    """Verify install.sh exposes custom global runtime through a fresh shell."""
    mirror = workspace / "install-mirror"
    release_dir = mirror / "download" / f"v{version}"
    release_dir.mkdir(parents=True)
    shutil.copy2(artifact, release_dir / artifact.name)

    home = workspace / "install-home"
    home.mkdir()
    rc = home / ".bashrc"
    rc.write_text('export PATH="/usr/local/bin:$PATH"')
    install_root = workspace / "custom install root"
    install_env = {
        **env,
        "HOME": str(home),
        "DEVKIT_HOME": str(install_root),
        "LEEDEVKIT_RELEASE_BASE_URL": mirror.as_uri(),
    }
    _run(
        ["bash", str(repo_root / "install.sh"), f"v{version}"],
        workspace,
        install_env,
    )
    path_dir = install_root / "current" / "bin"
    escaped_path = str(path_dir).replace(" ", "\\ ")
    path_entry = f'export PATH={escaped_path}:"$PATH"'
    rc_text = rc.read_text()
    if path_entry not in rc_text:
        raise RuntimeError("installer did not add custom runtime to .bashrc")
    if not rc_text.endswith("\n"):
        raise RuntimeError("installer left shell startup file without a newline")

    shell = subprocess.run(
        [
            "bash",
            "--noprofile",
            "--rcfile",
            str(rc),
            "-ic",
            "command -v leedevkit && leedevkit version",
        ],
        cwd=workspace,
        env={**install_env, "PATH": "/usr/bin:/bin"},
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    if shell.returncode != 0:
        raise RuntimeError(
            f"fresh shell could not run leedevkit:\nstdout:\n{shell.stdout}\nstderr:\n{shell.stderr}"
        )
    output = shell.stdout.splitlines()
    expected_binary = str(install_root / "current" / "bin" / "leedevkit")
    if (
        not output
        or output[0].strip() != expected_binary
        or output[-1].strip() != version
    ):
        raise RuntimeError(f"fresh shell resolved unexpected command: {shell.stdout!r}")

    rerun = _run(
        ["bash", str(repo_root / "install.sh"), f"v{version}"],
        workspace,
        install_env,
    )
    if rc.read_text().count(path_entry) != 1:
        raise RuntimeError("installer duplicated PATH entry")
    if "install" not in rerun.stdout:
        raise RuntimeError("installer rerun produced no status output")

    no_rc_home = workspace / "install-home-no-rc"
    no_rc_home.mkdir()
    no_rc_root = workspace / "no-rc install root"
    no_rc = _run(
        ["bash", str(repo_root / "install.sh"), f"v{version}"],
        workspace,
        {
            **install_env,
            "HOME": str(no_rc_home),
            "DEVKIT_HOME": str(no_rc_root),
        },
    )
    no_rc_path = str(no_rc_root / "current" / "bin").replace(" ", "\\ ")
    expected_export = f'export PATH={no_rc_path}:"$PATH"'
    if expected_export not in no_rc.stdout:
        raise RuntimeError(
            f"no-RC install omitted current-shell PATH guidance: {no_rc.stdout!r}"
        )


def run_acceptance(repo_root: Path, artifact: Path | None = None) -> Path:
    """Build when needed, then validate and exercise the release artifact."""
    version = (repo_root / "VERSION").read_text().strip()
    owned_workspace: tempfile.TemporaryDirectory[str] | None = None
    if artifact is None:
        owned_workspace = tempfile.TemporaryDirectory(
            prefix="leedevkit-acceptance-artifact-"
        )
        workspace = Path(owned_workspace.name)
        binaries = workspace / "go-core"
        debug_binaries = workspace / "debug-mcp"
        build_matrix(
            repo_root, binaries, version, compose_cmd=("/usr/bin/docker", "compose")
        )
        from _go_core_matrix import build_debug_matrix

        build_debug_matrix(
            repo_root,
            debug_binaries,
            version,
            compose_cmd=("/usr/bin/docker", "compose"),
        )
        artifact = build_release(
            repo_root, workspace / "dist", binaries, debug_binaries
        )
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
            _assert_go_core_source_contract(extracted, version)
            _assert_offline_fast_paths(extracted, version, env)
            _assert_global_install(repo_root, artifact, version, workspace, env)
            _assert_bootstrap(repo_root, artifact, extracted, version, workspace, env)
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
