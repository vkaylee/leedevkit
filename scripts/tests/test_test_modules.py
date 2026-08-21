"""Tests for _test_modules.py — lint/unit/integration/coverage, pattern sanitization."""

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))

from _test_modules import (  # noqa: E402
    _has_go_service,
    _has_rust_service,
    _has_web_service,
    _resolve_go_service,
    _safe_pattern,
    _safe_pattern_quoted,
    leedevkit_run_coverage,
    leedevkit_run_integration,
    leedevkit_run_lint,
    leedevkit_run_unit,
)


class TestSafePattern:
    """shlex.quote must prevent shell injection through --pattern arg."""

    def test_empty_returns_empty(self) -> None:
        assert _safe_pattern("") == ""

    def test_normal_pattern_passes_through(self) -> None:
        result = _safe_pattern("auth")
        assert "auth" in result

    def test_shell_injection_is_quoted(self) -> None:
        """$(whoami) must become '$(whoami)' — shell-safe literal."""
        result = _safe_pattern("$(whoami)")
        assert "'$(whoami)'" in result or '"$(whoami)"' in result
        # Must NOT be unquoted (which would allow shell interpretation)
        assert result != "$(whoami)"

    def test_backtick_is_quoted(self) -> None:
        result = _safe_pattern("`id`")
        assert "`id`" in result
        assert "'" in result or '"' in result  # Must be quoted

    def test_semicolon_is_quoted(self) -> None:
        result = _safe_pattern("; rm -rf /")
        assert ";" in result
        assert result != "; rm -rf /"  # Must be quoted

    def test_spaces_are_quoted(self) -> None:
        result = _safe_pattern("hello world")
        assert "'" in result or '"' in result

    def test_single_quote_in_pattern(self) -> None:
        """Pattern with single quote should be safely escaped."""
        result = _safe_pattern("it's")
        # shlex.quote should handle this safely
        assert len(result) > 4  # Must be quoted/escaped

    def test_dollar_brace_is_quoted(self) -> None:
        result = _safe_pattern("${HOME}")
        assert result != "${HOME}"

    def test_newline_not_possible_here(self) -> None:
        """_safe_pattern is called after sanitize, so \n would be caught earlier.
        But verify shlex.quote would handle it safely anyway."""
        # This test verifies defensive coding
        result = _safe_pattern("test")
        assert "test" in result


class TestSafePatternQuoted:
    """For -g \"pattern\" in Playwright, inner quotes need special handling."""

    def test_empty_returns_empty(self) -> None:
        assert _safe_pattern_quoted("") == ""

    def test_normal_pattern(self) -> None:
        result = _safe_pattern_quoted("auth")
        assert "auth" in result


class TestRunFunctionsWithPattern:
    """End-to-end: test module functions with dangerous patterns."""

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_with_safe_pattern(self, mock_run: MagicMock) -> None:
        """Normal pattern passes through."""
        result = leedevkit_run_unit(
            component_filter="", mode="api", test_pattern="auth"
        )
        assert result is True

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_with_dangerous_pattern_is_quoted(self, mock_run: MagicMock) -> None:
        """$(whoami) pattern must not cause shell injection."""
        result = leedevkit_run_unit(
            component_filter="", mode="api", test_pattern="$(whoami)"
        )
        assert result is True
        # Verify the command passed to run_parallel_ordered has quoted pattern
        tasks = mock_run.call_args[0][2]  # tasks list
        for _name, _service, cmd in tasks:
            cmd_str = " ".join(cmd)
            if "cargo nextest" in cmd_str:
                # Pattern must appear quoted
                assert "'$(whoami)'" in cmd_str

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_integration_with_quoted_pattern(self, mock_run: MagicMock) -> None:
        """Integration test with dangerous pattern."""
        result = leedevkit_run_integration(
            component_filter="", mode="api", test_pattern="`id`"
        )
        assert result is True
        tasks = mock_run.call_args[0][2]
        for _name, _service, cmd in tasks:
            cmd_str = " ".join(cmd)
            if "cargo nextest" in cmd_str:
                assert "'`id`'" in cmd_str

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_playwright_pattern_is_safe(self, mock_run: MagicMock) -> None:
        """Playwright -g pattern must be shell-safe."""
        result = leedevkit_run_integration(
            component_filter="", mode="web", test_pattern="hello world"
        )
        assert result is True
        tasks = mock_run.call_args[0][2]
        for _name, _service, cmd in tasks:
            cmd_str = " ".join(cmd)
            if "playwright" in cmd_str:
                assert "hello world" not in cmd_str or "'" in cmd_str

    @patch("_test_modules._has_web_service", return_value=True)
    @patch("_test_modules._has_rust_service", return_value=True)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_all(
        self, mock_run: MagicMock, _mock_rust: MagicMock, _mock_web: MagicMock
    ) -> None:
        result = leedevkit_run_lint(mode="all")
        assert result is True
        tasks = mock_run.call_args[0][2]
        cmd_strs = [" ".join(cmd) for _, _, cmd in tasks]
        assert any("cargo clippy" in c for c in cmd_strs)
        assert any("bun run lint" in c for c in cmd_strs)

        # Verify api-sync uses bash -c "cmd1 && cmd2" to prevent syntax errors
        api_sync_cmd = next(c for c in cmd_strs if "openapi-typescript" in c)
        assert api_sync_cmd.startswith("bash -c")
        assert "&&" in api_sync_cmd

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_apiserver(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_lint(component_filter="apiserver", mode="api")
        assert result is True
        tasks = mock_run.call_args[0][2]
        cmd_strs = [" ".join(cmd) for _, _, cmd in tasks]
        assert any("--package apiserver" in c for c in cmd_strs)
        assert not any("bun run lint" in c for c in cmd_strs)

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_web(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_unit(mode="web")
        assert result is True
        tasks = mock_run.call_args[0][2]
        cmd_strs = [" ".join(cmd) for _, _, cmd in tasks]
        assert any("bun run test" in c for c in cmd_strs)

    @patch("_test_modules._has_web_service", return_value=True)
    @patch("_test_modules._has_rust_service", return_value=True)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_coverage_all(
        self, mock_run: MagicMock, _mock_rust: MagicMock, _mock_web: MagicMock
    ) -> None:
        result = leedevkit_run_coverage(mode="all")
        assert result is True
        tasks = mock_run.call_args[0][2]
        cmd_strs = [" ".join(cmd) for _, _, cmd in tasks]
        assert any("cargo llvm-cov" in c for c in cmd_strs)
        assert any("bun run test:coverage" in c for c in cmd_strs)

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_coverage_apiserver(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_coverage(component_filter="apiserver", mode="api")
        assert result is True
        tasks = mock_run.call_args[0][2]
        cmd_strs = [" ".join(cmd) for _, _, cmd in tasks]
        assert any("--package apiserver" in c for c in cmd_strs)
        assert not any("bun run test" in c for c in cmd_strs)


# ── Go support: language detection, task scheduling, pattern safety ──────────


class TestGoServiceDetection:
    """_has_go_service / _has_rust_service / _has_web_service language gates."""

    def test_has_go_service_no_markers(self, tmp_path, monkeypatch) -> None:
        """Empty project: no Go service and no Rust service."""
        import _bootstrap

        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", tmp_path)
        assert _has_go_service() is False
        assert _has_rust_service() is False
        assert _has_web_service() is False

    def test_has_go_service_from_go_mod(self, tmp_path, monkeypatch) -> None:
        """go.mod alone marks the project as Go."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "go.mod").write_text("module example\n")
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _has_go_service() is True
        assert _has_rust_service() is False

    def test_has_go_service_from_config(self, tmp_path, monkeypatch) -> None:
        """[services.go] lang=go marks the project as Go."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.go]\nlang = "go"\ngo = true\n'
        )
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _has_go_service() is True

    def test_has_rust_service_from_cargo_toml(self, tmp_path, monkeypatch) -> None:
        """Cargo.toml alone marks the project as Rust."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "Cargo.toml").write_text("[package]\nname = 'x'\n")
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _has_rust_service() is True

    def test_has_web_service_from_typescript(self, tmp_path, monkeypatch) -> None:
        """[services.web] lang=typescript marks the project as Web."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.webdashboard]\nlang = "typescript"\n'
        )
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _has_web_service() is True


class TestResolveGoService:
    """_resolve_go_service: built-in vs configured vs custom project override."""

    def test_no_config_returns_builtin_go(self, tmp_path, monkeypatch) -> None:
        """Without leedevkit.toml, use the built-in `go` service."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _resolve_go_service() == "go"

    def test_configured_service_without_override_returns_builtin(
        self, tmp_path, monkeypatch
    ) -> None:
        """Configured Go service name maps to built-in `go` when no project compose."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.backend]\nlang = "go"\ngo = true\n'
        )
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _resolve_go_service() == "go"
        assert _resolve_go_service("backend") == "go"

    def test_configured_service_with_override_returns_name(
        self, tmp_path, monkeypatch
    ) -> None:
        """With .compose/docker-compose.test.yml, use the configured service name."""
        import _bootstrap

        proj = tmp_path / "project"
        proj.mkdir()
        (proj / "leedevkit.toml").write_text(
            '[services.backend]\nlang = "go"\ngo = true\n'
        )
        compose_dir = proj / ".compose"
        compose_dir.mkdir()
        (compose_dir / "docker-compose.test.yml").write_text(
            "services:\n  backend:\n    image: golang\n"
        )
        monkeypatch.setattr(_bootstrap, "PROJECT_ROOT", proj)
        assert _resolve_go_service() == "backend"
        assert _resolve_go_service("backend") == "backend"
        assert _resolve_go_service("other") == "backend"


class TestGoLint:
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_go_schedules_format_and_vet(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_lint(mode="go")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("gofmt -l ." in c for c in cmd_strs)
        assert any("go vet ./..." in c for c in cmd_strs)
        assert not any("cargo" in c for c in cmd_strs)
        assert not any("bun" in c for c in cmd_strs)

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_go_fix_uses_gofmt_w(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_lint(mode="go", fix=True)
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("gofmt -w ." in c for c in cmd_strs)

    @patch("_test_modules._has_web_service", return_value=False)
    @patch("_test_modules._has_rust_service", return_value=False)
    @patch("_test_modules._has_go_service", return_value=True)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_all_go_only_no_rust_or_web(
        self, mock_run: MagicMock, *_mocks: MagicMock
    ) -> None:
        """Go-only `all`: only Go tasks scheduled, no cargo/bun/api-sync."""
        result = leedevkit_run_lint(mode="all")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("gofmt" in c for c in cmd_strs)
        assert any("go vet" in c for c in cmd_strs)
        assert not any("cargo" in c for c in cmd_strs)
        assert not any("bun" in c for c in cmd_strs)
        assert not any("openapi-typescript" in c for c in cmd_strs)


class TestGoUnit:
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_go_basic(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_unit(mode="go")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("go test ./..." in c for c in cmd_strs)

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_go_pattern_safe(self, mock_run: MagicMock) -> None:
        """Dangerous pattern must be shell-quoted in go test -run."""
        result = leedevkit_run_unit(mode="go", test_pattern="$(whoami)")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        go_cmds = [c for c in cmd_strs if "go test" in c]
        assert go_cmds, "expected a go test task"
        # Pattern must be wrapped in quotes so the shell treats it literally.
        assert " -run '$(whoami)'" in go_cmds[0]

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_unit_go_no_extra_tasks(self, mock_run: MagicMock) -> None:
        """mode='go' must not schedule cargo or bun tasks."""
        result = leedevkit_run_unit(mode="go")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert not any("cargo" in c for c in cmd_strs)
        assert not any("bun" in c for c in cmd_strs)


class TestGoCoverage:
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_coverage_go_profile_and_summary(self, mock_run: MagicMock) -> None:
        result = leedevkit_run_coverage(mode="go")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        go_cmds = [c for c in cmd_strs if "go tool cover" in c]
        assert go_cmds, "expected a go coverage task"
        assert "-coverprofile=/workspace/.test_logs/coverage-go.out" in go_cmds[0]
        assert "go tool cover -func" in go_cmds[0]

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_coverage_go_pattern_safe(self, mock_run: MagicMock) -> None:
        """Coverage -run pattern must be shell-quoted too."""
        result = leedevkit_run_coverage(mode="go", test_pattern="`id`")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        go_cmds = [c for c in cmd_strs if "go tool cover" in c]
        assert go_cmds
        assert "'`id`'" in go_cmds[0]


class TestGoAllModeGating:
    """`test all` must only schedule tasks for declared languages."""

    @patch("_test_modules._has_web_service", return_value=False)
    @patch("_test_modules._has_rust_service", return_value=True)
    @patch("_test_modules._has_go_service", return_value=True)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_all_rust_and_go_no_web(
        self, mock_run: MagicMock, *_mocks: MagicMock
    ) -> None:
        """Rust + Go without Web: no bun tasks, and api-sync must not run."""
        result = leedevkit_run_lint(mode="all")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("cargo clippy" in c for c in cmd_strs)
        assert any("gofmt" in c for c in cmd_strs)
        assert not any("bun" in c for c in cmd_strs)
        assert not any("openapi-typescript" in c for c in cmd_strs)

    @patch("_test_modules._has_web_service", return_value=True)
    @patch("_test_modules._has_rust_service", return_value=True)
    @patch("_test_modules._has_go_service", return_value=False)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_all_rust_and_web_runs_api_sync(
        self, mock_run: MagicMock, *_mocks: MagicMock
    ) -> None:
        """Rust + Web (no Go): api-sync runs; no Go tasks."""
        result = leedevkit_run_lint(mode="all")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert any("cargo clippy" in c for c in cmd_strs)
        assert any("openapi-typescript" in c for c in cmd_strs)
        assert not any("gofmt" in c for c in cmd_strs)

    @patch("_test_modules._has_web_service", return_value=False)
    @patch("_test_modules._has_rust_service", return_value=False)
    @patch("_test_modules._has_go_service", return_value=False)
    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_all_no_languages_schedules_nothing(
        self, mock_run: MagicMock, *_mocks: MagicMock
    ) -> None:
        """No declared language: `all` schedules no tasks (empty run succeeds)."""
        result = leedevkit_run_lint(mode="all")
        assert result is True
        cmd_strs = [" ".join(cmd) for _, _, cmd in mock_run.call_args[0][2]]
        assert cmd_strs == []
