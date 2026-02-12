package cli

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	appconfig "github.com/myuon/track/internal/config"
	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

const (
	listIDWidth       = 10
	listStatusWidth   = 12
	listPriorityWidth = 10
	listTitleWidth    = 50
	listLabelsWidth   = 24
)

type issueListLayout struct {
	idWidth       int
	statusWidth   int
	priorityWidth int
	titleWidth    int
	labelsWidth   int
}

func formatIssueListRow(id, status, priority, title, labels string) string {
	layout := issueListLayout{
		idWidth:       listIDWidth,
		statusWidth:   listStatusWidth,
		priorityWidth: listPriorityWidth,
		titleWidth:    listTitleWidth,
		labelsWidth:   listLabelsWidth,
	}
	return formatIssueListRowWithLayout(layout, id, status, priority, title, labels)
}

func formatIssueListRowWithLayout(layout issueListLayout, id, status, priority, title, labels string) string {
	parts := []string{
		formatListColumn(id, layout.idWidth, true),
		formatListColumn(status, layout.statusWidth, false),
		formatListColumn(priority, layout.priorityWidth, false),
		formatListColumn(title, layout.titleWidth, true),
		formatListColumn(labels, layout.labelsWidth, true),
	}
	return strings.Join(parts, " ")
}

func issueListLayoutForItems(items []issue.Item) issueListLayout {
	layout := issueListLayout{
		idWidth:       listIDWidth,
		statusWidth:   listDisplayWidth("STATUS"),
		priorityWidth: listDisplayWidth("PRIORITY"),
		titleWidth:    listTitleWidth,
		labelsWidth:   listLabelsWidth,
	}
	for _, it := range items {
		if w := listDisplayWidth(it.Status); w > layout.statusWidth {
			layout.statusWidth = w
		}
		if w := listDisplayWidth(it.Priority); w > layout.priorityWidth {
			layout.priorityWidth = w
		}
	}
	return layout
}

func fitListColumn(v string, width int) string {
	if listDisplayWidth(v) <= width {
		return v
	}
	if width <= 3 {
		return truncateListColumn(v, width)
	}
	return truncateListColumn(v, width-3) + "..."
}

func formatListColumn(v string, width int, truncate bool) string {
	fit := v
	if truncate {
		fit = fitListColumn(v, width)
	}
	padding := width - listDisplayWidth(fit)
	if padding <= 0 {
		return fit
	}
	return fit + strings.Repeat(" ", padding)
}

func truncateListColumn(v string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var b strings.Builder
	width := 0
	for _, r := range v {
		rw := listRuneWidth(r)
		if width+rw > maxWidth {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String()
}

func listDisplayWidth(v string) int {
	width := 0
	for _, r := range v {
		width += listRuneWidth(r)
	}
	return width
}

func listRuneWidth(r rune) int {
	w := runewidth.RuneWidth(r)
	if w <= 0 {
		return 0
	}
	return w
}

func newIssueCommands() []*cobra.Command {
	return []*cobra.Command{
		newNewCmd(),
		newListCmd(),
		newShowCmd(),
		newEditCmd(),
		newSetCmd(),
		newStatusCmd(),
		newLabelCmd(),
		newPlanningCmd(),
		newReplyCmd(),
		newNextCmd(),
		newDoneCmd(),
		newArchiveCmd(),
		newReorderCmd(),
	}
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Manage issue statuses",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available statuses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			statuses, err := store.ListStatuses(ctx)
			if err != nil {
				return err
			}
			for _, st := range statuses {
				fmt.Fprintln(cmd.OutOrStdout(), st)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add <name>",
		Short: "Add a custom status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.AddStatus(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a custom status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.RemoveStatus(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	})
	return cmd
}

func newNewCmd() *cobra.Command {
	var (
		body     string
		labels   []string
		priority string
		due      string
		assignee string
		tui      bool
	)

	cmd := &cobra.Command{
		Use:   "new [title]",
		Short: "Create an issue",
		Args: func(cmd *cobra.Command, args []string) error {
			if tui {
				if len(args) > 1 {
					return cobra.MaximumNArgs(1)(cmd, args)
				}
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			in := newIssueInput{
				Body:     body,
				Priority: priority,
				Assignee: assignee,
				Due:      due,
				Labels:   labels,
			}
			if len(args) > 0 {
				in.Title = args[0]
			}
			if tui {
				initialTitle := ""
				if len(args) > 0 {
					initialTitle = args[0]
				}
				prompted, cancelled, err := promptNewIssueInput(cmd.InOrStdin(), cmd.OutOrStdout(), initialTitle)
				if err != nil {
					return err
				}
				if cancelled {
					fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
					return nil
				}
				in = prompted
			}

			item, err := createIssueFromInput(ctx, store, in)
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
	cmd.Flags().BoolVar(&tui, "tui", false, "Create issue with interactive prompts")

	return cmd
}

type newIssueInput struct {
	Title    string
	Body     string
	Priority string
	Assignee string
	Due      string
	Labels   []string
}

func createIssueFromInput(ctx context.Context, store *sqlite.Store, in newIssueInput) (issue.Item, error) {
	if err := issue.ValidateTitle(in.Title); err != nil {
		return issue.Item{}, err
	}
	if err := issue.ValidatePriority(in.Priority); err != nil {
		return issue.Item{}, err
	}
	if err := issue.ValidateDue(in.Due); err != nil {
		return issue.Item{}, err
	}

	return store.CreateIssue(ctx, issue.Item{
		Title:    in.Title,
		Status:   issue.StatusTodo,
		Priority: in.Priority,
		Assignee: issue.NormalizeAssignee(in.Assignee),
		Due:      in.Due,
		Labels:   in.Labels,
		Body:     in.Body,
	})
}

func promptNewIssueInput(in io.Reader, out io.Writer, initialTitle string) (newIssueInput, bool, error) {
	reader := bufio.NewReader(in)

	title := strings.TrimSpace(initialTitle)
	for {
		if title == "" {
			v, err := readPromptLine(reader, out, "title: ")
			if err != nil {
				return newIssueInput{}, false, err
			}
			title = strings.TrimSpace(v)
		}
		if err := issue.ValidateTitle(title); err != nil {
			fmt.Fprintln(out, err)
			title = ""
			continue
		}
		break
	}

	body, err := readPromptLine(reader, out, "body: ")
	if err != nil {
		return newIssueInput{}, false, err
	}

	var priority string
	for {
		v, err := readPromptLine(reader, out, "priority (none|p0|p1|p2|p3) [none]: ")
		if err != nil {
			return newIssueInput{}, false, err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			v = "none"
		}
		if err := issue.ValidatePriority(v); err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		priority = v
		break
	}

	labelsLine, err := readPromptLine(reader, out, "labels (comma separated): ")
	if err != nil {
		return newIssueInput{}, false, err
	}
	assignee, err := readPromptLine(reader, out, "assignee: ")
	if err != nil {
		return newIssueInput{}, false, err
	}

	var due string
	for {
		v, err := readPromptLine(reader, out, "due (YYYY-MM-DD): ")
		if err != nil {
			return newIssueInput{}, false, err
		}
		v = strings.TrimSpace(v)
		if err := issue.ValidateDue(v); err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		due = v
		break
	}

	confirm, err := readPromptLine(reader, out, "confirm create? [y/N]: ")
	if err != nil {
		return newIssueInput{}, false, err
	}
	confirmed := strings.EqualFold(strings.TrimSpace(confirm), "y") || strings.EqualFold(strings.TrimSpace(confirm), "yes")
	if !confirmed {
		return newIssueInput{}, true, nil
	}

	return newIssueInput{
		Title:    title,
		Body:     strings.TrimSpace(body),
		Priority: priority,
		Assignee: strings.TrimSpace(assignee),
		Due:      due,
		Labels:   parseCommaSeparatedLabels(labelsLine),
	}, false, nil
}

func readPromptLine(reader *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return line, nil
		}
		if err == io.EOF {
			return "", fmt.Errorf("input ended before prompt completion")
		}
		return "", err
	}
	return line, nil
}

func parseCommaSeparatedLabels(v string) []string {
	parts := strings.Split(v, ",")
	labels := make([]string, 0, len(parts))
	for _, p := range parts {
		label := strings.TrimSpace(p)
		if label == "" {
			continue
		}
		labels = append(labels, label)
	}
	return labels
}

func newListCmd() *cobra.Command {
	var (
		status   string
		label    string
		assignee string
		search   string
		project  string
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

			statuses, err := parseStatusFilter(status, func(v string) error {
				return store.ValidateStatus(ctx, v)
			})
			if err != nil {
				return err
			}

			items, err := store.ListIssues(ctx, sqlite.ListFilter{
				Statuses:        statuses,
				ExcludeDone:     len(statuses) == 0,
				ExcludeArchived: len(statuses) == 0,
				Label:           label,
				Assignee:        issue.NormalizeAssignee(assignee),
				Search:          search,
				Project:         project,
				Sort:            sort,
			})
			if err != nil {
				return err
			}

			c := newCLIColor(cmd.OutOrStdout())
			layout := issueListLayoutForItems(items)
			fmt.Fprintln(cmd.OutOrStdout(), formatIssueListRowWithLayout(layout, "ID", "STATUS", "PRIORITY", "TITLE", "LABELS"))
			for _, it := range items {
				fmt.Fprintln(
					cmd.OutOrStdout(),
					formatIssueListRowWithLayout(layout, it.ID, c.status(it.Status), c.priority(it.Priority), it.Title, strings.Join(it.Labels, ",")),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status filter (comma separated)")
	cmd.Flags().StringVar(&label, "label", "", "Label filter")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee filter")
	cmd.Flags().StringVar(&search, "search", "", "Search text")
	cmd.Flags().StringVar(&project, "project", "", "Project filter")
	cmd.Flags().StringVar(&sort, "sort", "manual", "Sort by priority|due|updated|manual")

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

			c := newCLIColor(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "id: %s\n", it.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "title: %s\n", it.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "status: %s\n", c.status(it.Status))
			fmt.Fprintf(cmd.OutOrStdout(), "priority: %s\n", c.priority(it.Priority))
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
			projectKey, err := store.GetIssueProject(ctx, it.ID)
			if err != nil {
				return err
			}
			if projectKey != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "project: %s\n", projectKey)
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

func parseStatusFilter(raw string, validate func(string) error) ([]string, error) {
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
		if err := validate(st); err != nil {
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
		title      string
		status     string
		priority   string
		due        string
		assignee   string
		nextAction string
		project    string
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
			if cmd.Flags().Changed("title") {
				if err := issue.ValidateTitle(title); err != nil {
					return err
				}
				in.Title = &title
			}
			if cmd.Flags().Changed("status") {
				if err := store.ValidateStatus(ctx, status); err != nil {
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
			hasProjectUpdate := cmd.Flags().Changed("project")
			if in.Title == nil && in.Status == nil && in.Priority == nil && in.Due == nil && in.Assignee == nil && in.NextAction == nil && !hasProjectUpdate {
				return fmt.Errorf("no fields to update")
			}

			issueID := normalizeIssueIDArg(args[0])
			updatedIssueID := issueID
			if in.Title != nil || in.Status != nil || in.Priority != nil || in.Due != nil || in.Assignee != nil || in.NextAction != nil {
				updated, err := store.UpdateIssue(ctx, issueID, in)
				if err != nil {
					return err
				}
				updatedIssueID = updated.ID
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
			}
			if hasProjectUpdate {
				projectValue := strings.TrimSpace(project)
				if projectValue == "none" {
					projectValue = ""
				}
				if err := store.SetIssueProject(ctx, issueID, projectValue); err != nil {
					return err
				}
			}
			if err := hooks.RunEvent(ctx, store, hooks.IssueUpdated, updatedIssueID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Title")
	cmd.Flags().StringVar(&status, "status", "", "Status")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority")
	cmd.Flags().StringVar(&due, "due", "", "Due date")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")
	cmd.Flags().StringVar(&nextAction, "next-action", "", "Next action")
	cmd.Flags().StringVar(&project, "project", "", "Project key or none")

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
				Sort:     "priority_manual",
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

func newPlanningCmd() *cobra.Command {
	var limit int
	var yes bool

	cmd := &cobra.Command{
		Use:   "planning [id]",
		Short: "Interactively review todo issues and mark ready",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return fmt.Errorf("limit must be >= 0")
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			var items []issue.Item
			if len(args) == 1 {
				it, err := store.GetIssue(ctx, normalizeIssueIDArg(args[0]))
				if err != nil {
					return err
				}
				items = []issue.Item{it}
			} else {
				items, err = store.ListIssues(ctx, sqlite.ListFilter{
					Statuses: []string{issue.StatusTodo},
					Assignee: "agent",
					Sort:     "manual",
				})
				if err != nil {
					return err
				}
			}

			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			updatedCount := 0
			skippedCount := 0
			out := cmd.OutOrStdout()
			reader := bufio.NewReader(cmd.InOrStdin())

			for _, it := range items {
				if it.Status != issue.StatusTodo {
					fmt.Fprintf(out, "%s skipped (status=%s)\n", it.ID, it.Status)
					skippedCount++
					continue
				}

				if !yes {
					line, err := readPromptLine(reader, out, fmt.Sprintf("start planning %s? [y/N]: ", it.ID))
					if err != nil {
						return err
					}
					confirmed := strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes")
					if !confirmed {
						fmt.Fprintf(out, "%s skipped (confirmation declined)\n", it.ID)
						skippedCount++
						continue
					}
				}

				fmt.Fprintf(out, "planning session start: %s\n", it.ID)
				if err := planningSessionRunner(ctx, cmd, it.ID); err != nil {
					return err
				}

				updated, err := store.GetIssue(ctx, it.ID)
				if err != nil {
					return err
				}
				if updated.Status == issue.StatusReady {
					updatedCount++
					fmt.Fprintf(out, "%s updated to ready\n", it.ID)
				} else {
					skippedCount++
					fmt.Fprintf(out, "%s skipped (status=%s)\n", it.ID, updated.Status)
				}
			}

			fmt.Fprintf(out, "updated: %d, skipped: %d\n", updatedCount, skippedCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Limit number of issues to process")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts and run planning for each target issue")
	return cmd
}

var planningSessionRunner = func(ctx context.Context, cmd *cobra.Command, issueID string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	prompt := buildPlanningPrompt(issueID)
	args := []string{"exec", "-C", cwd}
	trackHome, err := appconfig.HomeDir()
	if err == nil && strings.TrimSpace(trackHome) != "" {
		args = append(args, "--add-dir", trackHome)
	}
	args = append(args, prompt)
	c := exec.CommandContext(ctx, "codex", args...)
	if err == nil && strings.TrimSpace(trackHome) != "" {
		c.Env = append(os.Environ(), "TRACK_HOME="+trackHome)
	}
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func buildPlanningPrompt(issueID string) string {
	return strings.TrimSpace(fmt.Sprintf(`
$track-dev-cycle plan %s
Plan mode requirements: (1) update issue body with an implementation-ready, testable Spec (e.g. under '## Spec'); (2) if questions remain, append them under '## Questions for user' in body; (3) set assignee to user for unresolved issues; (4) continue planning remaining issues without stopping.
`, issueID))
}

func newReplyCmd() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "reply <id>",
		Short: "Append user reply to issue body and assign back to agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			issueID := normalizeIssueIDArg(args[0])
			current, err := store.GetIssue(ctx, issueID)
			if err != nil {
				return err
			}

			replyText := strings.TrimSpace(message)
			if replyText == "" {
				fmt.Fprint(cmd.OutOrStdout(), "reply> ")
				s := bufio.NewScanner(cmd.InOrStdin())
				if !s.Scan() {
					if err := s.Err(); err != nil {
						return err
					}
					return fmt.Errorf("reply message is required")
				}
				replyText = strings.TrimSpace(s.Text())
			}
			if replyText == "" {
				return fmt.Errorf("reply message is required")
			}

			body := appendReplyToBody(current.Body, replyText)
			assignee := "agent"
			updated, err := store.UpdateIssue(ctx, issueID, sqlite.UpdateIssueInput{
				Body:     &body,
				Assignee: &assignee,
			})
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

	cmd.Flags().StringVarP(&message, "message", "m", "", "Reply message")
	return cmd
}

func appendReplyToBody(body, reply string) string {
	const header = "## User replies"
	entry := "- " + strings.TrimSpace(reply)

	body = strings.TrimRight(body, "\n")
	if body == "" {
		return header + "\n" + entry + "\n"
	}
	if strings.Contains(body, header) {
		return body + "\n" + entry + "\n"
	}
	return body + "\n\n" + header + "\n" + entry + "\n"
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
