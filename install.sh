#!/usr/bin/env bash
# leedevkit global installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vkaylee/leedevkit/main/install.sh | bash
# Optional local mirror: LEEDEVKIT_RELEASE_BASE_URL=file:///path/to/releases bash install.sh VERSION
# Checksums detect archive corruption; they do not authenticate the publisher.

set -euo pipefail

REPO="vkaylee/leedevkit"
VERSION="${1:-latest}"
INSTALL_DIR="${DEVKIT_HOME:-$HOME/.leedevkit}"
[[ "$INSTALL_DIR" == /* ]] || INSTALL_DIR="$PWD/$INSTALL_DIR"
RELEASE_BASE_URL="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/$REPO/releases}"
DOWNLOAD_TIMEOUT="${LEEDEVKIT_DOWNLOAD_TIMEOUT:-120}"

fail() { printf 'leedevkit installer: %s\n' "$*" >&2; exit 1; }
valid_version() { [[ "$1" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]]; }
remove_path() {
    if [[ -L "$1" || -f "$1" ]]; then rm -f -- "$1"
    elif [[ -e "$1" ]]; then rm -rf -- "$1"
    fi
}
replace_link() {
    if [[ "$(uname -s)" == Darwin ]]; then mv -fh -- "$1" "$2"
    else mv -Tf -- "$1" "$2"
    fi
}
rollback() {
    set +e
    if (( CURRENT_ACTIVATED )); then
        if (( HAD_CURRENT )); then ln -sfn -- "$OLD_CURRENT_TARGET" "$CURRENT_LINK" && replace_link "$CURRENT_LINK" "$CURRENT"
        else remove_path "$CURRENT"; fi
    fi
    remove_path "$CURRENT_LINK"
    if (( VERSION_ACTIVATED )); then remove_path "$VERSION_DIR"; fi
}
finish() {
    status=$?
    trap - EXIT HUP INT TERM
    if (( status != 0 && TRANSACTION_ACTIVE )); then rollback; fi
    [[ -z "$TMP_DIR" ]] || rm -rf -- "$TMP_DIR"
    [[ -z "$LOCK_DIR" ]] || rm -rf -- "$LOCK_DIR"
    exit "$status"
}

if [[ "$VERSION" == latest ]]; then
    # ponytail: latest metadata is capped at 1 MiB; raise if the API response grows.
    metadata="$(curl --fail --silent --show-error --location --max-time "$DOWNLOAD_TIMEOUT" \
        -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$REPO/releases/latest" \
        | head -c 1048577)" || fail 'latest release download failed or exceeds limit'
    [[ $(LC_ALL=C printf '%s' "$metadata" | wc -c) -le 1048576 ]] || fail 'latest release metadata exceeds limit'
    VERSION_TAG="$(printf '%s\n' "$metadata" | awk -F'"' '/"tag_name"/ { print $4; exit }')"
else
    VERSION_TAG="$VERSION"
fi
valid_version "$VERSION_TAG" || fail "invalid release version: $VERSION_TAG"
VER="${VERSION_TAG#v}"
VERSION_TAG="v$VER"

mkdir -p "$INSTALL_DIR"
[[ -d "$INSTALL_DIR" && ! -L "$INSTALL_DIR" ]] || fail "install path must be a real directory: $INSTALL_DIR"
LOCK_DIR="$INSTALL_DIR/.install.lock"
mkdir "$LOCK_DIR" 2>/dev/null || fail "another install is running or stale lock exists: $LOCK_DIR"
TMP_DIR=""
STAGE_DIR=""
BACKUP_DIR=""
CURRENT_LINK="$INSTALL_DIR/.current.$$.new"
VERSION_DIR="$INSTALL_DIR/$VERSION_TAG"
CURRENT="$INSTALL_DIR/current"
VERSION_ACTIVATED=0
CURRENT_ACTIVATED=0
OLD_CURRENT_TARGET=""
HAD_CURRENT=0
TRANSACTION_ACTIVE=0
trap finish EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
TMP_DIR="$(mktemp -d "$INSTALL_DIR/.install.XXXXXX")"
STAGE_DIR="$TMP_DIR/extracted"
BACKUP_DIR="$TMP_DIR/previous"

[[ ! -e "$VERSION_DIR" && ! -L "$VERSION_DIR" ]] || fail "version already exists; refusing to overwrite: $VERSION_DIR"
mkdir -p "$STAGE_DIR" "$BACKUP_DIR"
TARBALL="leedevkit-$VER.tar.gz"
URL="$RELEASE_BASE_URL/download/$VERSION_TAG/$TARBALL"
printf 'Installing leedevkit %s globally...\n' "$VERSION_TAG"
# ponytail: archives and their decompressed tar streams are capped at 512 MiB.
curl --fail --silent --show-error --location --max-time "$DOWNLOAD_TIMEOUT" "$URL" \
    | head -c 536870913 > "$TMP_DIR/$TARBALL" || fail 'release download failed or exceeds limit'
[[ $(wc -c < "$TMP_DIR/$TARBALL") -le 536870912 ]] || fail 'release archive exceeds limit'
# ponytail: checksum sidecars are capped at 1024 bytes; raise only if the release format changes.
curl --fail --silent --show-error --location --max-time "$DOWNLOAD_TIMEOUT" "$URL.sha256" \
    | head -c 1025 > "$TMP_DIR/$TARBALL.sha256" || fail 'release checksum download failed or exceeds limit'
[[ $(wc -c < "$TMP_DIR/$TARBALL.sha256") -le 1024 ]] || fail 'release checksum exceeds limit'

expected="$(awk 'NR == 1 { print $1 }' "$TMP_DIR/$TARBALL.sha256")"
[[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail 'invalid release checksum format'
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMP_DIR/$TARBALL" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$TMP_DIR/$TARBALL" | awk '{ print $1 }')"
else
    fail 'sha256sum or shasum is required'
fi
[[ "$actual" == "$expected" ]] || fail 'release checksum mismatch'

# Count the entire decompressed stream, including padding and trailing gzip members.
gzip -dc "$TMP_DIR/$TARBALL" | head -c 536870913 | wc -c > "$TMP_DIR/expanded-size" \
    || fail 'invalid gzip or expanded release exceeds limit'
[[ $(<"$TMP_DIR/expanded-size") -le 536870912 ]] || fail 'expanded release exceeds limit'
ROOT="leedevkit-$VER"
# ponytail: reject more than 100000 archive entries; raise if release layout requires it.
tar -tzf "$TMP_DIR/$TARBALL" | awk 'NR > 100000 { exit 1 }' || fail 'archive exceeds entry limit or is invalid'
tar -tvzf "$TMP_DIR/$TARBALL" | awk 'substr($0, 1, 1) != "-" && substr($0, 1, 1) != "d" { exit 1 }' || fail 'archive contains unsupported entry type'
tar -tzf "$TMP_DIR/$TARBALL" > "$TMP_DIR/members" || fail 'invalid release archive'
while IFS= read -r member; do
    [[ -n "$member" && "$member" != /* && "$member" != *\\* ]] || fail "unsafe archive path: $member"
    clean="${member%/}"
    [[ "$clean" == "$ROOT" || "$clean" == "$ROOT/"* ]] || fail "unexpected archive path: $member"
    [[ "/$clean/" != *"/../"* && "/$clean/" != *"/./"* ]] || fail "unsafe archive path: $member"
done < "$TMP_DIR/members"
tar -xzf "$TMP_DIR/$TARBALL" -C "$STAGE_DIR"
EXTRACTED="$STAGE_DIR/$ROOT"
[[ -d "$EXTRACTED" && ! -L "$EXTRACTED" ]] || fail 'release root missing or invalid'
actual_version="$(tr -d '[:space:]' < "$EXTRACTED/VERSION" 2>/dev/null || true)"
[[ "${actual_version#v}" == "$VER" ]] || fail 'release version mismatch'

case "$(uname -s):$(uname -m)" in
    Linux:x86_64) platform_key=linux-amd64 ;;
    Linux:aarch64|Linux:arm64) platform_key=linux-arm64 ;;
    Darwin:x86_64) platform_key=darwin-amd64 ;;
    Darwin:arm64) platform_key=darwin-arm64 ;;
    *) fail 'unsupported platform' ;;
esac
native_binary="bin/go/leedevkit-go-$platform_key"
[[ -f "$EXTRACTED/bin/leedevkit" && -f "$EXTRACTED/$native_binary" ]] || fail 'release missing required executable'
chmod +x "$EXTRACTED/bin/leedevkit" "$EXTRACTED/$native_binary"
cp -- "$EXTRACTED/$native_binary" "$EXTRACTED/bin/leedevkit"
chmod 0755 "$EXTRACTED/bin/leedevkit"
"$EXTRACTED/bin/leedevkit" --help >/dev/null 2>&1 || fail 'staged launcher validation failed'

if [[ -e "$CURRENT" || -L "$CURRENT" ]]; then
    [[ -L "$CURRENT" ]] || fail "current path is not a symlink; refusing to replace: $CURRENT"
    HAD_CURRENT=1
    OLD_CURRENT_TARGET="$(readlink "$CURRENT")"
fi
TRANSACTION_ACTIVE=1
mv -- "$EXTRACTED" "$VERSION_DIR"
VERSION_ACTIVATED=1
ln -s "$VERSION_DIR" "$CURRENT_LINK"
replace_link "$CURRENT_LINK" "$CURRENT"
CURRENT_ACTIVATED=1
TRANSACTION_ACTIVE=0
printf 'Installed leedevkit %s at %s\n' "$VERSION_TAG" "$CURRENT"
printf 'Run: %s/leedevkit --help\n' "$CURRENT/bin"
printf 'Add %s/bin to PATH to use leedevkit globally.\n' "$CURRENT"
