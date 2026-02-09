package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	dbPath := filepath.Join(tmp, "track.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("track.db should exist: %v", err)
	}

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestNextIssueIDSequence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	id1, err := store.NextIssueID(ctx)
	if err != nil {
		t.Fatalf("NextIssueID #1 error: %v", err)
	}
	id2, err := store.NextIssueID(ctx)
	if err != nil {
		t.Fatalf("NextIssueID #2 error: %v", err)
	}

	if id1 != "TRK-1" {
		t.Fatalf("id1 = %q, want TRK-1", id1)
	}
	if id2 != "TRK-2" {
		t.Fatalf("id2 = %q, want TRK-2", id2)
	}
}
