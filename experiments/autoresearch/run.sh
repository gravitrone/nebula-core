#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Build if needed
[ -f autoresearch ] || go build -o autoresearch .

# Run
./autoresearch "$@"
