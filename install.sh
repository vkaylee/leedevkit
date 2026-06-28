#!/bin/bash
# leeattend-devkit installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leeattend-devkit/main/install.sh | bash

set -euo pipefail

REPO="vkaylee/leeattend-devkit"
VERSION="${1:-latest}"
INSTALL_DIR="${DEVKIT_HOME:-$HOME/.leeattend-devkit}"

echo "🚀 Installing leeattend-devkit $VERSION..."

# Create install directory
mkdir -p "$INSTALL_DIR"

# Determine version tag
if [ "$VERSION" = "latest" ]; then
    VERSION_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')
else
    VERSION_TAG="v$VERSION"
fi

VERSION_DIR="$INSTALL_DIR/$VERSION_TAG"
echo "   Version: $VERSION_TAG"
echo "   Target:  $VERSION_DIR"

# Download and extract
TMP_DIR=$(mktemp -d)
trap 'rm -rf $TMP_DIR' EXIT

# Use git clone for now (future: release tarball)
git clone --depth 1 --branch main "https://github.com/$REPO.git" "$TMP_DIR" 2>/dev/null
rm -rf "$VERSION_DIR"
mkdir -p "$VERSION_DIR"
cp -r "$TMP_DIR/scripts" "$VERSION_DIR/"
cp -r "$TMP_DIR/.agent" "$VERSION_DIR/"
cp -r "$TMP_DIR/templates" "$VERSION_DIR/"
cp "$TMP_DIR/VERSION" "$VERSION_DIR/"
chmod +x "$VERSION_DIR"/scripts/*.py

# Generate checksum manifest
cd "$VERSION_DIR"
if python3 -c "from _devkit_integrity import write_manifest; write_manifest()" 2>/dev/null; then
    :
else
    # Fallback: run script directly
    (cd "$VERSION_DIR" && python3 scripts/_devkit_integrity.py checksum) 2>/dev/null || true
fi

# Update current symlink
ln -sfn "$VERSION_DIR" "$INSTALL_DIR/current"

echo ""
echo "✅ leeattend-devkit $VERSION_TAG installed"
echo "   Location: $VERSION_DIR"
echo ""
echo "   To use in your project:"
echo "     cd my-project"
echo "     ln -sf $INSTALL_DIR/current/bin/test.sh ./test.sh"
echo "     ln -sf $INSTALL_DIR/current/bin/manage.sh ./manage.sh"
