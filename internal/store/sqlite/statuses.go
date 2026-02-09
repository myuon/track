package sqlite

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/myuon/track/internal/issue"
)

var statusNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func (s *Store) ListStatuses(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM statuses
		ORDER BY
			CASE name
				WHEN 'todo' THEN 0
				WHEN 'ready' THEN 1
				WHEN 'in_progress' THEN 2
				WHEN 'done' THEN 3
				WHEN 'archived' THEN 4
				ELSE 5
			END,
			name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list statuses: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 8)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan status: %w", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate statuses: %w", err)
	}
	return out, nil
}

func (s *Store) ValidateStatus(ctx context.Context, v string) error {
	if err := issue.ValidateStatus(v); err == nil {
		return nil
	}
	statuses, err := s.ListStatuses(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(statuses, v) {
		return nil
	}
	return fmt.Errorf("invalid status: %s", v)
}

func (s *Store) AddStatus(ctx context.Context, name string) error {
	name = normalizeStatusName(name)
	if err := validateStatusName(name); err != nil {
		return err
	}
	if err := s.ValidateStatus(ctx, name); err == nil {
		return fmt.Errorf("status already exists: %s", name)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO statuses(name, system, created_at, updated_at)
		VALUES(?, 0, ?, ?)`,
		name, now, now,
	)
	if err != nil {
		return fmt.Errorf("add status: %w", err)
	}
	return nil
}

func (s *Store) RemoveStatus(ctx context.Context, name string) error {
	name = normalizeStatusName(name)
	if err := validateStatusName(name); err != nil {
		return err
	}

	var system int
	if err := s.db.QueryRowContext(ctx, `SELECT system FROM statuses WHERE name = ?`, name).Scan(&system); err != nil {
		return fmt.Errorf("status not found: %s", name)
	}
	if system == 1 {
		return fmt.Errorf("cannot remove built-in status: %s", name)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM issues WHERE status = ?`, name).Scan(&count); err != nil {
		return fmt.Errorf("count issues by status: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("status is in use: %s", name)
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM statuses WHERE name = ?`, name); err != nil {
		return fmt.Errorf("remove status: %w", err)
	}
	return nil
}

func normalizeStatusName(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func validateStatusName(v string) error {
	if !statusNameRe.MatchString(v) {
		return fmt.Errorf("invalid status name: %s", v)
	}
	return nil
}
