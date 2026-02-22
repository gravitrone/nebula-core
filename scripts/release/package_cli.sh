#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI_DIR="$ROOT_DIR/cli/src"
VERSION="${1:-${NEBULA_RELEASE_VERSION:-dev}}"
OUT_DIR="${2:-$ROOT_DIR/scripts/release/out}"

mkdir -p "$OUT_DIR"

platforms=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "linux/arm64"
)

for platform in "${platforms[@]}"; do
  os="${platform%/*}"
  arch="${platform#*/}"
  target="${os}_${arch}"
  tmp_dir="$(mktemp -d)"

  (
    cd "$CLI_DIR"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$VERSION" -o "$tmp_dir/nebula" ./cmd/nebula
  )

  tar -C "$tmp_dir" -czf "$OUT_DIR/nebula-${VERSION}-${target}.tar.gz" nebula
  rm -rf "$tmp_dir"

done
