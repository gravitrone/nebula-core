#!/usr/bin/env bash
set -euo pipefail

# TODO(cto): re-enable the release installer flow once channels/manifest infra is finalized.
# Current state is intentionally paused.

print_box() {
  local title="$1"
  local body="$2"
  local width=92
  local rule
  rule=$(printf '%*s' "$width" '' | tr ' ' '─')
  printf '╭%s╮\n' "${rule}"
  printf '│ %-88s │\n' "[ ${title} ]"
  printf '├%s┤\n' "${rule}"
  while IFS= read -r line; do
    printf '│ %-88s │\n' "$line"
  done <<<"$body"
  printf '╰%s╯\n' "${rule}"
}

print_box "install paused" "installer bootstrap is parked for now.\nmanual setup is still available via repo docs.\ncheck artifacts/Refactor-TODO.md for the next rollout steps."
exit 1
