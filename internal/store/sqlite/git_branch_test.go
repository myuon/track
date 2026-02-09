package sqlite

import (
	"context"
	"testing"
)

func TestUpsertAndDeleteGitBranchLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.UpsertGitBranchLink(ctx, "TRK-1", "feature/a"); err != nil {
		t.Fatalf("UpsertGitBranchLink() error: %v", err)
	}
	link, err := store.GetGitBranchLink(ctx, "TRK-1")
	if err != nil {
		t.Fatalf("GetGitBranchLink() error: %v", err)
	}
	if link.BranchName != "feature/a" {
		t.Fatalf("branch = %q, want feature/a", link.BranchName)
	}

	if err := store.UpsertGitBranchLink(ctx, "TRK-1", "feature/b"); err != nil {
		t.Fatalf("UpsertGitBranchLink() update error: %v", err)
	}
	link, err = store.GetGitBranchLink(ctx, "TRK-1")
	if err != nil {
		t.Fatalf("GetGitBranchLink() after update error: %v", err)
	}
	if link.BranchName != "feature/b" {
		t.Fatalf("branch after update = %q, want feature/b", link.BranchName)
	}

	if err := store.DeleteGitBranchLink(ctx, "TRK-1"); err != nil {
		t.Fatalf("DeleteGitBranchLink() error: %v", err)
	}
	if _, err := store.GetGitBranchLink(ctx, "TRK-1"); err == nil {
		t.Fatalf("GetGitBranchLink() should fail after delete")
	}
}
