package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var prNumberRe = regexp.MustCompile(`([0-9]+)$`)

type ghPRState struct {
	State    string  `json:"state"`
	MergedAt *string `json:"mergedAt"`
}

func newGitHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh",
		Short: "GitHub integration",
	}
	cmd.AddCommand(newGHLinkCmd())
	cmd.AddCommand(newGHStatusCmd())
	cmd.AddCommand(newGHWatchCmd())
	return cmd
}

func newGHLinkCmd() *cobra.Command {
	var prRef string
	var repo string

	cmd := &cobra.Command{
		Use:   "link <issue_id>",
		Short: "Link issue with PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prRef == "" {
				return fmt.Errorf("--pr is required")
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if _, err := store.GetIssue(ctx, args[0]); err != nil {
				return err
			}
			if err := store.UpsertGitHubLink(ctx, args[0], normalizePRRef(prRef), repo); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&prRef, "pr", "", "PR number or URL")
	cmd.Flags().StringVar(&repo, "repo", "", "owner/name")
	return cmd
}

func newGHStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <issue_id>",
		Short: "Show linked PR status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			link, err := store.GetGitHubLink(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "issue: %s\npr: %s\nrepo: %s\n", link.IssueID, link.PRRef, link.Repo)
			return nil
		},
	}
}

func newGHWatchCmd() *cobra.Command {
	var repo string
	var interval string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch PR states and update linked issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh command is required")
			}

			dur, err := time.ParseDuration(interval)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			ctx := cmd.Context()
			if err := runGHWatchOnce(ctx, repo, cmd.OutOrStdout()); err != nil {
				return err
			}

			ticker := time.NewTicker(dur)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := runGHWatchOnce(ctx, repo, cmd.OutOrStdout()); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "watch error: %v\n", err)
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "owner/name")
	cmd.Flags().StringVar(&interval, "interval", "30s", "Polling interval")
	return cmd
}

func runGHWatchOnce(ctx context.Context, repo string, out io.Writer) error {
	store, err := sqlite.Open(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	links, err := store.ListGitHubLinks(ctx, "")
	if err != nil {
		return err
	}

	for _, link := range links {
		merged, err := fetchPRMerged(ctx, link, repo)
		if err != nil {
			return err
		}
		if !merged {
			continue
		}
		status := issue.StatusDone
		updated, err := store.UpdateIssue(ctx, link.IssueID, sqlite.UpdateIssueInput{Status: &status})
		if err != nil {
			return err
		}
		if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
			return err
		}
		if err := hooks.RunEvent(ctx, store, hooks.IssueStatusChange, updated.ID); err != nil {
			return err
		}
		if err := hooks.RunEvent(ctx, store, hooks.IssueCompleted, updated.ID); err != nil {
			return err
		}
		fmt.Fprintf(out, "updated %s -> done (pr %s)\n", link.IssueID, link.PRRef)
	}
	return nil
}

func fetchPRMerged(ctx context.Context, link sqlite.GitHubLink, repoOverride string) (bool, error) {
	pr := normalizePRRef(link.PRRef)
	args := []string{"pr", "view", pr, "--json", "state,mergedAt"}
	repo := link.Repo
	if repoOverride != "" {
		repo = repoOverride
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	raw, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("gh pr view failed for %s: %w", pr, err)
	}

	var state ghPRState
	if err := json.Unmarshal(raw, &state); err != nil {
		return false, fmt.Errorf("decode gh response: %w", err)
	}
	if state.MergedAt != nil {
		return true, nil
	}
	return strings.EqualFold(state.State, "MERGED"), nil
}

func normalizePRRef(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	m := prNumberRe.FindStringSubmatch(v)
	if len(m) == 2 {
		return m[1]
	}
	return v
}
