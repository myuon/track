package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/myuon/track/internal/issue"
)

type ListFilter struct {
	Statuses    []string
	ExcludeDone bool
	Label       string
	Assignee    string
	Search      string
	Sort        string
}

type UpdateIssueInput struct {
	Title    *string
	Body     *string
	Status   *string
	Priority *string
	Due      *string
	Assignee *string
}

func (s *Store) CreateIssue(ctx context.Context, item issue.Item) (issue.Item, error) {
	if err := issue.ValidateStatus(item.Status); err != nil {
		return issue.Item{}, err
	}
	if err := issue.ValidatePriority(item.Priority); err != nil {
		return issue.Item{}, err
	}
	if err := issue.ValidateDue(item.Due); err != nil {
		return issue.Item{}, err
	}

	id, err := s.NextIssueID(ctx)
	if err != nil {
		return issue.Item{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	item.ID = id
	item.CreatedAt = now
	item.UpdatedAt = now

	labelsJSON, err := json.Marshal(item.Labels)
	if err != nil {
		return issue.Item{}, fmt.Errorf("marshal labels: %w", err)
	}
	nextOrder, err := s.nextOrderIndex(ctx)
	if err != nil {
		return issue.Item{}, err
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO issues(
			id, title, status, priority, assignee, due, labels_json, next_action, body, order_index, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Title, item.Status, item.Priority, nullable(item.Assignee), nullable(item.Due), string(labelsJSON), nullable(item.NextAction), nullable(item.Body), nextOrder, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return issue.Item{}, fmt.Errorf("insert issue: %w", err)
	}

	return item, nil
}

func (s *Store) GetIssue(ctx context.Context, id string) (issue.Item, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, title, status, priority, assignee, due, labels_json, next_action, body, created_at, updated_at FROM issues WHERE id = ?`, id)
	return scanIssueRow(row)
}

func (s *Store) UpdateIssue(ctx context.Context, id string, in UpdateIssueInput) (issue.Item, error) {
	current, err := s.GetIssue(ctx, id)
	if err != nil {
		return issue.Item{}, err
	}

	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.Body != nil {
		current.Body = *in.Body
	}
	if in.Status != nil {
		if err := issue.ValidateStatus(*in.Status); err != nil {
			return issue.Item{}, err
		}
		current.Status = *in.Status
	}
	if in.Priority != nil {
		if err := issue.ValidatePriority(*in.Priority); err != nil {
			return issue.Item{}, err
		}
		current.Priority = *in.Priority
	}
	if in.Due != nil {
		if err := issue.ValidateDue(*in.Due); err != nil {
			return issue.Item{}, err
		}
		current.Due = *in.Due
	}
	if in.Assignee != nil {
		current.Assignee = issue.NormalizeAssignee(*in.Assignee)
	}

	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE issues SET title=?, status=?, priority=?, assignee=?, due=?, body=?, updated_at=? WHERE id=?`,
		current.Title, current.Status, current.Priority, nullable(current.Assignee), nullable(current.Due), nullable(current.Body), current.UpdatedAt, id,
	)
	if err != nil {
		return issue.Item{}, fmt.Errorf("update issue: %w", err)
	}

	return current, nil
}

func (s *Store) ListIssues(ctx context.Context, f ListFilter) ([]issue.Item, error) {
	base := `SELECT id, title, status, priority, assignee, due, labels_json, next_action, body, created_at, updated_at FROM issues WHERE 1=1`
	args := make([]any, 0, 4)

	if len(f.Statuses) > 0 {
		base += ` AND status IN (`
		for i, st := range f.Statuses {
			if i > 0 {
				base += `, `
			}
			base += `?`
			args = append(args, st)
		}
		base += `)`
	} else if f.ExcludeDone {
		base += ` AND status <> ?`
		args = append(args, issue.StatusDone)
	}
	if f.Assignee != "" {
		base += ` AND assignee = ?`
		args = append(args, f.Assignee)
	}
	if f.Search != "" {
		base += ` AND (title LIKE ? OR body LIKE ?)`
		needle := "%" + f.Search + "%"
		args = append(args, needle, needle)
	}
	if f.Label != "" {
		base += ` AND labels_json LIKE ?`
		args = append(args, "%\""+f.Label+"\"%")
	}

	sort := strings.ToLower(f.Sort)
	switch sort {
	case "priority":
		base += ` ORDER BY CASE priority WHEN 'p0' THEN 0 WHEN 'p1' THEN 1 WHEN 'p2' THEN 2 ELSE 3 END, updated_at DESC`
	case "due":
		base += ` ORDER BY CASE WHEN due IS NULL OR due = '' THEN 1 ELSE 0 END, due ASC, updated_at DESC`
	case "manual":
		base += ` ORDER BY order_index ASC, updated_at DESC`
	default:
		base += ` ORDER BY updated_at DESC`
	}

	rows, err := s.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	items := make([]issue.Item, 0)
	for rows.Next() {
		item, err := scanIssueRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issues: %w", err)
	}
	return items, nil
}

func (s *Store) AddLabel(ctx context.Context, id, label string) (issue.Item, error) {
	if strings.TrimSpace(label) == "" {
		return issue.Item{}, fmt.Errorf("label must not be empty")
	}
	it, err := s.GetIssue(ctx, id)
	if err != nil {
		return issue.Item{}, err
	}
	if !slices.Contains(it.Labels, label) {
		it.Labels = append(it.Labels, label)
	}
	return s.updateLabels(ctx, it)
}

func (s *Store) RemoveLabel(ctx context.Context, id, label string) (issue.Item, error) {
	it, err := s.GetIssue(ctx, id)
	if err != nil {
		return issue.Item{}, err
	}
	next := make([]string, 0, len(it.Labels))
	for _, l := range it.Labels {
		if l != label {
			next = append(next, l)
		}
	}
	it.Labels = next
	return s.updateLabels(ctx, it)
}

func (s *Store) SetNextAction(ctx context.Context, id, text string) (issue.Item, error) {
	it, err := s.GetIssue(ctx, id)
	if err != nil {
		return issue.Item{}, err
	}
	it.NextAction = strings.TrimSpace(text)
	it.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `UPDATE issues SET next_action=?, updated_at=? WHERE id=?`, nullable(it.NextAction), it.UpdatedAt, id)
	if err != nil {
		return issue.Item{}, fmt.Errorf("set next_action: %w", err)
	}
	return it, nil
}

func (s *Store) Reorder(ctx context.Context, id, beforeID, afterID string) error {
	if (beforeID == "" && afterID == "") || (beforeID != "" && afterID != "") {
		return fmt.Errorf("specify either --before or --after")
	}

	items, err := s.ListIssues(ctx, ListFilter{Sort: "manual"})
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	src := slices.Index(ids, id)
	if src == -1 {
		return fmt.Errorf("issue not found: %s", id)
	}
	ids = slices.Delete(ids, src, src+1)

	var dst int
	if beforeID != "" {
		idx := slices.Index(ids, beforeID)
		if idx == -1 {
			return fmt.Errorf("reference issue not found: %s", beforeID)
		}
		dst = idx
	} else {
		idx := slices.Index(ids, afterID)
		if idx == -1 {
			return fmt.Errorf("reference issue not found: %s", afterID)
		}
		dst = idx + 1
	}
	ids = slices.Insert(ids, dst, id)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	for i, issueID := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET order_index=?, updated_at=? WHERE id=?`, i+1, time.Now().UTC().Format(time.RFC3339), issueID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update order: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
}

func (s *Store) updateLabels(ctx context.Context, it issue.Item) (issue.Item, error) {
	raw, err := json.Marshal(it.Labels)
	if err != nil {
		return issue.Item{}, fmt.Errorf("marshal labels: %w", err)
	}
	it.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `UPDATE issues SET labels_json=?, updated_at=? WHERE id=?`, string(raw), it.UpdatedAt, it.ID)
	if err != nil {
		return issue.Item{}, fmt.Errorf("update labels: %w", err)
	}
	return it, nil
}

func (s *Store) nextOrderIndex(ctx context.Context) (int, error) {
	var max sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(order_index) FROM issues`).Scan(&max); err != nil {
		return 0, fmt.Errorf("read max order index: %w", err)
	}
	if !max.Valid {
		return 1, nil
	}
	return int(max.Int64) + 1, nil
}

func scanIssueRow(row *sql.Row) (issue.Item, error) {
	var (
		item      issue.Item
		assignee  sql.NullString
		due       sql.NullString
		nextAct   sql.NullString
		body      sql.NullString
		labelsRaw string
	)
	if err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Status,
		&item.Priority,
		&assignee,
		&due,
		&labelsRaw,
		&nextAct,
		&body,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return issue.Item{}, fmt.Errorf("issue not found")
		}
		return issue.Item{}, fmt.Errorf("scan issue: %w", err)
	}
	if assignee.Valid {
		item.Assignee = assignee.String
	}
	if due.Valid {
		item.Due = due.String
	}
	if nextAct.Valid {
		item.NextAction = nextAct.String
	}
	if body.Valid {
		item.Body = body.String
	}
	if err := json.Unmarshal([]byte(labelsRaw), &item.Labels); err != nil {
		return issue.Item{}, fmt.Errorf("decode labels: %w", err)
	}
	return item, nil
}

func scanIssueRows(rows *sql.Rows) (issue.Item, error) {
	var (
		item      issue.Item
		assignee  sql.NullString
		due       sql.NullString
		nextAct   sql.NullString
		body      sql.NullString
		labelsRaw string
	)
	if err := rows.Scan(
		&item.ID,
		&item.Title,
		&item.Status,
		&item.Priority,
		&assignee,
		&due,
		&labelsRaw,
		&nextAct,
		&body,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return issue.Item{}, fmt.Errorf("scan issue: %w", err)
	}
	if assignee.Valid {
		item.Assignee = assignee.String
	}
	if due.Valid {
		item.Due = due.String
	}
	if nextAct.Valid {
		item.NextAction = nextAct.String
	}
	if body.Valid {
		item.Body = body.String
	}
	if err := json.Unmarshal([]byte(labelsRaw), &item.Labels); err != nil {
		return issue.Item{}, fmt.Errorf("decode labels: %w", err)
	}
	return item, nil
}

func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
