package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

func newIOCommands() []*cobra.Command {
	return []*cobra.Command{newExportCmd(), newImportCmd()}
}

func newExportCmd() *cobra.Command {
	var format string
	var status string
	var label string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export issues",
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

			items, err := store.ListIssues(ctx, sqlite.ListFilter{Statuses: statuses, Label: label, Sort: "manual"})
			if err != nil {
				return err
			}

			switch format {
			case "text":
				return writeTextExport(cmd.OutOrStdout(), items)
			case "csv":
				return writeCSVExport(cmd.OutOrStdout(), items)
			case "json":
				return writeJSONExport(cmd.OutOrStdout(), items)
			case "jsonl":
				return writeJSONLExport(cmd.OutOrStdout(), items)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "Export format: text|csv|json|jsonl")
	cmd.Flags().StringVar(&status, "status", "", "Status filter")
	cmd.Flags().StringVar(&label, "label", "", "Label filter")
	return cmd
}

func newImportCmd() *cobra.Command {
	var format string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "import --format text|csv|json|jsonl <path>",
		Short: "Import issues",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()

			items, err := readImport(format, f)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: %d issues\n", len(items))
				return nil
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			for _, it := range items {
				if it.Status == "" {
					it.Status = issue.StatusTodo
				}
				if it.Priority == "" {
					it.Priority = "none"
				}
				if _, err := store.CreateIssue(ctx, it); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported: %d issues\n", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "Import format: text|csv|json|jsonl")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate input without writing")
	return cmd
}

func writeTextExport(out io.Writer, items []issue.Item) error {
	for _, it := range items {
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", it.ID, it.Status, it.Priority, strings.ReplaceAll(it.Title, "\t", " "))
		if err != nil {
			return err
		}
	}
	return nil
}

func writeCSVExport(out io.Writer, items []issue.Item) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"id", "title", "status", "priority", "assignee", "due", "labels", "next_action", "body"}); err != nil {
		return err
	}
	for _, it := range items {
		if err := w.Write([]string{it.ID, it.Title, it.Status, it.Priority, it.Assignee, it.Due, strings.Join(it.Labels, ","), it.NextAction, it.Body}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeJSONLExport(out io.Writer, items []issue.Item) error {
	enc := json.NewEncoder(out)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONExport(out io.Writer, items []issue.Item) error {
	enc := json.NewEncoder(out)
	return enc.Encode(items)
}

func readImport(format string, in io.Reader) ([]issue.Item, error) {
	switch format {
	case "text":
		return readTextImport(in)
	case "csv":
		return readCSVImport(in)
	case "json":
		return readJSONImport(in)
	case "jsonl":
		return readJSONLImport(in)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func readTextImport(in io.Reader) ([]issue.Item, error) {
	s := bufio.NewScanner(in)
	items := make([]issue.Item, 0)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid text import line: %q", line)
		}
		items = append(items, issue.Item{
			Title:    parts[3],
			Status:   parts[1],
			Priority: parts[2],
		})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func readCSVImport(in io.Reader) ([]issue.Item, error) {
	r := csv.NewReader(in)
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}

	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	items := make([]issue.Item, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		get := func(k string) string {
			i, ok := idx[k]
			if !ok || i >= len(rec) {
				return ""
			}
			return rec[i]
		}
		it := issue.Item{
			Title:      get("title"),
			Status:     get("status"),
			Priority:   get("priority"),
			Assignee:   get("assignee"),
			Due:        get("due"),
			NextAction: get("next_action"),
			Body:       get("body"),
		}
		if raw := get("labels"); raw != "" {
			it.Labels = strings.Split(raw, ",")
		}
		items = append(items, it)
	}
	return items, nil
}

func readJSONLImport(in io.Reader) ([]issue.Item, error) {
	s := bufio.NewScanner(in)
	items := make([]issue.Item, 0)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var it issue.Item
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func readJSONImport(in io.Reader) ([]issue.Item, error) {
	dec := json.NewDecoder(in)
	var items []issue.Item
	if err := dec.Decode(&items); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("invalid json import: multiple top-level JSON values")
		}
		return nil, err
	}
	return items, nil
}
