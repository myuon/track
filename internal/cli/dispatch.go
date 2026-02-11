package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

const dispatchFinishedStatus = "finished"

type dispatchOptions struct {
	Runner      string
	Mode        string
	Base        string
	MergeMethod string
	NoMerge     bool
}

type dispatchCommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
	RunInteractive(ctx context.Context, dir string, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error
}

type realDispatchCommandRunner struct{}

func (r realDispatchCommandRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	raw, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(raw))
	if err != nil {
		if out == "" {
			return "", fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
		}
		return out, fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), out)
	}
	return out, nil
}

func (r realDispatchCommandRunner) RunInteractive(ctx context.Context, dir string, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func newDispatchCmd() *cobra.Command {
	var opts dispatchOptions

	cmd := &cobra.Command{
		Use:   "dispatch <issue_id>",
		Short: "Run issue implementation cycle from worktree to PR merge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Runner != "codex" && opts.Runner != "claude" {
				return fmt.Errorf("invalid --runner: %s", opts.Runner)
			}
			if opts.Mode != "execution" && opts.Mode != "plan" {
				return fmt.Errorf("invalid --mode: %s", opts.Mode)
			}
			if opts.MergeMethod != "merge" && opts.MergeMethod != "squash" && opts.MergeMethod != "rebase" {
				return fmt.Errorf("invalid --merge-method: %s", opts.MergeMethod)
			}

			ctx := cmd.Context()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			issueID := normalizeIssueIDArg(args[0])
			return runDispatch(ctx, store, cmd.OutOrStdout(), cwd, issueID, opts, realDispatchCommandRunner{}, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&opts.Runner, "runner", "codex", "Runner (codex|claude)")
	cmd.Flags().StringVar(&opts.Mode, "mode", "execution", "Mode (execution|plan)")
	cmd.Flags().StringVar(&opts.Base, "base", "main", "Base branch")
	cmd.Flags().StringVar(&opts.MergeMethod, "merge-method", "merge", "Merge method (merge|squash|rebase)")
	cmd.Flags().BoolVar(&opts.NoMerge, "no-merge", false, "Skip merge after CI success")

	return cmd
}

func runDispatch(
	ctx context.Context,
	store *sqlite.Store,
	out io.Writer,
	cwd string,
	issueID string,
	opts dispatchOptions,
	runner dispatchCommandRunner,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	var (
		it          issue.Item
		repoRoot    string
		worktreeDir string
		branch      string
		prRef       string
		hasDirty    bool
		hasCommits  bool
	)

	stepIndex := 0
	runStep := func(name string, fn func() error) error {
		stepIndex++
		fmt.Fprintf(out, "step %d: %s\n", stepIndex, name)
		if err := fn(); err != nil {
			fmt.Fprintf(out, "failed step %d (%s): %v\n", stepIndex, name, err)
			return err
		}
		return nil
	}

	if err := runStep("prepare issue status", func() error {
		var err error
		it, err = store.GetIssue(ctx, issueID)
		if err != nil {
			return err
		}
		if err := updateIssueStatus(ctx, store, issueID, issue.StatusInProgress); err != nil {
			return err
		}
		if err := ensureFinishedStatus(ctx, store); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	if err := runStep("prepare worktree", func() error {
		root, err := runner.Run(ctx, cwd, "git", "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		repoRoot = strings.TrimSpace(root)
		if repoRoot == "" {
			return fmt.Errorf("resolve repository root")
		}

		slug := issueSlug(issueID)
		branch = "codex/" + slug
		worktreeDir = filepath.Join(repoRoot, ".worktree", slug)
		if err := os.MkdirAll(filepath.Join(repoRoot, ".worktree"), 0o755); err != nil {
			return err
		}

		info, err := os.Stat(worktreeDir)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("worktree path exists and is not a directory: %s", worktreeDir)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}

		_, branchErr := runner.Run(ctx, repoRoot, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if branchErr == nil {
			_, err = runner.Run(ctx, repoRoot, "git", "worktree", "add", worktreeDir, branch)
			return err
		}
		_, err = runner.Run(ctx, repoRoot, "git", "worktree", "add", "-b", branch, worktreeDir, opts.Base)
		return err
	}); err != nil {
		return err
	}

	if err := runStep("run implementation runner", func() error {
		runnerCmd := filepath.Join(worktreeDir, "exec_"+opts.Runner)
		if _, err := os.Stat(runnerCmd); err != nil {
			runnerCmd = "exec_" + opts.Runner
		}
		return runner.RunInteractive(
			ctx,
			worktreeDir,
			stdin,
			stdout,
			stderr,
			runnerCmd,
			"--sandbox",
			"danger-full-access",
			opts.Mode,
			issueID,
		)
	}); err != nil {
		return err
	}

	if err := runStep("detect changes", func() error {
		statusOut, err := runner.Run(ctx, worktreeDir, "git", "status", "--porcelain")
		if err != nil {
			return err
		}
		hasDirty = strings.TrimSpace(statusOut) != ""

		revOut, err := runner.Run(ctx, worktreeDir, "git", "rev-list", "--count", fmt.Sprintf("%s..HEAD", opts.Base))
		if err != nil {
			return err
		}
		count, err := strconv.Atoi(strings.TrimSpace(revOut))
		if err != nil {
			return fmt.Errorf("parse ahead commit count: %w", err)
		}
		hasCommits = count > 0
		return nil
	}); err != nil {
		return err
	}

	if !hasDirty && !hasCommits {
		fmt.Fprintln(out, "no changes detected; skipping commit/push/pr")
		return nil
	}

	if err := runStep("commit and push", func() error {
		if hasDirty {
			if _, err := runner.Run(ctx, worktreeDir, "git", "add", "-A"); err != nil {
				return err
			}
			commitMsg := fmt.Sprintf("chore: apply %s via track dispatch", issueID)
			if _, err := runner.Run(ctx, worktreeDir, "git", "commit", "-m", commitMsg); err != nil {
				return err
			}
		}
		_, err := runner.Run(ctx, worktreeDir, "git", "push", "-u", "origin", branch)
		return err
	}); err != nil {
		return err
	}

	if err := runStep("create or reuse PR", func() error {
		ref, err := ensureDispatchPR(ctx, runner, worktreeDir, branch, opts.Base, issueID, it.Title)
		if err != nil {
			return err
		}
		prRef = ref
		return nil
	}); err != nil {
		return err
	}

	if err := runStep("mark issue finished", func() error {
		return updateIssueStatus(ctx, store, issueID, dispatchFinishedStatus)
	}); err != nil {
		return err
	}

	if err := runStep("watch CI checks", func() error {
		_, err := runner.Run(ctx, worktreeDir, "gh", "pr", "checks", prRef, "--watch")
		return err
	}); err != nil {
		return err
	}

	if opts.NoMerge {
		fmt.Fprintln(out, "dispatch complete with --no-merge (issue status: finished)")
		return nil
	}

	if err := runStep("merge PR", func() error {
		methodFlag := "--merge"
		if opts.MergeMethod == "squash" {
			methodFlag = "--squash"
		}
		if opts.MergeMethod == "rebase" {
			methodFlag = "--rebase"
		}
		_, err := runner.Run(ctx, worktreeDir, "gh", "pr", "merge", prRef, methodFlag, "--delete-branch")
		return err
	}); err != nil {
		return err
	}

	if err := runStep("mark issue done", func() error {
		return updateIssueStatus(ctx, store, issueID, issue.StatusDone)
	}); err != nil {
		return err
	}

	fmt.Fprintf(out, "dispatch complete: %s -> done\n", issueID)
	return nil
}

func ensureFinishedStatus(ctx context.Context, store *sqlite.Store) error {
	if err := store.AddStatus(ctx, dispatchFinishedStatus); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "status already exists") {
			return nil
		}
		return err
	}
	return nil
}

func updateIssueStatus(ctx context.Context, store *sqlite.Store, issueID, status string) error {
	updated, err := store.UpdateIssue(ctx, issueID, sqlite.UpdateIssueInput{Status: &status})
	if err != nil {
		return err
	}
	if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
		return err
	}
	if err := hooks.RunEvent(ctx, store, hooks.IssueStatusChange, updated.ID); err != nil {
		return err
	}
	if status == issue.StatusDone {
		if err := hooks.RunEvent(ctx, store, hooks.IssueCompleted, updated.ID); err != nil {
			return err
		}
	}
	return nil
}

type ghPRListItem struct {
	Number int `json:"number"`
}

func ensureDispatchPR(ctx context.Context, runner dispatchCommandRunner, worktreeDir, branch, base, issueID, issueTitle string) (string, error) {
	listOut, err := runner.Run(ctx, worktreeDir, "gh", "pr", "list", "--head", branch, "--state", "open", "--json", "number")
	if err != nil {
		return "", err
	}

	var prs []ghPRListItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(listOut)), &prs); err != nil {
		return "", fmt.Errorf("decode gh pr list output: %w", err)
	}
	if len(prs) > 0 {
		return strconv.Itoa(prs[0].Number), nil
	}

	title := fmt.Sprintf("%s: %s", issueID, issueTitle)
	body := fmt.Sprintf("## Summary\n- Automated by `track dispatch`\n\nCloses %s\n", issueID)
	createOut, err := runner.Run(
		ctx,
		worktreeDir,
		"gh",
		"pr",
		"create",
		"--head",
		branch,
		"--base",
		base,
		"--title",
		title,
		"--body",
		body,
	)
	if err != nil {
		return "", err
	}
	prRef := normalizePRRef(createOut)
	if prRef == "" {
		return "", fmt.Errorf("failed to resolve PR number from gh pr create output")
	}
	return prRef, nil
}

func issueSlug(issueID string) string {
	raw := strings.ToLower(strings.TrimSpace(issueID))
	var b strings.Builder
	prevDash := false
	for _, r := range raw {
		isAlpha := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isAllowed := r == '.' || r == '_' || r == '-'
		if isAlpha || isDigit || isAllowed {
			if r == '-' {
				if prevDash {
					continue
				}
				prevDash = true
			} else {
				prevDash = false
			}
			b.WriteRune(r)
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "issue"
	}
	return slug
}
