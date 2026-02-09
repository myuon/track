---
name: track-issue-flow
description: Operate the local-first `track` issue workflow from request intake to completion using the CLI. Use this when users ask to list issues, pick a numbered item from `track list`, inspect details with `track show`, update status/priority/labels, or move an issue into active work while keeping progress visible in the tracker.
---

# Track Issue Flow

Follow this workflow when handling issue work in repositories that use the `track` CLI.

## Standard Workflow

1. Identify candidate issues.
   - Run `track list`.
   - If the user refers to an item by list position (for example, "5"), map it to the exact issue ID in the current output before acting.

2. Confirm issue scope.
   - Run `track show <issue-id>`.
   - Extract status, priority, labels, and notes to understand expectations and missing details.

3. Start execution with explicit state changes.
   - When work begins, set status to active (`in_progress`) using `track set`.
   - Keep user-facing updates aligned with tracker state.

4. Implement and validate.
   - Make code/doc changes required by the issue.
   - Run project checks relevant to changed files.

5. Close the loop in the tracker.
   - Update labels/priority/status as needed.
   - Mark done only after the requested implementation and validation are complete.

## Command Reference

Read `references/track-cli-reference.md` for command patterns used by this skill.

## Guardrails

- Resolve IDs from fresh `track list` output; do not assume stale ordering.
- Prefer concrete IDs (`TRK-xx`) for mutations.
- Keep tracker updates small and explicit so history remains auditable.
- If issue details are unclear, capture assumptions in progress updates before making irreversible changes.
