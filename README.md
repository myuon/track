# Track

Track is a local-first, CLI-first issue tracker.

## Features (current)

- Local SQLite storage (`~/.track/track.db`)
- Issue lifecycle commands:
  - `new`, `list`, `show`, `edit`, `set`
  - `label add/rm`, `next`, `done`, `archive`, `reorder`
- Import/Export:
  - `export --format text|csv|jsonl`
  - `import --format text|csv|jsonl [--dry-run]`
- Hooks:
  - `hook add/list/rm/test`
  - events: `issue.created`, `issue.updated`, `issue.status_changed`, `issue.completed`, `sync.completed`
- GitHub integration (via `gh` CLI):
  - `gh link`, `gh status`, `gh watch`
- Optional local Web UI:
  - `ui --port <port> [--open]`

## Requirements

- Go 1.23+
- (`gh` command only if you use GitHub watch/status features)

## Build

```bash
go build -o track ./cmd/track
```

## Quick Start

```bash
./track new "Add login page" --label ready --priority p1
./track list --status todo --sort priority
./track set I-000001 --status in_progress
./track next I-000001 "Implement UI + validation"
./track done I-000001
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
