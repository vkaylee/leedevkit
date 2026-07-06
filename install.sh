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

echo "🚀 Bootstrapping leedevkit $VERSION..."

# Create install directory
mkdir -p "$INSTALL_DIR"

# Determine version tag
if [ "$VERSION" = "latest" ]; then
    VERSION_TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')
else
    VERSION_TAG="$VERSION"
fi

VERSION_DIR="$INSTALL_DIR/$VERSION_TAG"
echo "   Version: $VERSION_TAG"
echo "   Target:  $VERSION_DIR"

# Download release tarball
TMP_DIR=$(mktemp -d)
trap 'rm -rf $TMP_DIR' EXIT

TARBALL_URL="https://github.com/$REPO/releases/download/$VERSION_TAG/leedevkit-${VERSION_TAG}.tar.gz"
echo "   Downloading: $TARBALL_URL"

rm -rf "$VERSION_DIR"
mkdir -p "$VERSION_DIR"

if command -v curl &>/dev/null; then
    curl -fsSL "$TARBALL_URL" -o "$TMP_DIR/leedevkit.tar.gz"
elif command -v wget &>/dev/null; then
    wget -q "$TARBALL_URL" -O "$TMP_DIR/leedevkit.tar.gz"
else
    echo "❌ Need curl or wget to download release tarball"
    exit 1
fi

# Extract into version dir (tarball contains leedevkit-$VERSION/ root)
rm -rf "$VERSION_DIR"
mkdir -p "$VERSION_DIR"
tar -xzf "$TMP_DIR/leedevkit.tar.gz" -C "$TMP_DIR"
# Tarball contains single root dir (leedevkit-$VERSION/), move contents up
EXTRACTED=$(find "$TMP_DIR" -maxdepth 1 -type d -name 'leedevkit-*' | head -1)
if [ -n "$EXTRACTED" ]; then
    cp -r "$EXTRACTED"/. "$VERSION_DIR/"
else
    cp -r "$TMP_DIR"/. "$VERSION_DIR/"
fi

# Ensure scripts are executable
chmod +x "$VERSION_DIR"/scripts/*.py 2>/dev/null || true
chmod +x "$VERSION_DIR"/scripts/*.sh 2>/dev/null || true
chmod +x "$VERSION_DIR"/bin/* 2>/dev/null || true

# Update current symlink
ln -sfn "$VERSION_DIR" "$INSTALL_DIR/current"

# Add to PATH if not already
PATH_ADDITION='export PATH="$HOME/.leevdevkit/current/bin:$PATH"'
SHELL_RC=""
if [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
elif [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
fi

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
if [ -n "$SHELL_RC" ]; then
    if ! grep -q "\.leedevkit/current/bin" "$SHELL_RC" 2>/dev/null; then
        echo 'export PATH="$HOME/.leedevkit/current/bin:$PATH"' >> "$SHELL_RC"
        echo "   Added ~/.leedevkit/current/bin to PATH in $SHELL_RC"
        echo "   Run: source $SHELL_RC"
    fi
fi
