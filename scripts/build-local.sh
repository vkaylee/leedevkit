#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_FLAG=()
OUT=""
for arg in "$@"; do
    case "$arg" in
        --build)
            BUILD_FLAG=(--build)
            ;;
        -h|--help)
            printf '%s\n' "Usage: scripts/build-local.sh [--build] [output-path]"
            printf '%s\n' "Build local leedevkit-go binary from current source (default: build/leedevkit-go)."
            exit 0
            ;;
        -*)
            printf 'unsupported flag: %s\n' "$arg" >&2
            exit 2
            ;;
        *)
            if [ -n "$OUT" ]; then
                printf 'unexpected argument: %s\n' "$arg" >&2
                exit 2
            fi
            OUT="$arg"
            ;;
    esac
done
OUT="${OUT:-$ROOT/build/leedevkit-go}"
mkdir -p "$ROOT/build" "$(dirname "$OUT")"

TMP_BIN="$(mktemp "$ROOT/build/.leedevkit-go.XXXXXX")"
cleanup() { rm -f "$TMP_BIN"; }
trap cleanup EXIT

if command -v go >/dev/null 2>&1; then
    (cd "$ROOT" && CGO_ENABLED=0 go build -buildvcs=false -o "$TMP_BIN" ./cmd/leedevkit)
else
    ENGINE="docker"
    if ! command -v docker >/dev/null 2>&1 && command -v podman >/dev/null 2>&1; then
        ENGINE="podman"
    fi
    "$ENGINE" volume create leedevkit_go-module-cache >/dev/null 2>&1 || true
    "$ENGINE" volume create leedevkit_go-build-cache >/dev/null 2>&1 || true
    TMP_NAME="$(basename "$TMP_BIN")"
    LEEDEVKIT_PROJECT_ROOT="$ROOT" "$ENGINE" compose \
        -p leedevkit-local-build \
        -f "$ROOT/container/go/docker-compose.test.yml" \
        run --rm -T --no-deps "${BUILD_FLAG[@]}" \
        -e CGO_ENABLED=0 -e GOPROXY=off \
        go go build -buildvcs=false -o "/workspace/build/$TMP_NAME" ./cmd/leedevkit
fi

[ -x "$TMP_BIN" ] || chmod +x "$TMP_BIN"
mv -f "$TMP_BIN" "$OUT"
printf '%s\n' "$OUT"
