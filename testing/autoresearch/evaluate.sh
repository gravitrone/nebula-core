#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"

# Check dependencies
command -v gum >/dev/null 2>&1 || { echo "gum not installed: brew install gum"; exit 1; }
command -v vhs >/dev/null 2>&1 || { echo "vhs not installed: brew install vhs"; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "claude not installed"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq not installed: brew install jq"; exit 1; }

gum style --border rounded --padding "1 2" --border-foreground "#7f57b4" \
  "Nebula Autoresearch - Evaluate"

# Build fresh
gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Building nebula..." -- \
  bash -c "cd '$ROOT' && make build"

# Run VHS tapes to capture screenshots
gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Capturing screenshots..." -- \
  bash -c "cd '$ROOT/testing/vhs' && ./run.sh 2>/dev/null"

# Load scenarios
SCENARIOS=$(cat scenarios.json)
TOTAL=$(echo "$SCENARIOS" | jq 'length')
PASSING=0
TOTAL_CHECKS=0
RESULTS="[]"

# Read the analysis prompt template
PROMPT_TEMPLATE=$(cat prompts/analyze.md)

for i in $(seq 0 $((TOTAL - 1))); do
  scenario=$(echo "$SCENARIOS" | jq -c ".[$i]")
  name=$(echo "$scenario" | jq -r '.name')
  screenshot=$(echo "$scenario" | jq -r '.screenshot')
  checks=$(echo "$scenario" | jq -r '.checks | map("- " + .) | join("\n")')

  if [ ! -f "$ROOT/$screenshot" ]; then
    gum log --level warn "  $name - screenshot not found: $screenshot"
    continue
  fi

  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Analyzing: $name" -- sleep 0.1

  # Build the prompt with checks injected
  prompt=$(echo "$PROMPT_TEMPLATE" | sed "s|{{CHECKS}}|$checks|g")

  # Send screenshot + prompt to claude code headless
  result=$(claude -p "$prompt" \
    --headless \
    --output-format json \
    --image "$ROOT/$screenshot" 2>/dev/null || echo '{"checks":[]}')

  # Parse results
  scenario_checks=$(echo "$result" | jq '.checks // []')
  scenario_pass=$(echo "$scenario_checks" | jq '[.[] | select(.pass == true)] | length')
  scenario_total=$(echo "$scenario_checks" | jq 'length')

  PASSING=$((PASSING + scenario_pass))
  TOTAL_CHECKS=$((TOTAL_CHECKS + scenario_total))

  # Add to results
  scenario_result=$(jq -n \
    --arg name "$name" \
    --arg screenshot "$screenshot" \
    --argjson checks "$scenario_checks" \
    '{name: $name, screenshot: $screenshot, checks: $checks}')
  RESULTS=$(echo "$RESULTS" | jq ". + [$scenario_result]")

  if [ "$scenario_pass" -eq "$scenario_total" ] && [ "$scenario_total" -gt 0 ]; then
    gum log --level info "  $name: $scenario_pass/$scenario_total"
  else
    gum log --level error "  $name: $scenario_pass/$scenario_total"
  fi
done

# Compute score
if [ "$TOTAL_CHECKS" -gt 0 ]; then
  SCORE=$(echo "scale=2; $PASSING / $TOTAL_CHECKS" | bc)
else
  SCORE="0.00"
fi

# Write report
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
jq -n \
  --arg timestamp "$TIMESTAMP" \
  --arg score "$SCORE" \
  --argjson passing "$PASSING" \
  --argjson total "$TOTAL_CHECKS" \
  --argjson scenarios "$RESULTS" \
  '{timestamp: $timestamp, score: ($score | tonumber), passing: $passing, total: $total, scenarios: $scenarios}' \
  > reports/latest.json

# Copy to history
cp reports/latest.json "reports/history/$(date +%Y%m%d_%H%M%S).json"

echo ""
gum style --foreground "#7f57b4" --bold "Score: $PASSING/$TOTAL_CHECKS ($SCORE)"
