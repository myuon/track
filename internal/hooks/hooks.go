package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/shlex"
	"github.com/myuon/track/internal/store/sqlite"
)

const (
	IssueCreated      = "issue.created"
	IssueUpdated      = "issue.updated"
	IssueStatusChange = "issue.status_changed"
	IssueCompleted    = "issue.completed"
	SyncCompleted     = "sync.completed"
)

var validEvents = map[string]struct{}{
	IssueCreated:      {},
	IssueUpdated:      {},
	IssueStatusChange: {},
	IssueCompleted:    {},
	SyncCompleted:     {},
}

func ValidateEvent(event string) error {
	if _, ok := validEvents[event]; !ok {
		return fmt.Errorf("unknown hook event: %s", event)
	}
	return nil
}

func RunEvent(ctx context.Context, store *sqlite.Store, event, issueID string) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}
	hooks, err := store.ListHooks(ctx, event)
	if err != nil {
		return err
	}
	for _, h := range hooks {
		if err := runOne(ctx, h, event, issueID); err != nil {
			return err
		}
	}
	return nil
}

func runOne(ctx context.Context, h sqlite.Hook, event, issueID string) error {
	parts, err := shlex.Split(h.RunCmd)
	if err != nil {
		return fmt.Errorf("parse hook command: %w", err)
	}
	if len(parts) == 0 {
		return fmt.Errorf("empty hook command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if h.CWD != "" {
		cmd.Dir = h.CWD
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "TRACK_EVENT="+event, "TRACK_ISSUE_ID="+issueID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook(%d) failed: %w", h.ID, err)
	}
	return nil
}
