package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	appconfig "github.com/myuon/track/internal/config"
	_ "modernc.org/sqlite"
)

const (
	driverName = "sqlite"
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

	if err := db.PingContext(ctx); err != nil {
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
		`INSERT INTO meta(key, value) VALUES('next_issue_number', '1')
		 ON CONFLICT(key) DO NOTHING;`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return nil
}

func (s *Store) NextIssueID(ctx context.Context) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'next_issue_number'`).Scan(&n); err != nil {
		_ = tx.Rollback()
		return "", fmt.Errorf("read next issue number: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE meta SET value = ? WHERE key = 'next_issue_number'`, n+1); err != nil {
		_ = tx.Rollback()
		return "", fmt.Errorf("update next issue number: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit next issue number: %w", err)
	}

	return fmt.Sprintf("I-%06d", n), nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
