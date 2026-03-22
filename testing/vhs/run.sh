#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# Check dependencies
command -v gum >/dev/null 2>&1 || { echo "gum not installed: brew install gum"; exit 1; }
command -v vhs >/dev/null 2>&1 || { echo "vhs not installed: brew install vhs"; exit 1; }

gum style --border rounded --padding "1 2" \
  --border-foreground "#7f57b4" --foreground "#d7d9da" \
  "Nebula VHS Visual Regression"

# Build fresh binary
gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Building nebula..." -- \
  bash -c "cd ../.. && make build"

PASS=0
FAIL=0
TOTAL=0

for tape in tapes/*.tape; do
  name=$(basename "$tape" .tape)
  TOTAL=$((TOTAL + 1))

  if gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Recording: $name" -- vhs "$tape" 2>/dev/null; then
    PASS=$((PASS + 1))
    gum log --level info "  $name recorded"
  else
    FAIL=$((FAIL + 1))
    gum log --level error "  $name FAILED"
  fi
done

echo ""
if [ $FAIL -eq 0 ]; then
  gum style --foreground "#3f866b" --bold "All $TOTAL tapes recorded successfully"
else
  gum style --foreground "#d1606b" --bold "$FAIL/$TOTAL tapes failed"
  exit 1
fi
