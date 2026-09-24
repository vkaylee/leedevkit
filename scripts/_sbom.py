#!/usr/bin/env python3
"""Generate a deterministic CycloneDX SBOM from LeeDevKit's dependency lock."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

_LOCK_MARKER = "# Optional browser harness"


def _components(lock_file: Path) -> list[dict[str, Any]]:
    components: list[dict[str, Any]] = []
    optional = False
    for raw_line in lock_file.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if line == _LOCK_MARKER:
            optional = True
            continue
        if not line or line.startswith("#"):
            continue
        requirement, _, marker = line.partition(";")
        match = re.fullmatch(r"([A-Za-z0-9_.-]+)==([^;]+)", requirement.strip())
        if match is None:
            continue
        name, version = match.groups()
        component: dict[str, Any] = {
            "type": "library",
            "name": name,
            "version": version,
            "purl": f"pkg:pypi/{name.lower().replace('_', '-')}@{version}",
            "scope": "optional" if optional else "required",
        }
        if marker.strip():
            component["properties"] = [{"name": "lock.marker", "value": marker.strip()}]
        components.append(component)
    return sorted(components, key=lambda item: (item["name"].lower(), item["version"]))


def generate_sbom(lock_file: Path, version: str) -> dict[str, Any]:
    """Build CycloneDX JSON without timestamps or network access."""
    return {
        "$schema": "https://cyclonedx.org/schema/bom-1.5.schema.json",
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "serialNumber": f"urn:uuid:leedevkit-{version}",
        "metadata": {
            "component": {
                "type": "application",
                "name": "leedevkit",
                "version": version,
                "properties": [
                    {"name": "artifact.integrity", "value": "sha256-manifest"},
                    {"name": "artifact.authenticity", "value": "publisher-unverified"},
                    {"name": "provenance.signature", "value": "unsigned"},
                ],
            }
        },
        "components": _components(lock_file),
    }


def write_sbom(lock_file: Path, version: str, output: Path) -> Path:
    """Write deterministic SBOM and return output path."""
    output.write_text(
        json.dumps(generate_sbom(lock_file, version), indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return output


if __name__ == "__main__":  # pragma: no cover
    import argparse

    parser = argparse.ArgumentParser(description="Generate LeeDevKit CycloneDX SBOM")
    parser.add_argument("lock_file", type=Path)
    parser.add_argument("version")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    write_sbom(args.lock_file, args.version, args.output)
