#!/bin/bash
# leedevkit bootstrap — per-project install (no global)
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/bootstrap.sh | bash
#
# Downloads the devkit release and installs it into .leedevkit/ in the current
# directory. No global install, no PATH modification, no root permissions.

set -euo pipefail

REPO="vkaylee/leedevkit"
read_config_version() {
    local config="$1" value
    [ -f "$config" ] || { printf '%s\n' latest; return; }
    value="$(awk '
        /^\[/{ section=($0 == "[devkit]") }
        section && $1 == "version" { sub(/^[^=]*=[[:space:]]*/, ""); gsub(/["'"'"'[:space:]]/, ""); print; exit }
    ' "$config")"
    printf '%s\n' "${value:-latest}"
}

if [ "$#" -eq 0 ] && [ -f leedevkit.toml ]; then
    VERSION="$(read_config_version leedevkit.toml)"
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
if [ "$VERSION" = "latest" ] && [ -f leedevkit.toml ]; then
    VERSION="$(read_config_version leedevkit.toml)"
fi
if [ "$VERSION" = "latest" ]; then
    VERSION_TAG="$(curl --fail --silent --show-error --location --max-time "$DOWNLOAD_TIMEOUT" \
        -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$REPO/releases/latest" \
        | awk -F'"' '/"tag_name"/ { print $4; exit }')"
    [ -n "$VERSION_TAG" ] || { echo "latest release tag missing" >&2; exit 1; }
else
    VERSION_TAG="$VERSION"
    case "$VERSION_TAG" in
        v*) ;;
        *) VERSION_TAG="v$VERSION_TAG" ;;
    esac
fi

echo "   Version: $VERSION_TAG"

STAGE_DIR="$TMP_DIR/stage"
BACKUP_DIR="$TMP_DIR/previous"
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
curl --fail --silent --show-error --location --max-time "$DOWNLOAD_TIMEOUT" \
    "$TARBALL_URL" -o "$STAGE_DIR/release.tar.gz"
mkdir -p "$STAGE_DIR/extracted"
tar -xzf "$STAGE_DIR/release.tar.gz" -C "$STAGE_DIR/extracted"
EXTRACTED="$STAGE_DIR/extracted/leedevkit-$VER"
[ -d "$EXTRACTED" ] || { echo "release root missing" >&2; exit 1; }
actual="$(tr -d '[:space:]' < "$EXTRACTED/VERSION" | sed 's/^v//')"
[ "$actual" = "$VER" ] || { echo "release version mismatch" >&2; exit 1; }
case "$(uname -s):$(uname -m)" in
    Linux:x86_64) platform_key="linux-amd64" ;;
    Linux:aarch64|Linux:arm64) platform_key="linux-arm64" ;;
    Darwin:x86_64) platform_key="darwin-amd64" ;;
    Darwin:arm64) platform_key="darwin-arm64" ;;
    *) echo "unsupported platform" >&2; exit 1 ;;
esac
for REQUIRED in bin/leedevkit "bin/go/leedevkit-go-${platform_key}"; do
    if [ ! -f "$EXTRACTED/$REQUIRED" ]; then
        echo "❌ Release missing required file: $REQUIRED"
        exit 1
    fi
done
# Prepare project configuration before activation. Existing configuration is
# copied and updated in staging, so malformed config fails harmlessly.

if [ -f leedevkit.toml ]; then
    cp leedevkit.toml "$CONFIG_STAGE"
    awk -v version="$VER" '
        BEGIN { in_devkit=0; changed=0 }
        /^\[/{ in_devkit=($0 == "[devkit]") }
        in_devkit && $1 == "version" { print "version = \"" version "\""; changed=1; next }
        { print }
        END { if (!changed) exit 1 }
    ' "$CONFIG_STAGE" > "$CONFIG_STAGE.tmp" && mv "$CONFIG_STAGE.tmp" "$CONFIG_STAGE"
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
awk '
    $0 != "leedevkit" && $0 != "leedevkit/" && $0 != "/leedevkit" && $0 != "/leedevkit/" && $0 != "./leedevkit" && $0 != "./leedevkit/" { print }
    END { print "leedevkit"; print ".leedevkit/"; print ".leedevkit.bootstrap.lock"; print ".leedevkit-bootstrap-*/"; print ".leedevkit.new-*"; print ".leedevkit.previous-*" }
' "$GITIGNORE_STAGE" | awk '!seen[$0]++' > "$GITIGNORE_STAGE.tmp" && mv "$GITIGNORE_STAGE.tmp" "$GITIGNORE_STAGE"
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

# Ensure runtime scripts and platform binaries are executable before commit.
chmod +x .leedevkit/scripts/*.py 2>/dev/null || true
chmod +x .leedevkit/scripts/*.sh 2>/dev/null || true
chmod +x .leedevkit/bin/* 2>/dev/null || true
chmod 0755 .leedevkit/bin/go/* 2>/dev/null || true
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
echo "     leedevkit doctor"
echo "     leedevkit test all"
echo ""
echo "   Install global command with install.sh if leedevkit is not on PATH."
