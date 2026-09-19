#!/bin/bash
# Ensure Python venv exists with required dependencies.
# Outputs the python binary path on the last line for test.sh/manage.sh.

set -euo pipefail

DEVKIT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="$DEVKIT_ROOT/.venv"
PYTHON_BIN="$VENV_DIR/bin/python3"

REQUIRED_PACKAGES=(
    "pytest"
    "pytest-cov"
    "psutil"
    "tomli"
    "tomli-w"
    "pyyaml"
    "types-PyYAML"
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
else
    MISSING="$($PYTHON_BIN - <<'PY'
import importlib.metadata
import importlib.util

checks = {
    "pytest": importlib.util.find_spec("pytest") is None,
    "pytest_cov": importlib.util.find_spec("pytest_cov") is None,
    "psutil": importlib.util.find_spec("psutil") is None,
    "tomli": importlib.util.find_spec("tomli") is None,
    "tomli_w": importlib.util.find_spec("tomli_w") is None,
    "yaml": importlib.util.find_spec("yaml") is None,
    "types_PyYAML": False,
    "ruff": importlib.util.find_spec("ruff") is None,
    "mypy": importlib.util.find_spec("mypy") is None,
}
try:
    importlib.metadata.version("types-PyYAML")
except importlib.metadata.PackageNotFoundError:
    checks["types_PyYAML"] = True
print(" ".join(name for name, missing in checks.items() if missing))
PY
    )"
    MISSING_PACKAGES=()
    for package in $MISSING; do
        case "$package" in
            pytest) MISSING_PACKAGES+=("pytest") ;;
            pytest_cov) MISSING_PACKAGES+=("pytest-cov") ;;
            psutil) MISSING_PACKAGES+=("psutil") ;;
            tomli) MISSING_PACKAGES+=("tomli") ;;
            tomli_w) MISSING_PACKAGES+=("tomli-w") ;;
            yaml) MISSING_PACKAGES+=("pyyaml") ;;
            types_PyYAML) MISSING_PACKAGES+=("types-PyYAML") ;;
            ruff) MISSING_PACKAGES+=("ruff") ;;
            mypy) MISSING_PACKAGES+=("mypy") ;;
        esac
    done
    if [ ${#MISSING_PACKAGES[@]} -gt 0 ]; then
        echo "📦 Installing missing packages: ${MISSING_PACKAGES[*]}" >&2
        "$PYTHON_BIN" -m pip install --upgrade --quiet --timeout "$PIP_TIMEOUT" pip
        "$PYTHON_BIN" -m pip install --quiet --timeout "$PIP_TIMEOUT" "${MISSING_PACKAGES[@]}"
    fi
fi

# Playwright is required by the test harness but browsers are deliberately not downloaded here.
if ! "$PYTHON_BIN" -c "import playwright" 2>/dev/null; then
    echo "📦 Installing playwright (browsers skipped — no hang)" >&2
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 "$PYTHON_BIN" -m pip install --quiet --timeout "$PIP_TIMEOUT" playwright
fi

echo "$PYTHON_BIN"
