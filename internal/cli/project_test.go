package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestProjectAddListShowRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	addCmd := newProjectCmd()
	var out bytes.Buffer
	addCmd.SetOut(&out)
	addCmd.SetErr(&out)
	addCmd.SetArgs([]string{"add", "cli-refresh", "--name", "CLI Refresh", "--description", "refresh"})
	if err := addCmd.Execute(); err != nil {
		t.Fatalf("project add error: %v", err)
	}

	listCmd := newProjectCmd()
	out.Reset()
	listCmd.SetOut(&out)
	listCmd.SetErr(&out)
	listCmd.SetArgs([]string{"list"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("project list error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "cli-refresh\tCLI Refresh\t0" {
		t.Fatalf("unexpected list output: %q", out.String())
	}

	showCmd := newProjectCmd()
	out.Reset()
	showCmd.SetOut(&out)
	showCmd.SetErr(&out)
	showCmd.SetArgs([]string{"show", "cli-refresh"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("project show error: %v", err)
	}
	if !strings.Contains(out.String(), "key: cli-refresh\n") || !strings.Contains(out.String(), "issue_count: 0\n") {
		t.Fatalf("unexpected show output: %q", out.String())
	}

	rmCmd := newProjectCmd()
	out.Reset()
	rmCmd.SetOut(&out)
	rmCmd.SetErr(&out)
	rmCmd.SetArgs([]string{"rm", "cli-refresh"})
	if err := rmCmd.Execute(); err != nil {
		t.Fatalf("project rm error: %v", err)
	}
}

func TestProjectRemoveRequiresForceWhenLinked(t *testing.T) {
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
	it, err := store.CreateIssue(ctx, issue.Item{Title: "a", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}
	if err := store.SetIssueProject(ctx, it.ID, "cli-refresh"); err != nil {
		t.Fatalf("SetIssueProject() error: %v", err)
	}

	rmCmd := newProjectCmd()
	var out bytes.Buffer
	rmCmd.SetOut(&out)
	rmCmd.SetErr(&out)
	rmCmd.SetArgs([]string{"rm", "cli-refresh"})
	if err := rmCmd.Execute(); err == nil {
		t.Fatalf("project rm should fail without --force")
	}

	rmForceCmd := newProjectCmd()
	out.Reset()
	rmForceCmd.SetOut(&out)
	rmForceCmd.SetErr(&out)
	rmForceCmd.SetArgs([]string{"rm", "cli-refresh", "--force"})
	if err := rmForceCmd.Execute(); err != nil {
		t.Fatalf("project rm --force error: %v", err)
	}
}
