package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestHealthz(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	h := NewHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	assertJSONContentType(t, rr)
	if strings.TrimSpace(rr.Body.String()) != `{"ok":true}` {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestIssuesListDefaultExcludesDoneAndArchived(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mustCreateIssue(t, ctx, store, "todo issue", issue.StatusTodo, "p2")
	mustCreateIssue(t, ctx, store, "done issue", issue.StatusDone, "p2")
	mustCreateIssue(t, ctx, store, "archived issue", issue.StatusArchived, "p2")

	h := NewHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	assertJSONContentType(t, rr)

	var resp struct {
		Items []issueResponse `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].Status != issue.StatusTodo {
		t.Fatalf("status = %q, want %q", resp.Items[0].Status, issue.StatusTodo)
	}
}

func TestGetIssueAcceptsTRKAndNumericID(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it := mustCreateIssue(t, ctx, store, "lookup me", issue.StatusTodo, "p2")
	h := NewHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/issues/"+it.ID, nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("TRK status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/issues/"+strings.TrimPrefix(it.ID, "TRK-"), nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("numeric status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPatchIssueSuccess(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it := mustCreateIssue(t, ctx, store, "before", issue.StatusTodo, "p2")
	h := NewHandler()

	body := `{"title":"after","status":"in_progress","priority":"p1","next_action":"ship it"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/issues/"+it.ID, bytes.NewBufferString(body))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	assertJSONContentType(t, rr)

	var got issueResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Title != "after" {
		t.Fatalf("title = %q, want %q", got.Title, "after")
	}
	if got.Status != issue.StatusInProgress {
		t.Fatalf("status = %q, want %q", got.Status, issue.StatusInProgress)
	}
	if got.Priority != "p1" {
		t.Fatalf("priority = %q, want %q", got.Priority, "p1")
	}
	if got.NextAction != "ship it" {
		t.Fatalf("next_action = %q, want %q", got.NextAction, "ship it")
	}
}

func TestPatchIssueValidationError(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	it := mustCreateIssue(t, ctx, store, "before", issue.StatusTodo, "p2")
	h := NewHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/issues/"+it.ID, bytes.NewBufferString(`{"priority":"p9"}`))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	assertJSONContentType(t, rr)
	if !strings.Contains(rr.Body.String(), "invalid priority") {
		t.Fatalf("body should mention invalid priority, got: %q", rr.Body.String())
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	t.Setenv("TRACK_HOME", t.TempDir())
	h := NewHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/issues/TRK-9999", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	assertJSONContentType(t, rr)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/issues/TRK-1", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
	assertJSONContentType(t, rr)
}

func mustCreateIssue(t *testing.T, ctx context.Context, store *sqlite.Store, title, status, priority string) issue.Item {
	t.Helper()
	it, err := store.CreateIssue(ctx, issue.Item{Title: title, Status: status, Priority: priority})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	return it
}

func assertJSONContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	got := rr.Header().Get("Content-Type")
	if got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q, want %q", got, "application/json; charset=utf-8")
	}
}
