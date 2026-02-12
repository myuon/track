package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

type patchIssueRequest struct {
	Title      *string `json:"title"`
	Body       *string `json:"body"`
	Status     *string `json:"status"`
	Priority   *string `json:"priority"`
	Assignee   *string `json:"assignee"`
	Due        *string `json:"due"`
	NextAction *string `json:"next_action"`
}

type issueResponse struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	Assignee   string   `json:"assignee"`
	Due        string   `json:"due"`
	Labels     []string `json:"labels"`
	NextAction string   `json:"next_action"`
	Body       string   `json:"body"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/issues", issuesHandler)
	mux.HandleFunc("/issues/", issueDetailHandler)
	return mux
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func issuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	statuses, err := parseStatusesQuery(r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer store.Close()

	for _, st := range statuses {
		if err := store.ValidateStatus(ctx, st); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	items, err := store.ListIssues(ctx, sqlite.ListFilter{
		Statuses:        statuses,
		ExcludeDone:     len(statuses) == 0,
		ExcludeArchived: len(statuses) == 0,
		Label:           strings.TrimSpace(r.URL.Query().Get("label")),
		Assignee:        issue.NormalizeAssignee(r.URL.Query().Get("assignee")),
		Search:          strings.TrimSpace(r.URL.Query().Get("search")),
		Sort:            strings.TrimSpace(r.URL.Query().Get("sort")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	issues := make([]issueResponse, 0, len(items))
	for _, it := range items {
		issues = append(issues, toIssueResponse(it))
	}
	writeJSON(w, http.StatusOK, map[string][]issueResponse{"items": issues})
}

func issueDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/issues/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	id = normalizeIssueIDArg(id)

	switch r.Method {
	case http.MethodGet:
		getIssueHandler(w, r, id)
	case http.MethodPatch:
		patchIssueHandler(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func getIssueHandler(w http.ResponseWriter, _ *http.Request, id string) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer store.Close()

	item, err := store.GetIssue(ctx, id)
	if err != nil {
		if isNotFoundErr(err) {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toIssueResponse(item))
}

func patchIssueHandler(w http.ResponseWriter, r *http.Request, id string) {
	var req patchIssueRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == nil && req.Body == nil && req.Status == nil && req.Priority == nil && req.Assignee == nil && req.Due == nil && req.NextAction == nil {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer store.Close()

	updated, err := store.UpdateIssue(ctx, id, sqlite.UpdateIssueInput{
		Title:      req.Title,
		Body:       req.Body,
		Status:     req.Status,
		Priority:   req.Priority,
		Assignee:   req.Assignee,
		Due:        req.Due,
		NextAction: req.NextAction,
	})
	if err != nil {
		switch {
		case isNotFoundErr(err):
			writeError(w, http.StatusNotFound, "issue not found")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, toIssueResponse(updated))
}

func normalizeIssueIDArg(raw string) string {
	id := strings.TrimSpace(raw)
	if _, err := strconv.Atoi(id); err == nil {
		return "TRK-" + id
	}
	return id
}

func parseStatusesQuery(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("status filter is empty")
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func toIssueResponse(it issue.Item) issueResponse {
	return issueResponse{
		ID:         it.ID,
		Title:      it.Title,
		Status:     it.Status,
		Priority:   it.Priority,
		Assignee:   it.Assignee,
		Due:        it.Due,
		Labels:     it.Labels,
		NextAction: it.NextAction,
		Body:       it.Body,
		CreatedAt:  it.CreatedAt,
		UpdatedAt:  it.UpdatedAt,
	}
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "issue not found")
}
