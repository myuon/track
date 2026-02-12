package sqlite

import (
	"context"
	"testing"
)

func TestUpsertAndGetGitHubIssueLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.UpsertGitHubIssueLink(ctx, "TRK-1", "42", "https://github.com/owner/repo/issues/42", "owner/repo"); err != nil {
		t.Fatalf("UpsertGitHubIssueLink() error: %v", err)
	}

	link, err := store.GetGitHubIssueLink(ctx, "TRK-1")
	if err != nil {
		t.Fatalf("GetGitHubIssueLink() error: %v", err)
	}
	if link.GHIssueNumber != "42" {
		t.Fatalf("GHIssueNumber = %q, want %q", link.GHIssueNumber, "42")
	}
	if link.GHIssueURL != "https://github.com/owner/repo/issues/42" {
		t.Fatalf("GHIssueURL = %q", link.GHIssueURL)
	}
	if link.Repo != "owner/repo" {
		t.Fatalf("Repo = %q, want %q", link.Repo, "owner/repo")
	}
}

func TestUpsertGitHubIssueLinkOverwritesExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.UpsertGitHubIssueLink(ctx, "TRK-1", "42", "https://github.com/owner/repo/issues/42", "owner/repo"); err != nil {
		t.Fatalf("initial UpsertGitHubIssueLink() error: %v", err)
	}
	if err := store.UpsertGitHubIssueLink(ctx, "TRK-1", "43", "https://github.com/owner/repo/issues/43", "owner/repo"); err != nil {
		t.Fatalf("second UpsertGitHubIssueLink() error: %v", err)
	}

	link, err := store.GetGitHubIssueLink(ctx, "TRK-1")
	if err != nil {
		t.Fatalf("GetGitHubIssueLink() error: %v", err)
	}
	if link.GHIssueNumber != "43" {
		t.Fatalf("GHIssueNumber = %q, want %q", link.GHIssueNumber, "43")
	}
	if link.GHIssueURL != "https://github.com/owner/repo/issues/43" {
		t.Fatalf("GHIssueURL = %q", link.GHIssueURL)
	}
}
