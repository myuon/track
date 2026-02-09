package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestListIncludesLabelsColumn(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    "With labels",
		Status:   issue.StatusTodo,
		Priority: "p2",
		Labels:   []string{"ready", "ui"},
	})
	if err != nil {
		t.Fatalf("CreateIssue(with labels) error: %v", err)
	}
	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    "No labels",
		Status:   issue.StatusTodo,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue(no labels) error: %v", err)
	}

	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--sort", "manual"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2; out=%q", len(lines), out.String())
	}

	first := strings.Split(lines[0], "\t")
	if len(first) != 5 {
		t.Fatalf("first columns = %d, want 5; line=%q", len(first), lines[0])
	}
	if first[0] != "TRK-1" || first[4] != "ready,ui" {
		t.Fatalf("first line unexpected: %q", lines[0])
	}

	second := strings.Split(lines[1], "\t")
	if len(second) != 5 {
		t.Fatalf("second columns = %d, want 5; line=%q", len(second), lines[1])
	}
	if second[0] != "TRK-2" || second[4] != "" {
		t.Fatalf("second line unexpected: %q", lines[1])
	}
}

func TestShowAcceptsNumericIssueID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    "Numeric ID lookup",
		Status:   issue.StatusTodo,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newShowCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), "id: TRK-1\n") {
		t.Fatalf("output should contain normalized ID, got: %q", out.String())
	}
}

func TestListExcludesDoneByDefaultAndSupportsMultiStatus(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, _ = store.CreateIssue(ctx, issue.Item{Title: "todo", Status: issue.StatusTodo, Priority: "p2"})
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "ready", Status: issue.StatusReady, Priority: "p2"})
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "done", Status: issue.StatusDone, Priority: "p2"})

	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list default error: %v", err)
	}
	if strings.Contains(out.String(), "\tdone\t") {
		t.Fatalf("default list should exclude done, got: %q", out.String())
	}

	out.Reset()
	cmd = newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", "done"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --status done error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "\tdone\t") {
		t.Fatalf("--status done should include only done: %q", out.String())
	}

	out.Reset()
	cmd = newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", "todo,in_progress"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list multi status error: %v", err)
	}
	if strings.Contains(out.String(), "\tdone\t") {
		t.Fatalf("multi status should exclude done: %q", out.String())
	}
}

func TestShowIncludesLinkedBranch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "branch",
		Status:   issue.StatusTodo,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}
	if err := store.UpsertGitBranchLink(ctx, it.ID, "main"); err != nil {
		t.Fatalf("UpsertGitBranchLink() error: %v", err)
	}

	cmd := newShowCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show error: %v", err)
	}
	if !strings.Contains(out.String(), "branch: main\n") {
		t.Fatalf("show should print linked branch: %q", out.String())
	}
	if !strings.Contains(out.String(), "merged: ") {
		t.Fatalf("show should print merge status: %q", out.String())
	}
}

func TestNewDefaultsPriorityToNone(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"default priority"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("new command error: %v", err)
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	got, err := store.GetIssue(ctx, strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Priority != "none" {
		t.Fatalf("priority = %q, want none", got.Priority)
	}
}

func TestSetSupportsNextAction(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "next action",
		Status:   issue.StatusTodo,
		Priority: "none",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newSetCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID, "--next-action", "Write tests"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set command error: %v", err)
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.NextAction != "Write tests" {
		t.Fatalf("next_action = %q, want %q", got.NextAction, "Write tests")
	}
}

func TestNextShowsTopActionableIssue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	a, err := store.CreateIssue(ctx, issue.Item{Title: "A", Status: issue.StatusInProgress, Priority: "p1"})
	if err != nil {
		t.Fatalf("CreateIssue() A error: %v", err)
	}
	_ = a
	b, err := store.CreateIssue(ctx, issue.Item{Title: "B", Status: issue.StatusTodo, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue() B error: %v", err)
	}

	cmd := newNextCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("next command error: %v", err)
	}

	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, b.ID+"\t") {
		t.Fatalf("expected first actionable issue %s, got: %q", b.ID, line)
	}
}

func TestNextShowsNoActionableIssues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	_, _ = store.CreateIssue(ctx, issue.Item{Title: "done", Status: issue.StatusDone, Priority: "p1"})
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "archived", Status: issue.StatusArchived, Priority: "p1"})

	cmd := newNextCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("next command error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "no actionable issues" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestStatusCommandAndCustomStatusFlow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	statusAdd := newStatusCmd()
	var out bytes.Buffer
	statusAdd.SetOut(&out)
	statusAdd.SetErr(&out)
	statusAdd.SetArgs([]string{"add", "blocked"})
	if err := statusAdd.Execute(); err != nil {
		t.Fatalf("status add error: %v", err)
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "blocked issue",
		Status:   issue.StatusTodo,
		Priority: "none",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	setCmd := newSetCmd()
	out.Reset()
	setCmd.SetOut(&out)
	setCmd.SetErr(&out)
	setCmd.SetArgs([]string{it.ID, "--status", "blocked"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set custom status error: %v", err)
	}

	listCmd := newListCmd()
	out.Reset()
	listCmd.SetOut(&out)
	listCmd.SetErr(&out)
	listCmd.SetArgs([]string{"--status", "blocked"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("list custom status error: %v", err)
	}
	if !strings.Contains(out.String(), "\tblocked\t") {
		t.Fatalf("list should include custom status issue: %q", out.String())
	}
}
