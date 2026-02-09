package sqlite

import (
	"context"
	"fmt"
	"time"
)

type GitHubLink struct {
	IssueID   string
	PRRef     string
	Repo      string
	CreatedAt string
	UpdatedAt string
}

func (s *Store) UpsertGitHubLink(ctx context.Context, issueID, prRef, repo string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO github_links(issue_id, pr_ref, repo, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(issue_id) DO UPDATE SET
			pr_ref=excluded.pr_ref,
			repo=excluded.repo,
			updated_at=excluded.updated_at
	`, issueID, prRef, nullable(repo), now, now)
	if err != nil {
		return fmt.Errorf("upsert github link: %w", err)
	}
	return nil
}

func (s *Store) GetGitHubLink(ctx context.Context, issueID string) (GitHubLink, error) {
	var out GitHubLink
	err := s.db.QueryRowContext(ctx, `SELECT issue_id, pr_ref, COALESCE(repo, ''), created_at, updated_at FROM github_links WHERE issue_id = ?`, issueID).
		Scan(&out.IssueID, &out.PRRef, &out.Repo, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return GitHubLink{}, fmt.Errorf("get github link: %w", err)
	}
	return out, nil
}

func (s *Store) ListGitHubLinks(ctx context.Context, repo string) ([]GitHubLink, error) {
	query := `SELECT issue_id, pr_ref, COALESCE(repo, ''), created_at, updated_at FROM github_links`
	args := []any{}
	if repo != "" {
		query += ` WHERE repo = ?`
		args = append(args, repo)
	}
	query += ` ORDER BY issue_id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list github links: %w", err)
	}
	defer rows.Close()

	out := make([]GitHubLink, 0)
	for rows.Next() {
		var l GitHubLink
		if err := rows.Scan(&l.IssueID, &l.PRRef, &l.Repo, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan github link: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate github links: %w", err)
	}
	return out, nil
}
