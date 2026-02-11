package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
	"github.com/myuon/track/internal/store/sqlite"
)

func TestCSVRoundTrip(t *testing.T) {
	items := []issue.Item{{
		ID:         "TRK-1",
		Title:      "A",
		Status:     issue.StatusTodo,
		Priority:   "p1",
		Assignee:   "alice",
		Due:        "2026-02-10",
		Labels:     []string{"ready", "ui"},
		NextAction: "Write PR",
		Body:       "Body",
	}}

	var buf bytes.Buffer
	if err := writeCSVExport(&buf, items); err != nil {
		t.Fatalf("writeCSVExport() error: %v", err)
	}

	got, err := readCSVImport(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("readCSVImport() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Title != "A" || got[0].Priority != "p1" {
		t.Fatalf("unexpected record: %+v", got[0])
	}
}

func TestJSONLImport(t *testing.T) {
	raw := `{"Title":"A","Status":"todo","Priority":"p2"}
{"Title":"B","Status":"done","Priority":"p0"}
`
	items, err := readJSONLImport(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("readJSONLImport() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
}

func TestCSVRoundTripWithNonePriority(t *testing.T) {
	items := []issue.Item{{
		ID:       "TRK-2",
		Title:    "B",
		Status:   issue.StatusTodo,
		Priority: "none",
	}}

	var buf bytes.Buffer
	if err := writeCSVExport(&buf, items); err != nil {
		t.Fatalf("writeCSVExport() error: %v", err)
	}
	got, err := readCSVImport(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("readCSVImport() error: %v", err)
	}
	if len(got) != 1 || got[0].Priority != "none" {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestJSONExportWritesArray(t *testing.T) {
	items := []issue.Item{
		{ID: "TRK-1", Title: "A", Status: issue.StatusTodo, Priority: "p1", Labels: []string{"io"}},
		{ID: "TRK-2", Title: "B", Status: issue.StatusReady, Priority: "none"},
	}

	var buf bytes.Buffer
	if err := writeJSONExport(&buf, items); err != nil {
		t.Fatalf("writeJSONExport() error: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "[") {
		t.Fatalf("export must start with array token: %q", buf.String())
	}

	var got []issue.Item
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if len(got) != 2 || got[0].Title != "A" || got[1].Status != issue.StatusReady {
		t.Fatalf("unexpected export payload: %+v", got)
	}
}

func TestReadJSONImport(t *testing.T) {
	raw := `[
  {"Title":"A","Status":"todo","Priority":"p2","Labels":["ready","ui"],"Body":"Body A"},
  {"Title":"B","Status":"in_progress","Priority":"p1","Assignee":"agent"}
]`
	items, err := readJSONImport(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("readJSONImport() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Body != "Body A" || len(items[0].Labels) != 2 {
		t.Fatalf("unexpected first record: %+v", items[0])
	}
	if items[1].Assignee != "agent" || items[1].Priority != "p1" {
		t.Fatalf("unexpected second record: %+v", items[1])
	}
}

func TestReadJSONImportRejectsInvalidJSON(t *testing.T) {
	_, err := readJSONImport(strings.NewReader(`[{"Title":"A",]`))
	if err == nil {
		t.Fatalf("expected invalid JSON error")
	}
}

func TestReadJSONImportRejectsNonArray(t *testing.T) {
	_, err := readJSONImport(strings.NewReader(`{"Title":"A"}`))
	if err == nil {
		t.Fatalf("expected non-array JSON error")
	}
}

func TestImportCmdJSONDryRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	inPath := filepath.Join(tmp, "issues.json")
	if err := os.WriteFile(inPath, []byte(`[{"Title":"A","Status":"todo","Priority":"p2"}]`), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	cmd := newImportCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "json", "--dry-run", inPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}
	if !strings.Contains(out.String(), "dry-run: 1 issues") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("sqlite.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	items, err := store.ListIssues(ctx, sqlite.ListFilter{Sort: "manual"})
	if err != nil {
		t.Fatalf("ListIssues() error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("dry-run must not write issues, got=%d", len(items))
	}
}

func TestImportCmdJSONDefaultsStatusAndPriority(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	inPath := filepath.Join(tmp, "issues.json")
	raw := `[{"Title":"Imported","Status":"","Priority":"","Labels":["ready","ui"],"Body":"import body"}]`
	if err := os.WriteFile(inPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	cmd := newImportCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "json", inPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error: %v", err)
	}
	if !strings.Contains(out.String(), "imported: 1 issues") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	ctx := context.Background()
	store, err := sqlite.Open(ctx)
	if err != nil {
		t.Fatalf("sqlite.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	items, err := store.ListIssues(ctx, sqlite.ListFilter{Sort: "manual"})
	if err != nil {
		t.Fatalf("ListIssues() error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Status != issue.StatusTodo || items[0].Priority != "none" {
		t.Fatalf("defaults were not applied: %+v", items[0])
	}
	if items[0].Title != "Imported" || items[0].Body != "import body" {
		t.Fatalf("imported fields mismatch: %+v", items[0])
	}
	if strings.Join(items[0].Labels, ",") != "ready,ui" {
		t.Fatalf("labels mismatch: %+v", items[0].Labels)
	}
}
