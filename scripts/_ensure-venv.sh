#!/bin/bash
# Ensure Python venv exists with required dependencies.
# Outputs the python binary path on the last line for test.sh/manage.sh.

set -euo pipefail

DEVKIT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="$DEVKIT_ROOT/.venv"
PYTHON_BIN="$VENV_DIR/bin/python3"
LOCK_FILE="$DEVKIT_ROOT/scripts/requirements.lock"
FINGERPRINT_FILE="$VENV_DIR/.leedevkit-dependency-fingerprint"
PIP_TIMEOUT="${PIP_TIMEOUT:-120}"
REQUIRE_PLAYWRIGHT="${LEEDEVKIT_REQUIRE_PLAYWRIGHT:-0}"

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
EXPECTED_FINGERPRINT="$RUNTIME_ID lock=$LOCK_SHA playwright=$REQUIRE_PLAYWRIGHT"

CORE_REQUIREMENTS="$VENV_DIR/.leedevkit-core-requirements.txt"
OPTIONAL_REQUIREMENTS="$VENV_DIR/.leedevkit-optional-requirements.txt"
awk '/^# Optional browser harness/{exit} {print}' "$LOCK_FILE" > "$CORE_REQUIREMENTS"
awk 'optional {print} /^# Optional browser harness/{optional=1; next}' "$LOCK_FILE" > "$OPTIONAL_REQUIREMENTS"

MISSING="$("$PYTHON_BIN" - "$LOCK_FILE" "$REQUIRE_PLAYWRIGHT" <<'PY'
import importlib.metadata
import sys

lock_path, require_playwright = sys.argv[1:]
optional = False
missing = []
for raw_line in open(lock_path, encoding="utf-8"):
    line = raw_line.strip()
    if line.startswith("# Optional browser harness"):
        optional = True
        continue
    if not line or line.startswith("#") or (optional and require_playwright != "1"):
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
