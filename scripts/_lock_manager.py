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
    def _path(project_name: str) -> Path:
        return Path(tempfile.gettempdir()) / f"{project_name}.lock"

    @staticmethod
    def acquire(project_name: str) -> int | None:
        """Acquire an exclusive, non-blocking file lock for *project_name*."""
        lock_path = LockManager._path(project_name)
        fd: int | None = None
        try:
            fd = os.open(str(lock_path), os.O_CREAT | os.O_RDWR)
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            return fd
        except OSError:
            if fd is not None:
                with contextlib.suppress(OSError):
                    os.close(fd)
            return None

    @staticmethod
    def release(fd: int | None, project_name: str | None = None) -> None:
        """Release owned lock and remove its path only when inode still matches."""
        if fd is None:
            return

        lock_path = LockManager._path(project_name) if project_name else None
        owned_path = False
        try:
            if lock_path is not None:
                opened = os.fstat(fd)
                current = os.stat(lock_path)
                owned_path = (opened.st_dev, opened.st_ino) == (
                    current.st_dev,
                    current.st_ino,
                )
            fcntl.flock(fd, fcntl.LOCK_UN)
        except OSError:
            pass
        finally:
            with contextlib.suppress(OSError):
                os.close(fd)

        if owned_path and lock_path is not None:
            with contextlib.suppress(OSError):
                lock_path.unlink(missing_ok=True)
