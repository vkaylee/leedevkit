#!/usr/bin/env python3
"""Produce dependency, license, and container scan evidence without network access."""

from __future__ import annotations

import argparse
import importlib.metadata
import json
import re
import shutil
from pathlib import Path

_REQUIREMENT = re.compile(r"^(?P<name>[A-Za-z0-9_.-]+)==(?P<version>[^;#]+)")


def _read_lock(path: Path) -> list[dict[str, str]]:
    packages: list[dict[str, str]] = []
    seen: set[str] = set()
    optional = False
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if line == "# Optional browser harness":
            optional = True
            continue
        if not line or line.startswith("#"):
            continue
        requirement = line.split(";", 1)[0].strip()
        match = _REQUIREMENT.fullmatch(requirement)
        if match is None:
            raise ValueError(f"lock line {number} is not exact package pin: {line}")
        name = match.group("name")
        normalized = name.lower().replace("_", "-")
        if normalized in seen:
            raise ValueError(f"duplicate package in lock: {name}")
        seen.add(normalized)
        packages.append(
            {
                "name": name,
                "version": match.group("version"),
                "scope": "optional" if optional else "required",
            }
        )
    if not packages:
        raise ValueError("dependency lock is empty")
    return packages


def _license_evidence(packages: list[dict[str, str]]) -> list[dict[str, str]]:
    evidence: list[dict[str, str]] = []
    for package in packages:
        try:
            metadata = importlib.metadata.metadata(package["name"])
            license_name = metadata.get("License", "").strip()
            if not license_name:
                license_name = next(
                    (
                        value.removeprefix("License :: ").strip()
                        for value in metadata.get_all("Classifier", [])
                        if value.startswith("License :: ")
                    ),
                    "unknown",
                )
            installed = importlib.metadata.version(package["name"])
        except importlib.metadata.PackageNotFoundError:
            installed = "not-installed"
            license_name = "unknown"
        evidence.append(
            {
                **package,
                "installed": installed,
                "license": license_name or "unknown",
            }
        )
    return evidence


def _container_evidence(root: Path) -> dict[str, object]:
    dockerfiles = sorted(root.glob("**/Dockerfile"))
    tool = shutil.which("trivy")
    return {
        "status": "available" if tool else "unavailable",
        "tool": "trivy" if tool else None,
        "reason": None
        if tool
        else "trivy not installed; Dockerfile scan requires CI-provided scanner",
        "dockerfiles": [str(path.relative_to(root)) for path in dockerfiles],
    }


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Write LeeDevKit supply-chain evidence"
    )
    parser.add_argument("--lock", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--container-root", type=Path, default=Path("."))
    args = parser.parse_args()
    packages = _read_lock(args.lock)
    result = {
        "schema": "leedevkit.supply-chain.v1",
        "dependency_lock": str(args.lock),
        "dependencies": _license_evidence(packages),
        "container_scan": _container_evidence(args.container_root),
        "notes": [
            "Dependency versions come from reviewed exact lock entries.",
            "SHA-256 manifest detects artifact changes but does not authenticate publisher origin.",
        ],
    }
    args.output.write_text(
        json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
