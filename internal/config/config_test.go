package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TRACK_HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UIPort != 8787 {
		t.Fatalf("unexpected default ui_port: %d", cfg.UIPort)
	}

	path := filepath.Join(tmp, "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}
}

func TestSetAndGetRoundtrip(t *testing.T) {
	cfg := Default()

	if err := Set(&cfg, "ui_port", "9999"); err != nil {
		t.Fatalf("set ui_port: %v", err)
	}
	if err := Set(&cfg, "open_browser", "true"); err != nil {
		t.Fatalf("set open_browser: %v", err)
	}
	if err := Set(&cfg, "gh_repo", "owner/repo"); err != nil {
		t.Fatalf("set gh_repo: %v", err)
	}
	if err := Set(&cfg, "sync_auto", "true"); err != nil {
		t.Fatalf("set sync_auto: %v", err)
	}

	cases := map[string]string{
		"ui_port":      "9999",
		"open_browser": "true",
		"gh_repo":      "owner/repo",
		"sync_auto":    "true",
	}

	for key, want := range cases {
		got, err := Get(cfg, key)
		if err != nil {
			t.Fatalf("Get(%s) error: %v", key, err)
		}
		if got != want {
			t.Fatalf("Get(%s) = %q, want %q", key, got, want)
		}
	}
}
