#!/bin/bash
# Build release tarball with fresh manifest.
# Usage: scripts/build-release.sh <version-tag> [output-dir]
#
# Steps:
#   1. Clone the tag into a temp dir
#   2. Build tarball (no .git/.venv/__pycache__)
#   3. Extract tarball to staging directory
#   4. Generate fresh manifest inside staging (no .git files)
#   5. Verify integrity
#   6. Repack tarball with fresh manifest
#   7. Output to specified directory (default: /tmp)

set -euo pipefail

VERSION_TAG="${1:-}"
if [ -z "$VERSION_TAG" ]; then
    echo "Usage: $0 <version-tag> [output-dir]"
    exit 1
fi

OUTPUT_DIR="${2:-/tmp}"
REPO="${LEEDEVKIT_REPO:-https://github.com/vkaylee/leedevkit.git}"
VER="${VERSION_TAG#v}"
CANONICAL="leedevkit-${VER}"
TARBALL_NAME="${CANONICAL}.tar.gz"
WORKDIR="/tmp/leedevkit-build-$$"

mkdir -p "$OUTPUT_DIR"

cleanup() {
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

mkdir -p "$WORKDIR"

echo "📦 Building $TARBALL_NAME ..."

# Step 1: Clone
echo "   1/6 Cloning $VERSION_TAG ..."
git clone --depth 1 --branch "$VERSION_TAG" "$REPO" "$WORKDIR/src" 2>&1 | tail -1

# Step 2: Rename to canonical directory name, then build tarball
echo "   2/6 Building raw tarball ..."
mv "$WORKDIR/src" "$WORKDIR/$CANONICAL"
tar -czf "$WORKDIR/raw.tar.gz" \
    --exclude='.git' --exclude='.venv' --exclude='__pycache__' --exclude='*.pyc' \
    --exclude='*.bak' --exclude='.leedevkit' \
    -C "$WORKDIR" "$CANONICAL"

# Step 3: Extract to staging
echo "   3/6 Extracting to staging ..."
mkdir -p "$WORKDIR/stage"
tar -xzf "$WORKDIR/raw.tar.gz" -C "$WORKDIR/stage/"
STAGED="$WORKDIR/stage/$CANONICAL"
if [ ! -d "$STAGED" ]; then
    echo "❌ Raw tarball missing canonical root: $CANONICAL"
    exit 1
fi

# Step 4: Generate manifest from inside staging so a local .leedevkit/
# cannot override DEVKIT_HOME through the config resolver.
echo "   4/6 Generating manifest ..."
(
    cd "$STAGED"
    DEVKIT_HOME="$STAGED" python3 scripts/_devkit_integrity.py checksum
)

# Step 5: Verify
echo "   5/6 Verifying integrity ..."
if [ ! -f "$STAGED/devkit.manifest.json" ]; then
    echo "❌ Manifest generation failed"
    exit 1
fi
if ! (
    cd "$STAGED"
    DEVKIT_HOME="$STAGED" python3 scripts/_devkit_integrity.py verify
); then
    echo "❌ Integrity check failed"
    exit 1
fi

# Step 6: Repack with fresh manifest
echo "   6/6 Repacking with manifest ..."
rm -f "$OUTPUT_DIR/$TARBALL_NAME"
tar -czf "$OUTPUT_DIR/$TARBALL_NAME" \
    --exclude='.git' --exclude='.venv' --exclude='__pycache__' --exclude='*.pyc' \
    --exclude='*.bak' \
    -C "$WORKDIR/stage" "$CANONICAL"

echo ""
echo "✅ $TARBALL_NAME ready at $OUTPUT_DIR/$TARBALL_NAME"
echo "   Size: $(du -h "$OUTPUT_DIR/$TARBALL_NAME" | cut -f1)"
