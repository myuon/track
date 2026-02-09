package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestListAndDetailHandlers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it, err := store.CreateIssue(ctx, issue.Item{Title: "UI issue", Status: issue.StatusTodo, Priority: "p2"})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	h := NewHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), it.ID) {
		t.Fatalf("list should contain issue id: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/issues/"+it.ID, nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("detail status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "UI issue") {
		t.Fatalf("detail should contain title: %s", rr.Body.String())
	}
}
