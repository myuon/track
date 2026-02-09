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
