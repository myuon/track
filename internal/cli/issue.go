package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

func newIssueCommands() []*cobra.Command {
	return []*cobra.Command{
		newNewCmd(),
		newListCmd(),
		newShowCmd(),
		newEditCmd(),
		newSetCmd(),
		newLabelCmd(),
		newNextCmd(),
		newDoneCmd(),
		newArchiveCmd(),
		newReorderCmd(),
	}
}

func newNewCmd() *cobra.Command {
	var (
		body     string
		labels   []string
		priority string
		due      string
		assignee string
	)

	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := issue.ValidatePriority(priority); err != nil {
				return err
			}
			if err := issue.ValidateDue(due); err != nil {
				return err
			}

			item, err := store.CreateIssue(ctx, issue.Item{
				Title:    args[0],
				Status:   issue.StatusTodo,
				Priority: priority,
				Assignee: issue.NormalizeAssignee(assignee),
				Due:      due,
				Labels:   labels,
				Body:     body,
			})
			if err != nil {
				return err
			}

			if err := hooks.RunEvent(ctx, store, hooks.IssueCreated, item.ID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), item.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Issue body")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Issue label (repeatable)")
	cmd.Flags().StringVar(&priority, "priority", "none", "Priority (none|p0|p1|p2|p3)")
	cmd.Flags().StringVar(&due, "due", "", "Due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")

	return cmd
}

func newListCmd() *cobra.Command {
	var (
		status   string
		label    string
		assignee string
		search   string
		sort     string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			statuses, err := parseStatusFilter(status)
			if err != nil {
				return err
			}

			items, err := store.ListIssues(ctx, sqlite.ListFilter{
				Statuses:    statuses,
				ExcludeDone: len(statuses) == 0,
				Label:       label,
				Assignee:    issue.NormalizeAssignee(assignee),
				Search:      search,
				Sort:        sort,
			})
			if err != nil {
				return err
			}

			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", it.ID, it.Status, it.Priority, it.Title, strings.Join(it.Labels, ","))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status filter (comma separated)")
	cmd.Flags().StringVar(&label, "label", "", "Label filter")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee filter")
	cmd.Flags().StringVar(&search, "search", "", "Search text")
	cmd.Flags().StringVar(&sort, "sort", "updated", "Sort by priority|due|updated|manual")

	return cmd
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show issue detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			it, err := store.GetIssue(ctx, normalizeIssueIDArg(args[0]))
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "id: %s\n", it.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "title: %s\n", it.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "status: %s\n", it.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "priority: %s\n", it.Priority)
			if it.Assignee != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "assignee: %s\n", it.Assignee)
			}
			if it.Due != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "due: %s\n", it.Due)
			}
			if len(it.Labels) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "labels: %s\n", strings.Join(it.Labels, ","))
			}
			if it.NextAction != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "next_action: %s\n", it.NextAction)
			}
			branchLink, err := store.GetGitBranchLink(ctx, it.ID)
			if err != nil && !isNotFoundErr(err) {
				return err
			}
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "branch: %s\n", branchLink.BranchName)
				fmt.Fprintf(cmd.OutOrStdout(), "merged: %s\n", branchMergeStatus(ctx, branchLink.BranchName))
			}
			if it.Body != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "body: %s\n", it.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated_at: %s\n", it.UpdatedAt)
			return nil
		},
	}
}

func normalizeIssueIDArg(raw string) string {
	id := strings.TrimSpace(raw)
	if _, err := strconv.Atoi(id); err == nil {
		return "TRK-" + id
	}
	return id
}

func parseStatusFilter(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		st := strings.TrimSpace(part)
		if st == "" {
			continue
		}
		if err := issue.ValidateStatus(st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("status filter is empty")
	}
	return out, nil
}

func isNotFoundErr(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(sql.ErrNoRows.Error()))
}

func newEditCmd() *cobra.Command {
	var (
		title string
		body  string
	)

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit issue title/body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			var in sqlite.UpdateIssueInput
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if cmd.Flags().Changed("body") {
				in.Body = &body
			}
			if in.Title == nil && in.Body == nil {
				return fmt.Errorf("no fields to update")
			}

			updated, err := store.UpdateIssue(ctx, args[0], in)
			if err != nil {
				return err
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&body, "body", "", "New body")

	return cmd
}

func newSetCmd() *cobra.Command {
	var (
		status     string
		priority   string
		due        string
		assignee   string
		nextAction string
	)

	cmd := &cobra.Command{
		Use:   "set <id>",
		Short: "Set issue fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			var in sqlite.UpdateIssueInput
			if cmd.Flags().Changed("status") {
				if err := issue.ValidateStatus(status); err != nil {
					return err
				}
				in.Status = &status
			}
			if cmd.Flags().Changed("priority") {
				if err := issue.ValidatePriority(priority); err != nil {
					return err
				}
				in.Priority = &priority
			}
			if cmd.Flags().Changed("due") {
				if err := issue.ValidateDue(due); err != nil {
					return err
				}
				in.Due = &due
			}
			if cmd.Flags().Changed("assignee") {
				v := issue.NormalizeAssignee(assignee)
				in.Assignee = &v
			}
			if cmd.Flags().Changed("next-action") {
				in.NextAction = &nextAction
			}
			if in.Status == nil && in.Priority == nil && in.Due == nil && in.Assignee == nil && in.NextAction == nil {
				return fmt.Errorf("no fields to update")
			}

			updated, err := store.UpdateIssue(ctx, args[0], in)
			if err != nil {
				return err
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
				return err
			}
			if in.Status != nil {
				if err := hooks.RunEvent(ctx, store, hooks.IssueStatusChange, updated.ID); err != nil {
					return err
				}
				if *in.Status == issue.StatusDone {
					if err := hooks.RunEvent(ctx, store, hooks.IssueCompleted, updated.ID); err != nil {
						return err
					}
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority")
	cmd.Flags().StringVar(&due, "due", "", "Due date")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")
	cmd.Flags().StringVar(&nextAction, "next-action", "", "Next action")

	return cmd
}

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage issue labels",
	}

	attachRunE := func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, err := sqlite.Open(ctx)
		if err != nil {
			return err
		}
		defer store.Close()

		updated, err := store.GetIssue(ctx, args[0])
		if err != nil {
			return err
		}
		for _, label := range args[1:] {
			updated, err = store.AddLabel(ctx, args[0], label)
			if err != nil {
				return err
			}
		}
		if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "ok")
		return nil
	}

	detachRunE := func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, err := sqlite.Open(ctx)
		if err != nil {
			return err
		}
		defer store.Close()

		updated, err := store.GetIssue(ctx, args[0])
		if err != nil {
			return err
		}
		for _, label := range args[1:] {
			updated, err = store.RemoveLabel(ctx, args[0], label)
			if err != nil {
				return err
			}
		}
		if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "ok")
		return nil
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "attach <id> <label> [label...]",
		Short: "Attach one or more labels to issue",
		Args:  cobra.MinimumNArgs(2),
		RunE:  attachRunE,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "detach <id> <label> [label...]",
		Short: "Detach one or more labels from issue",
		Args:  cobra.MinimumNArgs(2),
		RunE:  detachRunE,
	})

	// Backward-compatible aliases.
	cmd.AddCommand(&cobra.Command{
		Use:   "add <id> <label>",
		Short: "Alias of label attach",
		Args:  cobra.ExactArgs(2),
		RunE:  attachRunE,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "rm <id> <label>",
		Short: "Alias of label detach",
		Args:  cobra.ExactArgs(2),
		RunE:  detachRunE,
	})
	return cmd
}

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show next actionable issue",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "track next no longer sets next action; use: track set <id> --next-action <text>")
				return fmt.Errorf("unexpected args")
			}
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			items, err := store.ListIssues(ctx, sqlite.ListFilter{
				Statuses: []string{issue.StatusTodo, issue.StatusReady},
				Sort:     "manual",
			})
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no actionable issues")
				return nil
			}
			next := items[0]
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", next.ID, next.Priority, next.Title)
			return nil
		},
	}
}

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark issue as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()
			status := issue.StatusDone
			updated, err := store.UpdateIssue(ctx, args[0], sqlite.UpdateIssueInput{Status: &status})
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
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}

func newArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()
			status := issue.StatusArchived
			updated, err := store.UpdateIssue(ctx, args[0], sqlite.UpdateIssueInput{Status: &status})
			if err != nil {
				return err
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updated.ID); err != nil {
				return err
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueStatusChange, updated.ID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}

func newReorderCmd() *cobra.Command {
	var beforeID string
	var afterID string

	cmd := &cobra.Command{
		Use:   "reorder <id>",
		Short: "Reorder issue in global manual queue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.Reorder(ctx, args[0], beforeID, afterID); err != nil {
				return err
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, args[0]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&beforeID, "before", "", "Move before issue ID")
	cmd.Flags().StringVar(&afterID, "after", "", "Move after issue ID")
	return cmd
}
