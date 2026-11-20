"""Opt-in Go core protocol bridge.

Python remains default control plane. Go protocol failures fail closed when
explicitly enabled; callers can keep Python fallback disabled for required use.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
from pathlib import Path
from typing import Any

from _platform import go_binary


def _go_binary(binary_path: str | None = None) -> str:
    executable = binary_path or shutil.which("leedevkit-core")
    if executable:
        return executable
    runtime = os.environ.get("DEVKIT_HOME")
    if runtime:
        try:
            return str(go_binary(Path(runtime)))
        except RuntimeError:
            pass
    raise RuntimeError("Go core enabled but binary missing")


def _run_protocol(
    command: str,
    payload: dict[str, Any],
    *,
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any]:
    executable = _go_binary(binary_path)
    try:
        result = subprocess.run(
            [executable, command],
            input=json.dumps(payload, separators=(",", ":")),
            text=True,
            capture_output=True,
            check=False,
            timeout=timeout,
            env={
                key: value
                for key, value in os.environ.items()
                if key
                in {"PATH", "HOME", "TMPDIR", "DEVKIT_HOME", "LEEDEVKIT_USE_GO_CORE"}
            },
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError(f"Go core {command} timed out") from exc
    if result.returncode != 0:
        raise RuntimeError(
            f"Go core {command} failed with exit code {result.returncode}: "
            f"{result.stderr.strip()}"
        )
    try:
        response = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Go core {command} returned malformed JSON") from exc
    if not isinstance(response, dict) or response.get("schema_version") != 1:
        raise RuntimeError(f"Go core {command} returned unsupported schema")
    return response


def resolve_provider_go(
    payload: dict[str, Any],
    *,
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any] | None:
    """Validate normalized provider through Go in opt-in shadow mode."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return None
    return _run_protocol(
        "--resolve-provider",
        {"schema_version": 1, "provider": payload},
        binary_path=binary_path,
        timeout=timeout,
    )


def plan_gates_go(
    payload: dict[str, Any],
    *,
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any] | None:
    """Return Go gate plan when enabled, or None for Python default path."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return None
    response = _run_protocol(
        "--plan-gates", payload, binary_path=binary_path, timeout=timeout
    )
    if not isinstance(response.get("gates"), list):
        raise RuntimeError("Go core returned invalid gate plan")
    return response


def plan_lifecycle_go(
    payload: dict[str, Any],
    *,
    operation: str = "up",
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any] | None:
    """Return Go lifecycle argv plan when shadow mode is enabled."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return None
    return _run_protocol(
        "--plan-lifecycle",
        {"schema_version": 1, "operation": operation, "plan": payload},
        binary_path=binary_path,
        timeout=timeout,
    )


def plan_quality_gates_go(
    gates: list[dict[str, Any]],
    *,
    names: list[str] | None = None,
    binary_path: str | None = None,
    timeout: float = 10.0,
) -> dict[str, Any] | None:
    """Send normalized Python gates to Go for shadow planning."""
    if os.environ.get("LEEDEVKIT_USE_GO_CORE") != "1":
        return None
    definitions = {str(gate["name"]): gate for gate in gates}
    return plan_gates_go(
        {
            "schema_version": 1,
            "names": names or [str(gate["name"]) for gate in gates],
            "definitions": definitions,
        },
        binary_path=binary_path,
        timeout=timeout,
    )
