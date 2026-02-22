#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-}"
CHANNEL="${2:-stable}"
BASE_URL="${3:-}"
OUT_DIR="${ROOT_DIR}/scripts/release/out"

if [[ -z "$VERSION" || -z "$BASE_URL" ]]; then
  echo "usage: $0 <version> <channel> <base-url>" >&2
  echo "example: $0 v1.0.0 stable https://nebula.gravitrone.com/channels/stable" >&2
  exit 1
fi

"$ROOT_DIR/scripts/release/package_cli.sh" "$VERSION" "$OUT_DIR"
"$ROOT_DIR/scripts/release/package_runtime.sh" "$VERSION" "$OUT_DIR"
"$ROOT_DIR/scripts/release/build_manifest.py" \
  --version "$VERSION" \
  --channel "$CHANNEL" \
  --base-url "$BASE_URL" \
  --out-dir "$OUT_DIR"
