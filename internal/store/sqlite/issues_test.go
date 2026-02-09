package sqlite

import (
	"context"
	"testing"

	"github.com/myuon/track/internal/issue"
)

func TestCreateGetUpdateAndListIssues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	created, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Add login page",
		Status:   issue.StatusTodo,
		Priority: "p1",
		Assignee: "alice",
		Due:      "2026-02-10",
		Labels:   []string{"ready", "ui"},
		Body:     "Implement login form",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	got, err := store.GetIssue(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Title != "Add login page" || got.Priority != "p1" {
		t.Fatalf("unexpected issue payload: %+v", got)
	}

	newStatus := issue.StatusInProgress
	newAssignee := "bob"
	updated, err := store.UpdateIssue(ctx, created.ID, UpdateIssueInput{
		Status:   &newStatus,
		Assignee: &newAssignee,
	})
	if err != nil {
		t.Fatalf("UpdateIssue() error: %v", err)
	}
	if updated.Status != issue.StatusInProgress || updated.Assignee != "bob" {
		t.Fatalf("unexpected updated issue: %+v", updated)
	}

	items, err := store.ListIssues(ctx, ListFilter{Status: issue.StatusInProgress})
	if err != nil {
		t.Fatalf("ListIssues(status) error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	items, err = store.ListIssues(ctx, ListFilter{Label: "ready"})
	if err != nil {
		t.Fatalf("ListIssues(label) error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items label) = %d, want 1", len(items))
	}

	items, err = store.ListIssues(ctx, ListFilter{Search: "login"})
	if err != nil {
		t.Fatalf("ListIssues(search) error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items search) = %d, want 1", len(items))
	}
}

func TestListSortByPriority(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, _ = store.CreateIssue(ctx, issue.Item{Title: "p3", Status: issue.StatusTodo, Priority: "p3"})
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "p0", Status: issue.StatusTodo, Priority: "p0"})

	items, err := store.ListIssues(ctx, ListFilter{Sort: "priority"})
	if err != nil {
		t.Fatalf("ListIssues(priority) error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("len(items) = %d, want >=2", len(items))
	}
	if items[0].Priority != "p0" {
		t.Fatalf("first priority = %s, want p0", items[0].Priority)
	}
}

func TestLabelsNextAndReorder(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	a, err := store.CreateIssue(ctx, issue.Item{Title: "A", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue A error: %v", err)
	}
	b, err := store.CreateIssue(ctx, issue.Item{Title: "B", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue B error: %v", err)
	}

	got, err := store.AddLabel(ctx, a.ID, "ready")
	if err != nil {
		t.Fatalf("AddLabel() error: %v", err)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "ready" {
		t.Fatalf("unexpected labels after add: %+v", got.Labels)
	}
	got, err = store.AddLabel(ctx, a.ID, "ready")
	if err != nil {
		t.Fatalf("AddLabel() duplicate error: %v", err)
	}
	if len(got.Labels) != 1 {
		t.Fatalf("duplicate label should not be appended: %+v", got.Labels)
	}

	got, err = store.AddLabel(ctx, a.ID, "backend")
	if err != nil {
		t.Fatalf("AddLabel() second label error: %v", err)
	}
	if len(got.Labels) != 2 {
		t.Fatalf("expected two labels after second add: %+v", got.Labels)
	}

	got, err = store.RemoveLabel(ctx, a.ID, "ready")
	if err != nil {
		t.Fatalf("RemoveLabel() error: %v", err)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "backend" {
		t.Fatalf("labels should keep backend after removing ready: %+v", got.Labels)
	}
	got, err = store.RemoveLabel(ctx, a.ID, "backend")
	if err != nil {
		t.Fatalf("RemoveLabel() backend error: %v", err)
	}
	if len(got.Labels) != 0 {
		t.Fatalf("labels should be empty after removing all: %+v", got.Labels)
	}

	got, err = store.SetNextAction(ctx, a.ID, "Write PR")
	if err != nil {
		t.Fatalf("SetNextAction() error: %v", err)
	}
	if got.NextAction != "Write PR" {
		t.Fatalf("next_action = %q, want %q", got.NextAction, "Write PR")
	}

	if err := store.Reorder(ctx, b.ID, a.ID, ""); err != nil {
		t.Fatalf("Reorder() error: %v", err)
	}
	items, err := store.ListIssues(ctx, ListFilter{Sort: "manual"})
	if err != nil {
		t.Fatalf("ListIssues(manual) error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("len(items) = %d, want >=2", len(items))
	}
	if items[0].ID != b.ID {
		t.Fatalf("first issue = %s, want %s", items[0].ID, b.ID)
	}
}
