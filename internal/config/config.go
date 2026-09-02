package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/0xb0rn3/ron1n/internal/platform"
)

const Schema = 1

type Config struct {
	Schema     int         `json:"schema"`
	Listen     string      `json:"listen"`
	ContentDir string      `json:"content_dir"`
	Manifest   string      `json:"manifest"`
	StateDir   string      `json:"state_dir"`
	RecentHTTP int         `json:"recent_http_seconds"`
	Relay      RelayConfig `json:"relay"`
}

type RelayConfig struct {
	URL       string `json:"url,omitempty"`
	HostID    string `json:"host_id,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
	Workers   int    `json:"workers"`
}

func Defaults(p platform.Paths) Config {
	return Config{
		Schema:     Schema,
		Listen:     "0.0.0.0:8080",
		ContentDir: filepath.Join(p.DataDir, "content", "current"),
		Manifest:   filepath.Join(p.DataDir, "content", "current", "ron1n-manifest.json"),
		StateDir:   p.StateDir,
		RecentHTTP: 30,
		Relay: RelayConfig{
			TokenFile: filepath.Join(p.ConfigDir, "relay.token"),
			Workers:   4,
		},
	}
}

func File(p platform.Paths) string { return filepath.Join(p.ConfigDir, "config.json") }

func Load(path string, defaults Config) (Config, error) {
	cfg := defaults
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ApplyEnv(cfg)
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Schema != Schema {
		return Config{}, fmt.Errorf("unsupported config schema %d", cfg.Schema)
	}
	return ApplyEnv(cfg)
}

func ApplyEnv(cfg Config) (Config, error) {
	if value := os.Getenv("RON1N_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("RON1N_PORT must be between 1 and 65535")
		}
		host, _, err := net.SplitHostPort(cfg.Listen)
		if err != nil {
			host = "0.0.0.0"
		}
		cfg.Listen = net.JoinHostPort(host, value)
	}
	if value := os.Getenv("RON1N_RECENT_HTTP_SECONDS"); value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds < 1 || seconds > int((24*time.Hour)/time.Second) {
			return Config{}, fmt.Errorf("RON1N_RECENT_HTTP_SECONDS must be between 1 and 86400")
		}
		cfg.RecentHTTP = seconds
	}
	if value := os.Getenv("RON1N_CONTENT_DIR"); value != "" {
		cfg.ContentDir = value
	}
	if value := os.Getenv("RON1N_RELAY_URL"); value != "" {
		cfg.Relay.URL = value
	}
	return cfg, Validate(cfg)
}

func Validate(cfg Config) error {
	if cfg.Schema != Schema {
		return fmt.Errorf("unsupported config schema %d", cfg.Schema)
	}
	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return fmt.Errorf("invalid listen address %q: %w", cfg.Listen, err)
	}
	if cfg.ContentDir == "" || cfg.Manifest == "" || cfg.StateDir == "" {
		return errors.New("content_dir, manifest, and state_dir are required")
	}
	if cfg.RecentHTTP < 1 {
		return errors.New("recent_http_seconds must be positive")
	}
	if cfg.Relay.Workers < 1 || cfg.Relay.Workers > 32 {
		return errors.New("relay workers must be between 1 and 32")
	}
	return nil
}

func Save(path string, cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create config temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
