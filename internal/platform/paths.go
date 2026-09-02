package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	ConfigDir string
	DataDir   string
	StateDir  string
	CacheDir  string
}

func UserPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home: %w", err)
	}
	return PathsFor(runtime.GOOS, home, os.Getenv), nil
}

func PathsFor(goos, home string, getenv func(string) string) Paths {
	if goos == "windows" {
		local := getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		roaming := getenv("APPDATA")
		if roaming == "" {
			roaming = filepath.Join(home, "AppData", "Roaming")
		}
		return Paths{
			ConfigDir: filepath.Join(roaming, "ron1n"),
			DataDir:   filepath.Join(local, "ron1n", "data"),
			StateDir:  filepath.Join(local, "ron1n", "state"),
			CacheDir:  filepath.Join(local, "ron1n", "cache"),
		}
	}
	if goos == "darwin" {
		applicationSupport := filepath.Join(home, "Library", "Application Support", "ron1n")
		return Paths{
			ConfigDir: filepath.Join(applicationSupport, "config"),
			DataDir:   filepath.Join(applicationSupport, "data"),
			StateDir:  filepath.Join(applicationSupport, "state"),
			CacheDir:  filepath.Join(home, "Library", "Caches", "ron1n"),
		}
	}

	config := getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	data := getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	state := getenv("XDG_STATE_HOME")
	if state == "" {
		state = filepath.Join(home, ".local", "state")
	}
	cache := getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(home, ".cache")
	}
	return Paths{
		ConfigDir: filepath.Join(config, "ron1n"),
		DataDir:   filepath.Join(data, "ron1n"),
		StateDir:  filepath.Join(state, "ron1n"),
		CacheDir:  filepath.Join(cache, "ron1n"),
	}
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.ConfigDir, p.DataDir, p.StateDir, p.CacheDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}
