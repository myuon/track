package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
)

func TestAPICmdStartsOnConfiguredPort(t *testing.T) {
	orig := apiListenAndServe
	t.Cleanup(func() {
		apiListenAndServe = orig
	})

	var (
		gotAddr string
		gotNil  bool
	)
	apiListenAndServe = func(addr string, handler http.Handler) error {
		gotAddr = addr
		gotNil = handler == nil
		return nil
	}

	cmd := newAPICmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--port", "18888"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if gotAddr != "127.0.0.1:18888" {
		t.Fatalf("listen addr = %q, want %q", gotAddr, "127.0.0.1:18888")
	}
	if gotNil {
		t.Fatalf("handler should not be nil")
	}
	want := fmt.Sprintf("API running at %s\n", "http://127.0.0.1:18888")
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
