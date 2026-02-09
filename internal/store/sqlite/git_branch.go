package sqlite

import (
	"context"
	"fmt"
	"time"
)

type GitBranchLink struct {
	IssueID    string
	BranchName string
	CreatedAt  string
	UpdatedAt  string
}

func (s *Store) UpsertGitBranchLink(ctx context.Context, issueID, branchName string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO git_branch_links(issue_id, branch_name, created_at, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(issue_id) DO UPDATE SET
			branch_name=excluded.branch_name,
			updated_at=excluded.updated_at
	`, issueID, branchName, now, now)
	if err != nil {
		return fmt.Errorf("upsert git branch link: %w", err)
	}
	return nil
}

func (s *Store) GetGitBranchLink(ctx context.Context, issueID string) (GitBranchLink, error) {
	var out GitBranchLink
	err := s.db.QueryRowContext(ctx, `SELECT issue_id, branch_name, created_at, updated_at FROM git_branch_links WHERE issue_id = ?`, issueID).
		Scan(&out.IssueID, &out.BranchName, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return GitBranchLink{}, fmt.Errorf("get git branch link: %w", err)
	}
	return out, nil
}

func (s *Store) DeleteGitBranchLink(ctx context.Context, issueID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM git_branch_links WHERE issue_id = ?`, issueID)
	if err != nil {
		return fmt.Errorf("delete git branch link: %w", err)
	}
	return nil
}
