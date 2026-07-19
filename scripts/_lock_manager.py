#!/usr/bin/env python3
"""File-lock manager extracted from Orchestrator (Single Responsibility Principle).

Manages OS-level advisory file locks for test-run isolation,
preventing the garbage collector from sweeping active test environments.
"""

from __future__ import annotations

import contextlib
import fcntl
import os
import tempfile
from pathlib import Path


class LockManager:
    """Acquire and release OS-level file locks for test-run isolation.

    Usage:
        lock = LockManager()
        fd = lock.acquire("leedevkit-test-abc123")
        ...
        lock.release(fd, "leedevkit-test-abc123")
    """

    __slots__ = ()

    @staticmethod
    def acquire(project_name: str) -> int | None:
        """Acquire an exclusive, non-blocking file lock for *project_name*.

        Returns the open file descriptor on success, or ``None`` if the
        lock could not be acquired (already held by another process).
        """
        lock_path = Path(tempfile.gettempdir()) / f"{project_name}.lock"
        try:
            fd = os.open(str(lock_path), os.O_CREAT | os.O_RDWR)
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            return None
        return fd

    @staticmethod
    def release(fd: int | None, project_name: str | None = None) -> None:
        """Release a previously acquired lock and remove its lock file.

        Safe to call with ``fd=None`` (no-op).
        """
        if fd is None:
            return

        try:
            fcntl.flock(fd, fcntl.LOCK_UN)
            os.close(fd)
        except OSError:
            pass

        if project_name:
            lock_path = Path(tempfile.gettempdir()) / f"{project_name}.lock"
            with contextlib.suppress(OSError):
                lock_path.unlink(missing_ok=True)
