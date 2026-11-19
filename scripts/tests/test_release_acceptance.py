"""Black-box acceptance coverage for the built release artifact."""

import os
import shutil
import subprocess
import sys
import tarfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from _platform import platform_key  # noqa: E402
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
    from _go_core_matrix import build_matrix

    go_core_dir = tmp_path / "go-core"
    build_matrix(
        repo_root, go_core_dir, version, compose_cmd=("/usr/bin/docker", "compose")
    )
    artifact = build_release(repo_root, tmp_path / "dist", go_core_dir)
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
    installed_binary = install_dir / "current" / "bin" / "leedevkit"
    assert installed_binary.is_file()
    assert (install_dir / "current").resolve() == (
        install_dir / f"v{version}"
    ).resolve()

    outside = tmp_path / "outside"
    outside.mkdir()
    binary_result = subprocess.run(
        [str(installed_binary), "version"],
        cwd=outside,
        env={
            **_script_environment(tmp_path),
            "DEVKIT_HOME": str(install_dir / "current"),
        },
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    assert binary_result.returncode == 0, binary_result.stderr
    assert binary_result.stdout.strip() == version

    project = tmp_path / "global-project"
    nested = project / "nested"
    (project / ".git").mkdir(parents=True)
    nested.mkdir()
    (project / "leedevkit.toml").write_text(
        f'[devkit]\nversion = "{version}"\n[project]\nname = "global-project"\n'
    )
    doctor_result = subprocess.run(
        [str(installed_binary), "doctor"],
        cwd=nested,
        env={
            **_script_environment(tmp_path),
            "DEVKIT_HOME": str(install_dir / "current"),
        },
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    assert doctor_result.returncode == 0, doctor_result.stderr + doctor_result.stdout
    assert "Project:" in doctor_result.stdout + doctor_result.stderr


def test_global_binary_doctor_uses_project_root(tmp_path):
    """Global binary runs doctor against project root without a wrapper."""
    repo_root = Path(__file__).resolve().parents[2]
    version = (repo_root / "VERSION").read_text().strip()
    from _go_core_matrix import build_matrix

    go_core_dir = tmp_path / "go-core"
    build_matrix(
        repo_root, go_core_dir, version, compose_cmd=("/usr/bin/docker", "compose")
    )
    artifact = build_release(repo_root, tmp_path / "dist", go_core_dir)
    mirror_release = tmp_path / "mirror" / "download" / f"v{version}"
    mirror_release.mkdir(parents=True)
    shutil.copy2(artifact, mirror_release / artifact.name)
    (tmp_path / "tmp").mkdir(parents=True, exist_ok=True)
    project = tmp_path / "project"
    project.mkdir()
    (project / ".git").mkdir()

    env = _script_environment(
        tmp_path,
        LEEDEVKIT_RELEASE_BASE_URL=(tmp_path / "mirror").as_uri(),
    )
    bootstrap = subprocess.run(
        ["bash", str(repo_root / "bootstrap.sh"), f"v{version}"],
        cwd=project,
        env=env,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert bootstrap.returncode == 0, bootstrap.stderr or bootstrap.stdout

    global_runtime = tmp_path / "global-runtime"
    shutil.copytree(project / ".leedevkit", global_runtime)
    native = global_runtime / "bin" / "go" / f"leedevkit-go-{platform_key()}"
    shutil.copy2(native, global_runtime / "bin" / "leedevkit")
    (global_runtime / "bin" / "leedevkit").chmod(0o755)
    doctor = subprocess.run(
        [str(global_runtime / "bin" / "leedevkit"), "doctor"],
        cwd=project,
        env={**env, "DEVKIT_HOME": str(global_runtime)},
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    output = doctor.stdout + doctor.stderr
    assert doctor.returncode == 0, output
    assert "Project:" in output
    assert "unrecognized arguments: doctor" not in output


def test_project_wrapper_bootstraps_stale_runtime_without_unbound_lock(tmp_path):
    """A stale project runtime is replaced through the wrapper lock path."""
    repo_root = Path(__file__).resolve().parents[2]
    version = (repo_root / "VERSION").read_text().strip()
    from _go_core_matrix import build_matrix

    go_core_dir = tmp_path / "go-core"
    build_matrix(
        repo_root, go_core_dir, version, compose_cmd=("/usr/bin/docker", "compose")
    )
    artifact = build_release(repo_root, tmp_path / "dist", go_core_dir)
    mirror_release = tmp_path / "mirror" / "download" / f"v{version}"
    mirror_release.mkdir(parents=True)
    shutil.copy2(artifact, mirror_release / artifact.name)

    project = tmp_path / "project"
    project.mkdir()
    (project / ".git").mkdir()
    (project / "leedevkit.toml").write_text(
        f'[devkit]\nversion = "{version}"\n[project]\nname = "stale-runtime"\n'
    )
    runtime = project / ".leedevkit"
    (runtime / "bin").mkdir(parents=True)
    (runtime / "VERSION").write_text("0.0.1\n")
    shutil.copy2(repo_root / "bin" / "leedevkit", runtime / "bin" / "leedevkit")
    (runtime / "bin" / "leedevkit").chmod(0o755)
    shutil.copy2(repo_root / "leedevkit", project / "leedevkit")
    (project / "leedevkit").chmod(0o755)

    result = subprocess.run(
        [str(project / "leedevkit"), "version"],
        cwd=project,
        env=_script_environment(
            tmp_path,
            LEEDEVKIT_RELEASE_BASE_URL=(tmp_path / "mirror").as_uri(),
        ),
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )

    assert result.returncode == 0, result.stderr or result.stdout
    assert result.stdout.strip() == version
