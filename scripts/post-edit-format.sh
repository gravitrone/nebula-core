#!/usr/bin/env bash
# Auto-format files after Claude edits (PostToolUse hook)
set -euo pipefail

FILE_PATH="${1:-}"
if [[ -z "$FILE_PATH" ]]; then
    exit 0
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "$FILE_PATH" in
    *.py)
        cd "${ROOT_DIR}/server"
        if command -v uv >/dev/null 2>&1; then
            uv run ruff format "$FILE_PATH" 2>/dev/null || true
            uv run ruff check --fix "$FILE_PATH" 2>/dev/null || true
        fi
        ;;
    *.go)
        gofmt -w "$FILE_PATH" 2>/dev/null || true
        ;;
esac
