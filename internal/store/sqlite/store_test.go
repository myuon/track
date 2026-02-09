package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/myuon/track/internal/issue"
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

func TestConcurrentCreateIssueAvoidsLockedErrors(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	const workers = 6
	const perWorker = 8

	errCh := make(chan error, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				store, err := Open(ctx)
				if err != nil {
					errCh <- fmt.Errorf("open: %w", err)
					return
				}
				_, err = store.CreateIssue(ctx, issue.Item{
					Title:    fmt.Sprintf("w%d-%d", w, i),
					Status:   issue.StatusTodo,
					Priority: "p2",
				})
				_ = store.Close()
				if err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "database is locked") || strings.Contains(msg, "sqlite_busy") || strings.Contains(msg, "sqlite_locked") {
			t.Fatalf("locked error should be mitigated, got: %v", err)
		}
		t.Fatalf("unexpected concurrent error: %v", err)
	}

	store, err := Open(ctx)
	if err != nil {
		t.Fatalf("Open() final error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	items, err := store.ListIssues(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("ListIssues() error: %v", err)
	}
	want := workers * perWorker
	if len(items) != want {
		t.Fatalf("len(items) = %d, want %d", len(items), want)
	}
}
