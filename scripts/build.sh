#!/usr/bin/env bash
# Cross-platform build script for macOS / Linux / BSD.
# Usage: ./scripts/build.sh            # host platform
#        ./scripts/build.sh --all      # every release target
#        ./scripts/build.sh --test     # run tests first
set -euo pipefail

cd "$(dirname "$0")/.."
OUT=bin
VERSION="$(cat VERSION 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
LDFLAGS="-s -w -X github.com/Debajyoti0-0/aether/internal/version.Version=${VERSION} -X github.com/Debajyoti0-0/aether/internal/version.Commit=${COMMIT}"

ALL=0
TEST=0
for arg in "$@"; do
  case "$arg" in
    --all)  ALL=1 ;;
    --test) TEST=1 ;;
  esac
done

if [[ $TEST -eq 1 ]]; then
  go vet ./...
  go test ./...
fi

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
  "freebsd/amd64"
)

if [[ $ALL -eq 0 ]]; then
  host_os="$(go env GOOS)"
  host_arch="$(go env GOARCH)"
  targets=("${host_os}/${host_arch}")
fi

mkdir -p "$OUT"
for t in "${targets[@]}"; do
  os="${t%%/*}"
  arch="${t##*/}"
  ext=""
  [[ "$os" == "windows" ]] && ext=".exe"
  name="aether-${os}-${arch}${ext}"
  echo "Building ${name} ..."
  GOOS="$os" GOARCH="$arch" go build -ldflags "$LDFLAGS" -o "$OUT/$name" ./cmd/aether
done

echo ""
echo "Done. Binaries in $OUT"
