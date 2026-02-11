package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var projectKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

type Project struct {
	Key         string
	Name        string
	Description string
	IssueCount  int
	CreatedAt   string
	UpdatedAt   string
}

func ValidateProjectKey(key string) error {
	if !projectKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid project key: %s", key)
	}
	return nil
}

func (s *Store) CreateProject(ctx context.Context, key, name, description string) (Project, error) {
	key = strings.TrimSpace(key)
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	if err := ValidateProjectKey(key); err != nil {
		return Project{}, err
	}
	if name == "" {
		return Project{}, fmt.Errorf("project name must not be empty")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects(key, name, description, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?)
	`, key, name, nullable(description), now, now)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed") {
			return Project{}, fmt.Errorf("project already exists: %s", key)
		}
		return Project{}, fmt.Errorf("create project: %w", err)
	}

	return Project{
		Key:         key,
		Name:        name,
		Description: description,
		IssueCount:  0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Store) GetProject(ctx context.Context, key string) (Project, error) {
	key = strings.TrimSpace(key)
	var out Project
	err := s.db.QueryRowContext(ctx, `
		SELECT p.key, p.name, COALESCE(p.description, ''), p.created_at, p.updated_at, COUNT(l.issue_id)
		FROM projects p
		LEFT JOIN project_issue_links l ON p.key = l.project_key
		WHERE p.key = ?
		GROUP BY p.key, p.name, p.description, p.created_at, p.updated_at
	`, key).Scan(&out.Key, &out.Name, &out.Description, &out.CreatedAt, &out.UpdatedAt, &out.IssueCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return Project{}, fmt.Errorf("project not found: %s", key)
		}
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return out, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.key, p.name, COALESCE(p.description, ''), p.created_at, p.updated_at, COUNT(l.issue_id)
		FROM projects p
		LEFT JOIN project_issue_links l ON p.key = l.project_key
		GROUP BY p.key, p.name, p.description, p.created_at, p.updated_at
		ORDER BY p.created_at ASC, p.key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	out := make([]Project, 0)
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.IssueCount); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return out, nil
}

func (s *Store) SetIssueProject(ctx context.Context, issueID, projectKey string) error {
	issueID = strings.TrimSpace(issueID)
	projectKey = strings.TrimSpace(projectKey)

	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return err
	}

	if projectKey == "" {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM project_issue_links WHERE issue_id = ?`, issueID); err != nil {
			return fmt.Errorf("unlink issue project: %w", err)
		}
		return nil
	}

	if err := ValidateProjectKey(projectKey); err != nil {
		return err
	}
	if _, err := s.GetProject(ctx, projectKey); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_issue_links(issue_id, project_key, created_at, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(issue_id) DO UPDATE SET
			project_key=excluded.project_key,
			updated_at=excluded.updated_at
	`, issueID, projectKey, now, now)
	if err != nil {
		return fmt.Errorf("link issue project: %w", err)
	}
	return nil
}

func (s *Store) GetIssueProject(ctx context.Context, issueID string) (string, error) {
	issueID = strings.TrimSpace(issueID)
	var key string
	err := s.db.QueryRowContext(ctx, `SELECT project_key FROM project_issue_links WHERE issue_id = ?`, issueID).Scan(&key)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("get issue project: %w", err)
	}
	return key, nil
}

func (s *Store) DeleteProject(ctx context.Context, key string, force bool) error {
	key = strings.TrimSpace(key)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	var linkCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM project_issue_links WHERE project_key = ?`, key).Scan(&linkCount); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("count project links: %w", err)
	}
	if linkCount > 0 && !force {
		_ = tx.Rollback()
		return fmt.Errorf("project has linked issues: %s (use --force)", key)
	}
	if force {
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_issue_links WHERE project_key = ?`, key); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete project links: %w", err)
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE key = ?`, key)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete project: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete project: read affected rows: %w", err)
	}
	if affected == 0 {
		_ = tx.Rollback()
		return fmt.Errorf("project not found: %s", key)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete project: %w", err)
	}
	return nil
}
