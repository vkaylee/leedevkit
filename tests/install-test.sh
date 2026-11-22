#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="$ROOT_DIR/install.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
case "$(uname -s):$(uname -m)" in
    Linux:x86_64) platform=linux-amd64; activation_flag=-Tf ;;
    Linux:aarch64|Linux:arm64) platform=linux-arm64; activation_flag=-Tf ;;
    Darwin:x86_64) platform=darwin-amd64; activation_flag=-fh ;;
    Darwin:arm64) platform=darwin-arm64; activation_flag=-fh ;;
    *) fail 'unsupported test platform' ;;
esac
checksum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"; else shasum -a 256 "$1"; fi; }

make_release() {
    local version="$1" layout="${2:-valid}"
    local root="$WORK/build/leedevkit-$version"
    rm -rf "$WORK/build"
    mkdir -p "$root/bin/go" "$WORK/releases/download/v$version" "$WORK/home"
    printf '%s\n' "$version" > "$root/VERSION"
    if [[ "$layout" == invalid-launcher ]]; then
        printf '#!/bin/sh\nexit 1\n' > "$root/bin/leedevkit"
    else
        printf '#!/bin/sh\nexit 0\n' > "$root/bin/leedevkit"
    fi
    if [[ "$layout" == invalid-launcher ]]; then
        printf '#!/bin/sh\nexit 1\n' > "$root/bin/go/leedevkit-go-$platform"
    else
        printf '#!/bin/sh\nexit 0\n' > "$root/bin/go/leedevkit-go-$platform"
    fi
    tar -czf "$WORK/releases/download/v$version/leedevkit-$version.tar.gz" -C "$WORK/build" "leedevkit-$version"
    if [[ "$layout" == traversal ]]; then
        python3 -I - "$WORK/releases/download/v$version/leedevkit-$version.tar.gz" "$version" <<'PY'
import io, sys, tarfile
with tarfile.open(sys.argv[1], 'w:gz') as archive:
    entry = tarfile.TarInfo('leedevkit-' + sys.argv[2] + '/../escape')
    entry.size = 1
    archive.addfile(entry, io.BytesIO(b'x'))
PY
    elif [[ "$layout" == link ]]; then
        ln -s /etc/passwd "$root/outside"
        tar -czf "$WORK/releases/download/v$version/leedevkit-$version.tar.gz" -C "$WORK/build" "leedevkit-$version"
    elif [[ "$layout" == fifo ]]; then
        mkfifo "$root/fifo"
        tar -czf "$WORK/releases/download/v$version/leedevkit-$version.tar.gz" -C "$WORK/build" "leedevkit-$version"
    fi
    (cd "$WORK/releases/download/v$version" && checksum "$WORK/releases/download/v$version/leedevkit-$version.tar.gz" > "leedevkit-$version.tar.gz.sha256")
}

run_install() {
    DEVKIT_HOME="$WORK/home/.leedevkit" \
    LEEDEVKIT_RELEASE_BASE_URL="file://$WORK/releases" \
        bash "$INSTALLER" "$1"
}

make_release 1.2.3
run_install 1.2.3 || fail 'valid local mirror install failed'
[[ -L "$WORK/home/.leedevkit/current" ]] || fail 'current symlink missing'
[[ -x "$WORK/home/.leedevkit/current/bin/leedevkit" ]] || fail 'installed launcher not executable'

make_release 1.2.4
printf 'preserve-me\n' > "$WORK/home/.leedevkit/current/keep"
# Existing version collision must stop before touching active install.
run_install 1.2.3 && fail 'existing version overwrite accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on collision'

mkdir "$WORK/home/.leedevkit/.install.lock"
printf occupied > "$WORK/home/.leedevkit/.install.lock/owner"
run_install 1.2.4 && fail 'concurrent lock accepted'
grep -qx occupied "$WORK/home/.leedevkit/.install.lock/owner" || fail 'contended lock was altered'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed while locked'
rm -rf "$WORK/home/.leedevkit/.install.lock"

make_release 1.2.4
printf bad > "$WORK/releases/download/v1.2.4/leedevkit-1.2.4.tar.gz.sha256"
run_install 1.2.4 && fail 'bad checksum accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on checksum failure'

make_release 1.2.4
printf '%2048s\n' '' >> "$WORK/releases/download/v1.2.4/leedevkit-1.2.4.tar.gz.sha256"
run_install 1.2.4 && fail 'oversized checksum accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on oversized checksum'

make_release 1.2.4
archive="$WORK/releases/download/v1.2.4/leedevkit-1.2.4.tar.gz"
python3 -I - "$archive" <<'PY'
import pathlib, sys
path = pathlib.Path(sys.argv[1])
data = path.read_bytes()
path.write_bytes(data[:-8])
PY
checksum "$archive" > "$archive.sha256"
run_install 1.2.4 && fail 'truncated gzip accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on truncated gzip'

make_release 1.2.4
archive="$WORK/releases/download/v1.2.4/leedevkit-1.2.4.tar.gz"
gzip -dc "$archive" > "$WORK/large.tar"
python3 -I - "$WORK/large.tar" <<'PY'
import sys
with open(sys.argv[1], 'r+b') as file:
    file.truncate(513 * 1024 * 1024)
PY
gzip -c "$WORK/large.tar" > "$archive"
checksum "$archive" > "$archive.sha256"
run_install 1.2.4 && fail 'oversized expanded archive accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on oversized archive'

make_release 1.2.4
archive="$WORK/releases/download/v1.2.4/leedevkit-1.2.4.tar.gz"
python3 -I - "$archive" <<'PY'
import io, sys, tarfile
with tarfile.open(sys.argv[1], 'w:gz') as archive:
    for i in range(100001):
        entry = tarfile.TarInfo('leedevkit-1.2.4/entry-%d' % i)
        archive.addfile(entry, io.BytesIO())
PY
checksum "$archive" > "$archive.sha256"
run_install 1.2.4 > "$WORK/entry-output" 2>&1 && fail 'archive with too many entries accepted'
grep -q 'archive exceeds entry limit' "$WORK/entry-output" || fail 'excessive entries were not rejected by entry limit'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on excessive entries'

make_release 1.2.4
python3 -I - "$INSTALLER" "$WORK" <<'PY'
import http.server, os, pathlib, subprocess, sys, threading
installer, work = sys.argv[1], pathlib.Path(sys.argv[2])
archive = (work / 'releases/download/v1.2.4/leedevkit-1.2.4.tar.gz').read_bytes()
checksum = (work / 'releases/download/v1.2.4/leedevkit-1.2.4.tar.gz.sha256').read_bytes()
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = checksum + b' ' * 2048 if self.path.endswith('.sha256') else archive
        self.send_response(200)
        self.send_header('Transfer-Encoding', 'chunked')
        self.end_headers()
        try:
            for start in range(0, len(body), 128):
                chunk = body[start:start + 128]
                self.wfile.write(('%x\r\n' % len(chunk)).encode() + chunk + b'\r\n')
            self.wfile.write(b'0\r\n\r\n')
        except (BrokenPipeError, ConnectionResetError):
            pass
    def log_message(self, *_):
        pass
server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
try:
    env = dict(os.environ, DEVKIT_HOME=str(work / 'home/.leedevkit'),
               LEEDEVKIT_RELEASE_BASE_URL='http://127.0.0.1:%d' % server.server_port)
    result = subprocess.run(['bash', installer, '1.2.4'], env=env, capture_output=True, timeout=30)
    assert result.returncode != 0, 'oversized chunked checksum accepted'
    assert b'checksum' in result.stderr, result.stderr
    assert (work / 'home/.leedevkit/current/keep').read_text() == 'preserve-me\n'
finally:
    server.shutdown()
    server.server_close()
    thread.join()
PY

run_install '../escape' && fail 'invalid version accepted'

make_release 1.2.4 traversal
run_install 1.2.4 && fail 'archive traversal accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on traversal failure'

make_release 1.2.4 link
run_install 1.2.4 && fail 'archive symlink accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on link failure'

make_release 1.2.4 fifo
run_install 1.2.4 && fail 'archive FIFO accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on FIFO failure'

make_release 1.2.4 invalid-launcher
run_install 1.2.4 && fail 'invalid staged launcher accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'existing install changed on launcher failure'

make_release 1.2.4
mkdir -p "$WORK/fake-bin"
real_mv="$(command -v mv)"
printf '#!/bin/bash\nif [[ "$1" == %q ]]; then exit 1; fi\nexec %q "$@"\n' "$activation_flag" "$real_mv" > "$WORK/fake-bin/mv"
chmod +x "$WORK/fake-bin/mv"
PATH="$WORK/fake-bin:$PATH" run_install 1.2.4 && fail 'activation failure accepted'
grep -qx preserve-me "$WORK/home/.leedevkit/current/keep" || fail 'activation failure damaged active release'
[[ ! -e "$WORK/home/.leedevkit/v1.2.4" ]] || fail 'failed activation left new release'
[[ ! -e "$WORK/home/.leedevkit/.install.lock" ]] || fail 'failed activation left lock'

make_release 1.2.4
printf '#!/bin/bash\nif [[ "$1" == %q ]]; then %q "$@" || exit; kill -TERM "$PPID"; exit 0; fi\nexec %q "$@"\n' "$activation_flag" "$real_mv" "$real_mv" > "$WORK/fake-bin/mv"
PATH="$WORK/fake-bin:$PATH" run_install 1.2.4 && fail 'interrupted activation succeeded'
[[ -x "$WORK/home/.leedevkit/current/bin/leedevkit" ]] || fail 'interrupted activation broke current'
[[ "$(readlink "$WORK/home/.leedevkit/current")" == "$WORK/home/.leedevkit/v1.2.4" ]] || fail 'completed activation did not preserve new release'
grep -qx preserve-me "$WORK/home/.leedevkit/v1.2.3/keep" || fail 'interrupted activation damaged old release'
[[ ! -e "$WORK/home/.leedevkit/.install.lock" ]] || fail 'interrupted activation left lock'

make_release 1.2.5
printf '#!/bin/bash\n%q "$@" || exit\nif [[ "$1" != %q ]]; then kill -TERM "$PPID"; fi\n' "$real_mv" "$activation_flag" > "$WORK/fake-bin/mv"
PATH="$WORK/fake-bin:$PATH" run_install 1.2.5 && fail 'interrupted version move succeeded'
[[ -x "$WORK/home/.leedevkit/current/bin/leedevkit" ]] || fail 'version move interruption broke current'
[[ ! -e "$WORK/home/.leedevkit/v1.2.5" ]] || fail 'version move interruption left release behind'
[[ ! -e "$WORK/home/.leedevkit/.install.lock" ]] || fail 'version move interruption left lock'

make_release 1.2.6
real_mkdir="$(command -v mkdir)"
printf '#!/bin/bash\n%q "$@" || exit\nif [[ "$1" == */.install.lock ]]; then kill -TERM "$PPID"; fi\n' "$real_mkdir" > "$WORK/fake-bin/mkdir"
chmod +x "$WORK/fake-bin/mkdir"
PATH="$WORK/fake-bin:$PATH" run_install 1.2.6 && fail 'interrupted lock acquisition succeeded'
[[ ! -e "$WORK/home/.leedevkit/.install.lock" ]] || fail 'interrupted lock acquisition left stale lock'
[[ -x "$WORK/home/.leedevkit/current/bin/leedevkit" ]] || fail 'interrupted lock acquisition broke current'

printf 'install tests passed\n'
