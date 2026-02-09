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
