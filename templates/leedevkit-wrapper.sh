#!/bin/bash
# LeeDevKit — committed project launcher (self-bootstraps per-project runtime)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
RUNTIME="$ROOT/.leedevkit"
CONFIG="$ROOT/leedevkit.toml"
LOCK="$ROOT/.leedevkit.bootstrap.lock"

read_version() {
    python3 - "$CONFIG" <<'PY'
import sys
try:
    import tomllib
except ImportError:
    raise SystemExit("Python 3.11+ required to read leedevkit.toml")
with open(sys.argv[1], "rb") as stream:
    print(tomllib.load(stream).get("devkit", {}).get("version", "latest"))
PY
}

version_matches() {
    [ -f "$1/VERSION" ] && {
        expected="$(read_version)"
        [ "$expected" = "latest" ] || [ "$(cat "$1/VERSION")" = "$expected" ]
    }
}

link_runtime() {
    rm -rf "$RUNTIME"
    ln -s "$1" "$RUNTIME"
}

bootstrap_runtime() {
    local version tag base tmp extracted
    version="$(read_version)"
    [ "$version" != "latest" ] || {
        echo "leedevkit.toml must pin [devkit].version" >&2
        return 1
    }
    tag="v${version#v}"
    base="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/vkaylee/leedevkit/releases}"
    tmp="$(mktemp -d "$ROOT/.leedevkit-bootstrap-XXXXXX")"
    extracted="$(python3 - "$base/download/$tag/leedevkit-${version#v}.tar.gz" "$tmp" "$version" <<'PY'
import os
import pathlib
import sys
import tarfile
import urllib.request

url, temp_dir, expected = sys.argv[1:]
request = urllib.request.Request(url, headers={"User-Agent": "leedevkit"})
with urllib.request.urlopen(request, timeout=float(os.environ.get("LEEDEVKIT_DOWNLOAD_TIMEOUT", "120"))) as response:
    data = response.read(512 * 1024 * 1024 + 1)
if len(data) > 512 * 1024 * 1024:
    raise SystemExit("release exceeds maximum size")
archive = pathlib.Path(temp_dir) / "release.tar.gz"
archive.write_bytes(data)
stage = pathlib.Path(temp_dir) / "extracted"
stage.mkdir()
with tarfile.open(archive, "r:gz") as source:
    members = source.getmembers()
    for member in members:
        path = pathlib.PurePosixPath(member.name)
        if not path.parts or path.is_absolute() or ".." in path.parts or member.issym() or member.islnk():
            raise SystemExit(f"unsafe release member: {member.name}")
    source.extractall(stage, members=members)
children = list(stage.iterdir())
root = children[0] if len(children) == 1 and children[0].is_dir() else stage
actual = (root / "VERSION").read_text().strip()
if actual != expected:
    raise SystemExit(f"release version {actual!r} does not match {expected!r}")
print(root)
PY
)"
    rm -rf "$RUNTIME"
    mv "$extracted" "$RUNTIME"
    rm -rf "$tmp"
}

ensure_runtime() {
    version_matches "$RUNTIME" && return
    local common_git main_repo start
    common_git="$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
    if [ -n "$common_git" ]; then
        main_repo="$(dirname "$common_git")"
        version_matches "$main_repo/.leedevkit" && { link_runtime "$main_repo/.leedevkit"; return; }
    fi
    if [ -n "${DEVKIT_HOME:-}" ] && version_matches "$DEVKIT_HOME"; then
        link_runtime "$DEVKIT_HOME"
        return
    fi
    start=$SECONDS
    while ! mkdir "$LOCK" 2>/dev/null; do
        version_matches "$RUNTIME" && return
        (( SECONDS - start > 120 )) && {
            echo "Timed out waiting for $LOCK" >&2
            return 1
        }
        sleep 0.1
    done
    trap 'rmdir "$LOCK" 2>/dev/null || true' EXIT
    version_matches "$RUNTIME" || bootstrap_runtime
    rmdir "$LOCK"
    trap - EXIT
}

ensure_runtime
exec "$RUNTIME/bin/leedevkit" "$@"
