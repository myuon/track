package cli

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
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
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3; out=%q", len(lines), out.String())
	}
	if lines[0] != formatIssueListRow("ID", "STATUS", "PRIORITY", "TITLE", "LABELS") {
		t.Fatalf("unexpected header: %q", lines[0])
	}

	if lines[1] != formatIssueListRow("TRK-1", "todo", "p2", "With labels", "ready,ui") {
		t.Fatalf("first line unexpected: %q", lines[1])
	}

	if lines[2] != formatIssueListRow("TRK-2", "todo", "p2", "No labels", "") {
		t.Fatalf("second line unexpected: %q", lines[2])
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

func TestListExcludesDoneAndArchivedByDefaultAndSupportsStatusFilters(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	todoIssue, _ := store.CreateIssue(ctx, issue.Item{Title: "todo", Status: issue.StatusTodo, Priority: "p2"})
	readyIssue, _ := store.CreateIssue(ctx, issue.Item{Title: "ready", Status: issue.StatusReady, Priority: "p2"})
	doneIssue, _ := store.CreateIssue(ctx, issue.Item{Title: "finished", Status: issue.StatusDone, Priority: "p2"})
	archivedIssue, _ := store.CreateIssue(ctx, issue.Item{Title: "archived", Status: issue.StatusArchived, Priority: "p2"})

	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list default error: %v", err)
	}
	if !strings.Contains(out.String(), formatIssueListRow("ID", "STATUS", "PRIORITY", "TITLE", "LABELS")) {
		t.Fatalf("default list should include header, got: %q", out.String())
	}
	if !strings.Contains(out.String(), todoIssue.ID) || !strings.Contains(out.String(), readyIssue.ID) {
		t.Fatalf("default list should include non-done issues, got: %q", out.String())
	}
	if strings.Contains(out.String(), doneIssue.ID) {
		t.Fatalf("default list should exclude done, got: %q", out.String())
	}
	if strings.Contains(out.String(), archivedIssue.ID) {
		t.Fatalf("default list should exclude archived, got: %q", out.String())
	}

	out.Reset()
	cmd = newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", "archived"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --status archived error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("--status archived should include header + one row: %q", out.String())
	}
	if lines[0] != formatIssueListRow("ID", "STATUS", "PRIORITY", "TITLE", "LABELS") || !strings.Contains(lines[1], archivedIssue.ID) {
		t.Fatalf("--status archived should include only archived: %q", out.String())
	}

	out.Reset()
	cmd = newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", "todo,in_progress"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list multi status error: %v", err)
	}
	if strings.Contains(out.String(), doneIssue.ID) {
		t.Fatalf("multi status should exclude done: %q", out.String())
	}
	if strings.Contains(out.String(), archivedIssue.ID) {
		t.Fatalf("multi status should exclude archived: %q", out.String())
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
	if !strings.Contains(out.String(), formatIssueListRow(it.ID, "blocked", "none", "blocked issue", "")) {
		t.Fatalf("list should include custom status issue: %q", out.String())
	}
}

func TestPlanningInteractiveUpdatesAndSkipsWithLimit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	first, err := store.CreateIssue(ctx, issue.Item{Title: "first", Status: issue.StatusTodo, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue(first) error: %v", err)
	}
	second, err := store.CreateIssue(ctx, issue.Item{Title: "second", Status: issue.StatusTodo, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue(second) error: %v", err)
	}
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "in progress", Status: issue.StatusInProgress, Priority: "none"})

	origRunner := planningSessionRunner
	t.Cleanup(func() { planningSessionRunner = origRunner })
	called := make([]string, 0, 2)
	plannedReady := map[string]bool{first.ID: true}
	planningSessionRunner = func(_ context.Context, _ *cobra.Command, issueID string) error {
		called = append(called, issueID)
		if plannedReady[issueID] {
			status := issue.StatusReady
			if _, err := store.UpdateIssue(ctx, issueID, sqlite.UpdateIssueInput{Status: &status}); err != nil {
				return err
			}
		}
		return nil
	}

	cmd := newPlanningCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--limit", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("planning command error: %v", err)
	}
	if !slices.Equal(called, []string{first.ID, second.ID}) {
		t.Fatalf("planned issues = %v, want [%s %s]", called, first.ID, second.ID)
	}

	updatedFirst, err := store.GetIssue(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetIssue(first) error: %v", err)
	}
	if updatedFirst.Status != issue.StatusReady {
		t.Fatalf("first status = %q, want %q", updatedFirst.Status, issue.StatusReady)
	}

	updatedSecond, err := store.GetIssue(ctx, second.ID)
	if err != nil {
		t.Fatalf("GetIssue(second) error: %v", err)
	}
	if updatedSecond.Status != issue.StatusTodo {
		t.Fatalf("second status = %q, want %q", updatedSecond.Status, issue.StatusTodo)
	}

	if !strings.Contains(out.String(), "updated: 1, skipped: 1\n") {
		t.Fatalf("summary mismatch: %q", out.String())
	}
}

func TestPlanningByIDTargetsOnlySpecifiedIssue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	first, err := store.CreateIssue(ctx, issue.Item{Title: "first", Status: issue.StatusTodo, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue(first) error: %v", err)
	}
	second, err := store.CreateIssue(ctx, issue.Item{Title: "second", Status: issue.StatusTodo, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue(second) error: %v", err)
	}

	origRunner := planningSessionRunner
	t.Cleanup(func() { planningSessionRunner = origRunner })
	called := make([]string, 0, 1)
	planningSessionRunner = func(_ context.Context, _ *cobra.Command, issueID string) error {
		called = append(called, issueID)
		status := issue.StatusReady
		_, err := store.UpdateIssue(ctx, issueID, sqlite.UpdateIssueInput{Status: &status})
		return err
	}

	cmd := newPlanningCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{second.ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("planning by id error: %v", err)
	}
	if !slices.Equal(called, []string{second.ID}) {
		t.Fatalf("planned issues = %v, want [%s]", called, second.ID)
	}

	gotFirst, err := store.GetIssue(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetIssue(first) error: %v", err)
	}
	if gotFirst.Status != issue.StatusTodo {
		t.Fatalf("first status = %q, want %q", gotFirst.Status, issue.StatusTodo)
	}

	gotSecond, err := store.GetIssue(ctx, second.ID)
	if err != nil {
		t.Fatalf("GetIssue(second) error: %v", err)
	}
	if gotSecond.Status != issue.StatusReady {
		t.Fatalf("second status = %q, want %q", gotSecond.Status, issue.StatusReady)
	}
}

func TestPlanningSkipsNonTodoIssue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{Title: "ready issue", Status: issue.StatusReady, Priority: "none"})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	origRunner := planningSessionRunner
	t.Cleanup(func() { planningSessionRunner = origRunner })
	called := false
	planningSessionRunner = func(_ context.Context, _ *cobra.Command, _ string) error {
		called = true
		return nil
	}

	cmd := newPlanningCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("planning non-todo error: %v", err)
	}
	if called {
		t.Fatalf("planning session should not run for non-todo issue")
	}
	if !strings.Contains(out.String(), it.ID+" skipped (status="+issue.StatusReady+")\n") {
		t.Fatalf("expected status skip message, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "updated: 0, skipped: 1\n") {
		t.Fatalf("expected summary, got: %q", out.String())
	}
}
