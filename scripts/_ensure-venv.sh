#!/bin/bash
# Ensure Python venv exists with required dependencies.
# Outputs the python binary path on the last line for CLI wrappers.

set -euo pipefail

DEVKIT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="$DEVKIT_ROOT/.venv"
PYTHON_BIN="$VENV_DIR/bin/python3"
PROFILE="${1:-runtime}"
if [ "$PROFILE" != "runtime" ] && [ "$PROFILE" != "test" ]; then
    echo "❌ Profile must be runtime or test" >&2
    exit 2
fi
if [ "$PROFILE" = "test" ]; then
    LOCK_FILE="$DEVKIT_ROOT/scripts/requirements.lock"
else
    LOCK_FILE="$DEVKIT_ROOT/scripts/requirements-runtime.lock"
fi
if [ "$PROFILE" = "test" ]; then
    FINGERPRINT_FILE="$VENV_DIR/.leedevkit-test-dependency-fingerprint"
else
    FINGERPRINT_FILE="$VENV_DIR/.leedevkit-runtime-dependency-fingerprint"
fi
PIP_TIMEOUT="${PIP_TIMEOUT:-120}"
REQUIRE_PLAYWRIGHT="0"
if [ "$PROFILE" = "test" ]; then
    REQUIRE_PLAYWRIGHT="${LEEDEVKIT_REQUIRE_PLAYWRIGHT:-0}"
fi

if [ ! -f "$LOCK_FILE" ]; then
    echo "❌ Dependency lock missing: $LOCK_FILE" >&2
    exit 1
fi

if [ "$REQUIRE_PLAYWRIGHT" != "0" ] && [ "$REQUIRE_PLAYWRIGHT" != "1" ]; then
    echo "❌ LEEDEVKIT_REQUIRE_PLAYWRIGHT must be 0 or 1" >&2
    exit 2
fi

if [ ! -x "$PYTHON_BIN" ] || ! "$PYTHON_BIN" -c 'import sys' >/dev/null 2>&1; then
    if [ -d "$VENV_DIR" ]; then
        echo "⚠️  Recreating damaged virtual environment..." >&2
        rm -rf "$VENV_DIR"
    fi
    echo "🔧 Creating virtual environment..." >&2
    python3 -m venv "$VENV_DIR"
fi

LOCK_SHA="$(sha256sum "$LOCK_FILE" | cut -d' ' -f1)"
RUNTIME_ID="$("$PYTHON_BIN" -c 'import sys; print(f"{sys.implementation.name}:{sys.version_info.major}.{sys.version_info.minor}")')"
EXPECTED_FINGERPRINT="$PROFILE $RUNTIME_ID lock=$LOCK_SHA playwright=$REQUIRE_PLAYWRIGHT"

CORE_REQUIREMENTS="$VENV_DIR/.leedevkit-${PROFILE}-requirements.txt"
OPTIONAL_REQUIREMENTS="$VENV_DIR/.leedevkit-${PROFILE}-optional-requirements.txt"
if [ "$PROFILE" = "test" ]; then
    awk '/^# Optional browser harness/{exit} {print}' "$LOCK_FILE" > "$CORE_REQUIREMENTS"
    awk 'optional {print} /^# Optional browser harness/{optional=1; next}' "$LOCK_FILE" > "$OPTIONAL_REQUIREMENTS"
else
    cp "$LOCK_FILE" "$CORE_REQUIREMENTS"
    : > "$OPTIONAL_REQUIREMENTS"
fi

MISSING="$("$PYTHON_BIN" - "$CORE_REQUIREMENTS" "$OPTIONAL_REQUIREMENTS" "$REQUIRE_PLAYWRIGHT" <<'PY'
import importlib.metadata
import sys

core_path, optional_path, require_playwright = sys.argv[1:]
missing = []
paths = [core_path]
if require_playwright == "1":
    paths.append(optional_path)
for path in paths:
    for raw_line in open(path, encoding="utf-8"):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        requirement = line.split(";", 1)[0].strip()
        if "==" not in requirement:
            continue
        name, expected = requirement.split("==", 1)
        try:
            installed = importlib.metadata.version(name.strip())
        except importlib.metadata.PackageNotFoundError:
            missing.append(name.strip())
            continue
        if installed != expected.strip():
            missing.append(name.strip())
print(" ".join(missing))
PY
 )"

if [ ! -f "$FINGERPRINT_FILE" ] || [ "$(cat "$FINGERPRINT_FILE")" != "$EXPECTED_FINGERPRINT" ] || [ -n "$MISSING" ]; then
    echo "📦 Installing locked dependencies..." >&2
    "$PYTHON_BIN" -m pip install --upgrade --quiet --timeout "$PIP_TIMEOUT" --requirement "$CORE_REQUIREMENTS"
    if [ "$REQUIRE_PLAYWRIGHT" = "1" ]; then
        echo "📦 Installing optional Playwright dependency..." >&2
        PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 "$PYTHON_BIN" -m pip install --upgrade --quiet --timeout "$PIP_TIMEOUT" --requirement "$OPTIONAL_REQUIREMENTS"
    fi
    printf '%s' "$EXPECTED_FINGERPRINT" > "$FINGERPRINT_FILE"
fi

echo "$PYTHON_BIN"
