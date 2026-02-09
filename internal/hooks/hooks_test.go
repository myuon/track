package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/myuon/track/internal/store/sqlite"
)

func TestRunEventExecutesHook(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	outFile := filepath.Join(tmp, "hook.out")
	cmd := "/bin/sh -c 'echo $TRACK_EVENT:$TRACK_ISSUE_ID >> " + outFile + "'"
	if err := store.AddHook(ctx, IssueCompleted, cmd, ""); err != nil {
		t.Fatalf("add hook: %v", err)
	}

	if err := RunEvent(ctx, store, IssueCompleted, "TRK-1"); err != nil {
		t.Fatalf("run event: %v", err)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read hook output: %v", err)
	}
	if !strings.Contains(string(raw), "issue.completed:TRK-1") {
		t.Fatalf("unexpected hook output: %q", string(raw))
	}
}
