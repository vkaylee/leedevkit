"""Regression tests for the DevKit-local Python venv bootstrap."""

from __future__ import annotations

import hashlib
import os
import shutil
import stat
import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
SOURCE_SCRIPT = REPO_ROOT / "scripts" / "_ensure-venv.sh"
SOURCE_LOCK = REPO_ROOT / "scripts" / "requirements.lock"


def _write_executable(path: Path, content: str) -> None:
    path.write_text(content)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def _fake_venv_python(log_path: Path) -> str:
    return f'''#!/bin/sh
printf '%s\\n' "$*" >> "{log_path}"
if [ "$1" = "-c" ]; then
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
    shutil.copy2(SOURCE_LOCK, scripts / "requirements.lock")
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


def test_creates_venv_inside_devkit_and_installs_required_packages(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)

    result = subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )

    python_path = devkit / ".venv" / "bin" / "python3"
    assert result.stdout.strip() == str(python_path)
    assert python_path.is_file()
    assert not (tmp_path / ".venv").exists()
    commands = log_path.read_text()
    assert "--requirement" in commands
    assert (
        "pytest-cov"
        in (devkit / ".venv" / ".leedevkit-core-requirements.txt").read_text()
    )
    assert (
        "types-PyYAML"
        in (devkit / ".venv" / ".leedevkit-core-requirements.txt").read_text()
    )
    assert "playwright" not in commands


def test_dependency_fingerprint_change_recovers_environment(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    _write_executable(python_path, _fake_venv_python(log_path))
    lock = devkit / "scripts" / "requirements.lock"
    digest = hashlib.sha256(lock.read_bytes()).hexdigest()
    (python_path.parent.parent / ".leedevkit-dependency-fingerprint").write_text(
        f" lock={digest} playwright=0"
    )
    lock.write_text(lock.read_text() + "\n# changed\n")

    subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )
    assert any(
        "--requirement" in command for command in log_path.read_text().splitlines()
    )


def test_healthy_venv_does_not_invoke_pip(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    _write_executable(python_path, _fake_venv_python(log_path))
    lock = devkit / "scripts" / "requirements.lock"
    digest = hashlib.sha256(lock.read_bytes()).hexdigest()
    (python_path.parent.parent / ".leedevkit-dependency-fingerprint").write_text(
        f" lock={digest} playwright=0"
    )

    result = subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )

    assert result.stdout.strip() == str(python_path)
    commands = log_path.read_text().splitlines()
    assert not any("-m pip" in command for command in commands)


def test_interpreter_fingerprint_change_reinstalls_dependencies(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    _write_executable(python_path, _fake_venv_python(log_path))
    lock = devkit / "scripts" / "requirements.lock"
    digest = hashlib.sha256(lock.read_bytes()).hexdigest()
    (python_path.parent.parent / ".leedevkit-dependency-fingerprint").write_text(
        f"old-runtime lock={digest} playwright=0"
    )

    subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )
    assert any(
        "--requirement" in command for command in log_path.read_text().splitlines()
    )


def test_playwright_installs_only_when_explicitly_requested(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    env["LEEDEVKIT_REQUIRE_PLAYWRIGHT"] = "1"

    subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )

    commands = log_path.read_text()
    assert "--requirement" in commands
    optional = (devkit / ".venv" / ".leedevkit-optional-requirements.txt").read_text()
    assert "playwright" in optional


def test_damaged_venv_is_recreated(tmp_path):
    devkit, log_path, env = _make_fixture(tmp_path)
    python_path = devkit / ".venv" / "bin" / "python3"
    python_path.parent.mkdir(parents=True)
    python_path.write_text("#!/bin/sh\nexit 127\n")
    python_path.chmod(0o755)

    result = subprocess.run(
        ["bash", str(devkit / "scripts" / "_ensure-venv.sh")],
        capture_output=True,
        text=True,
        check=True,
        env=env,
    )

    assert result.stdout.strip() == str(python_path)
    assert "-m pip" in log_path.read_text()
