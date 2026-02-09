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
