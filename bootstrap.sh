#!/bin/bash
# leedevkit bootstrap — per-project install (no global)
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
#
# Downloads the devkit release and installs it into .leedevkit/ in the current
# directory. No global install, no PATH modification, no root permissions.

set -euo pipefail

REPO="vkaylee/leedevkit"
if [ "$#" -eq 0 ] && [ -f leedevkit.toml ]; then
    VERSION="$(python3 - leedevkit.toml <<'PY'
import sys
try:
    import tomllib
except ImportError:
    raise SystemExit("Python 3.11+ required to read leedevkit.toml")
with open(sys.argv[1], "rb") as stream:
    print(tomllib.load(stream).get("devkit", {}).get("version", "latest"))
PY
)"
else
    VERSION="${1:-latest}"
fi
RELEASE_BASE_URL="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/$REPO/releases}"

# Must be in a project directory (Git worktrees expose .git as a file).
if [ ! -e .git ] && [ ! -f leedevkit.toml ]; then
    echo "⚠️  No Git metadata or leedevkit.toml found in current directory."
    echo "   Run this from your project root, or create leedevkit.toml first."
    echo ""
    echo "   mkdir my-project && cd my-project"
    echo "   git init"
    echo "   curl -fsSL .../bootstrap.sh | bash"
    exit 1
fi

echo "🚀 Installing leedevkit into .leedevkit/ (per-project, no global)..."
BOOTSTRAP_LOCK=".leedevkit.bootstrap.lock"
BOOTSTRAP_LOCK_HELD=0
while ! mkdir "$BOOTSTRAP_LOCK" 2>/dev/null; do
    sleep 0.1
done
BOOTSTRAP_LOCK_HELD=1
DOWNLOAD_TIMEOUT="${LEEDEVKIT_DOWNLOAD_TIMEOUT:-120}"
TMP_DIR="$(mktemp -d)"
trap 'rmdir "$BOOTSTRAP_LOCK" 2>/dev/null || true' EXIT
SCRIPT_SOURCE="${BASH_SOURCE[0]:-}"
if [ -n "$SCRIPT_SOURCE" ] && [ -f "$SCRIPT_SOURCE" ]; then
    DOWNLOAD_HELPER="$(cd "$(dirname "$SCRIPT_SOURCE")" && pwd)/scripts/_download.py"
else
    DOWNLOAD_HELPER="$TMP_DIR/_download.py"
    python3 - "$DOWNLOAD_HELPER" "${LEEDEVKIT_DOWNLOADER_URL:-https://raw.githubusercontent.com/$REPO/main/scripts/_download.py}" "$DOWNLOAD_TIMEOUT" <<'PY'
import sys
import time
import urllib.request

destination, url, timeout = sys.argv[1], sys.argv[2], float(sys.argv[3])
deadline = time.monotonic() + timeout
request = urllib.request.Request(url, headers={"User-Agent": "leedevkit"})
with urllib.request.urlopen(request, timeout=timeout) as response, open(destination, "wb") as output:
    size = 0
    while True:
        if time.monotonic() >= deadline:
            raise SystemExit(f"downloader helper timed out after {timeout:g}s")
        chunk = response.read(1024 * 1024)
        if not chunk:
            break
        size += len(chunk)
        if size > 8 * 1024 * 1024:
            raise SystemExit("downloader helper exceeds maximum size")
        output.write(chunk)
PY
fi
if [ "$VERSION" = "latest" ] && [ -f leedevkit.toml ]; then
    VERSION="$(python3 - leedevkit.toml <<'PY'
import sys
try:
    import tomllib
except ImportError:
    raise SystemExit("Python 3.11+ required to read leedevkit.toml")
with open(sys.argv[1], "rb") as stream:
    print(tomllib.load(stream).get("devkit", {}).get("version", "latest"))
PY
)"
fi
if [ "$VERSION" = "latest" ]; then
    VERSION_TAG="$(LEEDEVKIT_BOOTSTRAP=1 python3 "$DOWNLOAD_HELPER" latest \
        "https://api.github.com/repos/$REPO/releases/latest" --timeout "$DOWNLOAD_TIMEOUT")"
else
    VERSION_TAG="$VERSION"
fi

echo "   Version: $VERSION_TAG"

STAGE_DIR="$TMP_DIR/stage"
BACKUP_DIR="$TMP_DIR/previous"
WRAPPER_STAGE="$TMP_DIR/leedevkit-wrapper"
CONFIG_STAGE="$TMP_DIR/leedevkit.toml"
GITIGNORE_STAGE="$TMP_DIR/gitignore"
TRANSACTION_ACTIVE=0
GITIGNORE_CHANGED=0

remove_path() {
    if [ -L "$1" ] || [ -f "$1" ]; then
        rm -f "$1"
    elif [ -e "$1" ]; then
        rm -rf "$1"
    fi
}

rollback() {
    set +e
    if [ -e "$BACKUP_DIR/devkit" ] || [ -L "$BACKUP_DIR/devkit" ]; then
        if [ -e .leedevkit/skills.d ] || [ -L .leedevkit/skills.d ]; then
            remove_path "$BACKUP_DIR/devkit/skills.d"
            mv .leedevkit/skills.d "$BACKUP_DIR/devkit/skills.d"
        fi
    fi
    remove_path .leedevkit
    if [ -e "$BACKUP_DIR/devkit" ] || [ -L "$BACKUP_DIR/devkit" ]; then
        mv "$BACKUP_DIR/devkit" .leedevkit
    fi
    remove_path leedevkit
    if [ -e "$BACKUP_DIR/wrapper" ] || [ -L "$BACKUP_DIR/wrapper" ]; then
        mv "$BACKUP_DIR/wrapper" leedevkit
    fi
    remove_path leedevkit.toml
    if [ -e "$BACKUP_DIR/config" ] || [ -L "$BACKUP_DIR/config" ]; then
        mv "$BACKUP_DIR/config" leedevkit.toml
    fi
    if [ "$GITIGNORE_CHANGED" -eq 1 ]; then
        remove_path .gitignore
        if [ -e "$BACKUP_DIR/gitignore" ] || [ -L "$BACKUP_DIR/gitignore" ]; then
            mv "$BACKUP_DIR/gitignore" .gitignore
        fi
    fi
}

finish() {
    status=$?
    if [ "$status" -ne 0 ] && [ "$TRANSACTION_ACTIVE" -eq 1 ]; then
        rollback
    fi
    if [ "$BOOTSTRAP_LOCK_HELD" -eq 1 ]; then
        rmdir "$BOOTSTRAP_LOCK" 2>/dev/null || true
    fi
    rm -rf "$TMP_DIR"
    exit "$status"
}
trap finish EXIT

# Download and validate complete release in staging. Existing install untouched.
VER="${VERSION_TAG#v}"
TARBALL_URL="$RELEASE_BASE_URL/download/$VERSION_TAG/leedevkit-${VER}.tar.gz"
echo "   Downloading: $TARBALL_URL"
mkdir -p "$STAGE_DIR"
LEEDEVKIT_BOOTSTRAP=1 python3 "$DOWNLOAD_HELPER" download "$TARBALL_URL" \
    "$STAGE_DIR" --timeout "$DOWNLOAD_TIMEOUT" --expected-version "$VER"
EXTRACTED="$STAGE_DIR"
for REQUIRED in bin/leedevkit scripts/_orchestrator.py scripts/_devkit_integrity.py; do
    if [ ! -f "$EXTRACTED/$REQUIRED" ]; then
        echo "❌ Release missing required file: $REQUIRED"
        exit 1
    fi
done
LEEDEVKIT_BOOTSTRAP=1 DEVKIT_HOME="$EXTRACTED" python3 "$EXTRACTED/scripts/_devkit_integrity.py" verify

# Prepare every project-side change before activation. Existing configuration
# is copied and updated in staging, so malformed config fails harmlessly.
cp "$EXTRACTED/templates/leedevkit-wrapper.sh" "$WRAPPER_STAGE"
chmod +x "$WRAPPER_STAGE"

if [ -f leedevkit.toml ]; then
    cp leedevkit.toml "$CONFIG_STAGE"
    python3 - "$VER" "$CONFIG_STAGE" <<'PY'
import re
import sys
from pathlib import Path

path = Path(sys.argv[2])
version = sys.argv[1]
content = path.read_text()
updated, count = re.subn(
    r'(\[devkit\][^\[]*?\bversion\s*=\s*)"[^"]*"',
    lambda match: f'{match.group(1)}"{version}"',
    content,
    count=1,
    flags=re.DOTALL,
)
if count == 0:
    raise SystemExit("❌ Existing leedevkit.toml has no [devkit] version entry")
path.write_text(updated)
PY
else
    PROJECT_NAME="$(basename "$(pwd)")"
    PROJECT_NAMESPACE="$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd 'a-z0-9-')"
    cat > "$CONFIG_STAGE" << TOML
[devkit]
version = "$VER"
install = "per-project"
source = "release"

[project]
name = "$PROJECT_NAME"
namespace = "$PROJECT_NAMESPACE"
languages = []

[targets]
all = []

[ai]
rules_dir = ".agent/rules"
override_manifest = ".agent/overrides.yaml"
TOML
fi

cp .gitignore "$GITIGNORE_STAGE" 2>/dev/null || :
if [ ! -f "$GITIGNORE_STAGE" ]; then
    : > "$GITIGNORE_STAGE"
fi
python3 - "$GITIGNORE_STAGE" <<'PY'
import sys
from pathlib import Path

path = Path(sys.argv[1])
legacy = {"leedevkit", "leedevkit/", "/leedevkit", "/leedevkit/", "./leedevkit", "./leedevkit/"}
lines = [line for line in path.read_text().splitlines() if line.strip() not in legacy]
for entry in (".leedevkit/", ".leedevkit.bootstrap.lock", ".leedevkit-bootstrap-*/", ".leedevkit.new-*", ".leedevkit.previous-*"):
    if entry not in lines:
        lines.append(entry)
path.write_text("\n".join(lines).rstrip() + "\n")
PY
if [ ! -f .gitignore ] || ! cmp -s .gitignore "$GITIGNORE_STAGE"; then
    GITIGNORE_CHANGED=1
fi

# Activation is one transaction. Keep the complete previous install and all
# project files until wrapper/config/gitignore operations have succeeded.
mkdir -p "$BACKUP_DIR"
TRANSACTION_ACTIVE=1
if [ -e .leedevkit ] || [ -L .leedevkit ]; then
    mv .leedevkit "$BACKUP_DIR/devkit"
fi
if [ -e leedevkit ] || [ -L leedevkit ]; then
    mv leedevkit "$BACKUP_DIR/wrapper"
fi
if [ -e leedevkit.toml ] || [ -L leedevkit.toml ]; then
    mv leedevkit.toml "$BACKUP_DIR/config"
fi
if [ "$GITIGNORE_CHANGED" -eq 1 ] && { [ -e .gitignore ] || [ -L .gitignore ]; }; then
    mv .gitignore "$BACKUP_DIR/gitignore"
fi
mv "$EXTRACTED" .leedevkit
if [ -e "$BACKUP_DIR/devkit/skills.d" ] || [ -L "$BACKUP_DIR/devkit/skills.d" ]; then
    remove_path .leedevkit/skills.d
    mv "$BACKUP_DIR/devkit/skills.d" .leedevkit/skills.d
fi

# Ensure scripts are executable before committing the transaction.
chmod +x .leedevkit/scripts/*.py 2>/dev/null || true
chmod +x .leedevkit/scripts/*.sh 2>/dev/null || true
chmod +x .leedevkit/bin/* 2>/dev/null || true
mv "$WRAPPER_STAGE" leedevkit
mv "$CONFIG_STAGE" leedevkit.toml
if [ "$GITIGNORE_CHANGED" -eq 1 ]; then
    mv "$GITIGNORE_STAGE" .gitignore
fi

rm -rf "$BACKUP_DIR"
TRANSACTION_ACTIVE=0

echo ""
echo "✅ leedevkit $VERSION_TAG installed in .leedevkit/"
echo ""
echo "   Run:"
echo "     ./leedevkit doctor"
echo "     ./leedevkit test all"
echo ""
echo "   No global install needed. Everything is in .leedevkit/"
