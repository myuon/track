package cli

import "testing"

func TestNormalizePRRef(t *testing.T) {
	cases := map[string]string{
		"123":                               "123",
		" https://github.com/a/b/pull/456 ": "456",
		"owner/repo#789":                    "789",
	}
	for in, want := range cases {
		if got := normalizePRRef(in); got != want {
			t.Fatalf("normalizePRRef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRunAndJobFromCheckLink(t *testing.T) {
	runID, jobID, ok := parseRunAndJobFromCheckLink("https://github.com/a/b/actions/runs/123456/job/9876")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if runID != "123456" || jobID != "9876" {
		t.Fatalf("unexpected ids: run=%s job=%s", runID, jobID)
	}
}

func TestSummarizeChecks(t *testing.T) {
	cases := []struct {
		name   string
		checks []ghCheck
		want   string
	}{
		{name: "success", checks: []ghCheck{{State: "success"}}, want: "success"},
		{name: "pending", checks: []ghCheck{{State: "in_progress"}}, want: "pending"},
		{name: "failure", checks: []ghCheck{{State: "success"}, {State: "failure"}}, want: "failure"},
	}
	for _, tc := range cases {
		if got := summarizeChecks(tc.checks); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestTailLines(t *testing.T) {
	raw := "1\n2\n3\n4\n5\n"
	got := tailLines(raw, 2)
	if got != "4\n5" {
		t.Fatalf("tailLines() = %q, want %q", got, "4\n5")
	}
}
