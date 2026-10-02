#!/usr/bin/env python3
"""System doctor — health check extracted from Orchestrator (SRP).

Checks: project config, .agent directory, devkit install, AI rules,
container engine, port conflicts, virtual environment, and running containers.
"""

from __future__ import annotations
import os
import shutil
import socket
import subprocess
from pathlib import Path

from _bootstrap import PROJECT_ROOT, ensure_project_gitignore
from _devkit_config import (
    _load_toml,
    get_devkit_root,
    load_project_config,
    resolve_ai_rules,
)
from _download import download_and_extract_tarball, normalize_version
from _logging import log_info, log_success, log_warn


def _runtime_matches(root: Path, version: str) -> bool:
    version_file = root / "VERSION"
    return version_file.is_file() and version_file.read_text().strip().lstrip(
        "v"
    ) == version.lstrip("v")


def _matching_runtime(version: str) -> Path | None:
    candidates = [PROJECT_ROOT / ".leedevkit"]
    common = subprocess.run(
        ["git", "rev-parse", "--path-format=absolute", "--git-common-dir"],
        capture_output=True,
        text=True,
        check=False,
    ).stdout.strip()
    if common:
        common_path = Path(common).resolve()
        if common_path.name == ".git":
            candidates.append(common_path.parent / ".leedevkit")
    env = os.environ.get("DEVKIT_HOME")
    if env:
        candidates.append(Path(env))
    return next(
        (candidate for candidate in candidates if _runtime_matches(candidate, version)),
        None,
    )


def _bootstrap_runtime(version: str) -> Path:
    version = normalize_version(version)
    base = os.environ.get(
        "LEEDEVKIT_RELEASE_BASE_URL",
        "https://github.com/vkaylee/leedevkit/releases",
    ).rstrip("/")
    target = PROJECT_ROOT / ".leedevkit"
    url = f"{base}/download/v{version}/leedevkit-{version}.tar.gz"
    download_and_extract_tarball(
        url,
        target,
        timeout=os.environ.get("LEEDEVKIT_DOWNLOAD_TIMEOUT"),
        expected_version=version,
    )
    return target


def _repair_environment() -> None:
    """Repair only DevKit-owned runtime, venv, and missing rulebooks."""
    ensure_project_gitignore(PROJECT_ROOT)
    config_path = PROJECT_ROOT / "leedevkit.toml"
    cfg = _load_toml(config_path) if config_path.is_file() else {}
    configured = str(cfg.get("devkit", {}).get("version", "latest"))
    if configured == "latest":
        raise RuntimeError("[devkit].version must be pinned before runtime repair")
    version = normalize_version(configured)
    runtime = PROJECT_ROOT / ".leedevkit"
    source = _matching_runtime(version)
    if source is not None and source != runtime:
        if runtime.is_symlink():
            runtime.unlink()
        elif runtime.exists():
            raise RuntimeError(f"Refusing to replace user-owned runtime: {runtime}")
        runtime.symlink_to(source, target_is_directory=True)
        log_success(f"✅ Linked worktree runtime: {runtime} → {source}")
    elif not _runtime_matches(runtime, version):
        _bootstrap_runtime(version)
        log_success(f"✅ Downloaded DevKit {version} into {runtime}")

    import _devkit_config

    _devkit_config._DEVKIT_ROOT = None
    devkit = get_devkit_root()
    ensure_venv = devkit / "scripts" / "_ensure-venv.sh"
    python_bin = devkit / ".venv" / "bin" / "python3"
    venv_ready = python_bin.is_file() and os.access(python_bin, os.X_OK)
    if venv_ready:
        probe = subprocess.run(
            [str(python_bin), "-c", "import sys"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        venv_ready = probe.returncode == 0
    if ensure_venv.is_file() and not venv_ready:
        result = subprocess.run(["bash", str(ensure_venv)], check=False)
        if result.returncode:
            raise RuntimeError("DevKit virtual environment repair failed")
        log_success("✅ Virtual environment repaired")

    rules_rel = cfg.get("ai", {}).get("rules_dir", ".agent/rules")
    source_rules = devkit / ".agent" / "rules"
    target_rules = PROJECT_ROOT / rules_rel
    if target_rules.is_symlink():
        log_warn(f"⚠️  Refusing to modify symlinked rulebook directory: {target_rules}")
        return
    target_rules.mkdir(parents=True, exist_ok=True)
    copied = 0
    for rule in source_rules.glob("*.md"):
        target = target_rules / rule.name
        if not target.exists() and not target.is_symlink():
            shutil.copy2(rule, target)
            copied += 1
    if copied:
        log_success(f"✅ Synchronized {copied} missing AI rulebook(s)")


def run_doctor(engine: str, fix: bool = False) -> None:
    """Run a full system health check and report findings.

    Args:
        engine: Container engine name ('podman' or 'docker').
    """
    if fix:
        try:
            _repair_environment()
        except (OSError, RuntimeError, ValueError) as error:
            log_warn(f"⚠️  Repair failed: {error}")
    log_info("🩺 Running LeeDevKit System Doctor...")

    # ── Project config ──
    try:
        cfg = load_project_config()
        name = cfg.get("project", {}).get("name", "unknown")
        targets = list(cfg.get("targets", {}).keys())
        log_success(f"✅ Project: {name} (targets: {', '.join(targets)})")
    except (OSError, ValueError, KeyError) as e:
        log_warn(f"⚠️  leedevkit.toml: {e}")

    # ── .agent directory (per-project, real dir not symlink) ──
    agent_dir = PROJECT_ROOT / ".agent"
    if agent_dir.is_symlink():
        log_warn(
            "⚠️  .agent is a symlink — expected real directory (run: leedevkit init)"
        )
    elif agent_dir.is_dir():
        rules_dir = agent_dir / "rules"
        rule_count = len(list(rules_dir.glob("*.md"))) if rules_dir.exists() else 0
        log_success(f"✅ .agent/ (real directory, {rule_count} rulebooks)")
    else:
        log_warn("⚠️  .agent directory missing (run: leedevkit init)")

    # ── DevKit install location ──
    devkit_root: Path | None = None
    try:
        dk = get_devkit_root()
        devkit_root = dk
        dk_version = (
            (dk / "VERSION").read_text().strip() if (dk / "VERSION").exists() else "?"
        )
        log_success(f"✅ DevKit: {dk} (v{dk_version})")
    except (OSError, ValueError) as e:
        log_warn(f"⚠️  DevKit: {e}")

    # ── AI rules ──
    try:
        rules = resolve_ai_rules()
        log_success(f"✅ AI rules: {len(rules)} files loaded")
    except (OSError, ValueError, KeyError) as e:
        log_warn(f"⚠️  AI rules: {e}")

    log_success(f"✅ Container Engine: {engine}")

    default_ports = [3000, 8000, 5432]
    connection_timeout = 0.5
    for port in default_ports:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(connection_timeout)
        if s.connect_ex(("127.0.0.1", port)) == 0:
            log_info(f"⚠️  Port {port} is occupied")
        s.close()

    venv_root = (
        (devkit_root / ".venv") if devkit_root is not None else (PROJECT_ROOT / ".venv")
    )
    if venv_root.is_dir():
        log_success("✅ Virtual Environment: Found")
    else:
        log_info("💡 Virtual Environment: Missing (will be created on next run)")

    if engine:
        res = subprocess.run(
            [engine, "ps", "--format", "{{.Names}}"],
            capture_output=True,
            text=True,
            stdin=subprocess.DEVNULL,
            check=False,
        )
        names = res.stdout.splitlines()
        for r in ["leedevkit-dev-db", "leedevkit-dev-api"]:
            if any(r in n for n in names):
                log_success(f"✅ Container {r}: Running")


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(prog="leedevkit doctor")
    parser.add_argument("--fix", action="store_true")
    args = parser.parse_args()
    engine = (
        "podman"
        if shutil.which("podman")
        else "docker"
        if shutil.which("docker")
        else ""
    )
    run_doctor(engine, fix=args.fix)
