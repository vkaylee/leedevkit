#!/usr/bin/env python3
"""Shared base class for Orchestrator delegate handlers.

All handler classes (DbHandler, TestHandler, RunHandler, InitHandler)
share identical forwarding boilerplate: __slots__, __init__, property
forwarding to the orchestrator, and an _execute_safe wrapper.

This module eliminates that duplication (Technical Debt #1 — DRY).
"""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from typing import Any


class HandlerBase:
    """Base class for orchestrator delegate handlers.

    Provides shared property forwarding to the orchestrator and
    a canonical _execute_safe() wrapper. Subclasses set __slots__
    to ("_orch",) and add handler-specific properties.
    """

    __slots__ = ("_orch",)

    def __init__(self, orchestrator: Any) -> None:
        self._orch = orchestrator

    # ── Shared property forwarding ──────────────────────────────────────

    @property
    def _engine(self) -> str:
        return self._orch.engine

    @property
    def _compose_engine(self) -> list[str]:
        return self._orch.compose_engine

    @property
    def _env_vars(self) -> dict[str, str]:
        return self._orch.env_vars

    @property
    def _dry_run(self) -> bool:
        return self._orch.dry_run

    # ── Command execution ───────────────────────────────────────────────

    def _execute_safe(
        self,
        cmd: list[str],
        env: dict[str, str] | None = None,
        timeout: int = 1800,
    ) -> None:
        self._orch.execute_safe(cmd, env=env, timeout=timeout)
