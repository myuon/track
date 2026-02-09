package cli

import "testing"

func TestCLIColorDisabled(t *testing.T) {
	c := cliColor{enabled: false}
	if got := c.status("todo"); got != "todo" {
		t.Fatalf("status(todo) = %q, want plain", got)
	}
	if got := c.priority("p0"); got != "p0" {
		t.Fatalf("priority(p0) = %q, want plain", got)
	}
}

func TestCLIColorEnabled(t *testing.T) {
	c := cliColor{enabled: true}
	if got := c.status("done"); got != "\x1b[32mdone\x1b[0m" {
		t.Fatalf("status(done) = %q", got)
	}
	if got := c.priority("p0"); got != "\x1b[1;31mp0\x1b[0m" {
		t.Fatalf("priority(p0) = %q", got)
	}
}
