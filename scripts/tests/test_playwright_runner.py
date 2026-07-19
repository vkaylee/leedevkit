"""Unit tests for standalone Playwright browser provisioning."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import Mock


RUNNER_PATH = (
    Path(__file__).resolve().parents[2]
    / ".agent"
    / "skills"
    / "webapp-testing"
    / "scripts"
    / "playwright_runner.py"
)


def _load_runner():
    spec = importlib.util.spec_from_file_location("playwright_runner", RUNNER_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def test_launch_chromium_does_not_install_when_browser_is_available(monkeypatch):
    runner = _load_runner()
    browser = object()
    chromium = Mock()
    chromium.launch.return_value = browser
    playwright = Mock(chromium=chromium)
    monkeypatch.setattr(runner, "_install_chromium", Mock())

    launched, error = runner._launch_chromium(playwright)

    assert launched is browser
    assert error is None
    runner._install_chromium.assert_not_called()


def test_launch_chromium_installs_and_retries_when_executable_is_missing(monkeypatch):
    runner = _load_runner()
    browser = object()
    chromium = Mock()
    chromium.launch.side_effect = [Exception("Executable doesn't exist"), browser]
    playwright = Mock(chromium=chromium)
    install = Mock(return_value=(True, ""))
    monkeypatch.setattr(runner, "_install_chromium", install)

    launched, error = runner._launch_chromium(playwright)

    assert launched is browser
    assert error is None
    install.assert_called_once_with()
    assert chromium.launch.call_count == 2


def test_launch_chromium_returns_remediation_when_browser_install_fails(monkeypatch):
    runner = _load_runner()
    chromium = Mock()
    chromium.launch.side_effect = Exception("Executable doesn't exist")
    playwright = Mock(chromium=chromium)
    monkeypatch.setattr(
        runner, "_install_chromium", Mock(return_value=(False, "offline"))
    )

    launched, error = runner._launch_chromium(playwright)

    assert launched is None
    assert "automatic setup failed: offline" in error
    assert "playwright install chromium" in error
