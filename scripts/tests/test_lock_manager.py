"""Tests for LockManager extracted from Orchestrator."""

from __future__ import annotations

import tempfile


class TestLockManager:
    """Unit tests for the extracted LockManager."""

    def test_acquire_success(self):
        """LockManager.acquire returns a valid fd for a new lock."""
        from _lock_manager import LockManager

        project = "test-lock-success"
        fd = LockManager.acquire(project)
        assert fd is not None
        assert fd > 0
        LockManager.release(fd, project)

    def test_acquire_conflict(self):
        """Second acquire for the same project returns None (lock held)."""
        from _lock_manager import LockManager

        project = "test-lock-conflict"
        fd1 = LockManager.acquire(project)
        assert fd1 is not None

        fd2 = LockManager.acquire(project)
        assert fd2 is None  # Already locked

        LockManager.release(fd1, project)

    def test_release_none_fd(self):
        """release() with fd=None is a safe no-op."""
        from _lock_manager import LockManager

        # Should not raise
        LockManager.release(None)

    def test_release_none_fd_no_project(self):
        """release() with fd=None and project_name=None is safe."""
        from _lock_manager import LockManager

        LockManager.release(None, None)

    def test_acquire_release_cycle(self):
        """Acquire, release, then re-acquire the same project."""
        from _lock_manager import LockManager

        project = "test-lock-cycle"
        fd = LockManager.acquire(project)
        assert fd is not None
        LockManager.release(fd, project)

        # Should be able to acquire again after release
        fd2 = LockManager.acquire(project)
        assert fd2 is not None
        LockManager.release(fd2, project)

    def test_acquire_unique_projects(self):
        """Different projects can be locked independently."""
        from _lock_manager import LockManager

        fd_a = LockManager.acquire("proj-a")
        fd_b = LockManager.acquire("proj-b")
        assert fd_a is not None
        assert fd_b is not None
        assert fd_a != fd_b
        LockManager.release(fd_a, "proj-a")
        LockManager.release(fd_b, "proj-b")

    def test_release_removes_lock_file(self):
        """After release with project_name, the lock file is cleaned up."""
        from _lock_manager import LockManager
        from pathlib import Path

        project = "test-lock-cleanup"
        fd = LockManager.acquire(project)
        assert fd is not None

        lock_path = Path(tempfile.gettempdir()) / f"{project}.lock"
        assert lock_path.exists()

        LockManager.release(fd, project)
        assert not lock_path.exists()
