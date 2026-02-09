package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	defaultUIPort = 8787
)

type Config struct {
	UIPort      int    `toml:"ui_port"`
	OpenBrowser bool   `toml:"open_browser"`
	GHRepo      string `toml:"gh_repo"`
	SyncAuto    bool   `toml:"sync_auto"`
}

func Default() Config {
	return Config{
		UIPort: defaultUIPort,
	}
}

func HomeDir() (string, error) {
	if v := os.Getenv("TRACK_HOME"); v != "" {
		return v, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".track"), nil
}

func ConfigPath() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.toml"), nil
}

func EnsureDir() error {
	home, err := HomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("create track home: %w", err)
	}
	return nil
}

func Load() (Config, error) {
	if err := EnsureDir(); err != nil {
		return Config{}, err
	}

	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := Save(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	} else if err != nil {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.UIPort == 0 {
		cfg.UIPort = defaultUIPort
	}

	return cfg, nil
}

func Save(cfg Config) error {
	if err := EnsureDir(); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

func Get(cfg Config, key string) (string, error) {
	switch key {
	case "ui_port":
		return fmt.Sprintf("%d", cfg.UIPort), nil
	case "open_browser":
		if cfg.OpenBrowser {
			return "true", nil
		}
		return "false", nil
	case "gh_repo":
		return cfg.GHRepo, nil
	case "sync_auto":
		if cfg.SyncAuto {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func Set(cfg *Config, key, value string) error {
	switch key {
	case "ui_port":
		var v int
		if _, err := fmt.Sscanf(value, "%d", &v); err != nil || v <= 0 || v > 65535 {
			return fmt.Errorf("invalid ui_port: %s", value)
		}
		cfg.UIPort = v
		return nil
	case "open_browser":
		switch value {
		case "true":
			cfg.OpenBrowser = true
		case "false":
			cfg.OpenBrowser = false
		default:
			return fmt.Errorf("invalid open_browser: %s", value)
		}
		return nil
	case "gh_repo":
		cfg.GHRepo = value
		return nil
	case "sync_auto":
		switch value {
		case "true":
			cfg.SyncAuto = true
		case "false":
			cfg.SyncAuto = false
		default:
			return fmt.Errorf("invalid sync_auto: %s", value)
		}
		return nil
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}

func ValidKeys() []string {
	return []string{"ui_port", "open_browser", "gh_repo", "sync_auto"}
}
