#!/usr/bin/env python3
"""Shared logging utilities for LeeDevKit.

All orchestrator and handler modules import from here instead of
defining their own copy-pasted logging functions (DRY principle).
"""

from __future__ import annotations

import sys
import typing


class Colors:
    """ANSI color codes for terminal output."""
    GREEN = "\033[0;32m"
    RED = "\033[0;31m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    BOLD = "\033[1m"
    NC = "\033[0m"


def log_info(msg: str) -> None:
    """Log an informational message to stderr."""
    print(f"{Colors.BLUE}{Colors.BOLD}ℹ️ {msg}{Colors.NC}", file=sys.stderr, flush=True)


def log_success(msg: str) -> None:
    """Log a success message to stderr."""
    print(f"{Colors.GREEN}{Colors.BOLD}✅ {msg}{Colors.NC}", file=sys.stderr, flush=True)


def log_warn(msg: str) -> None:
    """Log a warning message to stderr."""
    print(f"{Colors.YELLOW}{Colors.BOLD}⚠️ {msg}{Colors.NC}", file=sys.stderr, flush=True)


def log_error(msg: str, file: typing.TextIO = sys.stderr) -> None:
    """Log an error message to stderr (or a custom file handle)."""
    print(f"{Colors.RED}{Colors.BOLD}❌ {msg}{Colors.NC}", file=file, flush=True)
