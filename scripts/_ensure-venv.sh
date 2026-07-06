#!/bin/bash
# Ensure Python venv exists with required dependencies.
# Outputs the python binary path on the last line for test.sh/manage.sh.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# .venv lives in devkit root (parent of scripts/), not inside scripts/ itself
VENV_DIR="$(cd "$SCRIPT_DIR/.." && pwd)/.venv"
PYTHON_BIN="$VENV_DIR/bin/python3"

if [ ! -f "$PYTHON_BIN" ]; then
    echo "🔧 Creating virtual environment..." >&2
    python3 -m venv "$VENV_DIR"
    "$PYTHON_BIN" -m pip install --quiet pytest pytest-cov psutil tomli tomli-w pyyaml ruff mypy
    echo "✅ Virtual environment ready" >&2
fi

# Always output the python binary path as the last line
echo "$PYTHON_BIN"
