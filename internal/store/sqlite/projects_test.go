package sqlite

import (
	"context"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
)

func TestProjectCRUDAndIssueLinking(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, err := store.CreateProject(ctx, "cli-refresh", "CLI Refresh", "refresh CLI UX")
	if err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	if project.Key != "cli-refresh" {
		t.Fatalf("project key = %q, want cli-refresh", project.Key)
	}

	issueItem, err := store.CreateIssue(ctx, issue.Item{Title: "a", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}
	if err := store.SetIssueProject(ctx, issueItem.ID, "cli-refresh"); err != nil {
		t.Fatalf("SetIssueProject(link) error: %v", err)
	}

	project, err = store.GetProject(ctx, "cli-refresh")
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if project.IssueCount != 1 {
		t.Fatalf("IssueCount = %d, want 1", project.IssueCount)
	}

	key, err := store.GetIssueProject(ctx, issueItem.ID)
	if err != nil {
		t.Fatalf("GetIssueProject() error: %v", err)
	}
	if key != "cli-refresh" {
		t.Fatalf("project key = %q, want cli-refresh", key)
	}

	items, err := store.ListIssues(ctx, ListFilter{Project: "cli-refresh"})
	if err != nil {
		t.Fatalf("ListIssues(project) error: %v", err)
	}
	if len(items) != 1 || items[0].ID != issueItem.ID {
		t.Fatalf("unexpected project-filtered items: %+v", items)
	}

	if err := store.SetIssueProject(ctx, issueItem.ID, ""); err != nil {
		t.Fatalf("SetIssueProject(unlink) error: %v", err)
	}
	key, err = store.GetIssueProject(ctx, issueItem.ID)
	if err != nil {
		t.Fatalf("GetIssueProject() after unlink error: %v", err)
	}
	if key != "" {
		t.Fatalf("project key after unlink = %q, want empty", key)
	}
}

func TestDeleteProjectGuardAndForce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.CreateProject(ctx, "cli-refresh", "CLI Refresh", ""); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	issueItem, err := store.CreateIssue(ctx, issue.Item{Title: "a", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}
	if err := store.SetIssueProject(ctx, issueItem.ID, "cli-refresh"); err != nil {
		t.Fatalf("SetIssueProject() error: %v", err)
	}

	err = store.DeleteProject(ctx, "cli-refresh", false)
	if err == nil || !strings.Contains(err.Error(), "use --force") {
		t.Fatalf("DeleteProject() should require force, err=%v", err)
	}

	if err := store.DeleteProject(ctx, "cli-refresh", true); err != nil {
		t.Fatalf("DeleteProject(force) error: %v", err)
	}

	if _, err := store.GetProject(ctx, "cli-refresh"); err == nil {
		t.Fatalf("GetProject() should fail after delete")
	}
	key, err := store.GetIssueProject(ctx, issueItem.ID)
	if err != nil {
		t.Fatalf("GetIssueProject() after project delete error: %v", err)
	}
	if key != "" {
		t.Fatalf("project key after delete = %q, want empty", key)
	}
}

func TestCreateProjectValidatesKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.CreateProject(ctx, "A", "name", ""); err == nil {
		t.Fatalf("CreateProject() should reject invalid key")
	}
	if _, err := store.CreateProject(ctx, "ok-key", "name", ""); err != nil {
		t.Fatalf("CreateProject(valid key) error: %v", err)
	}
	if _, err := store.CreateProject(ctx, "ok-key", "name2", ""); err == nil {
		t.Fatalf("CreateProject(duplicate key) should fail")
	}
}
