package sqlite

import (
	"context"
	"fmt"
	"time"
)

type Hook struct {
	ID        int
	Event     string
	RunCmd    string
	CWD       string
	CreatedAt string
}

func (s *Store) ListHooks(ctx context.Context, event string) ([]Hook, error) {
	query := `SELECT id, event, run_cmd, cwd, created_at FROM hooks`
	args := []any{}
	if event != "" {
		query += ` WHERE event = ?`
		args = append(args, event)
	}
	query += ` ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list hooks: %w", err)
	}
	defer rows.Close()

	hooks := make([]Hook, 0)
	for rows.Next() {
		var h Hook
		if err := rows.Scan(&h.ID, &h.Event, &h.RunCmd, &h.CWD, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan hook: %w", err)
		}
		hooks = append(hooks, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hooks: %w", err)
	}
	return hooks, nil
}

func (s *Store) AddHook(ctx context.Context, event, runCmd, cwd string) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO hooks(event, run_cmd, cwd, created_at) VALUES(?, ?, ?, ?)`,
		event,
		runCmd,
		cwd,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert hook: %w", err)
	}
	return nil
}

func (s *Store) RemoveHook(ctx context.Context, hookID int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM hooks WHERE id = ?`, hookID)
	if err != nil {
		return fmt.Errorf("delete hook: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("hook rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("hook not found: %d", hookID)
	}
	return nil
}
