#!/usr/bin/env python3
"""Init handler extracted from Orchestrator (Single Responsibility Principle).

Handles: project initialization — devkit installation, venv setup, AI rule
copying, wrapper creation, version pinning, and legacy symlink detection.
"""

from __future__ import annotations

import os
import re
import subprocess
from pathlib import Path

from _devkit_config import _load_toml
from _download import download_and_extract_tarball
from _handler_base import HandlerBase
from _logging import log_info, log_success, log_warn


class InitHandler(HandlerBase):
    """Project initialization logic extracted from the Orchestrator god class.

    Inherits shared property forwarding from HandlerBase.
    Orchestrates the init flow: create config → install devkit → set up venv
    → copy AI rules → create wrapper → pin version → install skills.
    """

    # ── public API ──

    def handle_post_update_sync(self) -> None:
        """Lightweight sync after update: only sync rules and create symlinks.

        Unlike handle_init(), this skips heavy operations like:
        - Downloading devkit (already done by update)
        - Setting up venv (already exists)
        - Installing skills (handled separately)

        This is safe to call after update and won't hang on network/subprocess.
        """
        from _devkit_config import get_devkit_root

        root = Path.cwd()
        config_toml = root / "leedevkit.toml"
        cfg = _load_toml(config_toml) if config_toml.exists() else {}

        try:
            devkit = get_devkit_root()
        except FileNotFoundError:
            log_warn("DevKit not found, skipping post-update sync")
            return

        # Sync rules from devkit → project
        ai_cfg = cfg.get("ai", {})
        rules_rel = ai_cfg.get("rules_dir", ".agent/rules")
        project_rules = root / rules_rel
        devkit_rules = devkit / ".agent" / "rules"
        if devkit_rules.exists():
            project_rules.mkdir(parents=True, exist_ok=True)
            copied = 0
            for rule_file in sorted(devkit_rules.glob("*.md")):
                target = project_rules / rule_file.name
                if not target.exists():
                    target.write_text(rule_file.read_text())
                    copied += 1
            if copied:
                log_success(f"Synced {copied} new rulebook(s) to {rules_rel}/")

        # Create .agent/rules symlink if rules_dir is custom
        default_rules_rel = ".agent/rules"
        if rules_rel != default_rules_rel and project_rules.exists():
            default_rules = root / default_rules_rel
            self._ensure_rules_symlink(default_rules, project_rules, rules_rel)

        # Sync overrides.yaml if missing
        override_manifest = ai_cfg.get("override_manifest", ".agent/overrides.yaml")
        override_path = root / override_manifest
        if not override_path.exists():
            devkit_override = devkit / ".agent" / "overrides.yaml"
            if devkit_override.exists():
                override_path.parent.mkdir(parents=True, exist_ok=True)
                override_path.write_text(devkit_override.read_text())
                log_success(f"Created {override_manifest}")

    def handle_init(self, force: bool = False) -> None:
        """Set up project with per-project devkit install.

        Flow:
          1. Ensure .leedevkit/ exists (download release tarball if needed)
          2. Create project-local .venv inside .leedevkit/
          3. Copy base AI rules from devkit → project (not symlinks)
          4. Create ./leedevkit wrapper → .leedevkit/bin/leedevkit
          5. Install community skills from catalog/TOML
          6. Pin devkit version in leedevkit.toml
        """
        import argparse

        from _devkit_config import get_devkit_root

        root = Path.cwd()
        config_toml = root / "leedevkit.toml"

        # ── Step 0: Load or create leedevkit.toml ──
        if not config_toml.exists() or force:
            source_root = Path(__file__).resolve().parent.parent
            if (root / "Cargo.toml").exists():
                template = source_root / "templates" / "leedevkit.rust.toml"
            else:
                template = source_root / "templates" / "leedevkit.default.toml"
            if template.exists():
                config_toml.write_text(template.read_text())
                log_success("Created leedevkit.toml (edit it for your project)")

        cfg = _load_toml(config_toml) if config_toml.exists() else {}
        devkit_version = cfg.get("devkit", {}).get("version", "latest")

        # ── Step 1: Ensure .leedevkit/ exists with correct version ──
        leedevkit_dir = root / ".leedevkit"
        installed_version = self._read_installed_version(leedevkit_dir)

        if force or not leedevkit_dir.exists() or installed_version != devkit_version:
            log_info(f"Installing devkit {devkit_version} into .leedevkit/ ...")
            self._install_devkit(root, leedevkit_dir, devkit_version, force=force)
        else:
            log_info(f"Devkit {devkit_version} already installed in .leedevkit/")

        devkit = get_devkit_root()

        # ── Step 2: Ensure the DevKit-local virtual environment ──
        ensure_venv = devkit / "scripts" / "_ensure-venv.sh"
        if not ensure_venv.is_file():
            raise RuntimeError(f"DevKit venv bootstrap is missing: {ensure_venv}")
        setup = subprocess.run(
            ["bash", str(ensure_venv)],
            capture_output=True,
            text=True,
            check=False,
            stdin=subprocess.DEVNULL,
        )
        if setup.returncode != 0:
            detail = setup.stderr.strip() or "unknown error"
            raise RuntimeError(f"Could not set up DevKit virtual environment: {detail}")
        python_path = (
            Path(setup.stdout.strip().splitlines()[-1])
            if setup.stdout.strip()
            else None
        )
        expected_python = devkit / ".venv" / "bin" / "python3"
        if (
            python_path is None
            or python_path != expected_python
            or not python_path.is_file()
            or not os.access(python_path, os.X_OK)
        ):
            raise RuntimeError(
                "DevKit venv bootstrap returned an invalid Python executable"
            )
        log_success("Virtual environment ready")

        # ── Step 2b: Detect legacy symlinks from old global install ──
        agent_dir = root / ".agent"
        legacy_symlinks = self._detect_legacy_symlinks(agent_dir)
        if legacy_symlinks:
            log_warn(
                f"⚠️  Detected {len(legacy_symlinks)} legacy symlink(s) in .agent/ "
                "(from old global install):"
            )
            for name, target in legacy_symlinks:
                log_warn(f"      .agent/{name} → {target}")
            log_info("")
            log_info("   These are no longer needed with per-project install.")
            log_info("   To clean up: rm -rf .agent && ./leedevkit init")
            log_info("")

        # ── Step 3: Copy base AI rules from devkit → project (not symlinks) ──
        ai_cfg = cfg.get("ai", {})
        rules_rel = ai_cfg.get("rules_dir", ".agent/rules")
        project_rules = root / rules_rel
        devkit_rules = devkit / ".agent" / "rules"
        if devkit_rules.exists():
            project_rules.mkdir(parents=True, exist_ok=True)
            copied = 0
            for rule_file in sorted(devkit_rules.glob("*.md")):
                target = project_rules / rule_file.name
                if not target.exists() or force:
                    target.write_text(rule_file.read_text())
                    copied += 1
            if copied:
                log_success(f"Populated {rules_rel}/ with {copied} rulebook(s)")

        # ── Step 3b: Create .agent/rules symlink if rules_dir is custom ──
        # CLAUDE.md template hardcodes .agent/rules/, so we create a symlink
        # when the project uses a custom rules_dir (e.g., .attendagent/rules).
        # This ensures AI agents can find rulebooks regardless of config.
        default_rules_rel = ".agent/rules"
        if rules_rel != default_rules_rel and project_rules.exists():
            default_rules = root / default_rules_rel
            self._ensure_rules_symlink(default_rules, project_rules, rules_rel)

        # Copy overrides.yaml if project doesn't have one
        override_manifest = ai_cfg.get("override_manifest", ".agent/overrides.yaml")
        override_path = root / override_manifest
        if not override_path.exists():
            devkit_override = devkit / ".agent" / "overrides.yaml"
            if devkit_override.exists():
                override_path.parent.mkdir(parents=True, exist_ok=True)
                override_path.write_text(devkit_override.read_text())
                log_success(f"Created {override_manifest}")

        # ── Step 4: Create ./leedevkit wrapper (project-local, not global) ──
        wrapper = root / "leedevkit"
        wrapper_target = leedevkit_dir / "bin" / "leedevkit"
        wrapper_content = (
            "#!/bin/bash\n"
            "# LeeDevKit — project-local wrapper (auto-generated by init)\n"
            "# Delegates to .leedevkit/bin/leedevkit (per-project install)\n"
            f'exec "{wrapper_target}" "$@"\n'
        )
        wrapper.write_text(wrapper_content)
        wrapper.chmod(0o755)
        log_success("Created ./leedevkit → .leedevkit/bin/leedevkit")

        # ── Step 5: Pin devkit version in leedevkit.toml ──
        actual_version = (devkit / "VERSION").read_text().strip()
        current = cfg.get("devkit", {}).get("version", "")
        if current != actual_version and config_toml.exists():
            content = config_toml.read_text()
            if "version =" in content and "[devkit]" in content:
                content = re.sub(
                    r'(\[devkit\].*?version\s*=\s*)"[^"]*"',
                    f'\\1"{actual_version}"',
                    content,
                    flags=re.DOTALL,
                )
                config_toml.write_text(content)
            log_success(f"Pinned devkit version: {actual_version}")

        # ── Step 6: Auto-install community skills from leedevkit.toml ──
        from _skills_manager import SkillsManager

        SkillsManager().dispatch(argparse.Namespace(skills_action="install"))

        log_success("Project initialized. Run ./leedevkit --help to start.")

    def _detect_legacy_symlinks(self, agent_dir: Path) -> list[tuple[str, str]]:
        """Detect symlinks in .agent/ that point to the old global install.

        Returns list of (name, target_path) for symlinks whose target
        resolves under ~/.leedevkit/ (the legacy global install location).
        """
        legacy: list[tuple[str, str]] = []
        if not agent_dir.exists():
            return legacy
        global_prefix = str(Path.home() / ".leedevkit")
        for item in sorted(agent_dir.iterdir()):
            if item.is_symlink():
                try:
                    target = str(item.resolve())
                except OSError:
                    target = str(item.readlink())
                if target.startswith(global_prefix):
                    legacy.append((item.name, target))
        return legacy

    def _ensure_rules_symlink(
        self, default_rules: Path, project_rules: Path, rules_rel: str
    ) -> None:
        """Ensure .agent/rules symlink points to the custom rules directory.

        Creates or repairs symlink so CLAUDE.md's hardcoded .agent/rules/ paths
        work even when project uses a custom rules_dir (e.g., .attendagent/rules).

        Behavior:
          - If default_rules is a broken symlink → remove and recreate
          - If default_rules is a valid symlink to wrong target → remove and recreate
          - If default_rules is a real directory → skip (user may have created it manually)
          - If default_rules doesn't exist → create symlink
        """
        # Check if it's a symlink (valid or broken)
        if default_rules.is_symlink():
            try:
                current_target = default_rules.resolve()
                expected_target = project_rules.resolve()
                if current_target == expected_target:
                    # Symlink is correct
                    return
                # Symlink points to wrong target → remove and recreate
                log_warn(
                    f"⚠️  .agent/rules symlink points to wrong target, fixing..."
                )
                default_rules.unlink()
            except OSError:
                # Broken symlink → remove and recreate
                log_warn(
                    f"⚠️  .agent/rules symlink is broken, fixing..."
                )
                default_rules.unlink()
        elif default_rules.exists():
            # It's a real directory, not a symlink → don't touch it
            # User may have created it manually or it's from an old init
            return

        # Create symlink
        default_rules.parent.mkdir(parents=True, exist_ok=True)
        default_rules.symlink_to(project_rules.resolve())
        log_success(
            f"Created symlink .agent/rules → {rules_rel} (for CLAUDE.md compatibility)"
        )

    def _read_installed_version(self, leedevkit_dir: Path) -> str | None:
        """Read the version installed in .leedevkit/, or None if not installed."""
        version_file = leedevkit_dir / "VERSION"
        if version_file.exists():
            return version_file.read_text().strip()
        return None

    def _install_devkit(
        self, project_root: Path, target_dir: Path, version: str, force: bool = False
    ) -> None:
        """Download and install devkit into target_dir (.leedevkit/).

        Strategy (in order):
          1. Local path override (DEVKIT_LOCAL_PATH env) — for development
          2. GitHub release tarball — for production
          3. Copy from current devkit source — fallback for dogfooding
        """
        import shutil as _shutil

        if target_dir.exists() and force:
            _shutil.rmtree(target_dir)

        # Strategy 1: local path override
        local_path = os.environ.get("DEVKIT_LOCAL_PATH")
        if local_path and Path(local_path).exists():
            log_info(f"Installing from local path: {local_path}")
            self._extract_from_source(Path(local_path), target_dir)
            return

        # Strategy 2: GitHub release tarball
        ver = version.lstrip("v") if version else version
        if ver and version != "latest":
            url = (
                f"https://github.com/vkaylee/leedevkit/archive/refs/tags/"
                f"{version}.tar.gz"
            )
            log_info(f"Downloading {url} ...")
            try:
                download_and_extract_tarball(url, target_dir)
                return
            except Exception as e:
                log_warn(f"Download failed: {e}")

        # Strategy 3: copy from current devkit source (dogfooding)
        source_root = Path(__file__).resolve().parent.parent
        if (source_root / "scripts" / "_orchestrator.py").exists():
            log_info("Installing from local devkit source ...")
            self._extract_from_source(source_root, target_dir)
            return

        raise RuntimeError(
            f"Cannot install devkit {version}. Set DEVKIT_LOCAL_PATH or "
            "ensure GitHub release exists."
        )

    def _extract_from_source(self, source_root: Path, target_dir: Path) -> None:
        """Copy devkit artifacts from a source directory into target_dir."""
        import shutil as _shutil

        target_dir.mkdir(parents=True, exist_ok=True)
        # Copy immutable artifacts
        for item in ["scripts", "templates", "bin", ".agent"]:
            src = source_root / item
            dst = target_dir / item
            if src.exists():
                if dst.exists():
                    if src.is_dir():
                        _shutil.rmtree(dst)
                    else:
                        dst.unlink()
                if src.is_dir():
                    _shutil.copytree(src, dst)
                else:
                    _shutil.copy2(src, dst)
        # Copy metadata files
        for fname in ["VERSION", "devkit.manifest.json"]:
            src = source_root / fname
            if src.exists():
                _shutil.copy2(src, target_dir / fname)
        # Ensure bin scripts are executable
        bin_dir = target_dir / "bin"
        if bin_dir.exists():
            for f in bin_dir.iterdir():
                if f.is_file():
                    f.chmod(0o755)
        # Write dev-state.json
        version = (target_dir / "VERSION").read_text().strip()
        state = {"version": version, "source": "local"}
        (target_dir / "dev-state.json").write_text(
            __import__("json").dumps(state, indent=2)
        )
        log_success(f"Installed devkit {version} → {target_dir}")
