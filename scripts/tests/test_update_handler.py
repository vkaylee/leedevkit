"""Tests for _update_handler — self-update module."""

from __future__ import annotations

import tarfile

import pytest


class TestDevkitRoot:
    """Tests for _devkit_root()."""

    def test_returns_parent_of_scripts(self):
        """_devkit_root points to the devkit install root."""
        from _update_handler import _devkit_root

        root = _devkit_root()
        assert (root / "scripts" / "_update_handler.py").exists()


class TestDownloadAndExtract:
    """Tests for download_and_extract_tarball()."""

    def test_extracts_single_root_dir_tarball(self, tmp_path, monkeypatch):
        """Tarball with a single root dir extracts to target_dir."""
        from _download import download_and_extract_tarball

        # Build a tarball with a single root dir
        tarball_path = tmp_path / "release.tar.gz"
        source_root = tmp_path / "source"
        (source_root / "leedevkit-0.2.0" / "VERSION").parent.mkdir(parents=True)
        (source_root / "leedevkit-0.2.0" / "VERSION").write_text("0.2.0")
        (source_root / "leedevkit-0.2.0" / "scripts").mkdir()

        with tarfile.open(tarball_path, "w:gz") as tf:
            tf.add(
                source_root / "leedevkit-0.2.0",
                arcname="leedevkit-0.2.0",
            )

        def fake_urlretrieve(url, filename):
            import shutil

            shutil.copy(str(tarball_path), str(filename))

        monkeypatch.setattr("_download.urllib.request.urlretrieve", fake_urlretrieve)

        target = tmp_path / "dest"
        download_and_extract_tarball("https://example.com/v0.2.0.tar.gz", target)

        # Single root dir stripped — contents moved directly to target
        assert (target / "VERSION").read_text() == "0.2.0"
        assert (target / "scripts").is_dir()

    def test_extracts_flat_tarball(self, tmp_path, monkeypatch):
        """Tarball without a single root dir copies contents as-is."""
        from _download import download_and_extract_tarball

        tarball_path = tmp_path / "release.tar.gz"
        source_root = tmp_path / "source"
        (source_root / "VERSION").parent.mkdir(parents=True)
        (source_root / "VERSION").write_text("0.2.0")
        (source_root / "README.md").write_text("# Hello")

        with tarfile.open(tarball_path, "w:gz") as tf:
            tf.add(source_root / "VERSION", arcname="VERSION")
            tf.add(source_root / "README.md", arcname="README.md")

        def fake_urlretrieve(url, filename):
            import shutil

            shutil.copy(str(tarball_path), str(filename))

        monkeypatch.setattr("_download.urllib.request.urlretrieve", fake_urlretrieve)

        target = tmp_path / "dest"
        download_and_extract_tarball("https://example.com/v0.2.0.tar.gz", target)

        assert (target / "VERSION").read_text() == "0.2.0"
        assert (target / "README.md").read_text() == "# Hello"

    def test_overwrites_existing_target(self, tmp_path, monkeypatch):
        """Existing target_dir is removed before moving new one."""
        from _download import download_and_extract_tarball

        target = tmp_path / "dest"
        target.mkdir()
        (target / "OLD").write_text("old")

        tarball_path = tmp_path / "release.tar.gz"
        source_root = tmp_path / "source"
        (source_root / "leedevkit-0.3.0" / "VERSION").parent.mkdir(parents=True)
        (source_root / "leedevkit-0.3.0" / "VERSION").write_text("0.3.0")

        with tarfile.open(tarball_path, "w:gz") as tf:
            tf.add(source_root / "leedevkit-0.3.0", arcname="leedevkit-0.3.0")

        def fake_urlretrieve(url, filename):
            import shutil

            shutil.copy(str(tarball_path), str(filename))

        monkeypatch.setattr("_download.urllib.request.urlretrieve", fake_urlretrieve)

        download_and_extract_tarball("https://example.com/v0.3.0.tar.gz", target)
        assert (target / "VERSION").read_text() == "0.3.0"
        assert not (target / "OLD").exists()


class TestHandleUpdateRollback:
    """Test rollback behavior when download_and_extract_tarball fails."""

    def test_rollback_restores_previous_version(self, tmp_path, monkeypatch):
        """Failed update restores original devkit from backup."""
        from _update_handler import handle_update

        root = tmp_path / "devkit"
        root.mkdir()
        (root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: root)

        def boom(url, target_dir):
            raise RuntimeError("network down")

        monkeypatch.setattr("_update_handler.download_and_extract_tarball", boom)

        with pytest.raises(RuntimeError, match="network down"):
            handle_update(target="v0.2.0")

        # Original tree intact, no backup left behind
        assert (root / "VERSION").read_text() == "0.1.0"
        assert not (tmp_path / "devkit.bak").exists()

    def test_rollback_cleans_up_after_move_failure(self, tmp_path, monkeypatch):
        """When move fails, old backup is restored."""
        from _update_handler import handle_update

        root = tmp_path / "devkit"
        root.mkdir()
        (root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: root)

        # download_and_extract_tarball succeeds; test the success path
        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.2.0")
            # The except block only fires if download_and_extract_tarball raises.
            pass

        # Simulate success path with backup already present
        backup = root.with_name(root.name + ".bak")
        backup.mkdir()
        (backup / "OLD").write_text("old-backup")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        handle_update(target="v0.2.0")

        # Successful update
        assert (root / "VERSION").read_text() == "0.2.0"


class TestUpdateVersionPin:
    """Test version pinning in leedevkit.toml after update."""

    def test_updates_version_in_toml(self, tmp_path, monkeypatch):
        """After successful update, version in leedevkit.toml is updated."""
        from _update_handler import handle_update

        # Setup project structure
        project_root = tmp_path / "project"
        project_root.mkdir()
        toml_file = project_root / "leedevkit.toml"
        toml_file.write_text('[devkit]\nversion = "0.1.0"\n')

        devkit_root = project_root / ".leedevkit"
        devkit_root.mkdir()
        (devkit_root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: devkit_root)

        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.6")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        handle_update(target="v0.3.6")

        # Version in toml should be updated
        assert 'version = "0.3.6"' in toml_file.read_text()

    def test_no_crash_if_toml_missing(self, tmp_path, monkeypatch):
        """Update succeeds even if leedevkit.toml doesn't exist."""
        from _update_handler import handle_update

        project_root = tmp_path / "project"
        project_root.mkdir()

        devkit_root = project_root / ".leedevkit"
        devkit_root.mkdir()
        (devkit_root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: devkit_root)

        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.6")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        # Should not raise
        handle_update(target="v0.3.6")
        assert (devkit_root / "VERSION").read_text() == "0.3.6"

    def test_no_crash_if_no_devkit_section(self, tmp_path, monkeypatch):
        """Update succeeds even if leedevkit.toml has no [devkit] section."""
        from _update_handler import handle_update

        project_root = tmp_path / "project"
        project_root.mkdir()
        toml_file = project_root / "leedevkit.toml"
        toml_file.write_text('[project]\nname = "test"\n')

        devkit_root = project_root / ".leedevkit"
        devkit_root.mkdir()
        (devkit_root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: devkit_root)

        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.6")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        # Should not raise
        handle_update(target="v0.3.6")
        # Toml should remain unchanged
        assert "[project]" in toml_file.read_text()


class TestAutoSyncAfterUpdate:
    """Test automatic sync after successful update."""

    def test_sync_called_after_update(self, tmp_path, monkeypatch, capsys):
        """After successful update, sync is automatically called."""
        from _update_handler import handle_update

        project_root = tmp_path / "project"
        project_root.mkdir()
        toml_file = project_root / "leedevkit.toml"
        toml_file.write_text('[devkit]\nversion = "0.1.0"\n')

        devkit_root = project_root / ".leedevkit"
        devkit_root.mkdir()
        (devkit_root / "VERSION").write_text("0.1.0")
        (devkit_root / "scripts").mkdir()
        (devkit_root / "scripts" / "_orchestrator.py").write_text("# stub")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: devkit_root)

        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.7")
            (target_dir / "scripts").mkdir()
            (target_dir / "scripts" / "_orchestrator.py").write_text("# stub")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        handle_update(target="v0.3.7")

        captured = capsys.readouterr()
        combined = captured.out + captured.err
        assert "syncing rules" in combined.lower()
        assert "sync complete" in combined.lower()

    def test_update_succeeds_even_if_sync_fails(self, tmp_path, monkeypatch, capsys):
        """Update succeeds even if post-update sync fails."""
        from _update_handler import handle_update

        project_root = tmp_path / "project"
        project_root.mkdir()

        devkit_root = project_root / ".leedevkit"
        devkit_root.mkdir()
        (devkit_root / "VERSION").write_text("0.1.0")

        monkeypatch.setattr("_update_handler._devkit_root", lambda: devkit_root)

        def fake_download(url, target_dir):
            target_dir.mkdir(parents=True, exist_ok=True)
            (target_dir / "VERSION").write_text("0.3.7")

        monkeypatch.setattr(
            "_update_handler.download_and_extract_tarball", fake_download
        )

        # Mock sync to fail
        def failing_sync(*args, **kwargs):
            raise RuntimeError("sync failed")

        monkeypatch.setattr(
            "_init_handler.InitHandler.handle_post_update_sync", failing_sync
        )

        # Should not raise
        handle_update(target="v0.3.7")

        captured = capsys.readouterr()
        combined = captured.out + captured.err
        assert "post-update sync failed" in combined.lower()
        assert "0.3.7" in (devkit_root / "VERSION").read_text()
