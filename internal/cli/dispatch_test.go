package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

type dispatchExpectedCommand struct {
	interactive bool
	dir         string
	name        string
	args        []string
	output      string
	err         error
	after       func(t *testing.T)
}

type fakeDispatchRunner struct {
	t        *testing.T
	expected []dispatchExpectedCommand
	index    int
}

func (f *fakeDispatchRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	f.t.Helper()
	_ = ctx
	exp := f.next()
	if exp.interactive {
		f.t.Fatalf("expected interactive command at index %d, got non-interactive", f.index-1)
	}
	f.assertCommand(exp, dir, name, args)
	if exp.after != nil {
		exp.after(f.t)
	}
	return exp.output, exp.err
}

func (f *fakeDispatchRunner) RunInteractive(ctx context.Context, dir string, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	f.t.Helper()
	_ = ctx
	_ = stdin
	_ = stdout
	_ = stderr
	exp := f.next()
	if !exp.interactive {
		f.t.Fatalf("expected non-interactive command at index %d, got interactive", f.index-1)
	}
	f.assertCommand(exp, dir, name, args)
	if exp.after != nil {
		exp.after(f.t)
	}
	return exp.err
}

func (f *fakeDispatchRunner) next() dispatchExpectedCommand {
	f.t.Helper()
	if f.index >= len(f.expected) {
		f.t.Fatalf("unexpected extra command at index %d", f.index)
	}
	exp := f.expected[f.index]
	f.index++
	return exp
}

func (f *fakeDispatchRunner) assertDone() {
	f.t.Helper()
	if f.index != len(f.expected) {
		f.t.Fatalf("not all expected commands consumed: got=%d want=%d", f.index, len(f.expected))
	}
}

func (f *fakeDispatchRunner) assertCommand(exp dispatchExpectedCommand, dir, name string, args []string) {
	f.t.Helper()
	if exp.dir != dir {
		f.t.Fatalf("dir mismatch: got=%q want=%q", dir, exp.dir)
	}
	if exp.name != name {
		f.t.Fatalf("name mismatch: got=%q want=%q", name, exp.name)
	}
	if !reflect.DeepEqual(exp.args, args) {
		f.t.Fatalf("args mismatch:\n got=%v\nwant=%v", args, exp.args)
	}
}

func TestDispatchSuccessSetsDoneAndRunsSequence(t *testing.T) {
	ctx, store, repoRoot, issueID, issueTitle := setupDispatchTest(t)
	worktreeDir := filepath.Join(repoRoot, ".worktree", "trk-1")
	runnerScript := filepath.Join(worktreeDir, "exec_codex")

	var out bytes.Buffer
	runner := &fakeDispatchRunner{
		t: t,
		expected: []dispatchExpectedCommand{
			{dir: repoRoot, name: "git", args: []string{"rev-parse", "--show-toplevel"}, output: repoRoot},
			{dir: repoRoot, name: "git", args: []string{"show-ref", "--verify", "--quiet", "refs/heads/codex/trk-1"}, err: errors.New("not found")},
			{dir: repoRoot, name: "git", args: []string{"worktree", "add", "-b", "codex/trk-1", worktreeDir, "main"}, after: func(t *testing.T) {
				if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
					t.Fatalf("MkdirAll() error: %v", err)
				}
				if err := os.WriteFile(runnerScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatalf("WriteFile() error: %v", err)
				}
			}},
			{interactive: true, dir: worktreeDir, name: runnerScript, args: []string{"--sandbox", "danger-full-access", "execution", issueID}},
			{dir: worktreeDir, name: "git", args: []string{"status", "--porcelain"}, output: ""},
			{dir: worktreeDir, name: "git", args: []string{"rev-list", "--count", "main..HEAD"}, output: "1\n"},
			{dir: worktreeDir, name: "git", args: []string{"push", "-u", "origin", "codex/trk-1"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "list", "--head", "codex/trk-1", "--state", "open", "--json", "number"}, output: "[]"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "create", "--head", "codex/trk-1", "--base", "main", "--title", issueID + ": " + issueTitle, "--body", "## Summary\n- Automated by `track dispatch`\n\nCloses " + issueID + "\n"}, output: "https://github.com/myuon/track/pull/42"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "checks", "42", "--watch"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "merge", "42", "--merge", "--delete-branch"}, output: ""},
		},
	}

	err := runDispatch(
		ctx,
		store,
		&out,
		repoRoot,
		issueID,
		dispatchOptions{Runner: "codex", Mode: "execution", Base: "main", MergeMethod: "merge"},
		runner,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("runDispatch() error: %v", err)
	}
	runner.assertDone()

	got, err := store.GetIssue(ctx, issueID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Status != issue.StatusDone {
		t.Fatalf("status = %q, want %q", got.Status, issue.StatusDone)
	}

	statuses, err := store.ListStatuses(ctx)
	if err != nil {
		t.Fatalf("ListStatuses() error: %v", err)
	}
	if !slices.Contains(statuses, dispatchFinishedStatus) {
		t.Fatalf("status list should include %q, got=%v", dispatchFinishedStatus, statuses)
	}
}

func TestDispatchNoMergeLeavesFinished(t *testing.T) {
	ctx, store, repoRoot, issueID, issueTitle := setupDispatchTest(t)
	worktreeDir := filepath.Join(repoRoot, ".worktree", "trk-1")
	runnerScript := filepath.Join(worktreeDir, "exec_codex")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(runnerScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var out bytes.Buffer
	runner := &fakeDispatchRunner{
		t: t,
		expected: []dispatchExpectedCommand{
			{dir: repoRoot, name: "git", args: []string{"rev-parse", "--show-toplevel"}, output: repoRoot},
			{interactive: true, dir: worktreeDir, name: runnerScript, args: []string{"--sandbox", "danger-full-access", "execution", issueID}},
			{dir: worktreeDir, name: "git", args: []string{"status", "--porcelain"}, output: ""},
			{dir: worktreeDir, name: "git", args: []string{"rev-list", "--count", "main..HEAD"}, output: "1"},
			{dir: worktreeDir, name: "git", args: []string{"push", "-u", "origin", "codex/trk-1"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "list", "--head", "codex/trk-1", "--state", "open", "--json", "number"}, output: "[]"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "create", "--head", "codex/trk-1", "--base", "main", "--title", issueID + ": " + issueTitle, "--body", "## Summary\n- Automated by `track dispatch`\n\nCloses " + issueID + "\n"}, output: "42"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "checks", "42", "--watch"}, output: ""},
		},
	}

	err := runDispatch(
		ctx,
		store,
		&out,
		repoRoot,
		issueID,
		dispatchOptions{Runner: "codex", Mode: "execution", Base: "main", MergeMethod: "merge", NoMerge: true},
		runner,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("runDispatch() error: %v", err)
	}
	runner.assertDone()

	got, err := store.GetIssue(ctx, issueID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Status != dispatchFinishedStatus {
		t.Fatalf("status = %q, want %q", got.Status, dispatchFinishedStatus)
	}
}

func TestDispatchCIFailureStopsAtFinished(t *testing.T) {
	ctx, store, repoRoot, issueID, issueTitle := setupDispatchTest(t)
	worktreeDir := filepath.Join(repoRoot, ".worktree", "trk-1")
	runnerScript := filepath.Join(worktreeDir, "exec_codex")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(runnerScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var out bytes.Buffer
	runner := &fakeDispatchRunner{
		t: t,
		expected: []dispatchExpectedCommand{
			{dir: repoRoot, name: "git", args: []string{"rev-parse", "--show-toplevel"}, output: repoRoot},
			{interactive: true, dir: worktreeDir, name: runnerScript, args: []string{"--sandbox", "danger-full-access", "execution", issueID}},
			{dir: worktreeDir, name: "git", args: []string{"status", "--porcelain"}, output: ""},
			{dir: worktreeDir, name: "git", args: []string{"rev-list", "--count", "main..HEAD"}, output: "1"},
			{dir: worktreeDir, name: "git", args: []string{"push", "-u", "origin", "codex/trk-1"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "list", "--head", "codex/trk-1", "--state", "open", "--json", "number"}, output: "[]"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "create", "--head", "codex/trk-1", "--base", "main", "--title", issueID + ": " + issueTitle, "--body", "## Summary\n- Automated by `track dispatch`\n\nCloses " + issueID + "\n"}, output: "42"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "checks", "42", "--watch"}, err: errors.New("ci failed")},
		},
	}

	err := runDispatch(
		ctx,
		store,
		&out,
		repoRoot,
		issueID,
		dispatchOptions{Runner: "codex", Mode: "execution", Base: "main", MergeMethod: "merge"},
		runner,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatalf("runDispatch() expected error, got nil")
	}
	runner.assertDone()

	got, err := store.GetIssue(ctx, issueID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Status != dispatchFinishedStatus {
		t.Fatalf("status = %q, want %q", got.Status, dispatchFinishedStatus)
	}

	if !strings.Contains(out.String(), "failed step 8 (watch CI checks)") {
		t.Fatalf("output should include failed step, got: %q", out.String())
	}
}

func TestDispatchMergeMethodSquash(t *testing.T) {
	ctx, store, repoRoot, issueID, issueTitle := setupDispatchTest(t)
	worktreeDir := filepath.Join(repoRoot, ".worktree", "trk-1")
	runnerScript := filepath.Join(worktreeDir, "exec_codex")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(runnerScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	runner := &fakeDispatchRunner{
		t: t,
		expected: []dispatchExpectedCommand{
			{dir: repoRoot, name: "git", args: []string{"rev-parse", "--show-toplevel"}, output: repoRoot},
			{interactive: true, dir: worktreeDir, name: runnerScript, args: []string{"--sandbox", "danger-full-access", "execution", issueID}},
			{dir: worktreeDir, name: "git", args: []string{"status", "--porcelain"}, output: ""},
			{dir: worktreeDir, name: "git", args: []string{"rev-list", "--count", "main..HEAD"}, output: "1"},
			{dir: worktreeDir, name: "git", args: []string{"push", "-u", "origin", "codex/trk-1"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "list", "--head", "codex/trk-1", "--state", "open", "--json", "number"}, output: "[]"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "create", "--head", "codex/trk-1", "--base", "main", "--title", issueID + ": " + issueTitle, "--body", "## Summary\n- Automated by `track dispatch`\n\nCloses " + issueID + "\n"}, output: "42"},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "checks", "42", "--watch"}, output: ""},
			{dir: worktreeDir, name: "gh", args: []string{"pr", "merge", "42", "--squash", "--delete-branch"}, output: ""},
		},
	}

	err := runDispatch(
		ctx,
		store,
		io.Discard,
		repoRoot,
		issueID,
		dispatchOptions{Runner: "codex", Mode: "execution", Base: "main", MergeMethod: "squash"},
		runner,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("runDispatch() error: %v", err)
	}
	runner.assertDone()
}

func TestDispatchNoChangesSkipsPRAndKeepsInProgress(t *testing.T) {
	ctx, store, repoRoot, issueID, _ := setupDispatchTest(t)
	worktreeDir := filepath.Join(repoRoot, ".worktree", "trk-1")
	runnerScript := filepath.Join(worktreeDir, "exec_codex")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(runnerScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	var out bytes.Buffer
	runner := &fakeDispatchRunner{
		t: t,
		expected: []dispatchExpectedCommand{
			{dir: repoRoot, name: "git", args: []string{"rev-parse", "--show-toplevel"}, output: repoRoot},
			{interactive: true, dir: worktreeDir, name: runnerScript, args: []string{"--sandbox", "danger-full-access", "execution", issueID}},
			{dir: worktreeDir, name: "git", args: []string{"status", "--porcelain"}, output: ""},
			{dir: worktreeDir, name: "git", args: []string{"rev-list", "--count", "main..HEAD"}, output: "0"},
		},
	}

	err := runDispatch(
		ctx,
		store,
		&out,
		repoRoot,
		issueID,
		dispatchOptions{Runner: "codex", Mode: "execution", Base: "main", MergeMethod: "merge"},
		runner,
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("runDispatch() error: %v", err)
	}
	runner.assertDone()

	got, err := store.GetIssue(ctx, issueID)
	if err != nil {
		t.Fatalf("GetIssue() error: %v", err)
	}
	if got.Status != issue.StatusInProgress {
		t.Fatalf("status = %q, want %q", got.Status, issue.StatusInProgress)
	}
	if !strings.Contains(out.String(), "no changes detected; skipping commit/push/pr") {
		t.Fatalf("output should mention no-change summary, got: %q", out.String())
	}
}

func setupDispatchTest(t *testing.T) (context.Context, *sqlite.Store, string, string, string) {
	t.Helper()

	trackHome := t.TempDir()
	t.Setenv("TRACK_HOME", trackHome)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	created, err := store.CreateIssue(ctx, issue.Item{
		Title:    "dispatch spec implementation",
		Status:   issue.StatusTodo,
		Priority: "p2",
		Assignee: "agent",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error: %v", err)
	}

	repoRoot := t.TempDir()
	return ctx, store, repoRoot, created.ID, created.Title
}
