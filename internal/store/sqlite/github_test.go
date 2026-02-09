package sqlite

import (
	"context"
	"testing"
)

func TestUpsertAndGetGitHubLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.UpsertGitHubLink(ctx, "TRK-1", "123", "owner/repo"); err != nil {
		t.Fatalf("UpsertGitHubLink() error: %v", err)
	}

	link, err := store.GetGitHubLink(ctx, "TRK-1")
	if err != nil {
		t.Fatalf("GetGitHubLink() error: %v", err)
	}
	if link.PRRef != "123" || link.Repo != "owner/repo" {
		t.Fatalf("unexpected link: %+v", link)
	}
}
