package platform

import (
	"path/filepath"
	"testing"
)

func TestPathsForPlatforms(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"APPDATA":         `C:\Users\zoro\AppData\Roaming`,
		"LOCALAPPDATA":    `C:\Users\zoro\AppData\Local`,
		"XDG_CONFIG_HOME": "/xdg/config",
		"XDG_DATA_HOME":   "/xdg/data",
		"XDG_STATE_HOME":  "/xdg/state",
		"XDG_CACHE_HOME":  "/xdg/cache",
	}
	getenv := func(name string) string { return env[name] }

	linux := PathsFor("linux", "/home/zoro", getenv)
	if linux.ConfigDir != "/xdg/config/ron1n" || linux.StateDir != "/xdg/state/ron1n" {
		t.Fatalf("Linux paths = %#v", linux)
	}
	windows := PathsFor("windows", `C:\Users\zoro`, getenv)
	if filepath.Base(windows.ConfigDir) != "ron1n" || filepath.Base(windows.StateDir) != "state" {
		t.Fatalf("Windows paths = %#v", windows)
	}
	darwin := PathsFor("darwin", "/Users/zoro", getenv)
	if darwin.ConfigDir != "/Users/zoro/Library/Application Support/ron1n/config" || darwin.CacheDir != "/Users/zoro/Library/Caches/ron1n" {
		t.Fatalf("macOS paths = %#v", darwin)
	}
}
