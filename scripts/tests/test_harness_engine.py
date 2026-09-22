"""Behavior tests for harness-neutral AI context projections."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from _harness_engine import (  # noqa: E402
    MARKER_END,
    MARKER_START,
    detect_harnesses,
    resolve_harnesses,
    sync_harnesses,
)


def _make_devkit(tmp_path: Path) -> Path:
    devkit = tmp_path / ".leedevkit"
    skill = devkit / ".agent" / "skills" / "api-patterns"
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        "---\nname: api-patterns\ndescription: API design guidance\n---\n# API\n"
    )
    agents = devkit / ".agent" / "agents"
    agents.mkdir(parents=True)
    (agents / "backend-specialist.md").write_text("# Backend\n")
    return devkit


def test_explicit_harnesses_project_context_and_preserve_user_content(
    tmp_path: Path,
) -> None:
    root = tmp_path / "project"
    root.mkdir()
    devkit = _make_devkit(tmp_path)
    (root / "AGENTS.md").write_text("# Local rules\n\nKeep this text.\n")

    report = sync_harnesses(
        root,
        devkit,
        {"ai": {"harnesses": ["claude", "codex", "cursor", "gemini"]}},
    )

    assert set(report) == {"claude", "codex", "cursor", "gemini"}
    assert (root / ".claude/skills/api-patterns/SKILL.md").is_file()
    assert (root / "AGENTS.md").read_text().startswith("# Local rules\n")
    assert "Keep this text." in (root / "AGENTS.md").read_text()
    assert (root / ".cursor/rules/leedevkit.mdc").is_file()
    assert (root / "GEMINI.md").is_file()


def test_sync_is_idempotent_and_managed_block_is_single_instance(
    tmp_path: Path,
) -> None:
    root = tmp_path / "project"
    root.mkdir()
    devkit = _make_devkit(tmp_path)
    cfg = {"ai": {"harnesses": ["codex", "gemini"]}}

    first = sync_harnesses(root, devkit, cfg)
    second = sync_harnesses(root, devkit, cfg)

    assert first == {"codex": 1, "gemini": 1}
    assert second == {"codex": 0, "gemini": 0}
    agents = (root / "AGENTS.md").read_text()
    assert agents.count(MARKER_START) == 1
    assert agents.count(MARKER_END) == 1


def test_default_and_detected_harness_resolution_is_compatible(tmp_path: Path) -> None:
    root = tmp_path / "project"
    root.mkdir()
    assert resolve_harnesses(root, {}) == ["claude"]

    (root / ".cursor").mkdir()
    assert "cursor" in detect_harnesses(root)
    assert "cursor" in resolve_harnesses(root, {})


def test_unknown_harness_is_skipped_without_breaking_known_adapter(
    tmp_path: Path,
) -> None:
    root = tmp_path / "project"
    root.mkdir()
    devkit = _make_devkit(tmp_path)

    report = sync_harnesses(
        root, devkit, {"ai": {"harnesses": ["not-installed", "codex"]}}
    )

    assert report == {"codex": 1}
    assert (root / "AGENTS.md").is_file()


def test_omp_adapter_bridges_agents_skills_and_writes_omp_agents_md(
    tmp_path: Path,
) -> None:
    root = tmp_path / "project"
    root.mkdir()
    devkit = _make_devkit(tmp_path)

    report = sync_harnesses(root, devkit, {"ai": {"harnesses": ["omp"]}})

    assert report == {"omp": 3}
    assert (root / ".omp/agents/backend-specialist.md").is_symlink()
    assert (root / ".omp/skills/api-patterns/SKILL.md").is_file()
    assert (root / ".omp/AGENTS.md").is_file()
    assert MARKER_START in (root / ".omp/AGENTS.md").read_text()
    assert (
        "`.omp/skills/api-patterns/SKILL.md`" in (root / ".omp/AGENTS.md").read_text()
    )


def test_pi_adapter_bridges_skills_into_native_skills_and_writes_agents_md(
    tmp_path: Path,
) -> None:
    root = tmp_path / "project"
    root.mkdir()
    devkit = _make_devkit(tmp_path)

    report = sync_harnesses(root, devkit, {"ai": {"harnesses": ["pi"]}})

    assert report == {"pi": 2}
    assert (root / ".pi/skills/api-patterns/SKILL.md").is_file()
    assert (root / "AGENTS.md").is_file()
    assert MARKER_START in (root / "AGENTS.md").read_text()
    assert "`.pi/skills/api-patterns/SKILL.md`" in (root / "AGENTS.md").read_text()


def test_harness_aliases_and_detection(tmp_path: Path) -> None:
    root = tmp_path / "project"
    root.mkdir()

    assert resolve_harnesses(
        root, {"ai": {"harnesses": ["oh-my-pi", "pi-coding-agent"]}}
    ) == [
        "omp",
        "pi",
    ]

    (root / ".omp").mkdir()
    (root / ".pi").mkdir()
    detected = detect_harnesses(root)
    assert "omp" in detected
    assert "pi" in detected
