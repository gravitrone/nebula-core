#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-${NEBULA_RELEASE_VERSION:-dev}}"
OUT_DIR="${2:-$ROOT_DIR/scripts/release/out}"
WORK_DIR="$(mktemp -d)"
RUNTIME_DIR="$WORK_DIR/runtime"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$OUT_DIR" "$RUNTIME_DIR"

cp "$ROOT_DIR/scripts/release/templates/runtime/compose.yaml" "$RUNTIME_DIR/compose.yaml"
cp "$ROOT_DIR/scripts/release/templates/runtime/.env.example" "$RUNTIME_DIR/.env.example"
cp -R "$ROOT_DIR/database/migrations" "$RUNTIME_DIR/migrations"

RUNTIME_ARCHIVE="$OUT_DIR/nebula-runtime-${VERSION}.tar.gz"

tar -C "$RUNTIME_DIR" -czf "$RUNTIME_ARCHIVE" .

printf '%s\n' "$RUNTIME_ARCHIVE"
