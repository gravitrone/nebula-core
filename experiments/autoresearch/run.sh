#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
ROOT="$(cd ../.. && pwd)"

# ── config ──────────────────────────────────────────────────────────────────
MAX_ITERATIONS=${1:-5}
BRANCH="feat/charm-migration"
WORKTREE_DIR="$ROOT/.autoresearch-work"

# ── helpers ─────────────────────────────────────────────────────────────────
info()    { gum log --level info "$1"; }
warn()    { gum log --level warn "$1"; }
fail()    { gum log --level error "$1"; }

# ── dependency check ────────────────────────────────────────────────────────
for dep in gum claude jq git; do
  command -v "$dep" >/dev/null 2>&1 || { echo "$dep not installed"; exit 1; }
done

# ── banner ──────────────────────────────────────────────────────────────────
echo ""
gum style --bold --foreground 212 "Nebula Autoresearch"
gum style --faint --italic "Autonomous Visual Bug Hunter"
echo ""
gum log --level info "Iterations   $MAX_ITERATIONS"
gum log --level info "Branch       $BRANCH"
gum log --level info "Isolation    Git Worktrees"
echo ""

# ── main ────────────────────────────────────────────────────────────────────
TOTAL_FIXES=0
ITERATION=0

while [ $ITERATION -lt $MAX_ITERATIONS ]; do
  ITERATION=$((ITERATION + 1))
  echo ""
  gum format -- "## iteration $ITERATION/$MAX_ITERATIONS"

  # ── create isolated worktree ──────────────────────────────────────────────
  ITER_BRANCH="autoresearch/iter-$ITERATION"
  ITER_DIR="$WORKTREE_DIR/iter-$ITERATION"

  if [ -d "$ITER_DIR" ]; then
    cd "$ROOT"
    git worktree remove --force "$ITER_DIR" 2>/dev/null || true
    git branch -D "$ITER_BRANCH" 2>/dev/null || true
  fi

  cd "$ROOT"
  git branch "$ITER_BRANCH" "$BRANCH" 2>/dev/null || {
    git branch -D "$ITER_BRANCH" 2>/dev/null || true
    git branch "$ITER_BRANCH" "$BRANCH"
  }
  git worktree add "$ITER_DIR" "$ITER_BRANCH" 2>/dev/null
  info "worktree: $ITER_DIR"

  # ── run tests in worktree ─────────────────────────────────────────────────
  gum spin --spinner dot --title "running golden tests..." -- \
    bash -c "cd '$ITER_DIR/cli/src' && go test ./internal/ui/ -run TestGolden -count=1 -v > /tmp/ar-golden.txt 2>&1" || true

  GOLDEN_PASS=$(grep -c "--- PASS" /tmp/ar-golden.txt 2>/dev/null || echo "0")
  info "golden tests: $GOLDEN_PASS passing"

  gum spin --spinner dot --title "running full test suite..." -- \
    bash -c "cd '$ITER_DIR' && make test-cli > /tmp/ar-tests.txt 2>&1" || true

  if grep -q "^ok" /tmp/ar-tests.txt 2>/dev/null; then
    info "full suite: all passing"
  else
    FAILS=$(grep "FAIL" /tmp/ar-tests.txt 2>/dev/null || echo "unknown")
    warn "test issues: $FAILS"
  fi

  # ── build binary ──────────────────────────────────────────────────────────
  gum spin --spinner dot --title "building nebula..." -- \
    bash -c "cd '$ITER_DIR' && make build 2>/dev/null"

  # ── capture screenshots via tmux + freeze ─────────────────────────────────
  info "capturing TUI screenshots..."
  SCREENSHOTS_DIR="$ITER_DIR/screenshots"
  NEBULA_BIN="$ITER_DIR/cli/src/build/nebula"

  "$SCRIPT_DIR/capture.sh" "$SCREENSHOTS_DIR" "$NEBULA_BIN"

  SCREENSHOT_COUNT=$(ls "$SCREENSHOTS_DIR"/*.png 2>/dev/null | wc -l | tr -d ' ')
  info "captured $SCREENSHOT_COUNT screenshots"

  # ── build screenshot file list for prompt ────────────────────────────────
  SCREENSHOT_INSTRUCTIONS=""
  for png in "$SCREENSHOTS_DIR"/*.png; do
    [ -f "$png" ] || continue
    SCREENSHOT_INSTRUCTIONS="$SCREENSHOT_INSTRUCTIONS
- Read the image file at $png using the Read tool and analyze it visually"
  done

  # ── launch claude ─────────────────────────────────────────────────────────
  info "launching claude to analyze screenshots and fix visual bugs..."

  PROMPT=$(cat "$SCRIPT_DIR/program.md")
  SUITE_STATUS=$(tail -1 /tmp/ar-tests.txt 2>/dev/null || echo "unknown")
  PROMPT="$PROMPT

## Current State (iteration $ITERATION)
- Golden tests: $GOLDEN_PASS passing
- Full suite: $SUITE_STATUS

## CRITICAL: You MUST look at actual screenshots before making any changes

Screenshots have been captured at: $SCREENSHOTS_DIR/

Your FIRST action must be to Read each screenshot file listed below using the Read tool.
The Read tool can display images visually. Look at each one and identify visual bugs.
$SCREENSHOT_INSTRUCTIONS

## Task
1. FIRST: Read each screenshot GIF file listed above using the Read tool - actually LOOK at the rendered TUI
2. Identify visual bugs you can SEE in the screenshots (alignment, overflow, spacing, missing elements)
3. Fix any bugs you find in cli/src/internal/ui/
4. Run: make test-cli to verify no regressions
5. Only keep changes that pass tests
6. Be specific about what you SAW in the screenshots and what you fixed
7. When done fixing, use /commit-forge to commit your changes with a proper conventional commit message"

  (cd "$ITER_DIR" && echo "$PROMPT" | claude \
    --model opus \
    --max-turns 25 \
    2>/dev/null) || {
      warn "claude session ended"
    }

  # ── check if claude committed ───────────────────────────────────────────
  cd "$ITER_DIR"
  COMMITS_AHEAD=$(git log "$BRANCH..$ITER_BRANCH" --oneline 2>/dev/null | wc -l | tr -d ' ')

  if [ "$COMMITS_AHEAD" -eq 0 ]; then
    # claude didn't commit - check for uncommitted changes
    CHANGED=$(git diff --name-only 2>/dev/null || echo "")
    if [ -n "$CHANGED" ]; then
      # save work so it's not lost
      info "claude didn't commit, saving uncommitted changes..."
      git add -A
      git commit -m "fix(cli): autoresearch visual fixes (iteration $ITERATION)" 2>/dev/null
      COMMITS_AHEAD=1
    else
      info "no changes this iteration"
      cd "$ROOT"
      git worktree remove --force "$ITER_DIR" 2>/dev/null || true
      git branch -D "$ITER_BRANCH" 2>/dev/null || true
      continue
    fi
  fi

  info "$COMMITS_AHEAD commit(s) from claude"

  # ── verify tests pass ────────────────────────────────────────────────────
  gum spin --spinner dot --title "verifying tests after fixes..." -- \
    bash -c "cd '$ITER_DIR' && make test-cli > /dev/null 2>&1" || {
      fail "tests failed after fix, keeping branch for review: $ITER_BRANCH"
      cd "$ROOT"
      git worktree remove --force "$ITER_DIR" 2>/dev/null || true
      # keep the branch so changes aren't lost
      continue
    }

  # ── merge into main branch ────────────────────────────────────────────────
  cd "$ROOT"
  git checkout "$BRANCH" 2>/dev/null
  git merge "$ITER_BRANCH" --no-edit 2>/dev/null || {
    fail "merge conflict, keeping branch for review: $ITER_BRANCH"
    git merge --abort 2>/dev/null || true
    git worktree remove --force "$ITER_DIR" 2>/dev/null || true
    # keep the branch so changes aren't lost
    continue
  }

  info "merged iteration $ITERATION into $BRANCH"
  TOTAL_FIXES=$((TOTAL_FIXES + 1))

  # ── cleanup worktree (only after successful merge) ──────────────────────
  git worktree remove --force "$ITER_DIR" 2>/dev/null || true
  git branch -D "$ITER_BRANCH" 2>/dev/null || true
done

# ── summary ─────────────────────────────────────────────────────────────────
echo ""
gum style --bold --foreground 35 "Autoresearch Complete"
echo ""
gum log --level info "Iterations    $ITERATION"
gum log --level info "Fixes Merged  $TOTAL_FIXES"
gum log --level info "Branch        $BRANCH"

# cleanup worktree dir if empty
rmdir "$WORKTREE_DIR" 2>/dev/null || true

# ── TODO ────────────────────────────────────────────────────────────────────
# Phase 2: after autoresearch commits, spawn a review agent:
#   1. second claude instance reviews the diff for quality/safety
#   2. if review passes -> auto-merge
#   3. if review flags issues -> create follow-up iteration
#   4. full autonomous CI: fix -> commit -> PR -> review -> merge
