#!/usr/bin/env python3
"""Assess task complexity and route Claude Code subagents by model tier."""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path
from typing import Any

TIERS = ("haiku", "sonnet", "opus", "fable")
EFFORTS = ("low", "medium", "high", "xhigh")
_TARGET_TOOLS = {"Agent", "Task"}

_PATTERNS = (
    ("fable", "xhigh", re.compile(r"\b(ambiguous|novel|multi[- ]system|cross[- ]system|critical breach|failed escalation)\b", re.I)),
    ("opus", "high", re.compile(r"\b(debug|root cause|architecture|security|vulnerability|migration|race condition|performance|optimi[sz]e|production)\b", re.I)),
    ("sonnet", "medium", re.compile(r"\b(implement|feature|refactor|test|endpoint|api|integration|fix|update)\b", re.I)),
    ("haiku", "low", re.compile(r"\b(grep|find|list|read|count|format|rename|lint|typo|lookup|status)\b", re.I)),
)


def _project_root(start: Path | None = None) -> Path:
    current = (start or Path.cwd()).resolve()
    for parent in (current, *current.parents):
        if (parent / "leedevkit.toml").exists() or (parent / ".git").exists():
            return parent
    return current


def _load_toml(path: Path) -> dict[str, Any]:
    try:
        import tomllib

        with path.open("rb") as stream:
            return tomllib.load(stream)
    except (ImportError, ModuleNotFoundError):
        try:
            import tomli

            with path.open("rb") as stream:
                return tomli.load(stream)
        except ImportError:
            return {}


def _env_bool(value: str | None, default: bool) -> bool:
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def load_routing_config(project_root: Path | None = None) -> dict[str, Any]:
    """Load project routing config, with environment overrides."""
    root = _project_root(project_root)
    config_path = root / "leedevkit.toml"
    raw = _load_toml(config_path) if config_path.exists() else {}
    routing = raw.get("ai", {}).get("model_routing", {})
    if not isinstance(routing, dict):
        routing = {}
    enabled = _env_bool(
        os.environ.get("LEEDEVKIT_MODEL_ROUTING_ENABLED"),
        bool(routing.get("enabled", False)),
    )
    default_model = os.environ.get(
        "LEEDEVKIT_MODEL_ROUTING_DEFAULT_MODEL", routing.get("default_model", "sonnet")
    )
    if default_model not in TIERS:
        default_model = "sonnet"
    return {"enabled": enabled, "default_model": default_model}


def _file_count(files: int | list[Any] | tuple[Any, ...] | None) -> int:
    if isinstance(files, int):
        return max(0, files)
    if isinstance(files, (list, tuple)):
        return len(files)
    return 0


def assess_task(
    text: str,
    files: int | list[Any] | tuple[Any, ...] = 0,
    risk: str = "",
    config: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Return a deterministic model-tier recommendation for a task."""
    text = str(text or "")
    risk = str(risk or "")
    haystack = f"{text} {risk}".strip()
    file_count = _file_count(files)

    for tier, effort, pattern in _PATTERNS:
        if pattern.search(haystack):
            confidence = 0.9 if tier in {"opus", "fable"} else 0.82
            rationale = f"matched {tier} complexity signal"
            break
    else:
        tier, effort = "sonnet", "medium"
        confidence = 0.45
        rationale = "no strong complexity signal; using balanced default"

    if file_count >= 8 and tier == "haiku":
        tier, effort, confidence, rationale = (
            "sonnet",
            "medium",
            0.7,
            "many files require a broader context",
        )
    elif file_count >= 15 and tier in {"haiku", "sonnet"}:
        tier, effort, confidence, rationale = (
            "opus",
            "high",
            0.78,
            "large file scope requires deeper reasoning",
        )
    if len(text) >= 1200 and tier == "haiku":
        tier, effort, confidence, rationale = (
            "sonnet",
            "medium",
            0.65,
            "long task description suggests non-trivial scope",
        )
    if re.search(r"\b(critical|destructive|data loss|secret|credential)\b", haystack, re.I):
        tier, effort, confidence, rationale = (
            "opus",
            "high",
            0.92,
            "high-risk task requires stronger review",
        )

    if config and config.get("default_model") in TIERS and not any(
        pattern.search(haystack) for _, _, pattern in _PATTERNS
    ) and file_count == 0:
        tier = config["default_model"]
        effort = EFFORTS[min(TIERS.index(tier), len(EFFORTS) - 1)]
        rationale = "using configured default tier"

    return {
        "suggested_tier": tier,
        "suggested_effort": effort,
        "confidence": round(confidence, 2),
        "rationale": rationale,
    }


def route_pretool_input(payload: dict[str, Any], config: dict[str, Any] | None = None) -> dict[str, Any] | None:
    """Return updated tool input, or None when no change is needed."""
    if not isinstance(payload, dict) or payload.get("tool_name") not in _TARGET_TOOLS:
        return None
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, dict) or tool_input.get("model"):
        return None
    config = config or load_routing_config()
    if not config.get("enabled", False):
        return None
    text = " ".join(
        str(tool_input.get(key, ""))
        for key in ("description", "prompt", "task", "name", "subagent_type")
    )
    assessment = assess_task(text, config=config)
    updated = dict(tool_input)
    updated["model"] = assessment["suggested_tier"]
    return updated


def main() -> int:
    """Run as a Claude Code PreToolUse hook."""
    try:
        payload = json.load(sys.stdin)
        updated = route_pretool_input(payload)
        if updated is not None:
            json.dump(
                {
                    "hookSpecificOutput": {
                        "hookEventName": "PreToolUse",
                        "updatedInput": updated,
                    }
                },
                sys.stdout,
                separators=(",", ":"),
            )
            sys.stdout.write("\n")
    except Exception:
        return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
