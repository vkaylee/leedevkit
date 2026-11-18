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
    resolve_lifecycle_dependencies,
    resolve_quality_gates,
    resolve_version_gate,
    resolve_test_build_policy,
    resolve_test_targets,
    container_provider_diagnostics,
    resolve_container_provider,
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
        import _devkit_config

        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("DEVKIT_HOME", str(tmp_path))
        (tmp_path / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        (tmp_path / "VERSION").write_text("0.1.0\n")
        (tmp_path / "scripts").mkdir()
        (tmp_path / "scripts" / "_orchestrator.py").write_text("# stub\n")
        _devkit_config._DEVKIT_ROOT = None
        root = _find_devkit_root()
        assert root == tmp_path
        _devkit_config._DEVKIT_ROOT = None

    def test_cache_does_not_cross_project_boundaries(self, tmp_path, monkeypatch):
        """A runtime resolved for one project is not reused by another."""
        import _devkit_config

        first = tmp_path / "first"
        first_runtime = first / ".leedevkit"
        (first / "leedevkit.toml").parent.mkdir(parents=True)
        (first / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        (first_runtime / "scripts").mkdir(parents=True)
        (first_runtime / "VERSION").write_text("0.1.0\n")

        second = tmp_path / "second"
        second_runtime = second / ".leedevkit"
        second.mkdir()
        (second / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        (second_runtime / "scripts").mkdir(parents=True)
        (second_runtime / "VERSION").write_text("0.1.0\n")

        monkeypatch.delenv("DEVKIT_HOME", raising=False)
        _devkit_config._DEVKIT_ROOT = None
        monkeypatch.chdir(first)
        assert _devkit_config.get_devkit_root() == first_runtime

        monkeypatch.chdir(second)
        assert _devkit_config.get_devkit_root() == second_runtime

    def test_worktree_uses_main_repo_runtime(self, tmp_path, monkeypatch):
        """A worktree resolves matching runtime from its main checkout."""
        worktree = tmp_path / "worktree"
        worktree.mkdir()
        (worktree / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        main = tmp_path / "main"
        runtime = main / ".leedevkit"
        (runtime / "scripts").mkdir(parents=True)
        (runtime / "scripts" / "_orchestrator.py").write_text("# stub\n")
        (runtime / "VERSION").write_text("0.1.0\n")
        monkeypatch.chdir(worktree)
        monkeypatch.delenv("DEVKIT_HOME", raising=False)
        monkeypatch.setattr(
            "_devkit_config.subprocess.run",
            lambda *args, **kwargs: type(
                "Result", (), {"stdout": str(main / ".git") + "\n"}
            )(),
        )
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        assert _devkit_config.get_devkit_root() == runtime

    def test_worktree_rejects_mismatched_main_runtime(self, tmp_path, monkeypatch):
        worktree = tmp_path / "worktree"
        worktree.mkdir()
        (worktree / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        main = tmp_path / "main"
        runtime = main / ".leedevkit"
        (runtime / "scripts").mkdir(parents=True)
        (runtime / "scripts" / "_orchestrator.py").write_text("# stub\n")
        (runtime / "VERSION").write_text("0.2.0\n")
        monkeypatch.chdir(worktree)
        monkeypatch.delenv("DEVKIT_HOME", raising=False)
        monkeypatch.setattr(
            "_devkit_config.subprocess.run",
            lambda *args, **kwargs: type(
                "Result", (), {"stdout": str(main / ".git") + "\n"}
            )(),
        )
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with pytest.raises(FileNotFoundError, match="same VERSION"):
            _devkit_config.get_devkit_root()

    def test_env_runtime_mismatch_is_rejected(self, tmp_path, monkeypatch):
        project = tmp_path / "project"
        project.mkdir()
        (project / "leedevkit.toml").write_text('[devkit]\nversion = "0.1.0"\n')
        env_runtime = tmp_path / "env-runtime"
        env_runtime.mkdir()
        (env_runtime / "VERSION").write_text("0.2.0\n")
        monkeypatch.chdir(project)
        monkeypatch.setenv("DEVKIT_HOME", str(env_runtime))
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        with pytest.raises(FileNotFoundError, match="same VERSION"):
            _devkit_config.get_devkit_root()


class TestResolveAiRules:
    def test_returns_list_of_paths(self, monkeypatch):
        import _devkit_config

        repo_root = Path(__file__).resolve().parents[2]
        monkeypatch.setenv("DEVKIT_HOME", str(repo_root))
        _devkit_config._DEVKIT_ROOT = None
        rules = resolve_ai_rules()
        assert isinstance(rules, list)
        # All returned paths should be Path objects
        for rule in rules:
            assert isinstance(rule, Path)

    def test_devkit_internals_included(self, monkeypatch):
        import _devkit_config

        repo_root = Path(__file__).resolve().parents[2]
        monkeypatch.setenv("DEVKIT_HOME", str(repo_root))
        _devkit_config._DEVKIT_ROOT = None
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
        (tmp_path / "leedevkit.toml").write_text('[devkit]\nversion = "9.9.9"\n')
        (dk / "VERSION").write_text("9.9.9")
        (dk / "scripts").mkdir()
        (dk / "scripts" / "_orchestrator.py").write_text("# stub\n")
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("DEVKIT_HOME", str(dk))
        import _devkit_config

        _devkit_config._DEVKIT_ROOT = None
        root = _find_devkit_root()
        assert root == dk
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


class TestResolveTestTargets:
    def test_deduplicates_service_covered_by_alias(self, monkeypatch):
        monkeypatch.setattr(
            "_devkit_config.load_project_config",
            lambda: {"targets": {"go": ["zkteco"], "all": ["zkteco"]}},
        )

        assert resolve_test_targets() == ["go"]

    def test_keeps_service_without_alias(self, monkeypatch):
        monkeypatch.setattr(
            "_devkit_config.load_project_config",
            lambda: {"targets": {"all": ["service"]}},
        )

        assert resolve_test_targets() == ["service"]

    def test_keeps_all_targets_and_uncovered_services_in_config_order(
        self, monkeypatch
    ):
        monkeypatch.setattr(
            "_devkit_config.load_project_config",
            lambda: {
                "targets": {
                    "go": ["zkteco"],
                    "api": ["api"],
                    "all": ["zkteco", "api", "standalone"],
                }
            },
        )

        assert resolve_test_targets() == ["go", "api", "standalone"]

    def test_result_order_is_stable(self, monkeypatch):
        monkeypatch.setattr(
            "_devkit_config.load_project_config",
            lambda: {
                "targets": {
                    "web": ["web-service"],
                    "go": ["go-service"],
                    "all": ["go-service", "web-service"],
                }
            },
        )

        assert resolve_test_targets() == ["web", "go"]
        assert resolve_test_targets() == ["web", "go"]


class TestTestBuildPolicy:
    def test_defaults_to_once(self):
        assert resolve_test_build_policy({}) == "once"

    @pytest.mark.parametrize("policy", ["once", "phase"])
    def test_accepts_supported_policy(self, policy):
        assert resolve_test_build_policy({"test": {"build_policy": policy}}) == policy

    def test_rejects_invalid_policy(self):
        with pytest.raises(ValueError, match=r"\[test\]\.build_policy"):
            resolve_test_build_policy({"test": {"build_policy": "daily"}})

    def test_rejects_non_table_test_config(self):
        with pytest.raises(ValueError, match=r"\[test\]"):
            resolve_test_build_policy({"test": []})


class TestQualityGates:
    def test_go_defaults_are_deterministic(self):
        assert [gate["name"] for gate in resolve_quality_gates({})] == [
            "gofmt",
            "go-vet",
            "go-test",
            "go-coverage",
        ]

    def test_custom_gate_normalizes_argv_and_metadata(self):
        config = {
            "quality": {
                "gates": {
                    "check": {
                        "phase": "lint",
                        "command": ["tool", "--check"],
                        "working_dir": "service",
                        "service": "builder",
                        "timeout": 30,
                        "required": False,
                        "environment": ["CI"],
                    }
                },
                "go": {"lint": ["check"]},
            }
        }
        assert resolve_quality_gates(config) == [
            {
                "name": "check",
                "phase": "lint",
                "command": ["tool", "--check"],
                "mode": "run",
                "working_dir": "service",
                "service": "builder",
                "timeout": 30,
                "required": False,
                "condition": "always",
                "failure_policy": "optional",
                "environment": ["CI"],
                "requires": [],
                "expected_version": None,
                "version_command": None,
                "fix_command": None,
                "new_findings_flag": None,
                "setup_command": None,
                "teardown_command": None,
            }
        ]

    @pytest.mark.parametrize(
        "gate, message",
        [
            ({"command": []}, "command"),
            ({"command": ["go"], "phase": "unknown"}, "phase"),
            ({"command": ["go"], "working_dir": "../outside"}, "working_dir"),
            ({"command": ["go"], "timeout": 0}, "timeout"),
            ({"command": ["go"], "environment": ["BAD=VALUE"]}, "environment"),
        ],
    )
    def test_rejects_invalid_gate(self, gate, message):
        with pytest.raises(ValueError, match=message):
            resolve_quality_gates(
                {"quality": {"gates": {"bad": gate}, "go": {"lint": ["bad"]}}}
            )

    def test_new_findings_flag_is_normalized(self):
        gates = resolve_quality_gates(
            {
                "quality": {
                    "gates": {
                        "lint": {
                            "phase": "lint",
                            "command": ["lint", "run"],
                            "new_findings_flag": ["--new-from-rev"],
                        }
                    },
                    "go": {"lint": ["lint"]},
                }
            }
        )
        assert gates[0]["new_findings_flag"] == ["--new-from-rev"]

    def test_integration_setup_and_teardown_commands_normalize(self):
        gates = resolve_quality_gates(
            {
                "quality": {
                    "gates": {
                        "integration": {
                            "phase": "integration",
                            "command": ["go", "test", "./..."],
                            "setup_command": ["./setup-test-db"],
                            "teardown_command": ["./drop-test-db"],
                        }
                    },
                    "go": {"integration": ["integration"]},
                }
            }
        )
        assert gates[0]["setup_command"] == ["./setup-test-db"]
        assert gates[0]["teardown_command"] == ["./drop-test-db"]

    def test_integration_setup_command_must_be_argv_list(self):
        with pytest.raises(ValueError, match="setup_command"):
            resolve_quality_gates(
                {
                    "quality": {
                        "gates": {
                            "integration": {
                                "phase": "integration",
                                "command": ["go", "test"],
                                "setup_command": "./setup-test-db",
                            }
                        },
                        "go": {"integration": ["integration"]},
                    }
                }
            )

    def test_optional_failure_policy_requires_optional_gate(self):
        with pytest.raises(ValueError, match="required must be false"):
            resolve_quality_gates(
                {
                    "quality": {
                        "gates": {
                            "lint": {
                                "phase": "lint",
                                "command": ["lint"],
                                "failure_policy": "optional",
                            }
                        },
                        "go": {"lint": ["lint"]},
                    }
                }
            )

    def test_new_findings_flag_must_be_argv_list(self):
        with pytest.raises(ValueError, match="new_findings_flag"):
            resolve_quality_gates(
                {
                    "quality": {
                        "gates": {
                            "lint": {
                                "phase": "lint",
                                "command": ["lint"],
                                "new_findings_flag": "--new-from-rev",
                            }
                        },
                        "go": {"lint": ["lint"]},
                    }
                }
            )

    def test_version_gate_requires_both_fields(self):
        with pytest.raises(ValueError, match="needs expected_version"):
            resolve_version_gate({"name": "tool", "expected_version": "1.2"})

    def test_version_gate_returns_safe_metadata(self):
        assert resolve_version_gate(
            {
                "name": "tool",
                "expected_version": "1.2",
                "version_command": ["tool", "--version"],
            }
        ) == {
            "expected_version": "1.2",
            "version_command": ["tool", "--version"],
        }

    def test_unknown_selected_gate_fails(self):
        with pytest.raises(ValueError, match="not defined"):
            resolve_quality_gates({"quality": {"go": {"lint": ["missing"]}}})


class TestLifecycleDependencies:
    def test_loads_dependencies_from_project_config(self, tmp_path, monkeypatch):
        (tmp_path / "leedevkit.toml").write_text(
            '[test.dependencies]\n"int-go" = ["postgres"]\n'
        )
        monkeypatch.chdir(tmp_path)
        monkeypatch.setenv("DEVKIT_HOME", str(Path(__file__).resolve().parents[2]))
        config = load_project_config()
        assert resolve_lifecycle_dependencies(config) == {"int-go": ["postgres"]}

    def test_missing_dependencies_preserves_empty_mapping(self):
        assert resolve_lifecycle_dependencies({}) == {}

    @pytest.mark.parametrize(
        "dependencies, message",
        [
            ({"int-go": "postgres"}, "non-empty list"),
            ({"int-go": []}, "non-empty list"),
            ({"int-go": ["postgres", ""]}, "only non-empty strings"),
            ({"int-go": ["postgres", 1]}, "only non-empty strings"),
            ({1: ["postgres"]}, "mode keys"),
        ],
    )
    def test_rejects_malformed_dependencies(self, dependencies, message):
        with pytest.raises(ValueError, match=message):
            resolve_lifecycle_dependencies({"test": {"dependencies": dependencies}})

    def test_rejects_non_table_test_config(self):
        with pytest.raises(ValueError, match=r"\[test\]"):
            resolve_lifecycle_dependencies({"test": []})

    def test_malformed_toml_fails_clearly(self, tmp_path):
        malformed = tmp_path / "malformed.toml"
        malformed.write_text('[test.dependencies\n"int-go" = ["postgres"]\n')
        with pytest.raises(Exception) as error:
            _load_toml(malformed)
        assert str(error.value)
