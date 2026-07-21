#!/bin/bash
# Ensure Python venv exists with required dependencies.
# Outputs the python binary path on the last line for test.sh/manage.sh.

set -euo pipefail

DEVKIT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Keep the venv inside the DevKit installation so the CLI and orchestrator use it.
VENV_DIR="$DEVKIT_ROOT/.venv"
PYTHON_BIN="$VENV_DIR/bin/python3"

# Required pip packages and their import names (pip-name:import-name).
# Parallel arrays keep this script compatible with macOS's Bash 3.2.
REQUIRED_PACKAGES=(
    "pytest"
    "pytest-cov"
    "psutil"
    "tomli"
    "tomli-w"
    "pyyaml"
    "ruff"
    "mypy"
)
REQUIRED_IMPORTS=(
    "pytest"
    "pytest_cov"
    "psutil"
    "tomli"
    "tomli_w"
    "yaml"
    "ruff"
    "mypy"
)

PIP_TIMEOUT="${PIP_TIMEOUT:-120}"

if [ ! -f "$PYTHON_BIN" ]; then
    echo "🔧 Creating virtual environment..." >&2
    python3 -m venv "$VENV_DIR"
    echo "⬆️  Upgrading pip..." >&2
    "$PYTHON_BIN" -m pip install --upgrade --quiet --timeout "$PIP_TIMEOUT" pip
    echo "📦 Installing packages..." >&2
    "$PYTHON_BIN" -m pip install --quiet --timeout "$PIP_TIMEOUT" "${REQUIRED_PACKAGES[@]}"
    echo "✅ Virtual environment ready" >&2
else
    # Check for missing packages and install them.
    MISSING=()
    for i in "${!REQUIRED_PACKAGES[@]}"; do
        pkg_name="${REQUIRED_PACKAGES[$i]}"
        import_name="${REQUIRED_IMPORTS[$i]}"
        if ! "$PYTHON_BIN" -c "import $import_name" 2>/dev/null; then
            MISSING+=("$pkg_name")
        fi
    done
    if [ ${#MISSING[@]} -gt 0 ]; then
        echo "📦 Installing missing packages: ${MISSING[*]}" >&2
        "$PYTHON_BIN" -m pip install --upgrade --quiet --timeout "$PIP_TIMEOUT" pip
        "$PYTHON_BIN" -m pip install --quiet --timeout "$PIP_TIMEOUT" "${MISSING[@]}"
    fi
fi

# playwright is lazy-loaded — install without browser download (no hang)
if ! "$PYTHON_BIN" -c "import playwright" 2>/dev/null; then
    echo "📦 Installing playwright (browsers skipped — no hang)" >&2
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 "$PYTHON_BIN" -m pip install --quiet --timeout "$PIP_TIMEOUT" playwright
fi

# Always output the python binary path as the last line
echo "$PYTHON_BIN"
