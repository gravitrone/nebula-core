#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"

MAX_ITERATIONS=${1:-5}

command -v gum >/dev/null 2>&1 || { echo "gum not installed: brew install gum"; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "claude not installed"; exit 1; }

# Nebula theme colors
gum style --border double --padding "1 2" \
  --border-foreground "#7f57b4" --foreground "#d7d9da" \
  "Nebula Autoresearch Loop" \
  "" \
  "max iterations: $MAX_ITERATIONS" \
  "mode: claude code headless + golden file analysis"

for i in $(seq 1 "$MAX_ITERATIONS"); do
  echo ""
  gum style --foreground "#436b77" --bold "--- iteration $i/$MAX_ITERATIONS ---"

  # Run golden tests to check current visual state
  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Running golden tests..." -- \
    bash -c "cd '$ROOT/cli/src' && go test ./internal/ui/ -run TestGolden -count=1 > /tmp/golden-result.txt 2>&1" || true

  GOLDEN_RESULT=$(cat /tmp/golden-result.txt 2>/dev/null || echo "failed to run")
  GOLDEN_PASS=$(echo "$GOLDEN_RESULT" | grep -c "PASS" || echo "0")

  gum log --level info "golden tests: $GOLDEN_PASS passing"

  # Run full test suite
  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Running full test suite..." -- \
    bash -c "cd '$ROOT' && make test-cli > /tmp/test-result.txt 2>&1" || true

  TEST_RESULT=$(cat /tmp/test-result.txt 2>/dev/null || echo "failed")
  if echo "$TEST_RESULT" | grep -q "^ok"; then
    gum log --level info "all tests passing"
  else
    FAILURES=$(echo "$TEST_RESULT" | grep "FAIL" || echo "unknown")
    gum log --level error "test failures: $FAILURES"
  fi

  # Build fresh binary
  gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Building nebula..." -- \
    bash -c "cd '$ROOT' && make build 2>/dev/null"

  # Launch claude code headless to analyze and fix visual bugs
  gum style --foreground "#9ba0bf" "launching claude code headless to find and fix visual bugs..."

  PROMPT=$(cat program.md)
  PROMPT="$PROMPT

## Current State
- Golden tests: $GOLDEN_PASS passing
- Full suite: $(echo "$TEST_RESULT" | tail -1)

## Task
1. Build and run the nebula CLI binary (cli/src/build/nebula) to visually inspect it
2. Take a screenshot of the running TUI using the screenshot tool
3. Analyze the screenshot for visual bugs from the checks in this prompt
4. Fix any bugs you find in cli/src/internal/ui/
5. Run make test-cli to verify no regressions
6. Report what you fixed"

  claude -p "$PROMPT" \
    --headless \
    --allowedTools "Read,Edit,Write,Bash,screenshot" \
    --max-turns 20 \
    --cwd "$ROOT" 2>/dev/null || {
      gum log --level warn "claude session ended"
    }

  # Check if anything changed
  if [ -n "$(cd "$ROOT" && git diff --name-only)" ]; then
    # Verify tests pass
    gum spin --spinner dot --spinner.foreground "#7f57b4" --title "Verifying tests after fixes..." -- \
      bash -c "cd '$ROOT' && make test-cli > /dev/null 2>&1" || {
        gum log --level error "tests failed after fix, reverting"
        cd "$ROOT" && git checkout -- cli/src/internal/ui/
        continue
      }

    gum log --level info "tests pass, committing fix"
    cd "$ROOT"
    git add -A
    git commit -m "fix(cli): autoresearch iteration $i visual fixes"
    cd "$ROOT/testing/autoresearch"
  else
    gum log --level info "no changes made this iteration"
  fi
done

echo ""
gum style --border rounded --padding "1 2" \
  --border-foreground "#3f866b" --foreground "#d7d9da" \
  "autoresearch complete after $MAX_ITERATIONS iterations"
