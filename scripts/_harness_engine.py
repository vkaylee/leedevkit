#!/usr/bin/env python3
"""Harness adapter engine — project `.agent/` sources into harness discovery paths.

Contract
--------
* ``.agent/`` is the single source of truth: rules, agents, skills, workflows.
* Every adapter projects that source into one harness's discovery layout and
  never rewrites the source payload.
* Managed artifacts are only created, updated, or removed by the devkit.
  User-owned files and symlinks are never modified or deleted.
* Instruction files (``CLAUDE.md``, ``AGENTS.md``, ``GEMINI.md``) are edited
  through a marker block so surrounding user content survives re-sync.
* Precedence, highest first: project-local user entries, project rules,
  installed community skills (``skills.d/``), devkit built-ins.

Adding a harness means adding one adapter class and one name in
``ADAPTER_NAMES``. Nothing else in the devkit branches on harness identity.
"""

from __future__ import annotations

import os
from abc import ABC, abstractmethod
from pathlib import Path

from _logging import log_warn

# ── Harness-neutral contract constants ─────────────────────────────────────

ADAPTER_NAMES = ("claude", "codex", "cursor", "gemini", "omp", "pi")

#: Historical filename; the *path* is a stable contract referenced by every
#: generated instruction file, so it is not renamed with the adapters.
BASE_CONTEXT_REF = ".leedevkit/templates/CLAUDE.base.md"

MARKER_START = "<!-- leedevkit:begin -->"
MARKER_END = "<!-- leedevkit:end -->"

BASE_CONTEXT = f"""## LeeDevKit base context

This repository uses LeeDevKit. Before making changes, read and apply:
`{BASE_CONTEXT_REF}`.

Source of truth for this project's AI context lives in `.agent/`:

| Purpose | Location |
|---------|----------|
| Rulebooks (lazy-load by domain) | `.agent/rules/*.md` |
| Built-in skills | `.agent/skills/*/SKILL.md` |
| Installed community skills | `.leedevkit/skills.d/*/SKILL.md` |
| Specialist agents | `.agent/agents/*.md` |
| Devkit commands | `./leedevkit --help` |

Capability fallback: if this harness cannot run shell commands, dispatch
subagents, or track todos, follow the skill's prose directly and report the
missing capability. Never invent a tool call the harness does not provide.
"""


# ── Managed write helpers ──────────────────────────────────────────────────


def write_managed_block(path: Path, body: str) -> bool:
    """Create or refresh a marker-delimited block; preserve the surrounding file.

    Returns True when the file changed on disk.
    """
    block = f"{MARKER_START}\n{body.rstrip()}\n{MARKER_END}\n"
    previous = path.read_text(encoding="utf-8") if path.exists() else None

    if previous is None:
        updated = block
    elif MARKER_START in previous and MARKER_END in previous:
        head, _, rest = previous.partition(MARKER_START)
        _, _, tail = rest.partition(MARKER_END)
        tail = tail.lstrip("\n")
        updated = head + block + (f"\n{tail}" if tail else "")
    else:
        separator = "\n\n" if previous and not previous.endswith("\n\n") else ""
        updated = previous + separator + block

    if updated == previous:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(updated, encoding="utf-8")
    return True


def write_managed_file(path: Path, content: str) -> bool:
    """Write a fully devkit-owned file; returns True when content changed."""
    if path.exists() and path.read_text(encoding="utf-8") == content:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    return True


# ── Source discovery (shared by every adapter) ─────────────────────────────


def _link_target(source: Path, link: Path) -> str:
    """Return a stable link target for a project-local or external devkit."""
    source = source.resolve()
    project_root = link.parents[2] if len(link.parents) > 2 else link.parent
    if source == project_root or project_root in source.parents:
        return os.path.relpath(source, link.parent)
    return str(source)


def _is_managed(link: Path, devkit: Path) -> bool:
    """Check whether a symlink resolves into the devkit root."""
    if not link.is_symlink():
        return False
    try:
        target = (link.parent / os.readlink(link)).resolve()
        devkit = devkit.resolve()
        return target == devkit or devkit in target.parents
    except OSError:
        return False


def _bridge(source: Path, link: Path, devkit: Path) -> bool:
    """Create or repair one managed bridge without touching user resources."""
    expected = source.resolve()
    if link.is_symlink():
        try:
            if link.resolve() == expected:
                return False
        except OSError:
            pass
        if not _is_managed(link, devkit):
            return False
        link.unlink()
    elif link.exists():
        return False

    link.parent.mkdir(parents=True, exist_ok=True)
    target = _link_target(source, link)
    try:
        link.symlink_to(target, target_is_directory=source.is_dir())
    except (NotImplementedError, OSError):
        # ponytail: filesystems without symlink support use a non-managed copy;
        # add metadata only if copy-based ownership must later be synchronized.
        import shutil

        if source.is_dir():
            shutil.copytree(source, link)
        else:
            shutil.copy2(source, link)
    return True


def _prune_stale(dest_dir: Path, keep: set[str], devkit: Path) -> None:
    """Remove stale managed bridges while preserving user-owned entries."""
    if not dest_dir.exists():
        return
    for item in dest_dir.iterdir():
        if item.name not in keep and _is_managed(item, devkit):
            item.unlink()


def _register_skill_source(sources: dict[str, Path], name: str, path: Path) -> None:
    """Record a skill ID, warning when a later source collides with an earlier one."""
    if name in sources:
        log_warn(
            f"Skill ID '{name}' is defined in multiple locations: "
            f"{sources[name]} and {path}. Using the first one."
        )
    else:
        sources[name] = path


def _is_skill_source_path(relative_parts: tuple[str, ...]) -> bool:
    """Reject hidden directories while allowing the ``.claude/skills`` layout."""
    for index, part in enumerate(relative_parts):
        if not part.startswith("."):
            continue
        following = relative_parts[index + 1] if index + 1 < len(relative_parts) else ""
        if part == ".claude" and following == "skills":
            continue
        return False
    return True


def discover_skill_sources(*source_dirs: Path) -> dict[str, Path]:
    """Return skill IDs and package directories in source precedence order.

    Agent Skills roots are recursive in Pi/OMP, which also exposes grouped
    built-ins such as ``game-development/2d-games``. A package may instead
    ship skills under a plugin layout (``.claude/skills/*`` or ``skills/*``);
    the outer package directory is not itself a skill in that case.
    """
    sources: dict[str, Path] = {}
    for source_dir in source_dirs:
        if not source_dir.is_dir():
            continue
        skill_dirs = sorted(
            {path.parent for path in source_dir.rglob("SKILL.md")},
            key=lambda path: str(path.relative_to(source_dir)),
        )
        for skill_dir in skill_dirs:
            if not _is_skill_source_path(skill_dir.relative_to(source_dir).parts):
                continue
            _register_skill_source(sources, skill_dir.name, skill_dir)
    return sources


def read_skill_metadata(skill_md: Path) -> tuple[str, str]:
    """Return ``(name, description)`` from SKILL.md frontmatter.

    ponytail: line-based parse only; upgrade to a YAML parser if a skill needs
    block scalars or nested metadata.
    """
    fallback = skill_md.parent.name
    try:
        lines = skill_md.read_text(encoding="utf-8").splitlines()
    except (OSError, UnicodeDecodeError):
        return fallback, ""
    if not lines or lines[0].strip() != "---":
        return fallback, ""

    name = description = ""
    for line in lines[1:]:
        if line.strip() == "---":
            break
        key, separator, value = line.partition(":")
        if not separator:
            continue
        key = key.strip()
        value = value.strip().strip("\"'")
        if key == "name" and not name:
            name = value
        elif key == "description" and not description:
            description = value
    return name or fallback, description


def skill_index(
    devkit: Path,
    root: Path | None = None,
    projected_dir: str | None = None,
) -> str:
    """Render skills table shared by instruction-file adapters."""
    sources = discover_skill_sources(devkit / ".agent" / "skills", devkit / "skills.d")
    if not sources:
        return "_No skills discovered in this devkit._\n"

    rows = ["| Skill | When to use | Read |", "|---|---|---|"]
    for name, source in sorted(sources.items()):
        _, description = read_skill_metadata(source / "SKILL.md")
        if projected_dir is not None:
            rel_path = f"{projected_dir}/{name}/SKILL.md"
        elif root is not None:
            try:
                rel_path = os.path.relpath(
                    source.resolve() / "SKILL.md", root.resolve()
                )
            except ValueError:
                rel_path = f".agent/skills/{name}/SKILL.md"
        else:
            rel_path = f".agent/skills/{name}/SKILL.md"
        rows.append(f"| `{name}` | {description} | `{rel_path}` |")
    return "\n".join(rows) + "\n"


# ── Adapters ───────────────────────────────────────────────────────────────


class HarnessAdapter(ABC):
    """Project devkit sources into one harness's discovery layout."""

    name: str = ""

    def __init__(self, root: Path, devkit: Path) -> None:
        self.root = root
        self.devkit = devkit

    @property
    def agent_sources(self) -> dict[str, Path]:
        source_dir = self.devkit / ".agent" / "agents"
        if not source_dir.is_dir():
            return {}
        return {path.name: path for path in sorted(source_dir.glob("*.md"))}

    @property
    def skill_sources(self) -> dict[str, Path]:
        return discover_skill_sources(
            self.devkit / ".agent" / "skills", self.devkit / "skills.d"
        )

    def bridge_tree(self, sources: dict[str, Path], dest: Path) -> int:
        """Symlink every source into *dest*, pruning stale managed entries."""
        changed = 0
        for name, source in sorted(sources.items()):
            if _bridge(source, dest / name, self.devkit):
                changed += 1
        _prune_stale(dest, set(sources), self.devkit)
        return changed

    @abstractmethod
    def sync(self) -> int:
        """Project sources; return changed artifact count."""


class ClaudeCodeAdapter(HarnessAdapter):
    """Claude Code skill and agent bridges plus ``CLAUDE.md``."""

    name = "claude"

    def sync(self) -> int:
        changed = self.bridge_tree(self.agent_sources, self.root / ".claude" / "agents")
        changed += self.bridge_tree(
            self.skill_sources, self.root / ".claude" / "skills"
        )
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root)
        )
        if write_managed_block(self.root / "CLAUDE.md", body):
            changed += 1
        return changed


class AgentsFileAdapter(HarnessAdapter):
    """Harnesses that load project-root ``AGENTS.md`` instructions."""

    name = "codex"
    instruction_file = "AGENTS.md"

    def sync(self) -> int:
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root)
        )
        return int(write_managed_block(self.root / self.instruction_file, body))


class CursorAdapter(HarnessAdapter):
    """Cursor project rules under ``.cursor/rules``."""

    name = "cursor"

    def sync(self) -> int:
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root)
        )
        content = (
            "---\n"
            "description: LeeDevKit project context and skill index\n"
            "alwaysApply: true\n"
            "---\n\n" + body
        )
        return int(
            write_managed_file(
                self.root / ".cursor" / "rules" / "leedevkit.mdc", content
            )
        )


class GeminiAdapter(HarnessAdapter):
    """Gemini instruction file with inlined context and skill index."""

    name = "gemini"

    def sync(self) -> int:
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root)
        )
        return int(write_managed_block(self.root / "GEMINI.md", body))


class OmpAdapter(HarnessAdapter):
    """Oh My Pi resources plus native project context."""

    name = "omp"

    def sync(self) -> int:
        changed = self.bridge_tree(self.agent_sources, self.root / ".omp" / "agents")
        changed += self.bridge_tree(self.skill_sources, self.root / ".omp" / "skills")
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root, ".omp/skills")
        )
        if write_managed_block(self.root / ".omp" / "AGENTS.md", body):
            changed += 1
        return changed


class PiAdapter(HarnessAdapter):
    """Pi coding agent resources using its native project skill root."""

    name = "pi"

    def sync(self) -> int:
        changed = self.bridge_tree(self.skill_sources, self.root / ".pi" / "skills")
        body = (
            BASE_CONTEXT
            + "\n### Available skills\n\n"
            + skill_index(self.devkit, self.root, ".pi/skills")
        )
        if write_managed_block(self.root / "AGENTS.md", body):
            changed += 1
        return changed


_ADAPTERS: dict[str, type[HarnessAdapter]] = {
    adapter.name: adapter
    for adapter in (
        ClaudeCodeAdapter,
        AgentsFileAdapter,
        CursorAdapter,
        GeminiAdapter,
        OmpAdapter,
        PiAdapter,
    )
}


# Claude remains backward-compatible default. Other adapters activate when their
# marker exists or when explicitly listed in ``[ai].harnesses``.
DEFAULT_HARNESSES = ("claude",)

HARNESS_ALIASES = {
    "oh-my-pi": "omp",
    "ohmypi": "omp",
    "pi-coding-agent": "pi",
}


def detect_harnesses(root: Path) -> list[str]:
    """Return harnesses already present in project layout."""
    markers = {
        "claude": (".claude", "CLAUDE.md"),
        "codex": ("AGENTS.md",),
        "cursor": (".cursor", ".cursorrules"),
        "gemini": ("GEMINI.md",),
        "omp": (".omp",),
        "pi": (".pi",),
    }
    return [
        name
        for name, paths in markers.items()
        if any((root / path).exists() for path in paths)
    ]


def resolve_harnesses(root: Path, cfg: dict | None = None) -> list[str]:
    """Resolve adapters: explicit list wins, else default plus detected."""
    configured = (cfg or {}).get("ai", {}).get("harnesses")
    if isinstance(configured, list) and configured:
        selected = [
            HARNESS_ALIASES.get(name, name)
            for name in configured
            if isinstance(name, str)
        ]
    else:
        selected = [*DEFAULT_HARNESSES, *detect_harnesses(root)]

    unknown = sorted({name for name in selected if name not in _ADAPTERS})
    if unknown:
        log_warn(
            f"Unknown harness adapter(s): {', '.join(unknown)}. "
            f"Known: {', '.join(ADAPTER_NAMES)}"
        )
    return [name for name in dict.fromkeys(selected) if name in _ADAPTERS]


def sync_harnesses(root: Path, devkit: Path, cfg: dict | None = None) -> dict[str, int]:
    """Run every resolved adapter; return ``{harness: changed_count}``."""
    report: dict[str, int] = {}
    for name in resolve_harnesses(root, cfg):
        report[name] = _ADAPTERS[name](root, devkit).sync()
    return report
