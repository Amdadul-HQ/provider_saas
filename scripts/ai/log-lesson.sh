#!/usr/bin/env bash
# Usage: scripts/ai/log-lesson.sh "<area>" "<what broke>" "<root cause>" "<fix>" "<prevention rule>"
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FILE="$ROOT/.ai/errors/LESSONS.md"
[ $# -eq 5 ] || { echo "need 5 args: area, what broke, root cause, fix, prevention rule" >&2; exit 1; }
N=$(grep -c '^## L-' "$FILE" || true); N=$((N+1))
printf '\n## L-%03d · %s · %s\n- What broke: %s\n- Root cause: %s\n- Fix: %s\n- Prevention rule: %s\n' "$N" "$(date +%F)" "$1" "$2" "$3" "$4" "$5" >> "$FILE"
echo "logged L-$(printf %03d $N)"
