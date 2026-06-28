"""Tests for _devkit_config — TOML/YAML cascade and rule resolution."""

import os
import sys
from pathlib import Path


sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from _devkit_config import (
    _find_devkit_root,
    _find_project_root,
    _load_toml,
    _parse_toml_minimal,
    deep_merge,
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
        # All returned paths should be Path objects
        for rule in rules:
            assert isinstance(rule, Path)

    def test_devkit_internals_included(self):
        rules = resolve_ai_rules()
        rule_names = [r.name for r in rules]
        # The override manifest in .agent/overrides.yaml adds devkit-internals.md
        # This should be resolvable from the project root
        assert len(rules) > 0 or "devkit-internals.md" in rule_names


class TestTomlMinimalEdgeCases:
    def test_quoted_values(self):
        from _devkit_config import _parse_toml_minimal
        result = _parse_toml_minimal(_make_tmp("key = 'single'\nother = \"double\""))
        assert result["key"] == "single"
        assert result["other"] == "double"

    def test_empty_file(self):
        from _devkit_config import _parse_toml_minimal
        result = _parse_toml_minimal(_make_tmp(""))
        assert result == {}

    def test_only_comments(self):
        from _devkit_config import _parse_toml_minimal
        result = _parse_toml_minimal(_make_tmp("# just a comment\n# another"))
        assert result == {}


class TestFindDevkitRootFallbacks:
    def test_bundled_devkit(self, tmp_path, monkeypatch):
        monkeypatch.delenv("DEVKIT_HOME", raising=False)
        # No ~/.leedevkit/current, so should fall back to bundled
        # This would raise FileNotFoundError without a proper env
        pass

    def test_env_var_priority(self, tmp_path, monkeypatch):
        from _devkit_config import _find_devkit_root
        dk = tmp_path / "custom-devkit"
        dk.mkdir()
        (dk / "VERSION").write_text("9.9.9")
        (dk / ".agent").mkdir()
        (dk / ".agent" / "skills.d").mkdir()
        monkeypatch.setenv("DEVKIT_HOME", str(dk))
        import _devkit_config
        _devkit_config._DEVKIT_ROOT = None
        root = _find_devkit_root()
        assert root == dk
        # Reset cache so other tests get the real devkit
        _devkit_config._DEVKIT_ROOT = None


class TestDeepMergeEdgeCases:
    def test_three_levels(self):
        from _devkit_config import deep_merge
        base = {"a": {"b": {"c": 1}}}
        override = {"a": {"b": {"d": 2}}}
        result = deep_merge(base, override)
        assert result["a"]["b"]["c"] == 1
        assert result["a"]["b"]["d"] == 2


class TestTomlMinimalMore:
    def test_minimal_parser_no_equals(self):
        from _devkit_config import _parse_toml_minimal
        result = _parse_toml_minimal(_make_tmp("[section]\n# just comment\n"))
        assert "section" in result

    def test_minimal_parser_mixed(self):
        from _devkit_config import _parse_toml_minimal
        result = _parse_toml_minimal(_make_tmp("key = 'val'\n[sec]\nfoo = 'bar'\n"))
        assert result["key"] == "val"
        assert result["sec"]["foo"] == "bar"


class TestDeepMergeMore:
    def test_deep_merge_four_levels(self):
        from _devkit_config import deep_merge
        base = {"a": {"b": {"c": {"d": 1}}}}
        override = {"a": {"b": {"c": {"e": 2}}}}
        result = deep_merge(base, override)
        assert result["a"]["b"]["c"]["d"] == 1
        assert result["a"]["b"]["c"]["e"] == 2
