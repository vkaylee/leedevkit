"""Skills manager — install, update, remove, and list community skills.

Extracted from Orchestrator.handle_skills (Technical Debt #2).
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from _bootstrap import PROJECT_ROOT

# Re-use logging helpers from shared logging module.
from _logging import log_error, log_info, log_success, log_warn


GIT_TIMEOUT_SECONDS = 120


def _run_git(*args: str) -> subprocess.CompletedProcess[str] | None:
    """Run git with one bounded timeout contract."""
    try:
        return subprocess.run(
            ["git", *args],
            check=False,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=GIT_TIMEOUT_SECONDS,
        )
    except (OSError, subprocess.TimeoutExpired):
        return None


def _remove_path(path: Path) -> None:
    if path.is_symlink() or path.is_file():
        path.unlink(missing_ok=True)
    elif path.exists():
        shutil.rmtree(path)


def _lock_snapshot(path: Path) -> bytes | None:
    try:
        return path.read_bytes() if path.exists() else None
    except OSError:
        return None


def _restore_lock(path: Path, snapshot: bytes | None) -> None:
    try:
        if snapshot is None:
            path.unlink(missing_ok=True)
        else:
            path.write_bytes(snapshot)
    except OSError as exc:
        log_error(f"Failed to restore leedevkit.lock: {exc}")


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

    def _sync_claude_resources(self) -> bool:
        """Sync installed resources through configured harness adapters."""
        from _devkit_config import load_project_config
        from _harness_engine import sync_harnesses

        sync_harnesses(PROJECT_ROOT, self._devkit, load_project_config())
        return True

    @staticmethod
    def _git_error(
        result: subprocess.CompletedProcess[str] | None, default: str
    ) -> str:
        if result is None:
            return "git command timed out or could not start"
        return getattr(result, "stderr", "").strip() or default

    def _rollback_activation(
        self, activated: list[tuple[Path, Path | None]], lock_before: bytes | None
    ) -> None:
        for target, backup in reversed(activated):
            try:
                _remove_path(target)
                if backup is not None and backup.exists():
                    shutil.move(str(backup), str(target))
            except OSError as exc:
                log_error(f"Failed to restore skill {target.name}: {exc}")
        _restore_lock(self._lock_path(), lock_before)

    def _activate_staged(
        self,
        staged: list[tuple[Path, Path]],
        rollback_root: Path,
        lock_before: bytes | None,
    ) -> bool:
        activated: list[tuple[Path, Path | None]] = []
        try:
            for target, stage in staged:
                backup: Path | None = (
                    rollback_root / f"backup-{len(activated)}-{target.name}"
                )
                if target.exists():
                    shutil.move(str(target), str(backup))
                else:
                    backup = None
                activated.append((target, backup))
                shutil.move(str(stage), str(target))

            if self._write_lock() is False:
                raise OSError("could not write leedevkit.lock")
            if self._sync_claude_resources() is False:
                raise OSError("could not sync installed skills")
            return True
        except (OSError, RuntimeError, ValueError) as exc:
            self._rollback_activation(activated, lock_before)
            log_error(f"Skill transaction rolled back: {exc}")
            return False

    def _stage_clone(
        self,
        rollback_root: Path,
        name: str,
        url: str,
        version: str,
        sha: str | None,
        fallback_target: Path | None = None,
    ) -> Path | None:
        stage = rollback_root / f"stage-{name}"
        result = _run_git("clone", "--depth", "1", "--branch", version, url, str(stage))
        if result is None or result.returncode != 0:
            if fallback_target is not None:
                _remove_path(fallback_target)
            log_error(
                f"Failed to clone {name}: {self._git_error(result, 'clone failed')}"
            )
            _remove_path(stage)
            return None
        if not stage.exists():
            if fallback_target is not None and fallback_target.exists():
                shutil.move(str(fallback_target), str(stage))
            else:
                # Real git always creates destination; tolerate test doubles only.
                stage.mkdir(parents=True)
        if sha and not self._pin_repo(stage, sha):
            log_error(f"Failed to checkout locked SHA for {name}")
            _remove_path(stage)
            return None
        return stage

    def dispatch(self, args: argparse.Namespace) -> bool:
        """Dispatch skill action and return false on a failed state change."""
        action = getattr(args, "skills_action", "list")

        if action == "list":
            self._list()
            return True
        if action == "install":
            name = getattr(args, "name", None) or ""
            return self._install_by_name(name) if name else self._install_from_toml()
        if action == "update":
            return self._update_and_lock()
        if action == "add":
            return self._add_from_url(
                getattr(args, "url", ""), getattr(args, "version", "main")
            )
        if action == "remove":
            return self._remove(getattr(args, "name", ""))
        return True

    # -- Actions ------------------------------------------------------------

    def _list(self) -> None:
        from _harness_engine import discover_skill_sources

        builtins = set(discover_skill_sources(self._devkit / ".agent" / "skills"))
        community = discover_skill_sources(self._skills_d)
        installed = set(community)
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

        from _devkit_config import load_project_config
        from _harness_engine import resolve_harnesses

        log_info("")
        log_info(
            "Projected into: "
            + ", ".join(resolve_harnesses(PROJECT_ROOT, load_project_config()))
        )

        if not installed and not catalog:
            log_info("No community skills. Add one:")
            log_info("  leedevkit skills add <git-url>")

    def _install_by_name(self, name: str) -> bool:
        catalog = self._load_catalog()
        if name not in catalog:
            log_error(
                f"'{name}' not found in catalog. Use 'skills list' to see available skills."
            )
            log_error("Or install from URL: leedevkit skills add <git-url>")
            return False

        skill = catalog[name]
        target = self._skills_d / name
        if target.exists():
            log_warn(f"'{name}' already installed. Use 'skills update' to refresh.")
            return False

        log_info(f"Installing {skill['name']} from catalog...")
        lock_before = _lock_snapshot(self._lock_path())
        rollback_root = Path(
            tempfile.mkdtemp(prefix=".skills-install-", dir=str(self._skills_d))
        )
        try:
            stage = self._stage_clone(
                rollback_root,
                name,
                skill["url"],
                skill.get("version", "main"),
                None,
                target,
            )
            if stage is None:
                return False
            if not self._activate_staged([(target, stage)], rollback_root, lock_before):
                return False
            log_success(f"Installed {skill['name']} @ {skill.get('version', 'main')}")
            return True
        finally:
            shutil.rmtree(str(rollback_root), ignore_errors=True)

    @staticmethod
    def _pin_repo(repo: Path, sha: str) -> bool:
        """Checkout sha and verify exact resulting HEAD."""
        object_res = _run_git("-C", str(repo), "cat-file", "-e", f"{sha}^{{commit}}")
        if object_res is None or object_res.returncode != 0:
            fetch_res = _run_git(
                "-C", str(repo), "fetch", "--depth", "1", "origin", sha
            )
            if fetch_res is None or fetch_res.returncode != 0:
                return False
        checkout_res = _run_git("-C", str(repo), "checkout", "--detach", sha)
        if checkout_res is None or checkout_res.returncode != 0:
            return False
        head_res = _run_git("-C", str(repo), "rev-parse", "HEAD")
        return (
            head_res is not None
            and head_res.returncode == 0
            and head_res.stdout.strip() == sha
        )

    def _install_from_toml(self) -> bool:
        """Install configured skills as one transaction, honoring lock SHAs."""
        from _devkit_config import load_project_config

        try:
            entries = load_project_config().get("addons", {}).get("skills", [])
        except (OSError, ValueError, KeyError):
            entries = []
        if not entries:
            return True

        lock = self._read_lock()
        lock_before = _lock_snapshot(self._lock_path())
        rollback_root = Path(
            tempfile.mkdtemp(prefix=".skills-install-", dir=str(self._skills_d))
        )
        staged: list[tuple[Path, Path]] = []
        try:
            for entry in entries:
                if isinstance(entry, str):
                    url, version = entry, "main"
                else:
                    url = entry.get("url", "")
                    version = entry.get("version", "main")
                name = url.rstrip("/").split("/")[-1].replace(".git", "")
                if not name or not url:
                    log_error("Invalid skill entry: missing URL")
                    return False
                target = self._skills_d / name
                pinned_sha = lock.get(name)
                if target.exists() and not pinned_sha:
                    continue
                stage = self._stage_clone(
                    rollback_root,
                    name,
                    url,
                    version,
                    pinned_sha,
                    None if target.exists() else target,
                )
                if stage is None:
                    log_warn(f"Skill install transaction aborted at {name}")
                    return False
                staged.append((target, stage))

            if not staged:
                return True
            if not self._activate_staged(staged, rollback_root, lock_before):
                return False
            log_success(f"Installed {len(staged)} skill repo(s)")
            return True
        finally:
            shutil.rmtree(str(rollback_root), ignore_errors=True)

    def _add_from_url(self, url: str, version: str = "main") -> bool:
        if not url:
            log_error("Usage: leedevkit skills add <git-url> [--version main]")
            return False
        if not url.startswith(("http://", "https://", "git@")):
            catalog = self._load_catalog()
            if url in catalog:
                log_error(
                    f"'{url}' is in the skills catalog. Use: leedevkit skills install {url}"
                )
            else:
                log_error(
                    f"'{url}' is not a valid URL. Provide a git URL or use: leedevkit skills install <name>"
                )
            return False

        name = url.rstrip("/").split("/")[-1].replace(".git", "")
        target = self._skills_d / name
        if target.exists():
            log_warn(f"{name} already exists. Use 'skills update' to refresh.")
            return False
        lock_before = _lock_snapshot(self._lock_path())
        rollback_root = Path(
            tempfile.mkdtemp(prefix=".skills-add-", dir=str(self._skills_d))
        )
        try:
            stage = self._stage_clone(rollback_root, name, url, version, None, target)
            if stage is None:
                return False
            if not self._activate_staged([(target, stage)], rollback_root, lock_before):
                return False
            log_success(f"Installed {name} @ {version}")
            return True
        finally:
            shutil.rmtree(str(rollback_root), ignore_errors=True)

    def _update_and_lock(self) -> bool:
        """Pull latest for all installed skills as one transaction."""
        repos = [
            repo
            for repo in sorted(self._skills_d.iterdir())
            if repo.is_dir() and (repo / ".git").exists()
        ]
        if not repos:
            log_success("Updated 0 skill repo(s)")
            return True

        lock_before = _lock_snapshot(self._lock_path())
        rollback_root = Path(
            tempfile.mkdtemp(prefix=".skills-update-", dir=str(self._skills_d))
        )
        backups: list[tuple[Path, Path]] = []
        try:
            for index, repo in enumerate(repos):
                backup = rollback_root / f"backup-{index}-{repo.name}"
                shutil.copytree(repo, backup, symlinks=True)
                backups.append((repo, backup))

            for repo in repos:
                result = _run_git("-C", str(repo), "pull", "--ff-only")
                if result is None or result.returncode != 0:
                    error = self._git_error(result, "pull failed")
                    log_error(f"Failed to update {repo.name}: {error}")
                    self._rollback_update(backups, lock_before)
                    return False

            if self._write_lock() is False:
                raise OSError("could not write leedevkit.lock")
            if self._sync_claude_resources() is False:
                raise OSError("could not sync installed skills")
            log_success(f"Updated {len(repos)} skill repo(s)")
            return True
        except (OSError, RuntimeError, ValueError) as exc:
            self._rollback_update(backups, lock_before)
            log_error(f"Skill update transaction rolled back: {exc}")
            return False
        finally:
            shutil.rmtree(str(rollback_root), ignore_errors=True)

    def _rollback_update(
        self, backups: list[tuple[Path, Path]], lock_before: bytes | None
    ) -> None:
        for repo, backup in reversed(backups):
            try:
                _remove_path(repo)
                shutil.move(str(backup), str(repo))
            except OSError as exc:
                log_error(f"Failed to restore skill {repo.name}: {exc}")
        _restore_lock(self._lock_path(), lock_before)

    def _remove(self, name: str) -> bool:
        if not name:
            log_error("Usage: leedevkit skills remove <name>")
            return False
        target = self._skills_d / name
        if not target.exists():
            log_warn(f"{name} not found in skills.d/")
            return False

        lock_before = _lock_snapshot(self._lock_path())
        backup = self._skills_d / f".{name}.remove-backup"
        try:
            shutil.move(str(target), str(backup))
            if self._write_lock() is False:
                raise OSError("could not write leedevkit.lock")
            if self._sync_claude_resources() is False:
                raise OSError("could not sync installed skills")
            _remove_path(backup)
            log_success(f"Removed {name}")
            return True
        except (OSError, RuntimeError, ValueError) as exc:
            _remove_path(target)
            if backup.exists():
                shutil.move(str(backup), str(target))
            _restore_lock(self._lock_path(), lock_before)
            log_error(f"Failed to remove {name}: {exc}")
            return False

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

    def _write_lock(self) -> bool:
        lock: dict[str, str] = {}
        for repo in sorted(self._skills_d.iterdir()):
            if repo.is_dir() and (repo / ".git").exists():
                result = _run_git("-C", str(repo), "rev-parse", "HEAD")
                if result is None or result.returncode != 0:
                    log_error(f"Could not read HEAD for {repo.name}")
                    return False
                sha = result.stdout.strip()
                if not sha:
                    log_error(f"Could not read HEAD for {repo.name}")
                    return False
                lock[repo.name] = sha
        path = self._lock_path()
        path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = path.with_name(f".{path.name}.tmp-{os.getpid()}")
        try:
            import tomli_w

            with open(temp_path, "wb") as stream:
                tomli_w.dump(lock, stream)
        except ImportError:
            temp_path.write_text(json.dumps(lock, indent=2))
        except OSError as exc:
            log_error(f"Could not write leedevkit.lock: {exc}")
            temp_path.unlink(missing_ok=True)
            return False
        try:
            os.replace(temp_path, path)
        except OSError as exc:
            log_error(f"Could not activate leedevkit.lock: {exc}")
            temp_path.unlink(missing_ok=True)
            return False
        log_success(f"Updated leedevkit.lock ({len(lock)} entries)")
        return True
