#!/bin/bash
# LeeDevKit — committed project launcher (self-bootstraps per-project runtime)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
RUNTIME="$ROOT/.leedevkit"
CONFIG="$ROOT/leedevkit.toml"
if [ -f "$ROOT/VERSION" ] && [ -x "$ROOT/bin/leedevkit" ] && [ -f "$ROOT/scripts/_orchestrator.py" ]; then
    exec "$ROOT/bin/leedevkit" "$@"
fi


read_version() {
    python3 - "$CONFIG" <<'PY'
import re
import sys

try:
    import tomllib
except ImportError:
    raise SystemExit("Python 3.11+ required to read leedevkit.toml")

with open(sys.argv[1], "rb") as stream:
    value = str(tomllib.load(stream).get("devkit", {}).get("version", "latest"))
value = value.removeprefix("v")
if value != "latest" and not re.fullmatch(r"\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?", value):
    raise SystemExit(f"invalid release version: {value!r}")
print(value)
PY
}

version_matches() {
    local root="$1" expected actual
    [ -f "$root/VERSION" ] || return 1
    [ -x "$root/bin/leedevkit" ] || return 1
    expected="$(read_version)"
    actual="$(tr -d '[:space:]' < "$root/VERSION" | sed 's/^v//')"
    [ "$expected" != "latest" ] && [ "$actual" = "$expected" ]
}

replace_runtime() {
    local staged="$1" previous name
    previous="$(mktemp -d "$ROOT/.leedevkit.previous-XXXXXX")"
    rmdir "$previous"
    if [ -e "$RUNTIME" ] || [ -L "$RUNTIME" ]; then
        mv "$RUNTIME" "$previous"
    fi
    if mv "$staged" "$RUNTIME"; then
        if [ ! -L "$previous" ]; then
            for name in skills.d .venv; do
                if [ -e "$previous/$name" ] || [ -L "$previous/$name" ]; then
                    [ -e "$RUNTIME/$name" ] || [ -L "$RUNTIME/$name" ] || mv "$previous/$name" "$RUNTIME/$name"
                fi
            done
        fi
        rm -rf "$previous"
        return
    fi
    rm -rf "$RUNTIME"
    if [ -e "$previous" ] || [ -L "$previous" ]; then
        mv "$previous" "$RUNTIME"
    fi
    return 1
}

link_runtime() {
    local source="$1"
    local staged
    staged="$(mktemp -d "$ROOT/.leedevkit.new-XXXXXX")"
    rmdir "$staged"
    ln -s "$source" "$staged"
    replace_runtime "$staged"
}

bootstrap_runtime() {
    local version tag base tmp extracted staged
    version="$(read_version)"
    [ "$version" != "latest" ] || {
        echo "leedevkit.toml must pin [devkit].version" >&2
        return 1
    }
    tag="v${version#v}"
    base="${LEEDEVKIT_RELEASE_BASE_URL:-https://github.com/vkaylee/leedevkit/releases}"
    tmp="$(mktemp -d "$ROOT/.leedevkit-bootstrap-XXXXXX")"
    if ! extracted="$(python3 - "$base/download/$tag/leedevkit-${version#v}.tar.gz" "$tmp" "$version" <<'PY'
import os
import pathlib
import re
import sys
import tarfile
import time
import urllib.request

url, temp_dir, expected = sys.argv[1:]
expected = expected.removeprefix("v")
if not re.fullmatch(r"\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?", expected):
    raise SystemExit(f"invalid release version: {expected!r}")
timeout = float(os.environ.get("LEEDEVKIT_DOWNLOAD_TIMEOUT", "120"))
deadline = time.monotonic() + timeout
request = urllib.request.Request(url, headers={"User-Agent": "leedevkit"})
archive = pathlib.Path(temp_dir) / "release.tar.gz"
with urllib.request.urlopen(request, timeout=timeout) as response, archive.open("wb") as output:
    size = 0
    while True:
        if time.monotonic() >= deadline:
            raise SystemExit(f"download timed out after {timeout:g}s")
        chunk = response.read(1024 * 1024)
        if not chunk:
            break
        size += len(chunk)
        if size > 512 * 1024 * 1024:
            raise SystemExit("release exceeds maximum size")
        output.write(chunk)
stage = pathlib.Path(temp_dir) / "extracted"
stage.mkdir()
with tarfile.open(archive, "r:gz") as source:
    members = source.getmembers()
    for member in members:
        path = pathlib.PurePosixPath(member.name)
        if not path.parts or path.is_absolute() or ".." in path.parts:
            raise SystemExit(f"unsafe release member: {member.name}")
        if member.issym() or member.islnk() or not (member.isfile() or member.isdir()):
            raise SystemExit(f"unsupported release member: {member.name}")
    source.extractall(stage, members=members)
children = list(stage.iterdir())
root = children[0] if len(children) == 1 and children[0].is_dir() else stage
actual = (root / "VERSION").read_text().strip().removeprefix("v")
if actual != expected:
    raise SystemExit(f"release version {actual!r} does not match {expected!r}")
if not (root / "bin" / "leedevkit").is_file():
    raise SystemExit("release missing bin/leedevkit")
(root / "bin" / "leedevkit").chmod(0o755)
print(root)
PY
)"; then
        rm -rf "$tmp"
        return 1
    fi
    staged="$(mktemp -d "$ROOT/.leedevkit.new-XXXXXX")"
    rmdir "$staged"
    mv "$extracted" "$staged"
    replace_runtime "$staged"
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
        if [ "$((SECONDS - start))" -gt 120 ]; then
            echo "Timed out waiting for $LOCK" >&2
            return 1
        fi
        sleep 0.1
    done
    trap 'rmdir "$LOCK" 2>/dev/null || true' EXIT
    version_matches "$RUNTIME" || bootstrap_runtime
    rmdir "$LOCK"
    trap - EXIT
}

if [ "${1:-}" = "doctor" ] && [ "${2:-}" != "--fix" ]; then
    common_git="$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
    if version_matches "$RUNTIME" && [ -f "$RUNTIME/scripts/_doctor.py" ]; then
        exec python3 "$RUNTIME/scripts/_doctor.py" "${@:2}"
    fi
    if [ -n "$common_git" ] && version_matches "$(dirname "$common_git")/.leedevkit"; then
        runtime="$(dirname "$common_git")/.leedevkit"
        exec python3 "$runtime/scripts/_doctor.py" "${@:2}"
    fi
    if [ -n "${DEVKIT_HOME:-}" ] && version_matches "$DEVKIT_HOME"; then
        exec python3 "$DEVKIT_HOME/scripts/_doctor.py" "${@:2}"
    fi
    echo "LeeDevKit runtime missing or version-mismatched. Run './leedevkit doctor --fix' to repair." >&2
    exit 0
fi

ensure_runtime
exec "$RUNTIME/bin/leedevkit" "$@"
