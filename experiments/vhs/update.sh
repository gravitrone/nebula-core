#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

command -v gum >/dev/null 2>&1 || { echo "gum not installed"; exit 1; }

gum style --border rounded --padding "1 2" --border-foreground "#a7754e" \
  "Update VHS Baselines"

if ! gum confirm "This will regenerate all baseline screenshots. Continue?"; then
  gum log --level warn "Aborted."
  exit 0
fi

# Run all tapes to generate fresh baselines
./run.sh

COUNT=$(ls baselines/*.png 2>/dev/null | wc -l | tr -d ' ')
gum style --foreground "#3f866b" --bold "Updated $COUNT baselines"
gum log --level info "Review baselines/ directory and commit if correct."
