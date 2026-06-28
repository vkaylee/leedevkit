"""Shared test utilities — Python equivalent of _test-utils.sh.

Provides parallel task runner, timeout handling, log management,
and result reporting for lint/unit/integration/coverage phases.
"""

import os
import subprocess
import sys
from pathlib import Path

from _bootstrap import PROJECT_ROOT, bootstrap_env

LOG_DIR = PROJECT_ROOT / ".test_logs"

TIMEOUT_LINT = int(os.environ.get("TIMEOUT_LINT", "900"))
TIMEOUT_UNIT = int(os.environ.get("TIMEOUT_UNIT", "900"))
TIMEOUT_INTEGRATION = int(os.environ.get("TIMEOUT_INTEGRATION", "1200"))
TIMEOUT_BUILD = int(os.environ.get("TIMEOUT_BUILD", "600"))


def get_phase_timeout(phase_name: str) -> int:
    """Determine timeout based on phase name."""
    if "Lint" in phase_name:
        return TIMEOUT_LINT
    if "Unit" in phase_name:
        return TIMEOUT_UNIT
    if "Integration" in phase_name or "E2E" in phase_name:
        return TIMEOUT_INTEGRATION
    if "Coverage" in phase_name:
        return TIMEOUT_INTEGRATION
    return TIMEOUT_UNIT


def _ensure_log_dir() -> Path:
    """Ensure LOG_DIR exists and return it."""
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    return LOG_DIR


def _safe_log_name(phase_name: str, task_name: str) -> str:
    """Generate safe log filename from phase and task names."""
    safe_phase = phase_name.replace(" ", "_").replace("/", "_")
    safe_task = task_name.replace(" ", "_").replace("/", "_")
    return f"{safe_phase}_{safe_task}.log"


def build_compose_exec(
    service: str, command: str, workdir: str = "", mode: str = "all"
) -> list[str]:
    """Build a docker-compose exec command for running inside a container.

    Returns a list suitable for subprocess.run (shell=False).
    The command is passed to bash -c inside the container.
    """
    env = bootstrap_env(mode)
    compose_base = env.get("DOCKER_COMPOSE_CMD", "podman-compose").split()

    cmd = compose_base + [
        "exec",
        "-T",
    ]
    if workdir:
        cmd.extend(["-w", workdir])
    cmd.append(service)

    if sys.platform != "win32":
        cmd.extend(["bash", "-c", command])
    else:
        cmd.extend(["cmd", "/c", command])

    return cmd


def run_single_task(
    name: str,
    cmd: list[str],
    log_file: Path,
    timeout: int,
) -> int:
    """Run a single task and return its exit code."""
    with log_file.open("w") as f:
        proc = subprocess.Popen(
            cmd,
            cwd=PROJECT_ROOT,
            stdin=subprocess.DEVNULL,
            stdout=f,
            stderr=subprocess.STDOUT,
        )
        try:
            proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
            return 124  # timeout exit code
        return proc.returncode


def run_parallel_ordered(
    phase_name: str,
    component_filter: str,
    tasks: list[tuple[str, str, list[str]]],
    num_workers: int | None = None,
) -> bool:
    """Run tasks in parallel (or serial if num_workers <= 1).

    Each task is a tuple of (name, service, command_list).
    Returns True if all tasks pass.
    """
    _ensure_log_dir()

    if num_workers is None:
        num_workers = min(4, (os.cpu_count() or 4) // 2)
        num_workers = max(num_workers, 1)

    # Filter tasks by component filter
    filtered = []
    for name, service, cmd in tasks:
        if component_filter:
            if component_filter in name or component_filter in service:
                filtered.append((name, service, cmd))
        else:
            filtered.append((name, service, cmd))

    # Prioritize formatting tasks by moving them to the beginning of the list
    format_tasks = []
    other_tasks = []
    for task in filtered:
        task_name = task[0].lower()
        if "format" in task_name or "fmt" in task_name:
            format_tasks.append(task)
        else:
            other_tasks.append(task)
    filtered = format_tasks + other_tasks

    if not filtered:
        return True

    task_timeout = get_phase_timeout(phase_name)

    if num_workers <= 1:
        # Serial execution
        return _run_tasks_serial(filtered, phase_name, task_timeout)

    # Parallel execution
    return _run_tasks_parallel(filtered, phase_name, task_timeout, num_workers)


def _run_tasks_serial(
    tasks: list[tuple[str, str, list[str]]],
    phase_name: str,
    timeout: int,
) -> bool:
    """Run tasks one at a time, streaming output live."""
    import sys as _sys

    all_passed = True
    for name, _service, cmd in tasks:
        log_file = LOG_DIR / _safe_log_name(phase_name, name)
        _sys.stdout.write(f"\n  [{name}] Running...\n")
        _sys.stdout.flush()

        proc = subprocess.Popen(
            cmd,
            cwd=PROJECT_ROOT,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        with log_file.open("w") as lf:
            for line in proc.stdout:  # type: ignore[union-attr]
                decoded = line.decode("utf-8", errors="replace")
                lf.write(decoded)
                _sys.stdout.write(f"  [{name}] {decoded}")
                _sys.stdout.flush()

        exit_code = proc.wait(timeout=timeout)
        if exit_code != 0:
            all_passed = False
            _sys.stdout.write(f"\n  [{name}] FAILED (exit={exit_code})\n")
            _sys.stdout.flush()
        else:
            _sys.stdout.write(f"\n  [{name}] PASSED\n")
            _sys.stdout.flush()

    return all_passed


def _run_tasks_parallel(
    tasks: list[tuple[str, str, list[str]]],
    phase_name: str,
    timeout: int,
    num_workers: int,
) -> bool:
    """Run tasks in parallel with a worker pool."""
    import threading
    from collections import deque

    task_queue = deque(tasks)
    results: dict[str, int] = {}
    lock = threading.Lock()
    all_done = threading.Event()

    def worker() -> None:
        while True:
            with lock:
                if not task_queue:
                    return
                name, service, cmd = task_queue.popleft()

            log_file = LOG_DIR / _safe_log_name(phase_name, name)
            ret = run_single_task(name, cmd, log_file, timeout)
            with lock:
                results[name] = ret
            if ret != 0:
                all_done.set()

    threads = []
    actual_workers = min(num_workers, len(tasks))
    for _ in range(actual_workers):
        t = threading.Thread(target=worker)
        t.start()
        threads.append(t)

    for t in threads:
        t.join()

    # Check results
    for name, exit_code in results.items():
        if exit_code != 0:
            log_file = LOG_DIR / _safe_log_name(phase_name, name)
            _print_failure(name, log_file, exit_code)
            return False
        log_file = LOG_DIR / _safe_log_name(phase_name, name)
        _print_success(name, log_file)

    return all(v == 0 for v in results.values())


def _print_success(name: str, log_file: Path) -> None:
    """Print success message with last few lines of log."""
    import sys as _sys

    _sys.stdout.write(f"\n  [{name}] PASSED\n")
    _sys.stdout.flush()


def _print_failure(name: str, log_file: Path, exit_code: int) -> None:
    """Print failure message with log tail."""
    import sys as _sys

    _sys.stdout.write(f"\n  [{name}] FAILED (exit={exit_code})\n")
    _sys.stdout.flush()
    if log_file.exists():
        lines = log_file.read_text(errors="replace").splitlines()
        tail = lines[-30:] if len(lines) > 30 else lines
        for line in tail:
            _sys.stdout.write(f"  [{name}] {line}\n")
        _sys.stdout.flush()
