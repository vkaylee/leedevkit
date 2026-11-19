"""Opt-in Go core protocol bridge.

Python remains default control plane. Go protocol failures fail closed when
explicitly enabled; callers can keep Python fallback disabled for required use.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
from typing import Any


def plan_gates_go(
    payload: dict[str, Any],
    *,
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any] | None:
    """Return Go gate plan when enabled, or None for Python default path."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return None
    executable = binary_path or shutil.which("leedevkit-core")
    if not executable:
        raise RuntimeError("Go core enabled but binary missing")
    request = json.dumps(payload, separators=(",", ":"))
    try:
        result = subprocess.run(
            [executable, "--plan-gates"],
            input=request,
            text=True,
            capture_output=True,
            check=False,
            timeout=timeout,
            env={
                key: value
                for key, value in os.environ.items()
                if key in {"PATH", "HOME", "TMPDIR", "LEEDEVKIT_USE_GO_CORE"}
            },
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("Go core planning timed out") from exc
    if result.returncode != 0:
        raise RuntimeError(
            f"Go core planning failed with exit code {result.returncode}: "
            f"{result.stderr.strip()}"
        )
    try:
        response = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError("Go core returned malformed planning JSON") from exc
    if not isinstance(response, dict) or response.get("schema_version") != 1:
        raise RuntimeError("Go core returned unsupported planning schema")
    gates = response.get("gates")
    if not isinstance(gates, list):
        raise RuntimeError("Go core returned invalid gate plan")
    return response
