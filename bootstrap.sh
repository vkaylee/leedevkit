#!/bin/bash
# leedevkit bootstrap — per-project install (no global)
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
#
# Downloads the devkit release and installs it into .leedevkit/ in the current
# directory. No global install, no PATH modification, no root permissions.

set -euo pipefail

REPO="vkaylee/leedevkit"
VERSION="${1:-latest}"
RELEASE_BASE_URL="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/$REPO/releases}"

# Must be in a project directory (has .git or leedevkit.toml)
if [ ! -d .git ] && [ ! -f leedevkit.toml ]; then
    echo "⚠️  No .git or leedevkit.toml found in current directory."
    echo "   Run this from your project root, or create leedevkit.toml first."
    echo ""
    echo "   mkdir my-project && cd my-project"
    echo "   git init"
    echo "   curl -fsSL .../bootstrap.sh | bash"
    exit 1
fi

echo "🚀 Installing leedevkit into .leedevkit/ (per-project, no global)..."

if [ "$VERSION" = "latest" ]; then
    VERSION_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')
else
    VERSION_TAG="$VERSION"
fi

echo "   Version: $VERSION_TAG"

TMP_DIR=$(mktemp -d)
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
    rm -rf "$TMP_DIR"
    exit "$status"
}
trap finish EXIT

# Strip "v" prefix for tarball name (v0.1.0 → leedevkit-0.1.0.tar.gz)
VER="${VERSION_TAG#v}"
TARBALL_URL="$RELEASE_BASE_URL/download/$VERSION_TAG/leedevkit-${VER}.tar.gz"
echo "   Downloading: $TARBALL_URL"

if command -v curl &>/dev/null; then
    curl -fsSL "$TARBALL_URL" -o "$TMP_DIR/leedevkit.tar.gz"
elif command -v wget &>/dev/null; then
    wget -q "$TARBALL_URL" -O "$TMP_DIR/leedevkit.tar.gz"
else
    echo "❌ Need curl or wget"
    exit 1
fi

# Extract and validate in a staging directory. Link members are rejected so
# even a valid-looking archive cannot redirect extraction outside staging.
mkdir -p "$STAGE_DIR"
python3 - "$TMP_DIR/leedevkit.tar.gz" "$STAGE_DIR" <<'PY'
import sys
import tarfile
from pathlib import Path, PurePosixPath

archive_path = Path(sys.argv[1])
destination = Path(sys.argv[2]).resolve()
with tarfile.open(archive_path, "r:gz") as archive:
    members = archive.getmembers()
    for member in members:
        path = PurePosixPath(member.name)
        if not member.name or path.is_absolute() or ".." in path.parts:
            raise SystemExit(f"Unsafe archive path: {member.name}")
        if member.issym() or member.islnk():
            raise SystemExit(f"Unsafe archive link: {member.name}")
        if not (member.isfile() or member.isdir()):
            raise SystemExit(f"Unsupported archive member: {member.name}")
    archive.extractall(destination, members=members)
PY
EXTRACTED=$(find "$STAGE_DIR" -mindepth 1 -maxdepth 1 -type d -name 'leedevkit-*' -print -quit)
ENTRY_COUNT=$(find "$STAGE_DIR" -mindepth 1 -maxdepth 1 | wc -l)
if [ "$ENTRY_COUNT" -ne 1 ] || [ -z "$EXTRACTED" ]; then
    echo "❌ Unexpected tarball structure"
    exit 1
fi
if [ ! -f "$EXTRACTED/VERSION" ] || [ "$(tr -d '\r\n' < "$EXTRACTED/VERSION")" != "$VER" ]; then
    echo "❌ Release VERSION does not match $VER"
    exit 1
fi
for REQUIRED in bin/leedevkit scripts/_orchestrator.py scripts/_devkit_integrity.py; do
    if [ ! -f "$EXTRACTED/$REQUIRED" ]; then
        echo "❌ Release missing required file: $REQUIRED"
        exit 1
    fi
done
DEVKIT_HOME="$EXTRACTED" python3 "$EXTRACTED/scripts/_devkit_integrity.py" verify

# Prepare every project-side change before activation. Existing configuration
# is copied and updated in staging, so malformed config fails harmlessly.
cat > "$WRAPPER_STAGE" <<'WRAPPER'
#!/bin/bash
# LeeDevKit — project-local wrapper (auto-generated by bootstrap)
exec "$(cd "$(dirname "$0")" && pwd)/.leedevkit/bin/leedevkit" "$@"
WRAPPER
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

if [ -f .gitignore ]; then
    if ! grep -q '\.leedevkit/' .gitignore 2>/dev/null; then
        cp .gitignore "$GITIGNORE_STAGE"
        {
            echo ""
            echo "# DevKit (per-project install)"
            echo '.leedevkit/'
            echo 'leedevkit'
        } >> "$GITIGNORE_STAGE"
        GITIGNORE_CHANGED=1
    fi
else
    cat > "$GITIGNORE_STAGE" << 'GITIGNORE'
# DevKit (per-project install)
.leedevkit/
leedevkit
GITIGNORE
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
