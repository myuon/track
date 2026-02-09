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

	if err := store.UpsertGitHubLink(ctx, "I-000001", "123", "owner/repo"); err != nil {
		t.Fatalf("UpsertGitHubLink() error: %v", err)
	}

	link, err := store.GetGitHubLink(ctx, "I-000001")
	if err != nil {
		t.Fatalf("GetGitHubLink() error: %v", err)
	}
	if link.PRRef != "123" || link.Repo != "owner/repo" {
		t.Fatalf("unexpected link: %+v", link)
	}
}
