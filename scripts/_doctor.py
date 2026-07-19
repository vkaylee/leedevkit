#!/usr/bin/env python3
"""System doctor — health check extracted from Orchestrator (SRP).

Checks: project config, .agent directory, devkit install, AI rules,
container engine, port conflicts, virtual environment, and running containers.
"""

from __future__ import annotations

import socket
import subprocess

from _bootstrap import PROJECT_ROOT
from _devkit_config import get_devkit_root, load_project_config, resolve_ai_rules
from _logging import log_info, log_success, log_warn


def run_doctor(engine: str) -> None:
    """Run a full system health check and report findings.

    Args:
        engine: Container engine name ('podman' or 'docker').
    """
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
    try:
        dk = get_devkit_root()
        dk_version = (
            (dk / "VERSION").read_text().strip()
            if (dk / "VERSION").exists()
            else "?"
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

    if (PROJECT_ROOT / ".venv").exists():
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
