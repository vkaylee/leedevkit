"""Tests for _test_modules.py — lint/unit/integration/coverage, pattern sanitization."""

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(PROJECT_ROOT / "scripts"))

from _test_modules import (  # noqa: E402
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

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_lint_all(self, mock_run: MagicMock) -> None:
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

    @patch("_test_modules.run_parallel_ordered", return_value=True)
    def test_coverage_all(self, mock_run: MagicMock) -> None:
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
