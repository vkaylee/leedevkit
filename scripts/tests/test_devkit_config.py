"""Tests for _devkit_config — TOML/YAML cascade and rule resolution."""
import os
import sys
from pathlib import Path

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _devkit_config import (
    _find_devkit_root,
    _find_project_root,
    _load_toml,
    _parse_toml_minimal,
    deep_merge,
    load_project_config,
    resolve_ai_rules,
)


class TestDeepMerge:
    def test_simple_override(self):
        base = {"a": 1, "b": 2}
        override = {"b": 3, "c": 4}
        result = deep_merge(base, override)
        assert result == {"a": 1, "b": 3, "c": 4}

    def test_nested_merge(self):
        base = {"top": {"a": 1, "b": 2}}
        override = {"top": {"b": 3, "c": 4}}
        result = deep_merge(base, override)
        assert result == {"top": {"a": 1, "b": 3, "c": 4}}

    def test_override_replaces_non_dict(self):
        base = {"key": {"nested": "old"}}
        override = {"key": "not-a-dict"}
        result = deep_merge(base, override)
        assert result == {"key": "not-a-dict"}

    def test_empty_override(self):
        base = {"a": 1}
        assert deep_merge(base, {}) == {"a": 1}

    def test_empty_base(self):
        assert deep_merge({}, {"a": 1}) == {"a": 1}


class TestTomlMinimalParser:
    def test_flat_keys(self):
        result = _parse_toml_minimal(_make_tmp("key = 'value'\nother = 42"))
        assert result["key"] == "value"
        assert result["other"] == "42"

    def test_sections(self):
        content = "[section]\nfoo = 'bar'\n"
        result = _parse_toml_minimal(_make_tmp(content))
        assert result["section"]["foo"] == "bar"

    def test_nested_sections(self):
        content = "[parent]\n[parent.child]\nkey = 'val'\n"
        result = _parse_toml_minimal(_make_tmp(content))
        # Minimal parser creates flat sections: "[parent]" and "[parent.child]"
        assert "parent" in result
        assert "parent.child" in result
        assert result["parent.child"]["key"] == "val"

    def test_ignore_comments(self):
        content = "# comment\nkey = 'value'\n"
        result = _parse_toml_minimal(_make_tmp(content))
        assert result["key"] == "value"

    def test_empty_lines(self):
        content = "\n\nkey = 'value'\n\n"
        result = _parse_toml_minimal(_make_tmp(content))
        assert result["key"] == "value"


def _make_tmp(content):
    """Write content to a temp file and return its Path."""
    import tempfile
    fd, path = tempfile.mkstemp(suffix=".toml")
    with os.fdopen(fd, "w") as f:
        f.write(content)
    return Path(path)


class TestTomlLoader:
    def test_loads_toml_file(self, tmp_path):
        toml_file = tmp_path / "test.toml"
        toml_file.write_text("key = 'hello'\n[section]\nfoo = 'bar'\n")
        result = _load_toml(toml_file)
        assert result["key"] == "hello"
        assert result["section"]["foo"] == "bar"


class TestFindProjectRoot:
    def test_finds_by_toml(self, tmp_path, monkeypatch):
        (tmp_path / "leedevkit.toml").write_text("[project]\nname = 'test'")
        monkeypatch.chdir(tmp_path)
        root = _find_project_root()
        assert root == tmp_path

    def test_finds_by_git(self, tmp_path, monkeypatch):
        (tmp_path / ".git").mkdir()
        monkeypatch.chdir(tmp_path)
        root = _find_project_root()
        assert root == tmp_path

    def test_walks_up(self, tmp_path, monkeypatch):
        (tmp_path / "leedevkit.toml").write_text("[project]")
        subdir = tmp_path / "deep" / "subdir"
        subdir.mkdir(parents=True)
        monkeypatch.chdir(subdir)
        root = _find_project_root()
        assert root == tmp_path


class TestFindDevkitRoot:
    def test_from_env_var(self, tmp_path, monkeypatch):
        monkeypatch.setenv("DEVKIT_HOME", str(tmp_path))
        root = _find_devkit_root()
        assert root == tmp_path


class TestResolveAiRules:
    def test_returns_list_of_paths(self):
        rules = resolve_ai_rules()
        assert isinstance(rules, list)
        assert len(rules) > 0
        # All returned paths should be Path objects
        for rule in rules:
            assert isinstance(rule, Path)

    def test_devkit_internals_included(self):
        rules = resolve_ai_rules()
        rule_names = [r.name for r in rules]
        assert "devkit-internals.md" in rule_names
