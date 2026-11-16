"""Shared test utilities — Python equivalent of _test-utils.sh.

Provides parallel task runner, timeout handling, log management,
and result reporting for lint/unit/integration/coverage phases.
"""

import contextlib
import json
import os
import signal
import subprocess
import sys
import time
import typing
from pathlib import Path

import psutil
from _bootstrap import PROJECT_ROOT, bootstrap_env

LOG_DIR = PROJECT_ROOT / ".test_logs"
TIMEOUT_LINT = int(os.environ.get("TIMEOUT_LINT", "900"))
TIMEOUT_UNIT = int(os.environ.get("TIMEOUT_UNIT", "900"))
TIMEOUT_INTEGRATION = int(os.environ.get("TIMEOUT_INTEGRATION", "1200"))
TIMEOUT_BUILD = int(os.environ.get("TIMEOUT_BUILD", "600"))
LAST_PHASE_EXIT_CODE: int | None = None


def get_run_log_dir() -> Path:
    """Return log directory isolated by the current Compose project."""
    project_name = os.environ.get("COMPOSE_PROJECT_NAME", "").strip()
    if not project_name:
        return LOG_DIR
    safe_project = "".join(
        char if char.isalnum() or char in "-_." else "_" for char in project_name
    )
    return LOG_DIR / safe_project


LAST_PHASE_COMMANDS: dict[str, list[str]] = {}
LAST_PHASE_TASK_RESULTS: dict[str, dict[str, typing.Any]] = {}
QUIET_OUTPUT = False


def get_last_phase_task_results() -> dict[str, dict[str, typing.Any]]:
    """Return per-task exit, duration, and log metadata for the last phase."""
    return {name: dict(result) for name, result in LAST_PHASE_TASK_RESULTS.items()}


_PROCESS_TREE_WAIT_TIMEOUT = 3.0


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


def get_last_phase_exit_code() -> int | None:
    """Return last task exit code, including 124 for timeout."""
    return LAST_PHASE_EXIT_CODE


def set_quiet_output(enabled: bool) -> None:
    """Suppress task progress while preserving logs and exit status."""
    global QUIET_OUTPUT
    QUIET_OUTPUT = enabled


def get_last_phase_commands() -> dict[str, list[str]]:
    """Return filtered task commands from most recent phase."""
    return dict(LAST_PHASE_COMMANDS)


def clear_last_phase_commands() -> None:
    """Clear task metadata before a phase starts."""
    global LAST_PHASE_COMMANDS, LAST_PHASE_EXIT_CODE
    LAST_PHASE_COMMANDS = {}
    LAST_PHASE_EXIT_CODE = None


def _process_group_options() -> dict[str, int | bool]:
    """Return subprocess options that isolate the task on each platform."""
    if os.name == "nt":
        return {
            "creationflags": int(getattr(subprocess, "CREATE_NEW_PROCESS_GROUP", 0))
        }
    return {"start_new_session": True}


def _taskkill_tree(pid: int, force: bool = False) -> None:
    """Terminate a Windows process tree without affecting other processes."""
    command = ["taskkill", "/PID", str(pid), "/T"]
    if force:
        command.append("/F")
    subprocess.run(
        command,
        check=False,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def _terminate_process_tree(proc: subprocess.Popen) -> None:
    """Terminate and reap the task process and descendants only."""
    try:
        root = psutil.Process(proc.pid)
        descendants = root.children(recursive=True)
    except psutil.Error:
        root = None
        descendants = []

    if os.name == "nt":
        _taskkill_tree(proc.pid)
    else:
        with contextlib.suppress(ProcessLookupError):
            os.killpg(proc.pid, signal.SIGTERM)
        for process in reversed(descendants):
            with contextlib.suppress(psutil.Error):
                process.terminate()

    try:
        _, alive_descendants = psutil.wait_procs(
            descendants, timeout=_PROCESS_TREE_WAIT_TIMEOUT
        )
    except (psutil.Error, OSError):
        alive_descendants = descendants
    if alive_descendants:
        if os.name == "nt":
            _taskkill_tree(proc.pid, force=True)
        else:
            for process in alive_descendants:
                with contextlib.suppress(psutil.Error):
                    process.kill()
        psutil.wait_procs(alive_descendants, timeout=_PROCESS_TREE_WAIT_TIMEOUT)

    if root is not None:
        with contextlib.suppress(psutil.Error):
            root.terminate()
        _, alive_root = psutil.wait_procs([root], timeout=_PROCESS_TREE_WAIT_TIMEOUT)
        if alive_root:
            if os.name == "nt":
                _taskkill_tree(proc.pid, force=True)
            else:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(proc.pid, signal.SIGKILL)
            for process in alive_root:
                with contextlib.suppress(psutil.Error):
                    process.kill()
            psutil.wait_procs(alive_root, timeout=_PROCESS_TREE_WAIT_TIMEOUT)

    proc.wait()


def _ensure_log_dir() -> Path:
    """Ensure the current run's log directory exists and return it."""
    log_dir = get_run_log_dir()
    log_dir.mkdir(parents=True, exist_ok=True)
    return log_dir


def _safe_log_name(phase_name: str, task_name: str) -> str:
    """Generate safe log filename from phase and task names."""
    safe_phase = phase_name.replace(" ", "_").replace("/", "_")
    safe_task = task_name.replace(" ", "_").replace("/", "_")
    return f"{safe_phase}_{safe_task}.log"


def build_compose_exec(
    service: str,
    command: str,
    workdir: str = "",
    mode: str = "all",
    environment: list[str] | None = None,
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
    for name in environment or []:
        cmd.extend(["-e", name])
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
    *,
    stream_live: bool = False,
    output_lock: typing.Any = None,
) -> int:
    """Run one task, optionally teeing output to terminal."""
    import threading

    if output_lock is None:
        output_lock = threading.Lock()

    with log_file.open("w") as output:
        try:
            proc = subprocess.Popen(
                cmd,
                cwd=str(PROJECT_ROOT),
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE if stream_live else output,
                stderr=subprocess.STDOUT,
                **typing.cast(dict[str, typing.Any], _process_group_options()),
            )
        except FileNotFoundError:
            output.write(f"command not found: {cmd[0]}\n")
            return 127
        if not stream_live:
            try:
                proc.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                _terminate_process_tree(proc)
                return 124
            return proc.returncode

        assert proc.stdout is not None

        stream = proc.stdout
        assert stream is not None

        def stream_output() -> None:
            pending = b""
            for chunk in iter(lambda: stream.read(4096), b""):
                pending += chunk
                while b"\n" in pending:
                    raw_line, pending = pending.split(b"\n", 1)
                    emit_output(raw_line.decode("utf-8", errors="replace") + "\n")
            if pending:
                emit_output(pending.decode("utf-8", errors="replace"))

        def emit_output(output_line: str) -> None:
            with output_lock:
                output.write(output_line)
                output.flush()
                if not QUIET_OUTPUT:
                    sys.stdout.write(f"  [{name}] {output_line}")
                    sys.stdout.flush()

        reader = threading.Thread(target=stream_output)
        reader.start()
        try:
            proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            _terminate_process_tree(proc)
            exit_code = 124
        else:
            exit_code = proc.returncode
        reader.join()
        return exit_code


def version_matches(actual: str, expected: str) -> bool:
    """Match configured tool version as a token, not a shell expression."""
    actual_tokens = actual.strip().split()
    expected_tokens = expected.strip().split()
    return (
        bool(expected_tokens)
        and actual_tokens[: len(expected_tokens)] == expected_tokens
    )


def verify_seeded_gate(
    workspace: Path,
    seed_path: str | Path,
    run_gate: typing.Callable[[Path], bool],
) -> bool:
    """Verify gate rejects seed in temporary copy, then accepts clean copy."""
    import shutil
    import tempfile

    seed_relative = Path(seed_path)
    if seed_relative.is_absolute():
        try:
            seed_relative = seed_relative.relative_to(workspace)
        except ValueError:
            return False
    with tempfile.TemporaryDirectory(prefix="leedevkit-seeded-") as directory:
        isolated = Path(directory) / "workspace"
        shutil.copytree(workspace, isolated)
        isolated_seed = isolated / seed_relative
        isolated_seed.parent.mkdir(parents=True, exist_ok=True)
        isolated_seed.write_text("leedevkit-seeded-violation\n")
        try:
            if run_gate(isolated):
                return False
            isolated_seed.unlink()
            return run_gate(isolated)
        finally:
            isolated_seed.unlink(missing_ok=True)


def changed_paths_from_log(log_file: Path) -> list[str]:
    """Read safe newline-delimited changed paths from a gate log."""
    try:
        return [
            line for line in log_file.read_text(errors="replace").splitlines() if line
        ]
    except OSError:
        return []


def _go_pattern_evidence(log_file: Path, pattern: str) -> bool:
    """Require a selected Go test event when a filtered task succeeds."""
    if not pattern:
        return True
    try:
        import re

        matcher = re.compile(pattern)
        for line in log_file.read_text(errors="replace").splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            # `go test -run` emits only selected tests. Do not count package
            # events; run/skip test events prove that filtering selected one.
            if (
                event.get("Action") in {"run", "skip"}
                and isinstance(event.get("Test"), str)
                and matcher.search(event["Test"])
            ):
                return True
    except (OSError, ValueError):
        return False
    return False


def _task_result(
    name: str,
    exit_code: int,
    started: float,
    log_file: Path,
    required: bool = True,
) -> None:
    LAST_PHASE_TASK_RESULTS[name] = {
        "exit_code": exit_code,
        "duration_s": round(time.monotonic() - started, 3),
        "log": str(log_file.relative_to(PROJECT_ROOT))
        if log_file.is_relative_to(PROJECT_ROOT)
        else str(log_file),
        "required": required,
    }


def run_parallel_ordered(
    phase_name: str,
    component_filter: str,
    tasks: list[tuple[str, str, list[str]]],
    num_workers: int | None = None,
    *,
    pattern: str = "",
    optional_tasks: set[str] | None = None,
    require_task: bool = False,
) -> bool:
    global LAST_PHASE_EXIT_CODE, LAST_PHASE_COMMANDS, LAST_PHASE_TASK_RESULTS
    LAST_PHASE_EXIT_CODE = None
    LAST_PHASE_COMMANDS = {}
    LAST_PHASE_TASK_RESULTS = {}
    _ensure_log_dir()

    optional_tasks = optional_tasks or set()
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
        if require_task:
            LAST_PHASE_EXIT_CODE = 1
        return not require_task

    LAST_PHASE_COMMANDS = {name: list(cmd) for name, _service, cmd in filtered}
    task_timeout = get_phase_timeout(phase_name)

    if num_workers <= 1:
        return _run_tasks_serial(
            filtered, phase_name, task_timeout, pattern, optional_tasks
        )

    return _run_tasks_parallel(
        filtered, phase_name, task_timeout, num_workers, pattern, optional_tasks
    )


def _run_tasks_serial(
    tasks: list[tuple[str, str, list[str]]],
    phase_name: str,
    timeout: int,
    pattern: str = "",
    optional_tasks: set[str] | None = None,
) -> bool:
    """Run tasks one at a time, streaming output live."""
    global LAST_PHASE_EXIT_CODE
    import sys as _sys
    import threading

    optional_tasks = optional_tasks or set()
    all_passed = True
    for name, _service, cmd in tasks:
        log_file = get_run_log_dir() / _safe_log_name(phase_name, name)
        if not QUIET_OUTPUT:
            _sys.stdout.write(f"\n  [{name}] Running...\n")
            _sys.stdout.flush()

        started = time.monotonic()
        with log_file.open("w") as output:
            proc = subprocess.Popen(
                cmd,
                cwd=PROJECT_ROOT,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                **typing.cast(dict[str, typing.Any], _process_group_options()),
            )

            def stream_output() -> None:
                assert proc.stdout is not None
                for line in proc.stdout:
                    decoded = line.decode("utf-8", errors="replace")
                    output.write(decoded)
                    output.flush()
                    if not QUIET_OUTPUT:
                        _sys.stdout.write(f"  [{name}] {decoded}")
                        _sys.stdout.flush()

            reader = threading.Thread(target=stream_output)
            reader.start()
            try:
                exit_code = proc.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                _terminate_process_tree(proc)
                exit_code = 124
            reader.join()

        _task_result(name, exit_code, started, log_file, name not in optional_tasks)
        if exit_code != 0 and name not in optional_tasks:
            LAST_PHASE_EXIT_CODE = exit_code
            all_passed = False
        elif (
            pattern
            and name.startswith("go-")
            and not _go_pattern_evidence(log_file, pattern)
        ):
            LAST_PHASE_EXIT_CODE = 1
            all_passed = False
        elif not QUIET_OUTPUT:
            _sys.stdout.write(f"\n  [{name}] PASSED\n")
            _sys.stdout.flush()

    return all_passed


def _run_tasks_parallel(
    tasks: list[tuple[str, str, list[str]]],
    phase_name: str,
    timeout: int,
    num_workers: int,
    pattern: str = "",
    optional_tasks: set[str] | None = None,
) -> bool:
    """Run tasks in parallel with a worker pool."""
    global LAST_PHASE_EXIT_CODE
    import threading
    from collections import deque

    optional_tasks = optional_tasks or set()
    task_queue = deque(tasks)
    results: dict[str, int] = {}
    lock = threading.Lock()
    output_lock = threading.Lock()
    all_done = threading.Event()

    def worker() -> None:
        while True:
            with lock:
                if not task_queue:
                    return
                name, _service, cmd = task_queue.popleft()

            log_file = get_run_log_dir() / _safe_log_name(phase_name, name)
            started = time.monotonic()
            ret = run_single_task(
                name,
                cmd,
                log_file,
                timeout,
                stream_live=True,
                output_lock=output_lock,
            )
            if (
                ret == 0
                and pattern
                and name.startswith("go-")
                and not _go_pattern_evidence(log_file, pattern)
            ):
                ret = 1
            _task_result(name, ret, started, log_file, name not in optional_tasks)
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
        if exit_code != 0 and name not in optional_tasks:
            LAST_PHASE_EXIT_CODE = exit_code
            log_file = get_run_log_dir() / _safe_log_name(phase_name, name)
            _print_failure(name, log_file, exit_code)
            return False
        log_file = get_run_log_dir() / _safe_log_name(phase_name, name)
        _print_success(name, log_file)

    return all(v == 0 or name in optional_tasks for name, v in results.items())


def _print_success(name: str, log_file: Path) -> None:
    """Print task success unless quiet output is enabled."""
    if QUIET_OUTPUT:
        return
    import sys as _sys

    _sys.stdout.write(f"\n  [{name}] PASSED\n")
    _sys.stdout.flush()


def _print_failure(name: str, log_file: Path, exit_code: int) -> None:
    """Print compact failure in quiet mode, full log otherwise."""
    import sys as _sys

    _sys.stdout.write(f"\n  [{name}] FAILED (exit={exit_code})\n")
    _sys.stdout.flush()
    if QUIET_OUTPUT or not log_file.exists():
        return
    lines = log_file.read_text(errors="replace").splitlines()
    for line in lines:
        _sys.stdout.write(f"  [{name}] {line}\n")
    _sys.stdout.flush()
