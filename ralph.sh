#!/usr/bin/env bash
# ralph.sh — autonomous coding loop for probable-eureka
# Each iteration spawns a fresh Claude Code instance to implement one PRD story.
#
# Usage:
#   ./ralph.sh            # run up to MAX_ITERATIONS times
#   MAX_ITERATIONS=3 ./ralph.sh
set -euo pipefail

MAX_ITERATIONS=${MAX_ITERATIONS:-20}
PRD_FILE="prd.json"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' not found in PATH"; exit 1; }
}

require_cmd jq
require_cmd claude
require_cmd git

if [ ! -f "$PRD_FILE" ]; then
  echo "ERROR: $PRD_FILE not found. Run from the project root."
  exit 1
fi

for i in $(seq 1 "$MAX_ITERATIONS"); do
  incomplete=$(jq '[.stories[] | select(.status == "incomplete")] | length' "$PRD_FILE")

  if [ "$incomplete" -eq 0 ]; then
    echo "=== All stories complete. Done. ==="
    exit 0
  fi

  next_story=$(jq -r '[.stories[] | select(.status == "incomplete")][0].id' "$PRD_FILE")
  echo ""
  echo "=== Iteration $i / $MAX_ITERATIONS — next story: $next_story ($incomplete remaining) ==="

  # Spawn a fresh Claude Code instance.
  # --print runs non-interactively; the agent reads CLAUDE.md, prd.json, progress.txt
  # and follows the instructions there.
  claude --print \
    "You are working on the probable-eureka CNI plugin. Read CLAUDE.md and follow the instructions there exactly. The next incomplete story in prd.json is: $next_story"

  echo ""
  echo "--- Iteration $i complete ---"
done

echo "=== Reached MAX_ITERATIONS=$MAX_ITERATIONS. Check prd.json for remaining stories. ==="
exit 1
