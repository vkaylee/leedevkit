"""Runtime checks shared by direct Python entrypoints."""

from __future__ import annotations

import os
import sys
from pathlib import Path


class RuntimeInterpreterError(RuntimeError):
    """Raised when a DevKit script runs outside its project-local venv."""


def expected_python() -> Path:
    """Return interpreter required by scripts in this DevKit checkout."""
    return Path(__file__).resolve().parent.parent / ".venv" / "bin" / "python3"


def require_project_venv() -> None:
    """Reject host Python before a direct entrypoint performs any work."""
    expected = expected_python()
    expected_root = expected.parent.parent
    current = Path(sys.executable).absolute()
    current_root = Path(sys.prefix).resolve()
    if current != expected.absolute() or current_root != expected_root.resolve():
        raise RuntimeInterpreterError(
            "LeeDevKit Python scripts require project-local interpreter "
            f"{expected}; current interpreter is {current}. "
            "Run ./leedevkit or invoke required interpreter directly."
        )


def enforce_project_venv(*, allow_bootstrap: bool = False) -> None:
    """Fail cleanly unless entrypoint runs in project venv."""
    if allow_bootstrap and os.environ.get("LEEDEVKIT_BOOTSTRAP") == "1":
        return
    try:
        require_project_venv()
    except RuntimeInterpreterError as error:
        print(f"❌ {error}", file=sys.stderr)
        raise SystemExit(125) from error
