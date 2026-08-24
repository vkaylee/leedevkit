"""Claude Code compatibility checks for agents, skills, and rulebooks."""

from __future__ import annotations

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
AGENTS = ROOT / ".agent" / "agents"
SKILLS = ROOT / ".agent" / "skills"
SCAN_ROOTS = (ROOT / ".agent", ROOT / "templates", ROOT / "scripts")
VALID_TOOLS = {"Read", "Write", "Edit", "Bash", "Grep", "Glob"}
REQUIRED_AGENT_FIELDS = {"name", "description", "tools", "model"}
REQUIRED_SKILL_FIELDS = {"name", "description"}


def _frontmatter(path: Path) -> dict[str, str]:
    lines = path.read_text().splitlines()
    assert lines and lines[0].strip() == "---", f"Missing YAML frontmatter: {path}"
    try:
        end = lines.index("---", 1)
    except ValueError as exc:
        raise AssertionError(f"Unclosed YAML frontmatter: {path}") from exc

    values: dict[str, str] = {}
    for line in lines[1:end]:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, separator, value = line.partition(":")
        assert separator, f"Invalid frontmatter line in {path}: {line!r}"
        values[key.strip()] = value.strip().strip("'\"")
    return values


def _skill_ids() -> set[str]:
    return {
        path.parent.name
        for path in SKILLS.glob("**/SKILL.md")
        if path.parent.name != "_shared"
    }


def _agent_ids() -> set[str]:
    return {path.stem for path in AGENTS.glob("*.md")}


def _referenced_agents() -> set[str]:
    names: set[str] = set()
    roots = (SKILLS, ROOT / "templates", ROOT / ".agent" / "workflows", AGENTS)
    suffixes = (
        "-specialist",
        "-architect",
        "-engineer",
        "-developer",
        "-tester",
        "-auditor",
        "-optimizer",
        "-writer",
        "-owner",
        "-manager",
        "-planner",
        "-designer",
        "-agent",
    )
    for root in roots:
        for path in root.rglob("*.md"):
            text = path.read_text()
            for match in re.findall(r"`([a-z][a-z0-9-]+)`", text):
                if match.endswith(suffixes):
                    names.add(match)
            names.update(
                re.findall(r"subagent_type\s*:\s*[\"']([a-z][a-z0-9-]+)[\"']", text)
            )
    return names


def test_all_agents_have_valid_frontmatter() -> None:
    agents = list(AGENTS.glob("*.md"))
    assert agents
    for path in agents:
        metadata = _frontmatter(path)
        assert REQUIRED_AGENT_FIELDS <= metadata.keys(), path
        assert metadata["name"] == path.stem
        assert metadata["description"]
        assert metadata["model"]


def test_agent_tools_are_valid_claude_code_tools() -> None:
    for path in AGENTS.glob("*.md"):
        tools = {tool.strip() for tool in _frontmatter(path)["tools"].split(",")}
        assert tools <= VALID_TOOLS, f"{path}: {tools - VALID_TOOLS}"


def test_all_skills_have_valid_frontmatter() -> None:
    skills = list(SKILLS.glob("**/SKILL.md"))
    assert skills
    for path in skills:
        metadata = _frontmatter(path)
        assert REQUIRED_SKILL_FIELDS <= metadata.keys(), path
        assert metadata["name"] == path.parent.name
        assert metadata["description"]


def test_routing_agent_references_exist() -> None:
    missing = sorted(_referenced_agents() - _agent_ids())
    assert not missing


def test_agent_skill_references_exist() -> None:
    available = _skill_ids()
    missing: list[str] = []
    for path in AGENTS.glob("*.md"):
        for skill in _frontmatter(path).get("skills", "").split(","):
            skill = skill.strip()
            if skill and skill not in available:
                missing.append(f"{path.name}: {skill}")
    assert not missing


def test_no_legacy_framework_references() -> None:
    patterns = [
        "".join(["Anti", "gravity"]),
        "".join(["Gemini", " ", "CLI"]),
        "".join(["GEMINI", ".md"]),
        "".join(["run_", "command"]),
    ]
    forbidden = re.compile("|".join(patterns))
    violations: list[str] = []
    for root in SCAN_ROOTS:
        for path in root.rglob("*"):
            if path.is_file() and ".git" not in path.parts:
                try:
                    text = path.read_text()
                except UnicodeDecodeError:
                    continue
                if forbidden.search(text):
                    violations.append(str(path.relative_to(ROOT)))
    assert not violations


def test_markdown_python_and_bash_paths_exist() -> None:
    command = re.compile(
        r"\b(?:python3?|bash|\.leedevkit/\.venv/bin/python3)\s+([^\s`]+\.(?:py|sh))"
    )
    # Paths under these prefixes are installed at runtime, not shipped in the repo.
    runtime_prefixes = (".leedevkit/", "skills.d/")
    missing: list[str] = []
    for root in SCAN_ROOTS:
        for path in root.rglob("*.md"):
            for token in command.findall(path.read_text()):
                if token.startswith(("http://", "https://", "<")):
                    continue
                if token.startswith(runtime_prefixes):
                    continue
                candidate = (ROOT / token).resolve()
                if not candidate.is_file():
                    missing.append(f"{path.relative_to(ROOT)}: {token}")
    assert not missing, "Missing executable paths:\n" + "\n".join(sorted(missing))


def test_markdown_python_scripts_use_leedevkit_venv() -> None:
    raw_host_python = re.compile(
        r"(?<!\.leedevkit/\.venv/bin/)\bpython3?\s+([^\s`]+\.py)"
    )
    violations: list[str] = []
    for root in (
        ROOT / ".agent" / "skills",
        ROOT / ".agent" / "agents",
        ROOT / ".agent" / "workflows",
    ):
        for path in root.rglob("*.md"):
            text = path.read_text()
            for match in raw_host_python.finditer(text):
                violations.append(f"{path.relative_to(ROOT)}: {match.group(0)}")
    assert not violations, (
        "Host python commands found in skills/agents/workflows:\n"
        + "\n".join(violations)
    )


def test_claude_resource_bridge_includes_community_skills(tmp_path: Path) -> None:
    import sys

    sys.path.insert(0, str(ROOT / "scripts"))
    from _init_handler import sync_claude_resources

    devkit = tmp_path / "devkit"
    (devkit / ".agent" / "agents").mkdir(parents=True)
    (devkit / ".agent" / "skills" / "builtin").mkdir(parents=True)
    (devkit / "skills.d" / "community").mkdir(parents=True)
    (devkit / ".agent" / "agents" / "general-purpose.md").write_text(
        "---\nname: general-purpose\ndescription: test\ntools: Read\nmodel: inherit\n---\n"
    )
    (devkit / ".agent" / "skills" / "builtin" / "SKILL.md").write_text(
        "---\nname: builtin\ndescription: test\n---\n"
    )
    (devkit / "skills.d" / "community" / "SKILL.md").write_text(
        "---\nname: community\ndescription: test\n---\n"
    )

    assert sync_claude_resources(tmp_path / "project", devkit) == (1, 2)
    assert (
        tmp_path / "project" / ".claude" / "agents" / "general-purpose.md"
    ).is_symlink()
    assert (tmp_path / "project" / ".claude" / "skills" / "builtin").is_symlink()
    assert (tmp_path / "project" / ".claude" / "skills" / "community").is_symlink()


def test_discover_skill_sources_warns_on_duplicate_id(tmp_path: Path, capsys) -> None:
    import sys

    sys.path.insert(0, str(ROOT / "scripts"))
    from _init_handler import discover_skill_sources

    source_dir = tmp_path / "skills"
    (source_dir / "alpha").mkdir(parents=True)
    (source_dir / "alpha" / "SKILL.md").write_text(
        "---\nname: alpha\ndescription: t\n---\n"
    )
    # Plugin under alpha reuses the same skill ID "alpha".
    (source_dir / "alpha" / ".claude" / "skills" / "alpha").mkdir(parents=True)
    (source_dir / "alpha" / ".claude" / "skills" / "alpha" / "SKILL.md").write_text(
        "---\nname: alpha\ndescription: t\n---\n"
    )

    result = discover_skill_sources(source_dir)
    assert set(result) == {"alpha"}
    captured = capsys.readouterr()
    assert "alpha" in captured.err or "alpha" in captured.out
