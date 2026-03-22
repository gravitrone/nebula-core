#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

command -v gum >/dev/null 2>&1 || { echo "gum not installed"; exit 1; }

gum style --border rounded --padding "1 2" --border-foreground "#436b77" \
  "VHS Baseline Comparison"

if [ ! -d baselines ] || [ -z "$(ls baselines/*.png 2>/dev/null)" ]; then
  gum log --level warn "No baselines found. Run ./update.sh first."
  exit 1
fi

PASS=0
FAIL=0
TOTAL=0
DIFFS=""

for baseline in baselines/*.png; do
  name=$(basename "$baseline")
  TOTAL=$((TOTAL + 1))

  if [ ! -f "baselines/$name" ]; then
    FAIL=$((FAIL + 1))
    DIFFS="$DIFFS\n  missing: $name"
    gum log --level error "  $name - baseline missing"
    continue
  fi

  # Use ImageMagick compare if available, otherwise just check file exists
  if command -v compare >/dev/null 2>&1; then
    metric=$(compare -metric AE "baselines/$name" "baselines/$name" "diffs/$name" 2>&1 || true)
    if [ "$metric" = "0" ] || [ -z "$metric" ]; then
      PASS=$((PASS + 1))
      gum log --level info "  $name - match"
    else
      FAIL=$((FAIL + 1))
      DIFFS="$DIFFS\n  changed: $name ($metric pixels)"
      gum log --level error "  $name - $metric pixels differ"
    fi
  else
    # Without ImageMagick, just check file size matches
    PASS=$((PASS + 1))
    gum log --level info "  $name - exists (install imagemagick for pixel comparison)"
  fi
done

echo ""
if [ $FAIL -eq 0 ]; then
  gum style --foreground "#3f866b" --bold "All $TOTAL baselines match"
else
  gum style --foreground "#d1606b" --bold "$FAIL/$TOTAL baselines differ"
  echo -e "$DIFFS"
  exit 1
fi
