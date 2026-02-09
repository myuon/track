# Track

Track is a local-first, CLI-first issue tracker.

## Features (current)

- Local SQLite storage (`~/.track/track.db`)
- Issue lifecycle commands:
  - `new`, `list`, `show`, `edit`, `set`
  - `status add/list/remove` (custom status management)
  - `label attach/detach` (and backward-compatible `label add/rm`)
  - `next`, `done`, `archive`, `reorder`
- Import/Export:
  - `export --format text|csv|jsonl`
  - `import --format text|csv|jsonl [--dry-run]`
- Hooks:
  - `hook add/list/rm/test`
  - events: `issue.created`, `issue.updated`, `issue.status_changed`, `issue.completed`, `sync.completed`
- GitHub integration (via `gh` CLI):
  - `gh link`, `gh status`, `gh watch`, `gh auto-merge`
- Optional local Web UI:
  - `ui --port <port> [--open]`

## Requirements

- Go 1.24+
- (`gh` command only if you use GitHub watch/status features)

## Build

```bash
go build -o track ./cmd/track
```

## Install

For local development checkout:

```bash
go install ./cmd/track
```

For a published module:

```bash
go install github.com/myuon/track/cmd/track@latest
```

For stable usage, pin a version tag:

```bash
go install github.com/myuon/track/cmd/track@v0.1.0
```

If `track` is not found after install, add Go bin to your `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Quick Start

```bash
./track new "Add login page" --label ready --label backend --priority p1
./track list --status todo --sort priority
./track set TRK-1 --status in_progress
./track set TRK-1 --next-action "Implement UI + validation"
./track label attach TRK-1 blocked needs-refine
./track label detach TRK-1 blocked
./track status add blocked
./track next
./track done TRK-1
```

## Agent Skill

This repository includes agent skills for operating `track` workflows:

- `skills/track-issue-flow`
  - Purpose: help an agent resolve `track` CLI usage and move issues safely from discovery to status updates.
- `skills/track-dev-cycle`
  - Purpose: run issue work end-to-end in plan/execution modes with tracker state updates.

### Install the skill

Install into your Codex skills directory:

```bash
mkdir -p "$CODEX_HOME/skills"
cp -R skills/track-issue-flow "$CODEX_HOME/skills/"
cp -R skills/track-dev-cycle "$CODEX_HOME/skills/"
```

### Use the skill

Typical flow with the skill:

1. Run `track list` and map a requested list position to an issue ID.
2. Run `track show <issue-id>` to confirm scope and labels.
3. Apply updates with explicit ID-based commands such as:
   - `track set <issue-id> --status in_progress`
   - `track set <issue-id> --priority p2`
   - `track set <issue-id> --next-action "<text>"`
   - `track done <issue-id>`

Example for a request like "5番を進めたい":

```bash
track list
track show TRK-5
track set TRK-5 --status in_progress
track set TRK-5 --next-action "Implement first slice"
```

## Config

Track stores config at `~/.track/config.toml`.

```bash
./track config get ui_port
./track config set ui_port 8787
./track config set open_browser true
```

For testing or isolated runs, set `TRACK_HOME`:

```bash
export TRACK_HOME=$(mktemp -d)
./track version
```

## Web UI

```bash
./track ui --port 8787 --open
```

## Hooks example

```bash
./track hook add issue.completed --run "/bin/sh -c 'echo done:$TRACK_ISSUE_ID'"
```

## Development

```bash
go test ./...
```

## CI and PR Monitoring

GitHub Actions runs tests automatically on:

- `pull_request`
- `push` to `main`

After opening a PR, you can link it to an issue and monitor with `track gh`:

```bash
track gh link TRK-15 --pr <PR number or URL> --repo <owner/name>
track gh status TRK-15
track gh watch --repo <owner/name>
```
