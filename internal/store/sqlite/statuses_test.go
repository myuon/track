package sqlite

import (
	"context"
	"testing"

	"github.com/myuon/track/internal/issue"
)

func TestAddListRemoveStatus(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddStatus(ctx, "blocked"); err != nil {
		t.Fatalf("AddStatus() error: %v", err)
	}
	statuses, err := store.ListStatuses(ctx)
	if err != nil {
		t.Fatalf("ListStatuses() error: %v", err)
	}
	found := false
	for _, st := range statuses {
		if st == "blocked" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("blocked should exist in statuses: %+v", statuses)
	}

	if err := store.RemoveStatus(ctx, "blocked"); err != nil {
		t.Fatalf("RemoveStatus() error: %v", err)
	}
	if err := store.ValidateStatus(ctx, "blocked"); err == nil {
		t.Fatalf("blocked should not be valid after removal")
	}
}

func TestRemoveStatusFailsWhenInUse(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.AddStatus(ctx, "blocked"); err != nil {
		t.Fatalf("AddStatus() error: %v", err)
	}
	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    "has custom status",
		Status:   "blocked",
		Priority: "none",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	if err := store.RemoveStatus(ctx, "blocked"); err == nil {
		t.Fatalf("RemoveStatus() should fail for status in use")
	}
}
