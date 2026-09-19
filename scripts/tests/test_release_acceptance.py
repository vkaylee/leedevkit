"""Black-box acceptance coverage for the built release artifact."""

import os
import shutil
import subprocess
import sys
import tarfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from _release_build import build_release  # noqa: E402


def test_release_acceptance_command(tmp_path):
    repo_root = Path(__file__).resolve().parents[2]
    script = repo_root / "scripts" / "_release_acceptance.py"

    result = subprocess.run(
        [
            sys.executable,
            str(script),
            "--repo-root",
            str(repo_root),
        ],
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )

    assert result.returncode == 0, f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    assert "Release acceptance passed" in result.stdout


def _script_environment(tmp_path: Path, **extra: str) -> dict[str, str]:
    return {
        **os.environ,
        "HOME": str(tmp_path / "home"),
        "TMPDIR": str(tmp_path / "tmp"),
        "HTTP_PROXY": "http://127.0.0.1:9",
        "HTTPS_PROXY": "http://127.0.0.1:9",
        "NO_PROXY": "",
        **extra,
    }

    repo_root = Path(__file__).resolve().parents[2]
    version = (repo_root / "VERSION").read_text().strip()
    install_dir = tmp_path / "home" / ".leedevkit"
    version_dir = install_dir / f"v{version}"
    version_dir.mkdir(parents=True)
    sentinel = version_dir / "SENTINEL"
    sentinel.write_text("previous")
    (install_dir / "current").symlink_to(version_dir)
    (tmp_path / "tmp").mkdir()

    env = _script_environment(
        tmp_path,
        DEVKIT_HOME=str(install_dir),
        LEEDEVKIT_RELEASE_BASE_URL=(tmp_path / "missing-mirror").as_uri(),
    )
    result = subprocess.run(
        ["bash", str(repo_root / "install.sh"), "v0.7.8"],
        cwd=tmp_path,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )

    assert result.returncode != 0
    assert sentinel.read_text() == "previous"
    assert (install_dir / "current").resolve() == version_dir.resolve()


def test_bootstrap_rejects_archive_links_without_touching_install(tmp_path):
    """Bootstrap must reject symlink members before extraction or activation."""
    repo_root = Path(__file__).resolve().parents[2]
    version = "0.7.8"
    mirror = tmp_path / "mirror" / "download" / f"v{version}"
    mirror.mkdir(parents=True)
    archive = mirror / f"leedevkit-{version}.tar.gz"
    with tarfile.open(archive, "w:gz") as output:
        root = tarfile.TarInfo(f"leedevkit-{version}")
        root.type = tarfile.DIRTYPE
        root.mode = 0o755
        output.addfile(root)
        link = tarfile.TarInfo(f"leedevkit-{version}/escape")
        link.type = tarfile.SYMTYPE
        link.linkname = "../../outside"
        output.addfile(link)
        hardlink = tarfile.TarInfo(f"leedevkit-{version}/hardlink")
        hardlink.type = tarfile.LNKTYPE
        hardlink.linkname = f"leedevkit-{version}/escape"
        output.addfile(hardlink)

    project = tmp_path / "project"
    project.mkdir()
    (project / ".git").mkdir()
    installed = project / ".leedevkit"
    installed.mkdir()
    (installed / "SENTINEL").write_text("previous")
    outside = tmp_path / "outside"
    env = _script_environment(
        tmp_path,
        LEEDEVKIT_RELEASE_BASE_URL=(tmp_path / "mirror").as_uri(),
    )
    result = subprocess.run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"],
        cwd=project,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )

    assert result.returncode != 0
    assert (installed / "SENTINEL").read_text() == "previous"
    assert not outside.exists()


def test_install_success_activates_downloaded_release(tmp_path):
    """A valid local release mirror still installs and activates globally."""
    repo_root = Path(__file__).resolve().parents[2]
    version = (repo_root / "VERSION").read_text().strip()
    artifact = build_release(repo_root, tmp_path / "dist")
    mirror_release = tmp_path / "mirror" / "download" / f"v{version}"
    mirror_release.mkdir(parents=True)
    shutil.copy2(artifact, mirror_release / artifact.name)
    install_dir = tmp_path / "home" / ".leedevkit"
    (tmp_path / "tmp").mkdir()

    result = subprocess.run(
        ["bash", str(repo_root / "install.sh"), f"v{version}"],
        cwd=tmp_path,
        env=_script_environment(
            tmp_path,
            DEVKIT_HOME=str(install_dir),
            LEEDEVKIT_RELEASE_BASE_URL=(tmp_path / "mirror").as_uri(),
        ),
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )

    assert result.returncode == 0, result.stderr or result.stdout
    assert (install_dir / "current" / "bin" / "leedevkit").is_file()
    assert (install_dir / "current").resolve() == (
        install_dir / f"v{version}"
    ).resolve()
