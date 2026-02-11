package cli

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

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
	layout := issueListLayoutForItems([]issue.Item{
		{Status: issue.StatusTodo, Priority: "p2"},
		{Status: issue.StatusTodo, Priority: "p2"},
	})
	if lines[0] != formatIssueListRowWithLayout(layout, "ID", "STATUS", "PRIORITY", "TITLE", "LABELS") {
		t.Fatalf("unexpected header: %q", lines[0])
	}

	if lines[1] != formatIssueListRowWithLayout(layout, "TRK-1", "todo", "p2", "With labels", "ready,ui") {
		t.Fatalf("first line unexpected: %q", lines[1])
	}

	if lines[2] != formatIssueListRowWithLayout(layout, "TRK-2", "todo", "p2", "No labels", "") {
		t.Fatalf("second line unexpected: %q", lines[2])
	}
}

func TestListKeepsColumnsFixedWidthForLongValues(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	longTitle := strings.Repeat("title-", 20)
	longLabels := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"}
	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    longTitle,
		Status:   issue.StatusTodo,
		Priority: "p2",
		Labels:   longLabels,
	})
	if err != nil {
		t.Fatalf("CreateIssue(long values) error: %v", err)
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
	if listDisplayWidth(lines[0]) != listDisplayWidth(lines[1]) {
		t.Fatalf("header and row should have equal display width: header=%d row=%d out=%q", listDisplayWidth(lines[0]), listDisplayWidth(lines[1]), out.String())
	}
	if !strings.Contains(lines[1], "...") {
		t.Fatalf("row should include truncated value marker, got: %q", lines[1])
	}
}

func TestFormatIssueListRowMixedWidthLayout(t *testing.T) {
	layout := issueListLayout{
		idWidth:       listIDWidth,
		statusWidth:   listStatusWidth,
		priorityWidth: listPriorityWidth,
		titleWidth:    listTitleWidth,
		labelsWidth:   listLabelsWidth,
	}
	row := formatIssueListRowWithLayout(
		layout,
		"TRK-1",
		"todo",
		"p2",
		"日本語ABC日本語ABC日本語ABC日本語ABC日本語ABC日本語ABC日本語ABC",
		"label,日本語",
	)
	widths := []int{layout.idWidth, layout.statusWidth, layout.priorityWidth, layout.titleWidth, layout.labelsWidth}
	cols, ok := splitListRowColumnsByWidth(row, widths)
	if !ok {
		t.Fatalf("row should match fixed-width layout, row=%q", row)
	}
	if got, want := listDisplayWidth(row), listIDWidth+listStatusWidth+listPriorityWidth+listTitleWidth+listLabelsWidth+4; got != want {
		t.Fatalf("row display width = %d, want %d; row=%q", got, want, row)
	}
	if !strings.HasSuffix(strings.TrimRight(cols[3], " "), "...") {
		t.Fatalf("title column should be truncated with ellipsis: %q", cols[3])
	}
	if !strings.Contains(cols[4], "label") {
		t.Fatalf("labels column should remain visible, got=%q", cols[4])
	}
}

func splitListRowColumnsByWidth(row string, widths []int) ([]string, bool) {
	cols := make([]string, 0, len(widths))
	rest := row
	for i, width := range widths {
		col, next, ok := takeDisplayWidth(rest, width)
		if !ok {
			return nil, false
		}
		cols = append(cols, col)
		rest = next
		if i == len(widths)-1 {
			continue
		}
		if rest == "" || rest[0] != ' ' {
			return nil, false
		}
		rest = rest[1:]
	}
	return cols, rest == ""
}

func takeDisplayWidth(s string, width int) (string, string, bool) {
	if width == 0 {
		return "", s, true
	}
	current := 0
	end := 0
	for i, r := range s {
		rw := listRuneWidth(r)
		if current+rw > width {
			break
		}
		current += rw
		end = i + utf8.RuneLen(r)
		if current == width {
			return s[:end], s[end:], true
		}
	}
	return "", "", false
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
	defaultLayout := issueListLayoutForItems([]issue.Item{
		{Status: issue.StatusTodo, Priority: "p2"},
		{Status: issue.StatusReady, Priority: "p2"},
	})
	if !strings.Contains(out.String(), formatIssueListRowWithLayout(defaultLayout, "ID", "STATUS", "PRIORITY", "TITLE", "LABELS")) {
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
	archivedLayout := issueListLayoutForItems([]issue.Item{
		{Status: issue.StatusArchived, Priority: "p2"},
	})
	if lines[0] != formatIssueListRowWithLayout(archivedLayout, "ID", "STATUS", "PRIORITY", "TITLE", "LABELS") || !strings.Contains(lines[1], archivedIssue.ID) {
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

func TestSetSupportsTitle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "old title",
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
	cmd.SetArgs([]string{it.ID, "--title", "new title"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set command error: %v", err)
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Title != "new title" {
		t.Fatalf("title = %q, want %q", got.Title, "new title")
	}
}

func TestSetRejectsEmptyTitle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "old title",
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
	cmd.SetArgs([]string{it.ID, "--title", ""})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("set command should fail for empty title")
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Title != "old title" {
		t.Fatalf("title = %q, want %q", got.Title, "old title")
	}
}

func TestSetSupportsTitleWithOtherFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "old title",
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
	cmd.SetArgs([]string{it.ID, "--title", "new title", "--next-action", "Write tests"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set command error: %v", err)
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Title != "new title" {
		t.Fatalf("title = %q, want %q", got.Title, "new title")
	}
	if got.NextAction != "Write tests" {
		t.Fatalf("next_action = %q, want %q", got.NextAction, "Write tests")
	}
}

func TestSetAcceptsNumericIssueID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "numeric id",
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
	cmd.SetArgs([]string{strings.TrimPrefix(it.ID, "TRK-"), "--status", issue.StatusReady})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set command error: %v", err)
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Status != issue.StatusReady {
		t.Fatalf("status = %q, want %q", got.Status, issue.StatusReady)
	}
}

func TestSetSupportsProjectAndShowDisplaysProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "project link",
		Status:   issue.StatusTodo,
		Priority: "none",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}
	if _, err := store.CreateProject(ctx, "cli-refresh", "CLI Refresh", ""); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}

	setCmd := newSetCmd()
	var out bytes.Buffer
	setCmd.SetOut(&out)
	setCmd.SetErr(&out)
	setCmd.SetArgs([]string{it.ID, "--project", "cli-refresh"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set --project error: %v", err)
	}

	showCmd := newShowCmd()
	out.Reset()
	showCmd.SetOut(&out)
	showCmd.SetErr(&out)
	showCmd.SetArgs([]string{it.ID})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("show error: %v", err)
	}
	if !strings.Contains(out.String(), "project: cli-refresh\n") {
		t.Fatalf("show should include project, got: %q", out.String())
	}

	setCmd = newSetCmd()
	out.Reset()
	setCmd.SetOut(&out)
	setCmd.SetErr(&out)
	setCmd.SetArgs([]string{it.ID, "--project", "none"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("set --project none error: %v", err)
	}

	showCmd = newShowCmd()
	out.Reset()
	showCmd.SetOut(&out)
	showCmd.SetErr(&out)
	showCmd.SetArgs([]string{it.ID})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("show after unlink error: %v", err)
	}
	if strings.Contains(out.String(), "project: ") {
		t.Fatalf("show should omit project after unlink, got: %q", out.String())
	}
}

func TestListSupportsProjectFilter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.CreateProject(ctx, "cli-refresh", "CLI Refresh", ""); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	inProject, err := store.CreateIssue(ctx, issue.Item{Title: "in", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue(in) error: %v", err)
	}
	outProject, err := store.CreateIssue(ctx, issue.Item{Title: "out", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue(out) error: %v", err)
	}
	if err := store.SetIssueProject(ctx, inProject.ID, "cli-refresh"); err != nil {
		t.Fatalf("SetIssueProject() error: %v", err)
	}

	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--project", "cli-refresh", "--sort", "manual"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --project error: %v", err)
	}
	if !strings.Contains(out.String(), inProject.ID) {
		t.Fatalf("list should include linked issue, got: %q", out.String())
	}
	if strings.Contains(out.String(), outProject.ID) {
		t.Fatalf("list should exclude non-linked issue, got: %q", out.String())
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
	layout := issueListLayoutForItems([]issue.Item{
		{Status: "blocked", Priority: "none"},
	})
	if !strings.Contains(out.String(), formatIssueListRowWithLayout(layout, it.ID, "blocked", "none", "blocked issue", "")) {
		t.Fatalf("list should include custom status issue: %q", out.String())
	}
}

func TestListShowsFullStatusAndPriorityWithAlignedColumns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	longStatus := "custom_status_that_is_very_long"
	if err := store.AddStatus(ctx, longStatus); err != nil {
		t.Fatalf("AddStatus() error: %v", err)
	}

	_, err = store.CreateIssue(ctx, issue.Item{
		Title:    strings.Repeat("title-", 20),
		Status:   longStatus,
		Priority: "none",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", longStatus, "--sort", "manual"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2; out=%q", len(lines), out.String())
	}

	layout := issueListLayoutForItems([]issue.Item{
		{Status: longStatus, Priority: "none"},
	})
	if lines[0] != formatIssueListRowWithLayout(layout, "ID", "STATUS", "PRIORITY", "TITLE", "LABELS") {
		t.Fatalf("unexpected header: %q", lines[0])
	}

	widths := []int{layout.idWidth, layout.statusWidth, layout.priorityWidth, layout.titleWidth, layout.labelsWidth}
	cols, ok := splitListRowColumnsByWidth(lines[1], widths)
	if !ok {
		t.Fatalf("row should match layout widths, row=%q", lines[1])
	}
	if got := strings.TrimRight(cols[1], " "); got != longStatus {
		t.Fatalf("status should not be truncated: got=%q want=%q", got, longStatus)
	}
	if got := strings.TrimRight(cols[2], " "); got != "none" {
		t.Fatalf("priority should remain full text: got=%q", got)
	}
	if strings.Contains(cols[1], "...") || strings.Contains(cols[2], "...") {
		t.Fatalf("status/priority should not include ellipsis: row=%q", lines[1])
	}
	if !strings.HasSuffix(strings.TrimRight(cols[3], " "), "...") {
		t.Fatalf("title should preserve truncation behavior: %q", cols[3])
	}
	if listDisplayWidth(lines[0]) != listDisplayWidth(lines[1]) {
		t.Fatalf("header and row should have equal display width: header=%d row=%d", listDisplayWidth(lines[0]), listDisplayWidth(lines[1]))
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

	first, err := store.CreateIssue(ctx, issue.Item{Title: "first", Status: issue.StatusTodo, Priority: "none", Assignee: "agent"})
	if err != nil {
		t.Fatalf("CreateIssue(first) error: %v", err)
	}
	second, err := store.CreateIssue(ctx, issue.Item{Title: "second", Status: issue.StatusTodo, Priority: "none", Assignee: "agent"})
	if err != nil {
		t.Fatalf("CreateIssue(second) error: %v", err)
	}
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "not-assigned", Status: issue.StatusTodo, Priority: "none", Assignee: "user"})
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

func TestPlanningDefaultTargetsOnlyTodoAssignedToAgent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	agentTodo, err := store.CreateIssue(ctx, issue.Item{Title: "agent", Status: issue.StatusTodo, Priority: "none", Assignee: "agent"})
	if err != nil {
		t.Fatalf("CreateIssue(agentTodo) error: %v", err)
	}
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "user", Status: issue.StatusTodo, Priority: "none", Assignee: "user"})
	_, _ = store.CreateIssue(ctx, issue.Item{Title: "unassigned", Status: issue.StatusTodo, Priority: "none"})

	origRunner := planningSessionRunner
	t.Cleanup(func() { planningSessionRunner = origRunner })
	called := make([]string, 0, 1)
	planningSessionRunner = func(_ context.Context, _ *cobra.Command, issueID string) error {
		called = append(called, issueID)
		return nil
	}

	cmd := newPlanningCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("planning command error: %v", err)
	}

	if !slices.Equal(called, []string{agentTodo.ID}) {
		t.Fatalf("planned issues = %v, want [%s]", called, agentTodo.ID)
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

func TestReplyAppendsBodyAndAssignsBackToAgentWithFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "questioned",
		Status:   issue.StatusTodo,
		Priority: "none",
		Assignee: "user",
		Body:     "## Questions for user\n- Q1",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newReplyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID, "--message", "A1: use sqlite"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reply command error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "ok" {
		t.Fatalf("unexpected output: %q", out.String())
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Assignee != "agent" {
		t.Fatalf("assignee = %q, want agent", got.Assignee)
	}
	if !strings.Contains(got.Body, "## User replies\n- A1: use sqlite\n") {
		t.Fatalf("body missing reply section: %q", got.Body)
	}
}

func TestReplyReadsMessageFromStdin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "questioned",
		Status:   issue.StatusTodo,
		Priority: "none",
		Assignee: "user",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newReplyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("回答です\n"))
	cmd.SetArgs([]string{it.ID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reply command error: %v", err)
	}

	got, err := store.GetIssue(ctx, it.ID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Assignee != "agent" {
		t.Fatalf("assignee = %q, want agent", got.Assignee)
	}
	if !strings.Contains(got.Body, "- 回答です\n") {
		t.Fatalf("body missing reply entry: %q", got.Body)
	}
}
