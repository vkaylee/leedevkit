"""Pytest collection filter for ``leedevkit test infra --pattern``."""

from __future__ import annotations

import os
import re
from typing import Any

import pytest


def pytest_addoption(parser: Any) -> None:
    """Register pattern option without requiring project pytest config changes."""
    parser.addoption(
        "--leedevkit-pattern",
        action="store",
        default=None,
        help="LeeDevKit regex applied to collected pytest node IDs",
    )


def pytest_configure(config: Any) -> None:
    """Compile pattern early so invalid input fails before collection runs."""
    value = config.getoption("--leedevkit-pattern") or os.environ.get(
        "LEEDEVKIT_PATTERN", ""
    )
    if not value:
        config._leedevkit_pattern = None
        return
    try:
        config._leedevkit_pattern = re.compile(value)
    except re.error as exc:
        raise pytest.UsageError(f"Invalid --pattern regex for pytest: {exc}") from exc
    config._leedevkit_pattern_matches = 0


def pytest_collection_modifyitems(config: Any, items: list[Any]) -> None:
    """Deselect items whose node ID does not match configured regex."""
    pattern = getattr(config, "_leedevkit_pattern", None)
    if pattern is None:
        return
    selected = [item for item in items if pattern.search(item.nodeid)]
    deselected = [item for item in items if item not in selected]
    config._leedevkit_pattern_matches = len(selected)
    if deselected:
        config.hook.pytest_deselected(items=deselected)
    items[:] = selected


def pytest_sessionfinish(session: Any, exitstatus: int) -> None:
    """Turn zero selected tests into a deterministic failure."""
    if (
        getattr(session.config, "_leedevkit_pattern", None) is not None
        and getattr(session.config, "_leedevkit_pattern_matches", 0) == 0
        and exitstatus in (0, 5)
    ):
        session.exitstatus = 5
