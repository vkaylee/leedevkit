"""Build the Go core through the repository's Compose environment."""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
from pathlib import Path
from typing import Sequence


COMPOSE_SERVICE = "leedevkit-go"
DEBUG_MCP_SERVICE = "leedevkit-debug-build"
DEFAULT_TARGETS = (
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("windows", "amd64"),
)


def target_binary_name(goos: str, goarch: str) -> str:
    suffix = ".exe" if goos == "windows" else ""
    return f"leedevkit-go-{goos}-{goarch}{suffix}"


def build_command(
    *,
    compose_cmd: Sequence[str],
    compose_file: Path,
    output: Path,
    version: str,
    goos: str | None = None,
    goarch: str | None = None,
) -> list[str]:
    """Build a safe argv list for the Go core container."""
    command = [
        *compose_cmd,
        "-f",
        str(compose_file),
        "--profile",
        "go-core",
        "run",
        "--rm",
        "--build",
    ]
    if goos:
        command.extend(["-e", f"GOOS={goos}"])
    if goarch:
        command.extend(["-e", f"GOARCH={goarch}"])
    command.extend(
        [
            COMPOSE_SERVICE,
            "build",
            "-buildvcs=false",
            "-trimpath",
            "-ldflags",
            f"-X=main.version={version}",
            "-o",
            "/workspace/.leedevkit-go-build/leedevkit-go",
            "./cmd/leedevkit",
        ]
    )
    return command


def debug_binary_name(goos: str, goarch: str) -> str:
    suffix = ".exe" if goos == "windows" else ""
    return f"leedevkit-debug-{goos}-{goarch}{suffix}"


def debug_build_command(
    *,
    compose_cmd: Sequence[str],
    compose_file: Path,
    output: Path,
    version: str,
    goos: str | None = None,
    goarch: str | None = None,
) -> list[str]:
    """Build standalone Go Debug MCP binary through Compose."""
    command = [
        *compose_cmd,
        "-f",
        str(compose_file),
        "--profile",
        "go-core",
        "run",
        "--rm",
        "--build",
    ]
    if goos:
        command.extend(["-e", f"GOOS={goos}"])
    if goarch:
        command.extend(["-e", f"GOARCH={goarch}"])
    command.extend(
        [
            DEBUG_MCP_SERVICE,
            "build",
            "-buildvcs=false",
            "-trimpath",
            "-ldflags",
            f"-X=main.version={version}",
            "-o",
            "/workspace/.leedevkit-go-build/leedevkit-debug",
            "./cmd/debug-mcp",
        ]
    )
    return command


def build_environment(repo_root: Path) -> dict[str, str]:
    """Prepare Compose build environment, reusing local module cache when present."""
    environment = {
        **os.environ,
        "GO_VERSION": "1.26",
        "LEEDEVKIT_PROJECT_ROOT": str(repo_root),
        "LEEDEVKIT_GO_DOCKERFILE": str(repo_root / "container" / "go" / "Dockerfile"),
    }
    module_cache = os.environ.get("GOMODCACHE") or os.path.expanduser("~/go/pkg/mod")
    if os.path.isdir(module_cache):
        environment["LEEDEVKIT_GO_MODULE_CACHE"] = module_cache
        environment["GOPROXY"] = "off"
    return environment


def build_go_core(
    repo_root: Path,
    output: Path,
    version: str,
    *,
    compose_cmd: Sequence[str] = ("docker", "compose"),
    goos: str | None = None,
    goarch: str | None = None,
    timeout: int = 600,
) -> Path:
    """Build Go core in Compose and copy binary outside source checkout."""
    repo_root = repo_root.resolve()
    compose_file = repo_root / "container" / "go" / "docker-compose.core.yml"
    if not compose_file.is_file():
        raise FileNotFoundError(f"Go core Compose file missing: {compose_file}")
    if (
        not (repo_root / "go.mod").is_file()
        or not (repo_root / "cmd" / "leedevkit" / "main.go").is_file()
    ):
        raise FileNotFoundError("Go core source files missing")

    output = output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    staging = repo_root / ".leedevkit-go-build"
    if staging.exists():
        raise FileExistsError(f"Go core build directory already exists: {staging}")
    staging.mkdir()
    try:
        command = build_command(
            compose_cmd=compose_cmd,
            compose_file=compose_file,
            output=staging / "leedevkit-go",
            version=version,
            goos=goos,
            goarch=goarch,
        )
        environment = build_environment(repo_root)
        result = subprocess.run(
            command,
            cwd=repo_root,
            env=environment,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(
                f"Go core build failed ({result.returncode})\n"
                f"stdout:\n{result.stdout}\n"
                f"stderr:\n{result.stderr}"
            )
        built = staging / "leedevkit-go"
        if not built.is_file():
            raise RuntimeError(
                f"Go core build produced no binary at {built}; "
                f"stdout={result.stdout[-500:]!r}; stderr={result.stderr[-500:]!r}"
            )
        shutil.copy2(built, output)
        output.chmod(output.stat().st_mode | 0o111)
        return output
    finally:
        shutil.rmtree(staging, ignore_errors=True)


def build_debug_mcp(
    repo_root: Path,
    output: Path,
    version: str,
    *,
    compose_cmd: Sequence[str] = ("docker", "compose"),
    goos: str | None = None,
    goarch: str | None = None,
    timeout: int = 600,
) -> Path:
    """Build Go Debug MCP binary in Compose and copy it out."""
    repo_root = repo_root.resolve()
    compose_file = repo_root / "container" / "go" / "docker-compose.core.yml"
    if not compose_file.is_file():
        raise FileNotFoundError(f"Go core Compose file missing: {compose_file}")
    if (
        not (repo_root / "go.mod").is_file()
        or not (repo_root / "cmd" / "debug-mcp" / "main.go").is_file()
    ):
        raise FileNotFoundError("Debug MCP source files missing")
    output = output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    staging = repo_root / ".leedevkit-go-build"
    if staging.exists():
        raise FileExistsError(f"Go core build directory already exists: {staging}")
    staging.mkdir()
    try:
        command = debug_build_command(
            compose_cmd=compose_cmd,
            compose_file=compose_file,
            output=staging / "leedevkit-debug",
            version=version,
            goos=goos,
            goarch=goarch,
        )
        environment = build_environment(repo_root)
        result = subprocess.run(
            command,
            cwd=repo_root,
            env=environment,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(
                f"Debug MCP build failed ({result.returncode})\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )
        built = staging / "leedevkit-debug"
        if not built.is_file():
            raise RuntimeError(f"Debug MCP build produced no binary at {built}")
        shutil.copy2(built, output)
        output.chmod(output.stat().st_mode | 0o111)
        return output
    finally:
        shutil.rmtree(staging, ignore_errors=True)


def main() -> None:
    parser = argparse.ArgumentParser(description="Build the LeeDevKit Go core")
    parser.add_argument("--repo-root", default=".")
    parser.add_argument("--output", required=True)
    parser.add_argument("--version")
    parser.add_argument("--goos")
    parser.add_argument("--goarch")
    args = parser.parse_args()

    repo_root = Path(args.repo_root).resolve()
    version = args.version or (repo_root / "VERSION").read_text().strip()
    build_go_core(
        repo_root,
        Path(args.output),
        version,
        goos=args.goos,
        goarch=args.goarch,
    )


if __name__ == "__main__":
    main()
