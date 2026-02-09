package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/myuon/track/internal/issue"
)

type ListFilter struct {
	Status   string
	Label    string
	Assignee string
	Search   string
	Sort     string
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

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO issues(
			id, title, status, priority, assignee, due, labels_json, next_action, body, order_index, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Title, item.Status, item.Priority, nullable(item.Assignee), nullable(item.Due), string(labelsJSON), nullable(item.NextAction), nullable(item.Body), 0, item.CreatedAt, item.UpdatedAt,
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

	if f.Status != "" {
		base += ` AND status = ?`
		args = append(args, f.Status)
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
