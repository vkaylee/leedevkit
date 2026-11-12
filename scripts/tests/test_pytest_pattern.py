"""Tests for explicit pytest collection filtering used by infra pattern runs."""

from __future__ import annotations

from types import SimpleNamespace

import pytest

from _pytest_pattern import (
    pytest_collection_modifyitems,
    pytest_configure,
    pytest_sessionfinish,
)


class _Hook:
    def __init__(self) -> None:
        self.deselected: list[object] = []

    def pytest_deselected(self, *, items: list[object]) -> None:
        self.deselected.extend(items)


def _config(pattern: str | None) -> SimpleNamespace:
    config = SimpleNamespace(
        getoption=lambda _name: pattern,
        hook=_Hook(),
    )
    pytest_configure(config)
    return config


def test_collection_filter_matches_node_ids_and_preserves_selected_skips() -> None:
    config = _config(r"auth|test_login\[slow\]")
    matching = SimpleNamespace(nodeid="tests/test_auth.py::test_login[slow]")
    skipped = SimpleNamespace(nodeid="tests/test_auth.py::test_logout")
    other = SimpleNamespace(nodeid="tests/test_users.py::test_profile")
    items = [matching, skipped, other]

    pytest_collection_modifyitems(config, items)

    assert items == [matching, skipped]
    assert config._leedevkit_pattern_matches == 2
    assert config.hook.deselected == [other]


def test_invalid_regex_raises_clean_pytest_usage_error() -> None:
    with pytest.raises(pytest.UsageError, match="Invalid --pattern regex"):
        pytest_configure(SimpleNamespace(getoption=lambda _name: "[", hook=_Hook()))


def test_zero_matches_set_failure_exit_status() -> None:
    config = _config("does-not-match")
    items: list[object] = [SimpleNamespace(nodeid="tests/test_auth.py::test_login")]
    pytest_collection_modifyitems(config, items)
    session = SimpleNamespace(config=config, exitstatus=0)

    pytest_sessionfinish(session, 0)

    assert session.exitstatus == 5


def test_unfiltered_session_keeps_existing_exit_status() -> None:
    config = _config(None)
    session = SimpleNamespace(config=config, exitstatus=1)

    pytest_sessionfinish(session, 1)

    assert session.exitstatus == 1
