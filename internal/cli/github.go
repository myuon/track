package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	appconfig "github.com/myuon/track/internal/config"
	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var prNumberRe = regexp.MustCompile(`([0-9]+)$`)
var ghRunLinkRe = regexp.MustCompile(`actions/runs/([0-9]+)(?:/job/([0-9]+))?`)

type ghPRState struct {
	State    string  `json:"state"`
	MergedAt *string `json:"mergedAt"`
}

type ghCheck struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Link  string `json:"link"`
}

type ghIssueCreateResponse struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

func newGitHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh",
		Short: "GitHub integration",
	}
	cmd.AddCommand(newGHIssueCmd())
	cmd.AddCommand(newGHLinkCmd())
	cmd.AddCommand(newGHStatusCmd())
	cmd.AddCommand(newGHWatchCmd())
	cmd.AddCommand(newGHAutoMergeCmd())
	return cmd
}

func newGHIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "GitHub issue integration",
	}
	cmd.AddCommand(newGHIssueCreateCmd())
	return cmd
}

func newGHIssueCreateCmd() *cobra.Command {
	var repoOverride string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "create <issue_id>",
		Short: "Create GitHub issue from track issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			it, err := store.GetIssue(ctx, issueID)
			if err != nil {
				return err
			}

			resolvedRepo, err := resolveGitHubRepo(repoOverride)
			if err != nil {
				return err
			}
			body := formatGitHubIssueBody(it)

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "issue: %s\n", issueID)
				fmt.Fprintf(cmd.OutOrStdout(), "title: %s\n", it.Title)
				fmt.Fprintf(cmd.OutOrStdout(), "repo: %s\n", resolvedRepo)
				fmt.Fprintf(cmd.OutOrStdout(), "labels: %s\n", strings.Join(it.Labels, ","))
				fmt.Fprintln(cmd.OutOrStdout(), "body:")
				fmt.Fprintln(cmd.OutOrStdout(), body)
				return nil
			}
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh command is required")
			}

			argsCreate := []string{"issue", "create", "--title", it.Title, "--body", body}
			if resolvedRepo != "" {
				argsCreate = append(argsCreate, "--repo", resolvedRepo)
			}
			for _, label := range it.Labels {
				argsCreate = append(argsCreate, "--label", label)
			}
			argsCreate = append(argsCreate, "--json", "number,url")

			raw, err := exec.CommandContext(ctx, "gh", argsCreate...).CombinedOutput()
			if err != nil {
				msg := strings.TrimSpace(string(raw))
				if msg == "" {
					msg = err.Error()
				}
				return fmt.Errorf("gh issue create failed: %s", msg)
			}

			var res ghIssueCreateResponse
			if err := json.Unmarshal(raw, &res); err != nil {
				return fmt.Errorf("decode gh issue create response: %w", err)
			}
			if res.Number == 0 || strings.TrimSpace(res.URL) == "" {
				return fmt.Errorf("gh issue create returned incomplete response")
			}

			if err := store.UpsertGitHubIssueLink(ctx, issueID, fmt.Sprintf("%d", res.Number), res.URL, resolvedRepo); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "issue: %s\n", issueID)
			fmt.Fprintf(cmd.OutOrStdout(), "github_issue: %d\n", res.Number)
			fmt.Fprintf(cmd.OutOrStdout(), "url: %s\n", res.URL)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoOverride, "repo", "", "owner/name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show payload without creating GitHub issue")
	return cmd
}

func resolveGitHubRepo(repoOverride string) (string, error) {
	if strings.TrimSpace(repoOverride) != "" {
		return strings.TrimSpace(repoOverride), nil
	}
	cfg, err := appconfig.Load()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.GHRepo), nil
}

func formatGitHubIssueBody(it issue.Item) string {
	lines := []string{
		fmt.Sprintf("track id: %s", it.ID),
		fmt.Sprintf("status: %s", it.Status),
		fmt.Sprintf("priority: %s", it.Priority),
	}
	if body := strings.TrimSpace(it.Body); body != "" {
		lines = append(lines, "", body)
	}
	if len(it.Labels) > 0 {
		lines = append(lines, "", "labels: "+strings.Join(it.Labels, ", "))
	}
	return strings.Join(lines, "\n")
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

			issueID := normalizeIssueIDArg(args[0])
			if _, err := store.GetIssue(ctx, issueID); err != nil {
				return err
			}
			if err := store.UpsertGitHubLink(ctx, issueID, normalizePRRef(prRef), repo); err != nil {
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
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh command is required")
			}
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			link, err := store.GetGitHubLink(ctx, issueID)
			if err != nil {
				return err
			}
			checks, err := fetchPRChecks(ctx, link, "")
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "issue: %s\npr: %s\nrepo: %s\n", link.IssueID, link.PRRef, link.Repo)
			if len(checks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no checks found")
				fmt.Fprintln(cmd.OutOrStdout(), "overall: pending")
				return nil
			}

			sort.SliceStable(checks, func(i, j int) bool {
				return checks[i].Name < checks[j].Name
			})
			for _, check := range checks {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", check.Name, strings.ToLower(check.State), check.Link)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "overall: %s\n", summarizeChecks(checks))
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
			seenFailures := map[string]struct{}{}
			if err := runGHWatchOnce(ctx, repo, cmd.OutOrStdout(), seenFailures); err != nil {
				return err
			}

			ticker := time.NewTicker(dur)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := runGHWatchOnce(ctx, repo, cmd.OutOrStdout(), seenFailures); err != nil {
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

func newGHAutoMergeCmd() *cobra.Command {
	var method string
	var repoOverride string
	cmd := &cobra.Command{
		Use:   "auto-merge <issue_id>",
		Short: "Enable auto-merge for linked PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("gh"); err != nil {
				return fmt.Errorf("gh command is required")
			}
			if method != "squash" && method != "merge" && method != "rebase" {
				return fmt.Errorf("invalid --method: %s", method)
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			link, err := store.GetGitHubLink(ctx, issueID)
			if err != nil {
				return err
			}

			pr := normalizePRRef(link.PRRef)
			repo := link.Repo
			if repoOverride != "" {
				repo = repoOverride
			}
			argsMerge := []string{"pr", "merge", pr, "--auto"}
			switch method {
			case "squash":
				argsMerge = append(argsMerge, "--squash")
			case "merge":
				argsMerge = append(argsMerge, "--merge")
			case "rebase":
				argsMerge = append(argsMerge, "--rebase")
			}
			if repo != "" {
				argsMerge = append(argsMerge, "--repo", repo)
			}

			raw, err := exec.CommandContext(ctx, "gh", argsMerge...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("enable auto-merge failed: %s", strings.TrimSpace(string(raw)))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "auto-merge enabled for pr %s (method=%s)\n", pr, method)
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "squash", "Merge method: squash|merge|rebase")
	cmd.Flags().StringVar(&repoOverride, "repo", "", "owner/name")
	return cmd
}

func runGHWatchOnce(ctx context.Context, repo string, out io.Writer, seenFailures map[string]struct{}) error {
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
		checks, err := fetchPRChecks(ctx, link, repo)
		if err != nil {
			fmt.Fprintf(out, "checks error for %s (pr %s): %v\n", link.IssueID, link.PRRef, err)
		} else {
			for _, check := range checks {
				if !isFailureState(check.State) {
					continue
				}
				key := link.IssueID + "::" + check.Name + "::" + check.Link
				if _, ok := seenFailures[key]; ok {
					continue
				}
				seenFailures[key] = struct{}{}
				fmt.Fprintf(out, "failure: %s (%s)\n", check.Name, check.Link)
				logText, err := fetchFailureCheckLog(ctx, check, repoForLink(link, repo))
				if err != nil {
					fmt.Fprintf(out, "log fetch error: %v\n", err)
					continue
				}
				fmt.Fprintf(out, "--- log: %s ---\n%s\n", check.Name, tailLines(logText, 200))
			}
		}

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
	repo := repoForLink(link, repoOverride)
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

func fetchPRChecks(ctx context.Context, link sqlite.GitHubLink, repoOverride string) ([]ghCheck, error) {
	pr := normalizePRRef(link.PRRef)
	args := []string{"pr", "checks", pr, "--json", "name,state,link"}
	repo := repoForLink(link, repoOverride)
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	raw, err := exec.CommandContext(ctx, "gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr checks failed for %s: %w", pr, err)
	}
	var checks []ghCheck
	if err := json.Unmarshal(raw, &checks); err != nil {
		return nil, fmt.Errorf("decode gh checks response: %w", err)
	}
	return checks, nil
}

func fetchFailureCheckLog(ctx context.Context, check ghCheck, repo string) (string, error) {
	runID, jobID, ok := parseRunAndJobFromCheckLink(check.Link)
	if !ok {
		return "", fmt.Errorf("unsupported check link: %s", check.Link)
	}
	args := []string{"run", "view", runID, "--log"}
	if jobID != "" {
		args = append(args, "--job", jobID)
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	raw, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh run view failed: %s", strings.TrimSpace(string(raw)))
	}
	return string(raw), nil
}

func parseRunAndJobFromCheckLink(link string) (runID string, jobID string, ok bool) {
	m := ghRunLinkRe.FindStringSubmatch(link)
	if len(m) != 3 {
		return "", "", false
	}
	return m[1], m[2], true
}

func summarizeChecks(checks []ghCheck) string {
	if len(checks) == 0 {
		return "pending"
	}
	hasPending := false
	for _, check := range checks {
		if isFailureState(check.State) {
			return "failure"
		}
		if isPendingState(check.State) {
			hasPending = true
		}
	}
	if hasPending {
		return "pending"
	}
	return "success"
}

func isFailureState(state string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case "failure", "failed", "error", "timed_out", "cancelled", "action_required", "startup_failure":
		return true
	default:
		return false
	}
}

func isPendingState(state string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	switch s {
	case "pending", "queued", "in_progress", "waiting", "requested":
		return true
	default:
		return false
	}
}

func repoForLink(link sqlite.GitHubLink, repoOverride string) string {
	if repoOverride != "" {
		return repoOverride
	}
	return link.Repo
}

func tailLines(raw string, n int) string {
	raw = strings.TrimRight(raw, "\n")
	if raw == "" {
		return raw
	}
	lines := strings.Split(raw, "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	var buf bytes.Buffer
	for i := len(lines) - n; i < len(lines); i++ {
		if i > len(lines)-n {
			buf.WriteByte('\n')
		}
		buf.WriteString(lines[i])
	}
	return buf.String()
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
