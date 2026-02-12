#!/bin/sh
set -eu

NEXT_ACTION_DEFAULT="planning: refine spec and decide ready/user"

if [ -z "${TRACK_ISSUE_ID:-}" ]; then
  echo "error: TRACK_ISSUE_ID is required" >&2
  exit 2
fi

TRACK_BIN="${TRACK_BIN:-track}"

if ! SHOW_OUTPUT="$("$TRACK_BIN" show "$TRACK_ISSUE_ID" 2>&1)"; then
  echo "error: track show failed for $TRACK_ISSUE_ID: $SHOW_OUTPUT" >&2
  exit 3
fi

STATUS="$(printf '%s\n' "$SHOW_OUTPUT" | sed -n 's/^status:[[:space:]]*//p' | head -n1)"
if [ -z "$STATUS" ]; then
  echo "error: could not parse status from track show output for $TRACK_ISSUE_ID" >&2
  exit 4
fi

if [ "$STATUS" != "todo" ]; then
  echo "skip: $TRACK_ISSUE_ID status=$STATUS (target is todo only)" >&2
  exit 0
fi

HAS_ASSIGNEE=0
if printf '%s\n' "$SHOW_OUTPUT" | grep -q '^assignee:[[:space:]]*'; then
  HAS_ASSIGNEE=1
fi

HAS_NEXT_ACTION=0
if printf '%s\n' "$SHOW_OUTPUT" | grep -q '^next_action:[[:space:]]*'; then
  HAS_NEXT_ACTION=1
fi

if [ "$HAS_ASSIGNEE" -eq 1 ] && [ "$HAS_NEXT_ACTION" -eq 1 ]; then
  echo "skip: $TRACK_ISSUE_ID assignee/next_action already set" >&2
  exit 0
fi

if [ "$HAS_ASSIGNEE" -eq 0 ] && [ "$HAS_NEXT_ACTION" -eq 0 ]; then
  if ! "$TRACK_BIN" set "$TRACK_ISSUE_ID" --assignee "agent" --next-action "$NEXT_ACTION_DEFAULT"; then
    echo "error: failed to set assignee and next_action for $TRACK_ISSUE_ID" >&2
    exit 5
  fi
  exit 0
fi

if [ "$HAS_ASSIGNEE" -eq 0 ]; then
  if ! "$TRACK_BIN" set "$TRACK_ISSUE_ID" --assignee "agent"; then
    echo "error: failed to set assignee for $TRACK_ISSUE_ID" >&2
    exit 5
  fi
  exit 0
fi

if ! "$TRACK_BIN" set "$TRACK_ISSUE_ID" --next-action "$NEXT_ACTION_DEFAULT"; then
  echo "error: failed to set next_action for $TRACK_ISSUE_ID" >&2
  exit 5
fi

