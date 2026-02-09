package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/myuon/track/internal/config"
	_ "modernc.org/sqlite"
)

const (
	driverName    = "sqlite"
	issueIDPrefix = "TRK"
)

type Store struct {
	db *sql.DB
}

func DBPath() (string, error) {
	home, err := appconfig.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "track.db"), nil
}

func Open(ctx context.Context) (*Store, error) {
	if err := appconfig.EnsureDir(); err != nil {
		return nil, err
	}

	path, err := DBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := withSQLiteRetry(ctx, func() error {
		return db.PingContext(ctx)
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS issues (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			priority TEXT NOT NULL DEFAULT 'p2',
			assignee TEXT,
			due TEXT,
			labels_json TEXT NOT NULL DEFAULT '[]',
			next_action TEXT,
			body TEXT,
			order_index INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS hooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event TEXT NOT NULL,
			run_cmd TEXT NOT NULL,
			cwd TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS github_links (
			issue_id TEXT PRIMARY KEY,
			pr_ref TEXT NOT NULL,
			repo TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS statuses (
			name TEXT PRIMARY KEY,
			system INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS git_branch_links (
			issue_id TEXT PRIMARY KEY,
			branch_name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`INSERT INTO statuses(name, system, created_at, updated_at)
		 VALUES('todo', 1, datetime('now'), datetime('now'))
		 ON CONFLICT(name) DO NOTHING;`,
		`INSERT INTO statuses(name, system, created_at, updated_at)
		 VALUES('ready', 1, datetime('now'), datetime('now'))
		 ON CONFLICT(name) DO NOTHING;`,
		`INSERT INTO statuses(name, system, created_at, updated_at)
		 VALUES('in_progress', 1, datetime('now'), datetime('now'))
		 ON CONFLICT(name) DO NOTHING;`,
		`INSERT INTO statuses(name, system, created_at, updated_at)
		 VALUES('done', 1, datetime('now'), datetime('now'))
		 ON CONFLICT(name) DO NOTHING;`,
		`INSERT INTO statuses(name, system, created_at, updated_at)
		 VALUES('archived', 1, datetime('now'), datetime('now'))
		 ON CONFLICT(name) DO NOTHING;`,
		`INSERT INTO meta(key, value) VALUES('next_issue_number', '1')
		 ON CONFLICT(key) DO NOTHING;`,
	}

	for _, stmt := range stmts {
		if err := withSQLiteRetry(ctx, func() error {
			_, err := s.db.ExecContext(ctx, stmt)
			return err
		}); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return nil
}

func (s *Store) NextIssueID(ctx context.Context) (string, error) {
	var id string
	err := withSQLiteRetry(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		var n int
		if err := tx.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'next_issue_number'`).Scan(&n); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("read next issue number: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE meta SET value = ? WHERE key = 'next_issue_number'`, n+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update next issue number: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit next issue number: %w", err)
		}

		id = fmt.Sprintf("%s-%d", issueIDPrefix, n)
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func applySQLitePragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA synchronous=NORMAL;`,
		`PRAGMA busy_timeout=5000;`,
	}
	for _, stmt := range pragmas {
		if err := withSQLiteRetry(ctx, func() error {
			_, err := db.ExecContext(ctx, stmt)
			return err
		}); err != nil {
			return fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}
	return nil
}

func withSQLiteRetry(ctx context.Context, fn func() error) error {
	const maxAttempts = 8
	baseDelay := 25 * time.Millisecond
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if !isSQLiteRetryableErr(err) {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * baseDelay):
				continue
			}
		}
		return nil
	}
	return lastErr
}

func isSQLiteRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "sqlite_locked") ||
		strings.Contains(msg, "cannot start a transaction within a transaction") ||
		strings.Contains(msg, "unable to open database file")
}
