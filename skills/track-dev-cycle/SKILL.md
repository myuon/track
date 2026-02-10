---
name: track-dev-cycle
description: "Run a full development cycle for `track` issues in two modes: `plan` mode (investigate and clarify scope, then set status to `ready` or attach `Need to discuss`) and execution mode (branch, implement, commit incrementally, open PR, watch CI, fix, and merge). Use when users ask to work specific issue numbers through planning or implementation with `track show` and tracker state updates."
---

# Track Dev Cycle

Follow this skill to process one or more issue IDs end-to-end with `track`.

## Input Contract

- Accept mode plus one or more issue IDs.
- Support both explicit IDs (for example `TRK-12`) and numeric forms (for example `12`).
- If numeric IDs are provided, resolve to canonical IDs before mutation.
- When multiple issues are provided, first enumerate the full target issue list and add it to a TODO list (TodoWrite recommended).
- Process issues strictly one by one from top to bottom of that TODO list.
- After completing one issue, move to the next issue automatically without waiting for additional user approval.

## Mode Selection

- Use `plan` mode when the user asks to refine scope/spec before coding.
- Use execution mode when the user asks to implement and deliver.
- If mode is not explicit, ask one short question before mutating tracker state.

## Plan Mode Workflow

For each issue:

1. Inspect details.
   - Run `track show <issue-id>`.
   - Extract goals, constraints, dependencies, DoD, AC, and verification expectations.

2. Clarify scope until implementation-ready.
   - Gather required repository/context information.
   - Keep digging until there are no implementation-level unknowns and implementation can be fully delegated to the agent.
   - If gaps remain, ask targeted questions to remove ambiguity.
   - If technical selection is required, obtain explicit user approval for the technology to use before marking ready.
   - If AC, DoD, or verification method is ambiguous, confirm and fix those ambiguities in this phase.
   - Do not attach `Need to discuss` just to avoid deeper clarification work.
   - If the user explicitly wants to defer discussion or explicitly gives up on clarifying now, treat it as deferred and proceed with `Need to discuss`.

3. Persist plan spec in issue body.
   - Update `body` to include a concrete Spec that is implementation-ready and testable.
   - Keep existing useful context, but ensure the latest agreed Spec is clearly identifiable (for example, under `## Spec`).
   - If readiness is not reached, still record clarified points, open questions, and assumptions in `body`.

4. Update tracker state based on readiness.
   - If implementation can start immediately with no critical ambiguity, set status to `ready`.
   - If deeper clarification is still needed, attach label `Need to discuss`.

5. Report outcome.
   - Summarize what was clarified, open questions (if any), and the tracker updates performed.

## Execution Mode Workflow

For each issue:

1. Prepare branch.
   - Create a new branch before implementation.
   - Use repository branch conventions when available.

2. Inspect issue and acceptance gates.
   - Run `track show <issue-id>`.
   - Confirm DoD, AC, and verification method. If any are missing, identify assumptions explicitly before coding.

3. Mark active work.
   - Set issue status to `in_progress` (or the repository's equivalent active status).

4. Plan work slices.
   - Convert issue work into an ordered TODO list.
   - Register TODO items with available tooling (for example TodoWrite) and keep progress current.

5. Implement iteratively.
   - Complete TODO items sequentially.
   - Commit frequently after each meaningful completed slice.
   - If `commit` skill is available, use it for commit formatting.

6. Verify completion against issue gates.
   - Re-check DoD, AC, and verification method after implementation.
   - Run relevant tests/checks before opening PR.

7. Open PR and validate CI.
   - Create PR with clear summary and test evidence.
   - Run `gh pr checks --watch` (or equivalent `gh` watch command) to monitor CI.

8. Close delivery loop.
   - Fix CI/review issues and push updates until green.
   - Merge once checks pass and blocking feedback is resolved.

## Tracker Command Patterns

Use these patterns and adapt to local CLI options:

```bash
track show <issue-id>
track set <issue-id> --status ready
track set <issue-id> --status in_progress
track label attach <issue-id> "Need to discuss"
```

## Guardrails

- Never skip `track show` before changing tracker state.
- Keep tracker history auditable with small, explicit updates.
- In `plan` mode, do not finish without updating the issue `body` Spec.
- Do not mark `ready` in plan mode while unresolved critical ambiguities remain.
- Do not merge in execution mode before verification gates are satisfied.
- If checks are intentionally failing due to WIP/red-phase work, commit only with an explicit note in the commit message.
