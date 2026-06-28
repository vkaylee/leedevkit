#!/usr/bin/env python3
# ruff: noqa: N999
"""
safe-run.py

A bulletproof wrapper to run commands without leaking PTY file descriptors.
Replaces safe-run.sh with robust Python process management.
Optimized to use PTY for real-time line buffering while preventing background hang.
"""

import contextlib
import os
import pty
import select
import signal
import subprocess
import sys
import threading
import time as _time
from types import FrameType

import psutil
from _arg_sanitizer import ArgSanitizeError, sanitize


def read_from_pty(master_fd: int, stop_event: threading.Event) -> None:
    """Reads from the PTY master until stop_event is set and no more data is available."""
    while not stop_event.is_set():
        try:
            # Wait up to 0.1s for data to become available
            r, _, _ = select.select([master_fd], [], [], 0.1)
            if master_fd in r:
                data = os.read(master_fd, 4096)
                if not data:
                    break  # EOF  # pragma: no cover
                # Write directly to sys.stdout's underlying buffer to avoid encoding issues
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()
        except (OSError, ValueError):
            break  # EIO or closed FD

    # One last non-blocking read to drain buffer
    while True:  # pragma: no cover
        try:
            r, _, _ = select.select([master_fd], [], [], 0.0)
            if master_fd in r:
                data = os.read(master_fd, 4096)
                if not data:
                    break
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()
            else:
                break
        except (OSError, ValueError):
            break


def kill_process_tree(pid: int) -> None:
    """Kills a process and all its descendants using psutil."""
    try:
        parent: psutil.Process = psutil.Process(pid)
        children: list[psutil.Process] = parent.children(recursive=True)
        # Send SIGTERM to children
        for child in children:
            with contextlib.suppress(psutil.NoSuchProcess):  # pragma: no cover
                child.terminate()

        # Send SIGTERM to parent
        with contextlib.suppress(psutil.NoSuchProcess):
            parent.terminate()

        # Wait for them to die
        gone, alive = psutil.wait_procs(children + [parent], timeout=3.0)

        # Force kill if still alive
        for p in alive:
            with contextlib.suppress(psutil.NoSuchProcess):  # pragma: no cover
                p.kill()
    except psutil.NoSuchProcess:  # pragma: no cover
        pass


def validate_command(cmd_args: list[str] | str) -> None:
    """Check if the command contains PTY-hang risks: banned patterns, unsafe args."""
    banned_interpreters = [
        "python -c",
        "python3 -c",
        "node -e",
        "ruby -e",
        "perl -e",
        "php -r",
    ]
    banned_patterns = ["<<EOF", "<<-EOF"]

    cmd_str = cmd_args if isinstance(cmd_args, str) else " ".join(cmd_args).lower()

    for pattern in banned_interpreters + banned_patterns:
        if pattern in cmd_str:
            print("\n" + "!" * 80, file=sys.stderr)
            print(
                f"🚨 PTY SECURITY ERROR: Command contains banned pattern: '{pattern}'",
                file=sys.stderr,
            )
            print(
                "⚠️  Project Policy: 100% of logic must reside in physical script files.",
                file=sys.stderr,
            )
            print(
                "💡 Solution: Use ./scripts/ai-tools/run-inline -l <lang> -c 'code'",
                file=sys.stderr,
            )
            print("!" * 80 + "\n", file=sys.stderr)
            sys.exit(1)

    # Validate args through the AI arg sanitizer
    if isinstance(cmd_args, list) and len(cmd_args) > 0:
        try:
            sanitize(cmd_args)
        except ArgSanitizeError as e:
            print("\n" + "!" * 80, file=sys.stderr)
            print(f"🚨 PTY SAFETY ERROR: {e}", file=sys.stderr)
            print("!" * 80 + "\n", file=sys.stderr)
            sys.exit(1)


def execute_command(cmd_args: list[str], timeout_sec: float) -> int:
    """Execute the command in a PTY and wait for completion."""
    # Use a PTY to force child processes to use line-buffering
    master_fd, slave_fd = pty.openpty()

    stop_read_event = threading.Event()
    read_thread = threading.Thread(
        target=read_from_pty, args=(master_fd, stop_read_event)
    )
    read_thread.start()

    # Hybrid execution: if passed as a single string, assume shell string.
    use_shell = len(cmd_args) == 1
    exec_cmd = cmd_args[0] if use_shell else cmd_args

    try:
        proc = subprocess.Popen(
            exec_cmd,
            shell=use_shell,
            stdout=slave_fd,
            stderr=subprocess.STDOUT,
            stdin=subprocess.DEVNULL,
            start_new_session=True,
            close_fds=True,
        )  # nosec
    except FileNotFoundError:
        print(f"Error: Command not found: {cmd_args[0]}", file=sys.stderr)
        stop_read_event.set()
        read_thread.join()
        with contextlib.suppress(OSError):
            os.close(slave_fd)
            os.close(master_fd)
        return 127

    # Close slave fd in parent, so only child has it
    os.close(slave_fd)

    def cleanup_and_exit(
        signum: int, frame: FrameType | None
    ) -> None:  # pragma: no cover
        print("\n⚠️ Interrupted. Killing process tree...", file=sys.stderr)
        kill_process_tree(proc.pid)
        stop_read_event.set()
        read_thread.join()
        os.close(master_fd)
        sys.exit(130)

    signal.signal(signal.SIGINT, cleanup_and_exit)  # pragma: no cover
    signal.signal(signal.SIGTERM, cleanup_and_exit)  # pragma: no cover

    exit_code = 1
    try:
        proc.wait(timeout=timeout_sec)
        exit_code = proc.returncode
    except subprocess.TimeoutExpired:
        print(
            f"\n⏳ Timeout ({timeout_sec}s) reached. Killing process tree...",
            file=sys.stderr,
        )
        kill_process_tree(proc.pid)
        exit_code = 124  # standard timeout exit code
    finally:
        # ── CRITICAL: Kill remaining children to release PTY slave FD ──
        # Normal exit: fast os.killpg to nuke leftover children (conmon).
        # Timeout/signal: kill_process_tree already did full cleanup above.
        if exit_code != 124:
            try:
                os.killpg(proc.pid, signal.SIGKILL)
                _time.sleep(0.2)
            except OSError:
                pass  # Process group already gone

        stop_read_event.set()
        read_thread.join()
        with contextlib.suppress(OSError):
            os.close(master_fd)

    return exit_code


def main() -> None:
    """Main entry point for safe-run."""
    min_args = 3
    if len(sys.argv) < min_args:
        print(
            f"Usage: {sys.argv[0]} [timeout_seconds] 'command to run' [args...]",
            file=sys.stderr,
        )
        sys.exit(1)

    try:
        timeout_sec = float(sys.argv[1])
    except ValueError:
        print("Error: timeout must be a number", file=sys.stderr)
        sys.exit(1)

    cmd_args = sys.argv[2:]
    validate_command(cmd_args)

    print(
        f"🛡️ [Python] Running detached command with {timeout_sec}s timeout: {cmd_args}",
        file=sys.stderr,
    )

    exit_code = execute_command(cmd_args, timeout_sec)
    print(f"✅ [Python] Command finished with exit code {exit_code}", file=sys.stderr)
    sys.exit(exit_code)


if __name__ == "__main__":
    main()  # pragma: no cover
