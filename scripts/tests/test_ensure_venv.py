"""Regression tests for the DevKit-local Python venv bootstrap."""

from __future__ import annotations

import hashlib
import os
import shutil
import stat
import subprocess
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SOURCE_RUNTIME_LOCK = REPO_ROOT / "scripts" / "requirements-runtime.lock"
SOURCE_TEST_LOCK = REPO_ROOT / "scripts" / "requirements.lock"
SOURCE_SCRIPT = REPO_ROOT / "scripts" / "_ensure-venv.sh"


def _write_executable(path: Path, content: str) -> None:
    path.write_text(content)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def _fake_venv_python(log_path: Path) -> str:
    return f'''#!/bin/sh
printf '%s\\n' "$*" >> "{log_path}"
if [ "$1" = "-c" ]; then
    case "$2" in *sys.implementation.name*) echo 'cpython:3.14';; esac
    exit 0
fi
exit 0
'''


def _make_fixture(tmp_path: Path) -> tuple[Path, Path, dict[str, str]]:
    devkit = tmp_path / "devkit"
    scripts = devkit / "scripts"
    scripts.mkdir(parents=True)
    script = scripts / "_ensure-venv.sh"
    shutil.copy2(SOURCE_SCRIPT, script)
    shutil.copy2(SOURCE_RUNTIME_LOCK, scripts / "requirements-runtime.lock")
    shutil.copy2(SOURCE_TEST_LOCK, scripts / "requirements.lock")
    script.chmod(script.stat().st_mode | stat.S_IXUSR)

    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    log_path = tmp_path / "commands.log"
    _write_executable(
        bin_dir / "python3",
        "#!/bin/sh\n"
        'if [ "$1" = "-m" ] && [ "$2" = "venv" ]; then\n'
        '    mkdir -p "$3/bin"\n'
        "    cat > \"$3/bin/python3\" <<'PYTHON'\n"
        f"{_fake_venv_python(log_path)}"
        "PYTHON\n"
        '    chmod +x "$3/bin/python3"\n'
        "    exit 0\n"
        "fi\n"
        "printf 'unexpected system python call: %s\\n' \"$*\" >&2\n"
        "exit 1\n",
    )
    env = os.environ.copy()
    env["PATH"] = f"{bin_dir}:{env['PATH']}"
    return devkit, log_path, env


def _run_ensure(
    devkit: Path, env: dict[str, str], profile: str = "runtime"
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh"), profile],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )


def test_runtime_profile_installs_only_runtime_dependencies(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)

    result = _run_ensure(devkit, env)

    python_path = devkit / ".venv" / "bin" / "python3"
    assert result.stdout.strip() == str(python_path)
    assert python_path.is_file()
    assert not (tmp_path / ".venv").exists()
    requirements = (
        python_path.parent.parent / ".leedevkit-runtime-requirements.txt"
    ).read_text()
    assert "PyYAML==" in requirements and "psutil==" in requirements
    assert all(
        tool not in requirements for tool in ("pytest", "ruff", "mypy", "playwright")
    )
    assert (
        len(
            [
                line
                for line in log_path.read_text().splitlines()
                if "-m pip install" in line
            ]
        )
        == 1
    )


def test_test_profile_upgrades_runtime_environment_with_full_lock(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)

    _run_ensure(devkit, env, "runtime")
    _run_ensure(devkit, env, "test")

    requirements = (devkit / ".venv" / ".leedevkit-test-requirements.txt").read_text()
    assert (
        "pytest-cov" in requirements
        and "ruff==" in requirements
        and "mypy==" in requirements
    )
    assert (
        len(
            [
                line
                for line in log_path.read_text().splitlines()
                if "-m pip install" in line
            ]
        )
        == 2
    )


def test_runtime_profile_restores_minimal_lock_after_test_profile(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)

    _run_ensure(devkit, env, "test")
    _run_ensure(devkit, env, "runtime")

    requirements = (
        devkit / ".venv" / ".leedevkit-runtime-requirements.txt"
    ).read_text()
    assert "PyYAML==" in requirements and "psutil==" in requirements
    assert "pytest" not in requirements and "ruff" not in requirements
    assert (
        len(
            [
                line
                for line in log_path.read_text().splitlines()
                if "-m pip install" in line
            ]
        )
        == 2
    )


def test_unknown_profile_fails_before_creating_venv(tmp_path):
    devkit, _, env = _make_fixture(tmp_path)

    result = subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh"), "invalid"],
        capture_output=True,
        text=True,
        env=env,
    )

    assert result.returncode == 2
    assert "Profile must be runtime or test" in result.stderr
    assert not (devkit / ".venv").exists()


def test_healthy_runtime_profile_does_not_invoke_pip(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    _write_executable(python_path, _fake_venv_python(log_path))
    lock = devkit / "scripts" / "requirements-runtime.lock"
    lock.write_text("")
    digest = hashlib.sha256(lock.read_bytes()).hexdigest()
    (
        python_path.parent.parent / ".leedevkit-runtime-dependency-fingerprint"
    ).write_text(f"runtime cpython:3.14 lock={digest} playwright=0")

    _run_ensure(devkit, env)

    assert not any(
        "-m pip install" in command for command in log_path.read_text().splitlines()
    )


def test_interpreter_fingerprint_change_reinstalls_dependencies(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    _write_executable(python_path, _fake_venv_python(log_path))
    lock = devkit / "scripts" / "requirements-runtime.lock"
    digest = hashlib.sha256(lock.read_bytes()).hexdigest()
    (
        python_path.parent.parent / ".leedevkit-runtime-dependency-fingerprint"
    ).write_text(f"runtime old-runtime lock={digest} playwright=0")

    _run_ensure(devkit, env)

    assert any(
        "-m pip install" in command for command in log_path.read_text().splitlines()
    )


def test_playwright_installs_only_when_explicitly_requested(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    env["LEEDEVKIT_REQUIRE_PLAYWRIGHT"] = "1"

    _run_ensure(devkit, env, "test")

    optional = (
        devkit / ".venv" / ".leedevkit-test-optional-requirements.txt"
    ).read_text()
    assert "playwright" in optional
    assert "-m pip install" in log_path.read_text()


def test_damaged_venv_is_recreated(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    python_path.write_text("#!/bin/sh\\nexit 127\\n")
    python_path.chmod(0o755)

    result = _run_ensure(devkit, env)

    assert result.stdout.strip() == str(python_path)
    assert "-m pip install" in log_path.read_text()
