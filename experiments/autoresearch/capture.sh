#!/usr/bin/env bash
# Capture nebula TUI state using tmux capture-pane
# Saves raw ANSI terminal output that claude can read and analyze
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
  tmux capture-pane -t "$SESSION" -p -e > "$OUTPUT_DIR/$name.ans"
  echo "  captured: $name"
}

sendkeys() {
  tmux send-keys -t "$SESSION" "$@"
}

# ── start nebula in tmux ────────────────────────────────────────────────────
tmux new-session -d -s "$SESSION" -x "$WIDTH" -y "$HEIGHT" "$NEBULA"
sleep 3

# ── capture all tabs ────────────────────────────────────────────────────────
capture "startup"

sendkeys "2"; capture "entities"
sendkeys "3"; capture "relationships"
sendkeys "4"; capture "context"
sendkeys "5"; capture "jobs"
sendkeys "6"; capture "logs"
sendkeys "7"; capture "files"
sendkeys "8"; capture "protocols"
sendkeys "9"; capture "history"
sendkeys "0"; capture "settings"

# ── capture overlays ────────────────────────────────────────────────────────
sendkeys "/"; sleep 1; capture "command_palette"
sendkeys "Escape"; sleep 0.5
sendkeys "?"; sleep 1; capture "help"

# ── cleanup ─────────────────────────────────────────────────────────────────
sendkeys "q"; sleep 0.5
tmux kill-session -t "$SESSION" 2>/dev/null || true

echo ""
echo "captured $(ls "$OUTPUT_DIR"/*.ans 2>/dev/null | wc -l | tr -d ' ') snapshots to $OUTPUT_DIR/"
