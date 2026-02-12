package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	appconfig "github.com/myuon/track/internal/config"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestNormalizePRRef(t *testing.T) {
	cases := map[string]string{
		"123":                               "123",
		" https://github.com/a/b/pull/456 ": "456",
		"owner/repo#789":                    "789",
	}
	for in, want := range cases {
		if got := normalizePRRef(in); got != want {
			t.Fatalf("normalizePRRef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRunAndJobFromCheckLink(t *testing.T) {
	runID, jobID, ok := parseRunAndJobFromCheckLink("https://github.com/a/b/actions/runs/123456/job/9876")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if runID != "123456" || jobID != "9876" {
		t.Fatalf("unexpected ids: run=%s job=%s", runID, jobID)
	}
}

func TestSummarizeChecks(t *testing.T) {
	cases := []struct {
		name   string
		checks []ghCheck
		want   string
	}{
		{name: "success", checks: []ghCheck{{State: "success"}}, want: "success"},
		{name: "pending", checks: []ghCheck{{State: "in_progress"}}, want: "pending"},
		{name: "failure", checks: []ghCheck{{State: "success"}, {State: "failure"}}, want: "failure"},
	}
	for _, tc := range cases {
		if got := summarizeChecks(tc.checks); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestTailLines(t *testing.T) {
	raw := "1\n2\n3\n4\n5\n"
	got := tailLines(raw, 2)
	if got != "4\n5" {
		t.Fatalf("tailLines() = %q, want %q", got, "4\n5")
	}
}

func TestGHIssueCreateDryRunShowsPayloadWithoutExternalExecution(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Implement feature",
		Status:   issue.StatusInProgress,
		Priority: "p1",
		Labels:   []string{"gh", "integration"},
		Body:     "Spec body",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newGHIssueCreateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID, "--repo", "owner/repo", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "title: Implement feature\n") {
		t.Fatalf("dry-run output should include title, got: %q", got)
	}
	if !strings.Contains(got, "repo: owner/repo\n") {
		t.Fatalf("dry-run output should include repo, got: %q", got)
	}
	if !strings.Contains(got, "labels: gh,integration\n") {
		t.Fatalf("dry-run output should include labels, got: %q", got)
	}
	if !strings.Contains(got, "track id: "+it.ID+"\n") {
		t.Fatalf("dry-run output should include generated body metadata, got: %q", got)
	}
}

func TestGHIssueCreateSuccessStoresGitHubIssueLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)
	ghArgsFile := filepath.Join(tmp, "gh-args.txt")
	setupFakeGHForTest(t, tmp, ghArgsFile)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Create GitHub issue",
		Status:   issue.StatusReady,
		Priority: "p2",
		Labels:   []string{"gh", "integration"},
		Body:     "Body text",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	t.Setenv("GH_ARGS_FILE", ghArgsFile)
	t.Setenv("GH_STDOUT", `{"number":42,"url":"https://github.com/owner/repo/issues/42"}`)
	t.Setenv("GH_STDERR", "")
	t.Setenv("GH_EXIT_CODE", "0")

	cmd := newGHIssueCreateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{it.ID, "--repo", "owner/repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	link, err := store.GetGitHubIssueLink(ctx, it.ID)
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

	args := readArgsFile(t, ghArgsFile)
	wantArgs := []string{"issue", "create", "--title", "Create GitHub issue", "--repo", "owner/repo", "--label", "gh", "--label", "integration", "--json", "number,url"}
	for _, want := range wantArgs {
		if !slices.Contains(args, want) {
			t.Fatalf("gh args should include %q, got=%v", want, args)
		}
	}
}

func TestGHIssueCreateFailureDoesNotPersistLink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)
	ghArgsFile := filepath.Join(tmp, "gh-args.txt")
	setupFakeGHForTest(t, tmp, ghArgsFile)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Create GitHub issue",
		Status:   issue.StatusReady,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	t.Setenv("GH_ARGS_FILE", ghArgsFile)
	t.Setenv("GH_STDOUT", "")
	t.Setenv("GH_STDERR", "label not found")
	t.Setenv("GH_EXIT_CODE", "1")

	cmd := newGHIssueCreateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{it.ID, "--repo", "owner/repo"})

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("cmd.Execute() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "label not found") {
		t.Fatalf("error should include gh stderr, got: %v", err)
	}

	if _, err := store.GetGitHubIssueLink(ctx, it.ID); err == nil {
		t.Fatalf("GetGitHubIssueLink() should fail when create fails")
	}
}

func TestGHIssueCreateUsesConfigRepoWhenFlagMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)
	ghArgsFile := filepath.Join(tmp, "gh-args.txt")
	setupFakeGHForTest(t, tmp, ghArgsFile)

	if err := appconfig.Save(appconfig.Config{UIPort: 8787, GHRepo: "cfg/owner-repo"}); err != nil {
		t.Fatalf("config save error: %v", err)
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Configured repo",
		Status:   issue.StatusReady,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	t.Setenv("GH_ARGS_FILE", ghArgsFile)
	t.Setenv("GH_STDOUT", `{"number":43,"url":"https://github.com/cfg/owner-repo/issues/43"}`)
	t.Setenv("GH_STDERR", "")
	t.Setenv("GH_EXIT_CODE", "0")

	cmd := newGHIssueCreateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{it.ID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}

	args := readArgsFile(t, ghArgsFile)
	if !slices.Contains(args, "cfg/owner-repo") {
		t.Fatalf("gh args should include config repo, got=%v", args)
	}
}

func TestGHIssueCreateRequiresGHCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)
	t.Setenv("PATH", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{
		Title:    "Need gh binary",
		Status:   issue.StatusTodo,
		Priority: "p2",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	cmd := newGHIssueCreateCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{it.ID})

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when gh command is missing")
	}
	if !strings.Contains(err.Error(), "gh command is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setupFakeGHForTest(t *testing.T, tmp, argsFile string) {
	t.Helper()
	ghPath := filepath.Join(tmp, "gh")
	script := `#!/bin/sh
if [ -n "$GH_ARGS_FILE" ]; then
  : > "$GH_ARGS_FILE"
  for arg in "$@"; do
    printf '%s\n' "$arg" >> "$GH_ARGS_FILE"
  done
fi
if [ -n "$GH_STDERR" ]; then
  printf '%s\n' "$GH_STDERR" 1>&2
fi
if [ -n "$GH_STDOUT" ]; then
  printf '%s\n' "$GH_STDOUT"
fi
exit "${GH_EXIT_CODE:-0}"
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(gh) error: %v", err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
	t.Setenv("GH_ARGS_FILE", argsFile)
}

func readArgsFile(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}
