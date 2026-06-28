"""Argument sanitizer for AI-generated tool inputs.

Validates and sanitizes command-line arguments from AI agents
to prevent PTY hangs, shell injection, and control character leaks.

All Hermetic Toolbox scripts (_cargo, _npm, _diesel, _git) should
sanitize args through this module before passing to subprocess.
"""

import shlex
import sys
from typing import NoReturn


class ArgSanitizeError(Exception):
    """Raised when AI-provided arguments contain dangerous patterns."""


# Control characters that can cause PTY hangs (block outright)
# Excludes tab (0x09) as it's commonly legit in code/strings
TAB_CHAR_CODE = 9
_FORBIDDEN_CHARS: set[str] = {chr(c) for c in range(32) if c != TAB_CHAR_CODE}

# Shell injection / code execution patterns — block always
_INJECTION_PATTERNS: tuple[str, ...] = (
    "$(",  # command substitution
    "`",  # backtick substitution
    "${",  # variable expansion with braces
    "#{",  # Ruby-style interpolation
    "<(",  # process substitution
    ")>",  # process substitution close
    "\\x",  # hex escape (potential bypass)
    "<<<",  # here-string
)

# Multi-line / heredoc triggers — block in single args
# (multi-line commit messages go through _git_wrapper.py tempfile path)
_FORBIDDEN_IN_ARG: tuple[str, ...] = (
    "<<EOF",
    "<<-EOF",
    "\n",
    "\r",
)

# Args that should never reach the shell directly
_BANNED_FULL_ARGS: tuple[str, ...] = (
    "python",
    "python3",
    "node",
    "ruby",
    "perl",
    "php",
)


def _die(msg: str) -> NoReturn:
    """Print error to stderr and exit."""
    print(f"\n🚨 PTY SAFETY ERROR: {msg}", file=sys.stderr)
    print(
        "💡 Instead use: ./scripts/ai-tools/run-inline -l <lang> -c 'code'",
        file=sys.stderr,
    )
    print(
        "   Or pipe: echo 'code' | ./scripts/ai-tools/run-inline -l <lang>",
        file=sys.stderr,
    )
    print(
        "   Supported: python python3 bun node js bash sh",
        file=sys.stderr,
    )
    sys.exit(1)


def _validate_single_arg(arg: str) -> None:
    """Validate a single argument. Raises ArgSanitizeError on danger."""
    for ch in arg:
        if ch in _FORBIDDEN_CHARS:
            raise ArgSanitizeError(
                f"Control character U+{ord(ch):04X} in argument: {arg!r}"
            )

    # Unmatched quotes → shell hang
    for quote in ('"', "'"):
        if arg.count(quote) % 2 != 0:
            msg = f"Unmatched {quote} quote in argument (causes shell hang): {arg!r}"
            raise ArgSanitizeError(msg)

    # Shell injection patterns
    for pattern in _INJECTION_PATTERNS:
        if pattern in arg:
            raise ArgSanitizeError(
                f"Shell injection pattern '{pattern}' in argument: {arg!r}"
            )

    # Forbidden content in single args
    for pattern in _FORBIDDEN_IN_ARG:
        if pattern in arg:
            raise ArgSanitizeError(
                f"Forbidden pattern '{pattern!r}' in argument: {arg!r}"
            )


def _check_banned_interpreter(args: list[str]) -> None:
    """Block inline code execution (python -c, node -e) but allow script files."""
    MIN_INTERPRETER_ARGS = 2
    if not args or len(args) < MIN_INTERPRETER_ARGS:
        return
    first = args[0].lower()
    if first in _BANNED_FULL_ARGS:
        # Only block if -c or -e follows (inline code pattern)
        # python script.py is safe (temp file, no shell interpretation)
        second = args[1] if len(args) > 1 else ""
        if second in ("-c", "-e"):
            raise ArgSanitizeError(
                f"Inline '{first} -c/-e' execution blocked. "
                f"Use ./scripts/ai-tools/run-inline -l {first} instead."
            )


def sanitize(args: list[str], *, die_on_error: bool = False) -> list[str]:
    """Validate and sanitize AI-provided command arguments.

    Returns sanitized args (unchanged if all pass).
    Raises ArgSanitizeError on dangerous input.

    Set die_on_error=True to exit immediately instead of raising.
    """
    try:
        for arg in args:
            _validate_single_arg(arg)
        _check_banned_interpreter(args)
    except ArgSanitizeError as e:
        if die_on_error:
            _die(str(e))
        raise
    return args


def sanitize_shell_string(args: list[str]) -> str:
    """Build a shell-safe command string from arguments.

    Each arg is individually quoted via shlex.quote.
    Safe for use with bash -c.
    """
    sanitize(args)
    return " ".join(shlex.quote(a) for a in args)


def is_safe_for_list_form(args: list[str]) -> bool:
    """Check if args can safely be passed as subprocess list (no shell)."""
    try:
        sanitize(args)
        return True
    except ArgSanitizeError:
        return False
