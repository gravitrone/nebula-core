#!/usr/bin/env bash
# Capture nebula TUI screenshots using tmux + freeze
# Usage: ./capture.sh <output_dir> [nebula_binary]
set -euo pipefail

OUTPUT_DIR="${1:-.}"
NEBULA="${2:-nebula}"
SESSION="nebula-capture-$$"
WIDTH=120
HEIGHT=40

mkdir -p "$OUTPUT_DIR"

capture() {
  local name="$1"
  sleep 1
  tmux capture-pane -t "$SESSION" -p -e > "/tmp/nebula-pane-$$.txt"
  cat "/tmp/nebula-pane-$$.txt" | freeze --language ansi -o "$OUTPUT_DIR/$name.png" 2>/dev/null && {
    echo "  captured: $name.png"
  } || {
    echo "  failed: $name.png"
  }
}

sendkeys() {
  tmux send-keys -t "$SESSION" "$@"
}

# ── start nebula in tmux ────────────────────────────────────────────────────
tmux new-session -d -s "$SESSION" -x "$WIDTH" -y "$HEIGHT" "$NEBULA"
sleep 3

# ── capture startup (inbox tab) ────────────────────────────────────────────
capture "startup"

# ── capture entities tab ────────────────────────────────────────────────────
sendkeys "2"
sleep 1
capture "entities"

# ── capture relationships tab ───────────────────────────────────────────────
sendkeys "3"
sleep 1
capture "relationships"

# ── capture context tab ─────────────────────────────────────────────────────
sendkeys "4"
sleep 1
capture "context"

# ── capture jobs tab ────────────────────────────────────────────────────────
sendkeys "5"
sleep 1
capture "jobs"

# ── capture logs tab ────────────────────────────────────────────────────────
sendkeys "6"
sleep 1
capture "logs"

# ── capture files tab ───────────────────────────────────────────────────────
sendkeys "7"
sleep 1
capture "files"

# ── capture protocols tab ───────────────────────────────────────────────────
sendkeys "8"
sleep 1
capture "protocols"

# ── capture history tab ─────────────────────────────────────────────────────
sendkeys "9"
sleep 1
capture "history"

# ── capture settings tab ───────────────────────────────────────────────────
sendkeys "0"
sleep 1
capture "settings"

# ── capture command palette ─────────────────────────────────────────────────
sendkeys "/"
sleep 1
capture "command_palette"

sendkeys "Escape"
sleep 0.5

# ── capture help overlay ───────────────────────────────────────────────────
sendkeys "?"
sleep 1
capture "help"

# ── cleanup ─────────────────────────────────────────────────────────────────
sendkeys "q"
sleep 0.5
tmux kill-session -t "$SESSION" 2>/dev/null || true
rm -f "/tmp/nebula-pane-$$.txt"

echo ""
echo "captured $(ls "$OUTPUT_DIR"/*.png 2>/dev/null | wc -l | tr -d ' ') screenshots to $OUTPUT_DIR/"
