#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/dist-go}"
VERSION="$(tr -d '\n' < "$ROOT/VERSION")"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
CANONICAL="$STAGE/leedevkit-$VERSION"
mkdir -p "$CANONICAL"
for item in scripts templates bin .agent container cmd internal VERSION go.mod; do
    cp -a "$ROOT/$item" "$CANONICAL/"
done
mkdir -p "$OUT" "$OUT/.go-core" "$OUT/.debug-mcp"
COMPOSE=(docker compose -f "$ROOT/container/go/docker-compose.core.yml" --profile go-core)
TARGETS=("linux amd64" "linux arm64" "darwin amd64" "darwin arm64" "windows amd64")
for target in "${TARGETS[@]}"; do
    read -r goos goarch <<< "$target"
    name="leedevkit-go-$goos-$goarch"; [[ "$goos" == windows ]] && name+=".exe"
    rm -rf "$ROOT/.leedevkit-go-build"
    "${COMPOSE[@]}" run --rm --build -e "GOOS=$goos" -e "GOARCH=$goarch" leedevkit-go build -buildvcs=false -trimpath -ldflags "-X=main.version=$VERSION" -o /workspace/.leedevkit-go-build/leedevkit-go ./cmd/leedevkit
    cp "$ROOT/.leedevkit-go-build/leedevkit-go" "$OUT/.go-core/$name"
    chmod +x "$OUT/.go-core/$name"
    rm -rf "$ROOT/.leedevkit-go-build"
    dname="leedevkit-debug-$goos-$goarch"; [[ "$goos" == windows ]] && dname+=".exe"
    "${COMPOSE[@]}" run --rm --build -e "GOOS=$goos" -e "GOARCH=$goarch" leedevkit-debug-build build -buildvcs=false -trimpath -ldflags "-X=main.version=$VERSION" -o /workspace/.leedevkit-go-build/leedevkit-debug ./cmd/debug-mcp
    cp "$ROOT/.leedevkit-go-build/leedevkit-debug" "$OUT/.debug-mcp/$dname"
    chmod +x "$OUT/.debug-mcp/$dname"
done
cp -a "$OUT/.go-core/." "$CANONICAL/bin/go/"
cp -a "$OUT/.debug-mcp/." "$CANONICAL/bin/debug-mcp/"
docker run --rm -e GOPROXY=off -v "$ROOT":/source:ro -v "$CANONICAL":/workspace -v "$HOME/go/pkg/mod":/go/pkg/mod -w /source golang:1.26.11-bookworm go run ./cmd/release-manifest --root /workspace
cat > "$CANONICAL/sbom.cdx.json" <<EOF
{"bomFormat":"CycloneDX","specVersion":"1.5","version":1,"components":[{"type":"library","name":"github.com/pelletier/go-toml/v2","version":"v2.2.2","purl":"pkg:golang/github.com/pelletier/go-toml/v2@v2.2.2"}]}
EOF
cp "$CANONICAL/sbom.cdx.json" "$OUT/release-sbom.cdx.json"
rm -f "$OUT/leedevkit-$VERSION.tar.gz"
tar -czf "$OUT/leedevkit-$VERSION.tar.gz" -C "$STAGE" "leedevkit-$VERSION"
sha256sum "$OUT/leedevkit-$VERSION.tar.gz" > "$OUT/leedevkit-$VERSION.tar.gz.sha256"
printf '%s\n' "$OUT/leedevkit-$VERSION.tar.gz"
