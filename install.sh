#!/bin/bash
# leedevkit bootstrap installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
#
# Installs the `leedevkit` command globally as a bootstrap. Each project then
# self-installs its own devkit via `leedevkit init`.

set -euo pipefail

REPO="vkaylee/leedevkit"
VERSION="${1:-latest}"
INSTALL_DIR="${DEVKIT_HOME:-$HOME/.leedevkit}"
RELEASE_BASE_URL="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/$REPO/releases}"

echo "🚀 Bootstrapping leedevkit $VERSION..."

mkdir -p "$INSTALL_DIR"
TMP_DIR="$(mktemp -d "${INSTALL_DIR%/}/.install.XXXXXX")"
STAGE_DIR="$TMP_DIR/stage"
BACKUP_DIR="$TMP_DIR/previous"
VERSION_DIR=""
TRANSACTION_ACTIVE=0
VERSION_BACKED_UP=0
CURRENT_BACKED_UP=0
VERSION_ACTIVATED=0
CURRENT_ACTIVATED=0
SHELL_RC=""
PATH_RC_BACKUP="$TMP_DIR/shellrc"
PATH_RC_CHANGED=0

remove_path() {
    if [ -L "$1" ] || [ -f "$1" ]; then
        rm -f "$1"
    elif [ -e "$1" ]; then
        rm -rf "$1"
    fi
}

rollback() {
    set +e
    if [ "$CURRENT_ACTIVATED" -eq 1 ]; then
        remove_path "$INSTALL_DIR/current"
    fi
    if [ "$CURRENT_BACKED_UP" -eq 1 ] && { [ -e "$BACKUP_DIR/current" ] || [ -L "$BACKUP_DIR/current" ]; }; then
        mv "$BACKUP_DIR/current" "$INSTALL_DIR/current"
    fi
    if [ "$VERSION_ACTIVATED" -eq 1 ]; then
        remove_path "$VERSION_DIR"
    fi
    if [ "$VERSION_BACKED_UP" -eq 1 ] && { [ -e "$BACKUP_DIR/version" ] || [ -L "$BACKUP_DIR/version" ]; }; then
        mv "$BACKUP_DIR/version" "$VERSION_DIR"
    fi
    if [ "$PATH_RC_CHANGED" -eq 1 ] && [ -f "$PATH_RC_BACKUP" ] && [ -n "$SHELL_RC" ]; then
        cp "$PATH_RC_BACKUP" "$SHELL_RC"
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

# Determine version tag before touching any existing version directory.
if [ "$VERSION" = "latest" ]; then
    VERSION_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')
else
    VERSION_TAG="$VERSION"
fi

VERSION_DIR="$INSTALL_DIR/$VERSION_TAG"
echo "   Version: $VERSION_TAG"
echo "   Target:  $VERSION_DIR"

# Download and validate the complete release in staging. No existing install
# is touched until every archive and integrity check succeeds.
mkdir -p "$STAGE_DIR"
VER="${VERSION_TAG#v}"
TARBALL_URL="$RELEASE_BASE_URL/download/$VERSION_TAG/leedevkit-${VER}.tar.gz"
echo "   Downloading: $TARBALL_URL"
if command -v curl &>/dev/null; then
    curl -fsSL "$TARBALL_URL" -o "$TMP_DIR/leedevkit.tar.gz"
elif command -v wget &>/dev/null; then
    wget -q "$TARBALL_URL" -O "$TMP_DIR/leedevkit.tar.gz"
else
    echo "❌ Need curl or wget to download release tarball"
    exit 1
fi

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

# Prepare the PATH change before activation so a shell startup-file failure
# can still restore the previous install and symlink.
if [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
elif [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
fi
if [ -n "$SHELL_RC" ] && ! grep -q "\.leedevkit/current/bin" "$SHELL_RC" 2>/dev/null; then
    cp "$SHELL_RC" "$PATH_RC_BACKUP"
fi

mkdir -p "$BACKUP_DIR"
TRANSACTION_ACTIVE=1
if [ -e "$VERSION_DIR" ] || [ -L "$VERSION_DIR" ]; then
    mv "$VERSION_DIR" "$BACKUP_DIR/version"
    VERSION_BACKED_UP=1
fi
if [ -e "$INSTALL_DIR/current" ] || [ -L "$INSTALL_DIR/current" ]; then
    mv "$INSTALL_DIR/current" "$BACKUP_DIR/current"
    CURRENT_BACKED_UP=1
fi
mv "$EXTRACTED" "$VERSION_DIR"
VERSION_ACTIVATED=1
ln -s "$VERSION_DIR" "$INSTALL_DIR/current"
CURRENT_ACTIVATED=1

# Ensure scripts are executable before committing the transaction.
chmod +x "$VERSION_DIR"/scripts/*.py 2>/dev/null || true
chmod +x "$VERSION_DIR"/scripts/*.sh 2>/dev/null || true
chmod +x "$VERSION_DIR"/bin/* 2>/dev/null || true

if [ -n "$SHELL_RC" ] && [ -f "$PATH_RC_BACKUP" ]; then
    echo 'export PATH="$HOME/.leedevkit/current/bin:$PATH"' >> "$SHELL_RC"
    PATH_RC_CHANGED=1
fi

rm -rf "$BACKUP_DIR"
TRANSACTION_ACTIVE=0

echo ""
echo "✅ leedevkit $VERSION_TAG bootstrapped"
echo "   Location: $VERSION_DIR"
echo ""
echo "   Initialize a project (per-project install):"
echo "     cd my-project && git init"
echo "     ~/.leedevkit/current/bin/leedevkit init"
echo ""
echo "   Then run in your project:"
echo "     ./leedevkit doctor"
echo "     ./leedevkit test infra --lint-only"
echo ""
if [ -n "$SHELL_RC" ] && [ "$PATH_RC_CHANGED" -eq 1 ]; then
    echo "   Added ~/.leedevkit/current/bin to PATH in $SHELL_RC"
    echo "   Run: source $SHELL_RC"
fi
