"""Black-box acceptance coverage for the built release artifact."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


def test_release_acceptance_command(tmp_path):
    repo_root = Path(__file__).resolve().parents[2]
    script = repo_root / "scripts" / "_release_acceptance.py"

    result = subprocess.run(
        [
            sys.executable,
            str(script),
            "--repo-root",
            str(repo_root),
        ],
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )

    assert result.returncode == 0, f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    assert "Release acceptance passed" in result.stdout
