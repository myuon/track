package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/myuon/track/internal/issue"
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
