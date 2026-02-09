package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

func newGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Git integration",
	}
	cmd.AddCommand(newGitLinkBranchCmd())
	cmd.AddCommand(newGitUnlinkBranchCmd())
	return cmd
}

func newGitLinkBranchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link-branch <issue_id>",
		Short: "Link issue to current git branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			if _, err := store.GetIssue(ctx, issueID); err != nil {
				return err
			}

			branch, err := currentGitBranch(ctx)
			if err != nil {
				return err
			}
			if err := store.UpsertGitBranchLink(ctx, issueID, branch); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}

func newGitUnlinkBranchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink-branch <issue_id>",
		Short: "Remove issue-to-branch link",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			if err := store.DeleteGitBranchLink(ctx, issueID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}

func currentGitBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve current git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD: cannot link branch")
	}
	return branch, nil
}

func branchMergeStatus(ctx context.Context, branch string) string {
	defaultBranch, ok := detectDefaultBranch(ctx)
	if !ok {
		return "unknown (run git fetch)"
	}
	if !hasGitRef(ctx, "refs/heads/"+branch) {
		return "unknown (run git fetch)"
	}

	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", branch, defaultBranch)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return "unknown (run git fetch)"
		}
		if exitErr.ExitCode() == 1 {
			return "false"
		}
		return "unknown (run git fetch)"
	}
	return "true"
}

func detectDefaultBranch(ctx context.Context) (string, bool) {
	if hasGitRef(ctx, "refs/heads/main") {
		return "main", true
	}
	if hasGitRef(ctx, "refs/heads/master") {
		return "master", true
	}
	if hasGitRef(ctx, "refs/remotes/origin/main") {
		return "origin/main", true
	}
	if hasGitRef(ctx, "refs/remotes/origin/master") {
		return "origin/master", true
	}
	return "", false
}

func hasGitRef(ctx context.Context, ref string) bool {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}
