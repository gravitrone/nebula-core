#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
ROOT="$(cd ../.. && pwd)"

# ── config ──────────────────────────────────────────────────────────────────
MAX_ITERATIONS=${1:-5}
BRANCH="feat/charm-migration"
WORKTREE_DIR="$ROOT/.autoresearch-work"

# ── nebula theme ────────────────────────────────────────────────────────────
P="#7f57b4"   # primary (purple)
S="#436b77"   # secondary (teal)
A="#a7754e"   # accent (warm)
T="#d7d9da"   # text
M="#9ba0bf"   # muted
OK="#3f866b"  # success (green)
ERR="#d1606b" # error (red)

# ── helpers ─────────────────────────────────────────────────────────────────
banner() {
  echo ""
  local title=$(gum style --bold --foreground "$P" "◆ Nebula Autoresearch")
  local sub=$(gum style --faint --italic "  Autonomous Visual Bug Hunter")
  local sep=$(gum style --foreground "$S" "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
  local info=$(printf "  Iterations   %s\n  Branch       %s\n  Isolation    Git Worktrees" "$MAX_ITERATIONS" "$BRANCH")
  printf "%s\n%s\n%s\n%s" "$title" "$sub" "$sep" "$info" \
    | gum style --border rounded --border-foreground "$P" --padding "1 2"
}

header() {
  echo "" && echo "$1" | gum style --foreground "$S" --bold
}

info() {
  gum log --level info "$1"
}

success() {
  echo "$1" | gum style --foreground "$OK" --bold
}

warn() {
  echo "$1" | gum style --foreground "$A"
}

fail() {
  echo "$1" | gum style --foreground "$ERR" --bold
}

spin() {
  gum spin --spinner dot --spinner.foreground "$P" --title "$1" -- "${@:2}"
}

# ── dependency check ────────────────────────────────────────────────────────
for dep in gum claude jq git; do
  command -v "$dep" >/dev/null 2>&1 || { fail "$dep not installed"; exit 1; }
done

# ── main ────────────────────────────────────────────────────────────────────
banner

TOTAL_FIXES=0
ITERATION=0

while [ $ITERATION -lt $MAX_ITERATIONS ]; do
  ITERATION=$((ITERATION + 1))
  header "iteration $ITERATION/$MAX_ITERATIONS"

  # ── create isolated worktree ──────────────────────────────────────────────
  ITER_BRANCH="autoresearch/iter-$ITERATION"
  ITER_DIR="$WORKTREE_DIR/iter-$ITERATION"

  # clean up any leftover worktree from previous failed run
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
  spin "running golden tests..." \
    bash -c "cd '$ITER_DIR/cli/src' && go test ./internal/ui/ -run TestGolden -count=1 -v > /tmp/ar-golden.txt 2>&1" || true

  GOLDEN_PASS=$(grep -c "--- PASS" /tmp/ar-golden.txt 2>/dev/null || echo "0")
  info "golden tests: $GOLDEN_PASS passing"

  spin "running full test suite..." \
    bash -c "cd '$ITER_DIR' && make test-cli > /tmp/ar-tests.txt 2>&1" || true

  if grep -q "^ok" /tmp/ar-tests.txt 2>/dev/null; then
    info "full suite: all passing"
  else
    FAILS=$(grep "FAIL" /tmp/ar-tests.txt 2>/dev/null || echo "unknown")
    warn "test issues: $FAILS"
  fi

  # ── build binary ──────────────────────────────────────────────────────────
  spin "building nebula..." \
    bash -c "cd '$ITER_DIR' && make build 2>/dev/null"

  # ── launch claude code headless ───────────────────────────────────────────
  echo "launching claude to analyze and fix visual bugs..." | gum style --foreground "$M"

  PROMPT=$(cat "$SCRIPT_DIR/program.md")
  SUITE_STATUS=$(tail -1 /tmp/ar-tests.txt 2>/dev/null || echo "unknown")
  PROMPT="$PROMPT

## Current State (iteration $ITERATION)
- Golden tests: $GOLDEN_PASS passing
- Full suite: $SUITE_STATUS

## Task
1. Read the scenario checks from this prompt
2. Look at the TUI source code and identify visual bugs matching the check categories
3. Fix any bugs you find in cli/src/internal/ui/
4. Run: make test-cli to verify no regressions
5. Only keep changes that pass tests
6. Be specific about what you changed and why"

  (cd "$ITER_DIR" && claude -p "$PROMPT" \
    --model opus \
    --allowedTools "Read,Edit,Write,Bash" \
    --max-turns 25 \
    --dangerously-skip-permissions \
    2>/dev/null) || {
      warn "claude session ended"
    }

  # ── check results ─────────────────────────────────────────────────────────
  cd "$ITER_DIR"
  CHANGED=$(git diff --name-only 2>/dev/null || echo "")

  if [ -z "$CHANGED" ]; then
    info "no changes this iteration"
    cd "$ROOT"
    git worktree remove --force "$ITER_DIR" 2>/dev/null || true
    git branch -D "$ITER_BRANCH" 2>/dev/null || true
    continue
  fi

  FILE_COUNT=$(echo "$CHANGED" | wc -l | tr -d ' ')
  info "$FILE_COUNT files changed"

  # ── verify tests pass ────────────────────────────────────────────────────
  spin "verifying tests after fixes..." \
    bash -c "cd '$ITER_DIR' && make test-cli > /dev/null 2>&1" || {
      fail "tests failed after fix, discarding iteration"
      cd "$ROOT"
      git worktree remove --force "$ITER_DIR" 2>/dev/null || true
      git branch -D "$ITER_BRANCH" 2>/dev/null || true
      continue
    }

  # ── generate meaningful commit message ────────────────────────────────────
  DIFF_STAT=$(cd "$ITER_DIR" && git diff --stat)
  DIFF_SUMMARY=$(cd "$ITER_DIR" && git diff --no-color | head -200)

  COMMIT_MSG=$(claude -p "Generate a conventional commit message for this diff. Format: fix(cli): one-line description. Be specific about what visual bugs were fixed. No co-author tags. Output ONLY the commit message, nothing else.

Diff stat:
$DIFF_STAT

Diff preview:
$DIFF_SUMMARY" \
    --model haiku \
    --max-turns 1 2>/dev/null || echo "fix(cli): autoresearch visual fixes (iteration $ITERATION)")

  # clean up the message (remove quotes, markdown, etc)
  COMMIT_MSG=$(echo "$COMMIT_MSG" | grep -E "^(fix|feat|refactor|chore)" | head -1 || echo "fix(cli): autoresearch visual fixes (iteration $ITERATION)")

  # ── commit in worktree ────────────────────────────────────────────────────
  cd "$ITER_DIR"
  git add -A
  git commit -m "$COMMIT_MSG" 2>/dev/null

  info "committed: $COMMIT_MSG"

  # ── merge into main branch ────────────────────────────────────────────────
  cd "$ROOT"
  git checkout "$BRANCH" 2>/dev/null
  git merge "$ITER_BRANCH" --no-edit 2>/dev/null || {
    fail "merge conflict, discarding iteration"
    git merge --abort 2>/dev/null || true
    git worktree remove --force "$ITER_DIR" 2>/dev/null || true
    git branch -D "$ITER_BRANCH" 2>/dev/null || true
    continue
  }

  success "merged iteration $ITERATION into $BRANCH"
  TOTAL_FIXES=$((TOTAL_FIXES + 1))

  # ── cleanup worktree ──────────────────────────────────────────────────────
  git worktree remove --force "$ITER_DIR" 2>/dev/null || true
  git branch -D "$ITER_BRANCH" 2>/dev/null || true
done

# ── summary ─────────────────────────────────────────────────────────────────
echo ""
DONE_TITLE=$(gum style --bold --foreground "$OK" "◆ Autoresearch Complete")
DONE_SEP=$(gum style --foreground "$S" "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
DONE_INFO=$(printf "  Iterations    %s\n  Fixes Merged  %s\n  Branch        %s" "$ITERATION" "$TOTAL_FIXES" "$BRANCH")
printf "%s\n%s\n%s" "$DONE_TITLE" "$DONE_SEP" "$DONE_INFO" \
  | gum style --border rounded --border-foreground "$OK" --padding "1 2"

# cleanup worktree dir if empty
rmdir "$WORKTREE_DIR" 2>/dev/null || true

# ── TODO ────────────────────────────────────────────────────────────────────
# Phase 2: after autoresearch commits, spawn a review agent:
#   1. claude -p "review this PR" --headless creates PR via gh
#   2. second claude instance reviews the diff for quality/safety
#   3. if review passes -> auto-merge
#   4. if review flags issues -> create follow-up iteration
#   5. full autonomous CI: fix -> commit -> PR -> review -> merge
