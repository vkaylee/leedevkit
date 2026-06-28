#!/usr/bin/env python3
"""Devkit configuration cascade engine.

Loads `leedevkit.toml` from the project root and merges it with the devkit
default template. Implements a strict priority chain:

    1. Project leedevkit.toml       (highest)
    2. Devkit default template    (base fallback)
    3. Auto-detected values       (lowest)

Also resolves the AI rule override manifest (`.agent/overrides.yaml`).
"""

from __future__ import annotations

import os
import sys
from pathlib import Path
from typing import Any

# ── Bootstrap: locate devkit root ──────────────────────────────────────────

_DEVKIT_ROOT: Path | None = None


def _find_devkit_root() -> Path:
    """Resolve the installed devkit directory (versioned)."""
    global _DEVKIT_ROOT
    if _DEVKIT_ROOT is not None:
        return _DEVKIT_ROOT

    # 1. DEVKIT_HOME env var
    env = os.environ.get("DEVKIT_HOME")
    if env:
        _DEVKIT_ROOT = Path(env)
        return _DEVKIT_ROOT

    # 2. ~/.leedevkit/current symlink
    home = Path.home() / ".leedevkit" / "current"
    if home.exists():
        _DEVKIT_ROOT = home
        return _DEVKIT_ROOT

    # 3. Bundled: ./leedevkit/ relative to project root
    bundled = _find_project_root() / "leedevkit"
    if bundled.exists():
        _DEVKIT_ROOT = bundled
        return _DEVKIT_ROOT

    raise FileNotFoundError(
        "Cannot locate leedevkit. Set DEVKIT_HOME or install to ~/.leedevkit"
    )


def _find_project_root() -> Path:
    """Walk up from cwd until we find leedevkit.toml or .git."""
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "leedevkit.toml").exists() or (parent / ".git").exists():
            return parent
    return cwd


# ── TOML loader (stdlib fallback) ──────────────────────────────────────────

def _load_toml(path: Path) -> dict[str, Any]:
    """Load a TOML file. Uses tomllib (3.11+) or tomli (fallback)."""
    try:
        import tomllib  # Python 3.11+

        with open(path, "rb") as f:
            return tomllib.load(f)
    except ImportError:
        pass
    try:
        import tomli  # pip install tomli

        with open(path, "rb") as f:
            return tomli.load(f)
    except ImportError:
        pass
    # Minimal inline parser — handles flat keys and strings
    return _parse_toml_minimal(path)


def _parse_toml_minimal(path: Path) -> dict[str, Any]:
    """Fallback TOML parser for environments without tomli/tomllib."""
    result: dict[str, Any] = {}
    current_section = result
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("[") and line.endswith("]"):
                section = line[1:-1].strip()
                current_section = result.setdefault(section, {})
            elif "=" in line:
                key, _, val = line.partition("=")
                key = key.strip()
                val = val.strip().strip('"').strip("'")
                current_section[key] = val
    return result


# ── Merge engine ───────────────────────────────────────────────────────────


def deep_merge(base: dict, override: dict) -> dict:
    """Recursive dict merge: override keys win, nested dicts are merged."""
    result = dict(base)
    for key, value in override.items():
        if key in result and isinstance(result[key], dict) and isinstance(value, dict):
            result[key] = deep_merge(result[key], value)
        else:
            result[key] = value
    return result


def load_project_config() -> dict[str, Any]:
    """Load the fully merged project configuration.

    Returns a dict with all resolved settings.
    """
    project_root = _find_project_root()
    devkit_root = _find_devkit_root()

    # Layer 1: Devkit default
    default_path = devkit_root / "templates" / ".devkit.default.toml"
    config: dict[str, Any] = {}
    if default_path.exists():
        config = _load_toml(default_path)

    # Layer 2: Project override
    project_path = project_root / "leedevkit.toml"
    if project_path.exists():
        project_config = _load_toml(project_path)
        config = deep_merge(config, project_config)

    # Layer 3: Injected values
    config.setdefault("_meta", {})
    config["_meta"]["project_root"] = str(project_root)
    config["_meta"]["devkit_root"] = str(devkit_root)
    config["_meta"]["devkit_version"] = _read_version(project_root)

    return config


def _read_version(project_root: Path) -> str:
    """Read pinned devkit version from .devkit-version file."""
    version_file = project_root / ".devkit-version"
    if version_file.exists():
        return version_file.read_text().strip()
    return "latest"


# ── AI Rule resolver ───────────────────────────────────────────────────────


def _load_yaml(path: Path) -> dict[str, Any] | None:
    """Load YAML if available, else return None."""
    try:
        import yaml  # type: ignore[import-untyped]

        with open(path, encoding="utf-8") as f:
            return yaml.safe_load(f)
    except ImportError:
        return None


def resolve_ai_rules(project_root: Path | None = None) -> list[Path]:
    """Return the ordered list of AI rule files to load.

    Priority chain (loaded in order, later files override):
      1. devkit base rules
      2. project rules (replacing devkit where applicable)
      3. project add-only rules

    Returns: list of absolute Paths to .md rule files.
    """
    if project_root is None:
        project_root = _find_project_root()
    devkit_root = _find_devkit_root()

    # Read AI paths from leedevkit.toml, falling back to defaults
    cfg = load_project_config()
    ai_cfg = cfg.get("ai", {})
    rules_rel = ai_cfg.get("rules_dir", ".agent/rules")
    manifest_rel = ai_cfg.get("override_manifest", ".agent/overrides.yaml")

    devkit_rules_dir = devkit_root / ".agent" / "rules"
    project_rules_dir = project_root / rules_rel
    manifest_path = project_root / manifest_rel

    manifest: dict[str, Any] = _load_yaml(manifest_path) or {}
    replace: list[str] = manifest.get("replace", [])
    add: list[str] = manifest.get("add", [])
    extend: list[str] = manifest.get("extend", [])

    result: list[Path] = []

    # Devkit base rules (skip replaced ones)
    if devkit_rules_dir.exists():
        for rule_file in sorted(devkit_rules_dir.rglob("*.md")):
            rel = str(rule_file.relative_to(devkit_rules_dir))
            if rel not in replace:
                result.append(rule_file)

    # Project overrides
    if project_rules_dir.exists():
        # Project versions of devkit rules (replace/extend)
        for rule_file in sorted(project_rules_dir.rglob("*.md")):
            rel = str(rule_file.relative_to(project_rules_dir))
            if rel in replace or rel in extend:
                result.append(rule_file)

        # Project-only rules (add)
        if add:
            for rule_name in add:
                rule_path = project_rules_dir / rule_name
                if rule_path.exists():
                    result.append(rule_path)

    return result


# ── CLI ────────────────────────────────────────────────────────────────────

if __name__ == "__main__":  # pragma: no cover
    action = sys.argv[1] if len(sys.argv) > 1 else "config"
    if action == "config":
        import json

        cfg = load_project_config()
        print(json.dumps(cfg, indent=2, default=str))
    elif action == "rules":
        for p in resolve_ai_rules():
            print(p)
