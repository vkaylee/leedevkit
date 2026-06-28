#!/usr/bin/env python3
import atexit
import contextlib
import os
import subprocess
import sys
import tempfile
from pathlib import Path

from _arg_sanitizer import ArgSanitizeError, sanitize


def main() -> None:  # noqa: PLR0912
    script_dir = Path(__file__).parent.resolve()
    safe_run_path = script_dir / "_safe_run.py"

    timeout = os.environ.get("GIT_TIMEOUT", "120")
    args = sys.argv[1:]

    is_commit = "commit" in args

    processed_args: list[str] = []
    temp_files: list[str] = []

    def cleanup() -> None:
        for tf in temp_files:
            with contextlib.suppress(OSError):
                Path(tf).unlink()

    atexit.register(cleanup)

    if is_commit:
        messages = []
        skip_next = False

        for i, arg in enumerate(args):
            if skip_next:
                skip_next = False
                continue

            if arg in ("-m", "--message"):
                if i + 1 < len(args):
                    messages.append(args[i + 1])
                    skip_next = True
                else:
                    processed_args.append(arg)
            elif arg.startswith("-m") and len(arg) > 2:  # noqa: PLR2004
                messages.append(arg[2:])
            elif arg.startswith("--message="):
                messages.append(arg.split("=", 1)[1])
            else:
                processed_args.append(arg)

        if messages:
            # We found commit messages, write them to a file
            fd, temp_path = tempfile.mkstemp(
                prefix="git_ai_msg_", suffix=".txt", dir="/tmp"
            )
            temp_files.append(temp_path)

            with os.fdopen(fd, "w", encoding="utf-8") as f:
                # Git joins multiple -m messages with blank lines in between
                f.write("\n\n".join(messages))
                f.write("\n")

            # Add -F <temp_file>
            processed_args.append("-F")
            processed_args.append(temp_path)

        # Prevent PTY hangs with GPG agent by injecting --no-gpg-sign if not already present
        if (
            "--no-gpg-sign" not in processed_args
            and "-S" not in processed_args
            and "--gpg-sign" not in processed_args
        ):
            processed_args.append("--no-gpg-sign")
    else:
        processed_args = args

    # Sanitize remaining args (messages already extracted to temp files)
    try:
        processed_args = sanitize(processed_args)
    except ArgSanitizeError:
        # Safety: if sanitize fails, keep original args — never pass empty
        # to avoid git interactive mode + stdin=DEVNULL = infinite hang
        print("⚠️ Arg sanitizer warning: some args may be unsafe", file=sys.stderr)
        # Don't change processed_args — let _safe_run.py's validate_command
        # catch any real danger before Popen

    cmd = [sys.executable, str(safe_run_path), timeout, "git"] + processed_args
    sys.exit(subprocess.call(cmd))


if __name__ == "__main__":  # pragma: no cover
    main()
