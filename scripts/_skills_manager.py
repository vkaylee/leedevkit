"""Skills manager — install, update, remove, and list community skills.

Extracted from Orchestrator.handle_skills (Technical Debt #2).
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
from pathlib import Path

from _bootstrap import PROJECT_ROOT

# Re-use the logging helpers from the shared logging module.
from _logging import log_error, log_info, log_success, log_warn


class SkillsManager:
    """Manages community add-on skills in the devkit's skills.d/ directory.

    Usage:
        SkillsManager().dispatch(args)
    """

    def __init__(self) -> None:
        from _devkit_config import get_devkit_root

        self._devkit = get_devkit_root()
        self._skills_d = self._devkit / "skills.d"
        self._skills_d.mkdir(parents=True, exist_ok=True)
        self._catalog: dict | None = None

    def _sync_claude_resources(self) -> None:
        from _init_handler import sync_claude_resources

        sync_claude_resources(PROJECT_ROOT, self._devkit)

    # -- Public API ---------------------------------------------------------

    def dispatch(self, args: argparse.Namespace) -> None:
        """Entry point — replaces the old handle_skills if-elif chain."""
        action = getattr(args, "skills_action", "list")

        if action == "list":
            self._list()
        elif action == "install":
            name = getattr(args, "name", None) or ""
            if name:
                self._install_by_name(name)
            else:
                self._install_from_toml()
        elif action == "update":
            self._update_and_lock()
        elif action == "add":
            url = getattr(args, "url", "")
            version = getattr(args, "version", "main")
            self._add_from_url(url, version)
        elif action == "remove":
            name = getattr(args, "name", "")
            self._remove(name)

    # -- Actions ------------------------------------------------------------

    def _list(self) -> None:
        from _init_handler import discover_skill_sources

        # Read the same Claude discovery surface that sessions load.
        runtime_dir = PROJECT_ROOT / ".claude" / "skills"
        runtime = discover_skill_sources(runtime_dir)
        builtins = set(discover_skill_sources(self._devkit / ".agent" / "skills"))
        installed = set(runtime) - builtins
        community = discover_skill_sources(self._skills_d)
        catalog = self._load_catalog()
        catalog_runtime: set[str] = set()

        log_info("Skills")
        log_info("")

        log_info("Built-in (always available, shipped with devkit):")
        for name in sorted(builtins):
            log_info(f"  ▸ {name}")
        log_info(f"  ({len(builtins)} skills)")
        log_info("")

        if catalog:
            log_info("Community catalog (leedevkit skills install <name>):")
            for key, skill in sorted(catalog.items()):
                package = self._skills_d / key
                skill_ids = sorted(
                    name
                    for name, source in community.items()
                    if source == package or package in source.parents
                )
                catalog_runtime.update(skill_ids)
                status = "● installed" if skill_ids else "○ available"
                suffix = f" ({', '.join(skill_ids)})" if skill_ids else ""
                log_info(f"  {status}  {key}{suffix}")
                log_info(f"          {skill.get('description', '')}")
            log_info("")

        for name in sorted(installed - catalog_runtime):
            log_info(f"  ● {name} [external — not in catalog]")

        if not installed and not catalog:
            log_info("No community skills. Add one:")
            log_info("  leedevkit skills add <git-url>")

    def _install_by_name(self, name: str) -> None:
        catalog = self._load_catalog()
        if name not in catalog:
            log_error(
                f"'{name}' not found in catalog. "
                "Use 'skills list' to see available skills."
            )
            log_error("Or install from URL: leedevkit skills add <git-url>")
            return

        skill = catalog[name]
        url = skill["url"]
        version = skill.get("version", "main")
        target = self._skills_d / name
        if target.exists():
            log_warn(f"'{name}' already installed. Use 'skills update' to refresh.")
            return

        log_info(f"Installing {skill['name']} from catalog...")
        res = subprocess.run(
            ["git", "clone", "--depth", "1", "--branch", version, url, str(target)],
            check=False,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
        )
        if res.returncode != 0:
            if target.exists():
                shutil.rmtree(str(target), ignore_errors=True)
            err_msg = res.stderr.strip() or "git clone failed"
            log_error(f"Failed to install {skill['name']}: {err_msg}")
            return

        log_success(f"Installed {skill['name']} @ {version}")
        self._write_lock()
        self._sync_claude_resources()

    def _install_from_toml(self) -> None:
        """Install skills from leedevkit.toml [addons.skills], preferring lock SHAs."""
        from _devkit_config import load_project_config

        try:
            cfg = load_project_config()
            entries = cfg.get("addons", {}).get("skills", [])
        except (OSError, ValueError, KeyError):
            entries = []

        lock = self._read_lock()
        installed = 0
        failed: list[str] = []
        for entry in entries:
            if isinstance(entry, str):
                url, version = entry, "main"
            else:
                url = entry.get("url", "")
                version = entry.get("version", "main")
            name = url.rstrip("/").split("/")[-1].replace(".git", "")
            target = self._skills_d / name
            if target.exists():
                if name in lock:
                    checkout_res = subprocess.run(
                        ["git", "-C", str(target), "checkout", "--detach", lock[name]],
                        check=False,
                        stdin=subprocess.DEVNULL,
                        capture_output=True,
                    )
                    if checkout_res.returncode == 0:
                        log_success(f"  {name} @ {lock[name][:8]} (locked)")
                    else:
                        log_warn(f"  Failed to checkout locked version for {name}")
                continue

            log_info(f"Installing {name} @ {version}...")
            clone_res = subprocess.run(
                [
                    "git",
                    "clone",
                    "--depth",
                    "1",
                    "--branch",
                    version,
                    url,
                    str(target),
                ],
                check=False,
                stdin=subprocess.DEVNULL,
                capture_output=True,
                text=True,
            )
            if clone_res.returncode != 0:
                if target.exists():
                    shutil.rmtree(str(target), ignore_errors=True)
                err_msg = clone_res.stderr.strip() or "clone failed"
                log_error(f"Failed to clone {name}: {err_msg}")
                failed.append(name)
                continue

            if name in lock:
                fetch_res = subprocess.run(
                    [
                        "git",
                        "-C",
                        str(target),
                        "fetch",
                        "--depth",
                        "1",
                        "origin",
                        lock[name],
                    ],
                    check=False,
                    stdin=subprocess.DEVNULL,
                    capture_output=True,
                )
                checkout_res = subprocess.run(
                    ["git", "-C", str(target), "checkout", "--detach", lock[name]],
                    check=False,
                    stdin=subprocess.DEVNULL,
                    capture_output=True,
                )
                if fetch_res.returncode != 0 or checkout_res.returncode != 0:
                    log_warn(f"Failed to checkout locked SHA for {name}")
            installed += 1

        if failed:
            log_warn(f"Failed to install {len(failed)} skill repo(s): {', '.join(failed)}")
        log_success(f"Installed {installed} new skill repo(s)")
        if installed > 0:
            self._write_lock()
            self._sync_claude_resources()

    def _add_from_url(self, url: str, version: str = "main") -> None:
        if not url:
            log_error("Usage: leedevkit skills add <git-url> [--version main]")
            return
        if not url.startswith(("http://", "https://", "git@")):
            catalog = self._load_catalog()
            if url in catalog:
                log_error(
                    f"'{url}' is in the skills catalog. "
                    f"Use: leedevkit skills install {url}"
                )
            else:
                log_error(
                    f"'{url}' is not a valid URL. "
                    "Provide a git URL or use: leedevkit skills install <name>"
                )
            return

        name = url.rstrip("/").split("/")[-1].replace(".git", "")
        target = self._skills_d / name
        if target.exists():
            log_warn(f"{name} already exists. Use 'skills update' to refresh.")
            return

        res = subprocess.run(
            ["git", "clone", "--depth", "1", "--branch", version, url, str(target)],
            check=False,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
        )
        if res.returncode != 0:
            if target.exists():
                shutil.rmtree(str(target), ignore_errors=True)
            err_msg = res.stderr.strip() or "git clone failed"
            log_error(f"Failed to add skill from {url}: {err_msg}")
            return

        log_success(f"Installed {name} @ {version}")
        self._write_lock()
        self._sync_claude_resources()

    def _update_and_lock(self) -> None:
        """Pull latest for all installed skills, update lock file."""
        updated = 0
        failed: list[str] = []
        for repo in sorted(self._skills_d.iterdir()):
            if repo.is_dir() and (repo / ".git").exists():
                res = subprocess.run(
                    ["git", "-C", str(repo), "pull", "--ff-only"],
                    check=False,
                    stdin=subprocess.DEVNULL,
                    capture_output=True,
                    text=True,
                )
                if res.returncode == 0:
                    updated += 1
                else:
                    failed.append(repo.name)
        if failed:
            log_warn(f"Failed to update {len(failed)} skill repo(s): {', '.join(failed)}")
        log_success(f"Updated {updated} skill repo(s)")
        self._write_lock()
        self._sync_claude_resources()

    def _remove(self, name: str) -> None:
        if not name:
            log_error("Usage: leedevkit skills remove <name>")
            return
        target = self._skills_d / name
        if not target.exists():
            log_warn(f"{name} not found in skills.d/")
            return

        shutil.rmtree(str(target))
        log_success(f"Removed {name}")
        self._write_lock()
        self._sync_claude_resources()

    # -- Catalog & lock helpers --------------------------------------------

    def _load_catalog(self) -> dict:
        """Load the curated skills catalog from devkit."""
        catalog_path = self._devkit / ".agent" / "skills-catalog.toml"
        if not catalog_path.exists():
            return {}
        try:
            from _devkit_config import _load_toml

            return _load_toml(catalog_path).get("skills", {})
        except (OSError, ValueError, KeyError):
            return {}

    @staticmethod
    def _lock_path() -> Path:
        return PROJECT_ROOT / "leedevkit.lock"

    @classmethod
    def _read_lock(cls) -> dict:
        path = cls._lock_path()
        if path.exists():
            try:
                import tomllib

                with open(path, "rb") as f:
                    return tomllib.load(f)
            except (OSError, ValueError):
                pass
            try:
                return json.loads(path.read_text())
            except (OSError, ValueError, json.JSONDecodeError):
                pass
        return {}

    def _write_lock(self) -> None:
        lock: dict[str, str] = {}
        for repo in sorted(self._skills_d.iterdir()):
            if repo.is_dir() and (repo / ".git").exists():
                r = subprocess.run(
                    ["git", "-C", str(repo), "rev-parse", "HEAD"],
                    capture_output=True,
                    text=True,
                    check=False,
                )
                sha = r.stdout.strip()
                if sha:
                    lock[repo.name] = sha
        path = self._lock_path()
        path.parent.mkdir(exist_ok=True)
        try:
            import tomli_w

            with open(path, "wb") as f:
                tomli_w.dump(lock, f)
        except ImportError:
            path.write_text(json.dumps(lock, indent=2))
        log_success(f"Updated leedevkit.lock ({len(lock)} entries)")
