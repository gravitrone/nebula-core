#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"

MAX_ITERATIONS=${1:-20}
ITERATION=0

command -v gum >/dev/null 2>&1 || { echo "gum not installed: brew install gum"; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "claude not installed"; exit 1; }

# Nebula theme: primary=#7f57b4, secondary=#436b77, accent=#a7754e, text=#d7d9da, muted=#9ba0bf
gum style --border double --padding "1 2" \
  --border-foreground "#7f57b4" --foreground "#d7d9da" \
  "Nebula Autoresearch Loop" \
  "" \
  "max iterations: $MAX_ITERATIONS" \
  "pattern: evaluate -> fix -> verify -> commit/revert"

while [ $ITERATION -lt $MAX_ITERATIONS ]; do
  ITERATION=$((ITERATION + 1))

  echo ""
  gum style --foreground "#436b77" --bold "--- Iteration $ITERATION/$MAX_ITERATIONS ---"

  # Evaluate current state
  ./evaluate.sh

  SCORE=$(jq -r '.score' reports/latest.json)
  PASSING=$(jq -r '.passing' reports/latest.json)
  TOTAL=$(jq -r '.total' reports/latest.json)

  # Check if done
  if [ "$SCORE" = "1" ] || [ "$SCORE" = "1.00" ]; then
    gum style --foreground "#3f866b" --bold \
      "All $TOTAL checks passing. Score: 1.00. Done."
    break
  fi

  gum log --level info "Current score: $SCORE ($PASSING/$TOTAL)"

  # Extract failing checks
  FAILURES=$(jq '[.scenarios[].checks[] | select(.pass == false)]' reports/latest.json)
  FAILURE_COUNT=$(echo "$FAILURES" | jq 'length')

  if [ "$FAILURE_COUNT" -eq 0 ]; then
    gum log --level warn "No failing checks found but score < 1.0. Stopping."
    break
  fi

  gum log --level info "$FAILURE_COUNT failing checks found"

  # Ask claude code headless to fix the bugs
  PROGRAM=$(cat program.md)
  FIX_PROMPT="$PROGRAM

## Current State
Score: $SCORE ($PASSING/$TOTAL passing)

## Failing Checks
$(echo "$FAILURES" | jq -r '.[] | "- [\(.severity // "unknown")] \(.assertion): \(.issue // "no details")"')

## Instructions
Fix these visual bugs in the Go source code under cli/src/internal/ui/.
After fixing, run: cd $ROOT && make test-cli
Only proceed if all tests pass. If tests fail, revert your changes."

  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Claude fixing $FAILURE_COUNT visual bugs..." -- \
    claude -p "$FIX_PROMPT" \
      --headless \
      --allowedTools Read,Edit,Write,Bash \
      --max-turns 30 \
      --cwd "$ROOT" 2>/dev/null || true

  # Verify tests still pass
  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Verifying tests..." -- \
    bash -c "cd '$ROOT' && make test-cli > /dev/null 2>&1" || {
      gum log --level error "Tests failed after fix. Reverting."
      cd "$ROOT" && git checkout -- cli/src/internal/ui/
      continue
    }

  # Re-evaluate
  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Re-evaluating..." -- sleep 0.1
  ./evaluate.sh

  NEW_SCORE=$(jq -r '.score' reports/latest.json)
  NEW_PASSING=$(jq -r '.passing' reports/latest.json)

  # Compare scores
  IMPROVED=$(echo "$NEW_SCORE > $SCORE" | bc 2>/dev/null || echo "0")

  if [ "$IMPROVED" -eq 1 ]; then
    gum style --foreground "#3f866b" \
      "Improved: $SCORE -> $NEW_SCORE ($PASSING -> $NEW_PASSING passing)"

    cd "$ROOT"
    git add -A
    git commit -m "$(cat <<EOF
fix(cli): autoresearch visual fix (score $SCORE -> $NEW_SCORE)

Iteration $ITERATION: fixed $FAILURE_COUNT visual checks.
Passing: $NEW_PASSING/$TOTAL
EOF
    )"
    cd "$ROOT/testing/autoresearch"

    gum log --level info "Committed fix"
  else
    gum style --foreground "#a7754e" \
      "No improvement ($SCORE -> $NEW_SCORE). Reverting."
    cd "$ROOT" && git checkout -- cli/src/internal/ui/
    cd "$ROOT/testing/autoresearch"
  fi
done

echo ""
FINAL_SCORE=$(jq -r '.score' reports/latest.json 2>/dev/null || echo "N/A")
gum style --border rounded --padding "1 2" --border-foreground "#3f866b" \
  "Autoresearch complete" \
  "Iterations: $ITERATION" \
  "Final score: $FINAL_SCORE"
