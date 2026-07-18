"""LeeAttend Internal Bootstrap — Python equivalent of _bootstrap.sh.

Centralizes container engine detection, compose tool selection, profile
injection, and environment variable setup. Used by the Python Orchestrator
and all other infrastructure modules.
"""

import os
import shutil
from pathlib import Path


# Resolve the actual project root from CWD, not the devkit install location.
# Walks up from current directory looking for leedevkit.toml or .git.
def _find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "leedevkit.toml").exists() or (parent / ".git").exists():
            return parent
    # Fallback: devkit bundled mode (project root = devkit parent)
    return Path(__file__).resolve().parent.parent


PROJECT_ROOT = _find_project_root()
SCRIPTS_DIR = (
    Path(__file__).resolve().parent
)  # always where the orchestrator scripts live

# DevKit root: where the devkit is installed (per-project .leedevkit/ or global).
# Derived from SCRIPTS_DIR: scripts/ is always a direct child of devkit root.
DEVKIT_ROOT = SCRIPTS_DIR.parent


def _which(cmd: str) -> str | None:
    """Find executable in PATH (delegates to shutil.which)."""
    return shutil.which(cmd)


def detect_engine() -> str:
    """Detect container engine: podman preferred, fallback to docker."""
    if _which("podman") and _which("podman-compose"):
        return "podman"
    if _which("docker"):
        return "docker"
    if _which("podman"):
        return "podman"
    return "podman"


def detect_compose_cmd() -> list[str]:
    """Detect compose tool and return as argument list.

    Returns list for safe subprocess usage (no shell=True).
    """
    if _which("podman-compose"):
        return ["podman-compose"]
    if _which("docker"):
        return ["docker", "compose"]
    return ["podman-compose"]


PROFILES: dict[str, list[str]] = {
    "api": ["--profile", "api"],
    "integration": ["--profile", "api"],
    "web": ["--profile", "web"],
    "all": ["--profile", "api", "--profile", "web"],
}


LIFECYCLE_PROFILES: dict[str, list[str]] = {
    "api": [
        "--profile",
        "api",
        "--profile",
        "infra-db",
        "--profile",
        "infra-redis",
        "--profile",
        "infra-pooler",
    ],
    "int-api": [
        "--profile",
        "int-api",
        "--profile",
        "infra-db",
        "--profile",
        "infra-redis",
        "--profile",
        "infra-pooler",
    ],
    "integration": [
        "--profile",
        "api",
        "--profile",
        "infra-db",
        "--profile",
        "infra-redis",
        "--profile",
        "infra-pooler",
    ],
    "web": ["--profile", "web"],
    "all": [
        "--profile",
        "api",
        "--profile",
        "web",
        "--profile",
        "infra-db",
        "--profile",
        "infra-redis",
        "--profile",
        "infra-pooler",
    ],
}


def resolve_profiles(mode: str) -> list[str]:
    """Translate a mode string into docker-compose --profile flags."""
    custom = os.environ.get("PROFILES", "")
    if custom:
        return custom.split()
    return PROFILES.get(mode, ["--profile", mode])


def resolve_lifecycle_profiles(mode: str) -> list[str]:
    """Translate a mode into lifecycle (up/down) profile flags."""
    return LIFECYCLE_PROFILES.get(mode, ["--profile", mode])


def _find_container_compose() -> Path | None:
    """Find a language-specific compose file under container/<lang>/.

    Auto-discovers subdirectories — adding container/go/ or container/python/
    in the future requires no code changes here.
    """
    container_dir = DEVKIT_ROOT / "container"
    if not container_dir.exists():
        return None
    for lang_dir in sorted(container_dir.iterdir()):
        if not lang_dir.is_dir() or lang_dir.name == "__pycache__":
            continue
        candidate = lang_dir / "docker-compose.test.yml"
        if candidate.exists():
            return candidate
    return None


def bootstrap_env(mode: str = "all") -> dict[str, str]:
    """Build the full environment dict for the given mode.

    Returns a dict suitable for os.environ.copy().update().
    """
    engine = detect_engine()
    compose_cmd = detect_compose_cmd()
    project_name = os.environ.get("COMPOSE_PROJECT_NAME", "leeattend-test")

    profiles = resolve_profiles(mode)

    # Resolve compose file: project override first, then devkit default
    compose_file = PROJECT_ROOT / ".compose" / "docker-compose.test.yml"
    if not compose_file.exists():
        container_compose = _find_container_compose()
        if container_compose is not None:
            compose_file = container_compose

    compose_base = [
        "-p",
        project_name,
        "-f",
        str(compose_file),
    ]
    compose_full = compose_cmd + compose_base + profiles
    compose_str = " ".join(compose_full)

    return {
        "CONTAINER_ENGINE": engine,
        "COMPOSE_PROJECT_NAME": project_name,
        "PODMAN_COMPOSE_PROJECT_NAME": project_name,
        "DOCKER_COMPOSE_CMD": compose_str,
        "DOCKER_COMPOSE_BASE": " ".join(compose_cmd + compose_base),
        "DOCKER_COMPOSE_TOOL": " ".join(compose_cmd),
        "USE_DOCKER": "true",
        "PROJECT_ROOT": str(PROJECT_ROOT),
        "MODE": mode,
    }
