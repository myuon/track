package sqlite

import (
	"context"
	"fmt"
	"time"
)

type GitHubIssueLink struct {
	IssueID       string
	GHIssueNumber string
	GHIssueURL    string
	Repo          string
	CreatedAt     string
	UpdatedAt     string
}

func (s *Store) UpsertGitHubIssueLink(ctx context.Context, issueID, ghIssueNumber, ghIssueURL, repo string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO github_issue_links(issue_id, gh_issue_number, gh_issue_url, repo, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(issue_id) DO UPDATE SET
			gh_issue_number=excluded.gh_issue_number,
			gh_issue_url=excluded.gh_issue_url,
			repo=excluded.repo,
			updated_at=excluded.updated_at
	`, issueID, ghIssueNumber, ghIssueURL, nullable(repo), now, now)
	if err != nil {
		return fmt.Errorf("upsert github issue link: %w", err)
	}
	return nil
}

func (s *Store) GetGitHubIssueLink(ctx context.Context, issueID string) (GitHubIssueLink, error) {
	var out GitHubIssueLink
	err := s.db.QueryRowContext(ctx, `
		SELECT issue_id, gh_issue_number, gh_issue_url, COALESCE(repo, ''), created_at, updated_at
		FROM github_issue_links
		WHERE issue_id = ?
	`, issueID).Scan(&out.IssueID, &out.GHIssueNumber, &out.GHIssueURL, &out.Repo, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return GitHubIssueLink{}, fmt.Errorf("get github issue link: %w", err)
	}
	return out, nil
}
