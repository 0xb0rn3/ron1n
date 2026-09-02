package config

import (
	"path/filepath"
	"testing"

	"github.com/0xb0rn3/ron1n/internal/platform"
)

func TestSaveLoadAndEnvironmentCompatibility(t *testing.T) {
	paths := platform.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "config"),
		DataDir:   filepath.Join(t.TempDir(), "data"),
		StateDir:  filepath.Join(t.TempDir(), "state"),
		CacheDir:  filepath.Join(t.TempDir(), "cache"),
	}
	cfg := Defaults(paths)
	name := File(paths)
	if err := Save(name, cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RON1N_PORT", "8181")
	t.Setenv("RON1N_RECENT_HTTP_SECONDS", "45")
	loaded, err := Load(name, Defaults(paths))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Listen != "0.0.0.0:8181" || loaded.RecentHTTP != 45 {
		t.Fatalf("environment compatibility not applied: %#v", loaded)
	}
}

func TestInvalidEnvironment(t *testing.T) {
	paths := platform.Paths{ConfigDir: t.TempDir(), DataDir: t.TempDir(), StateDir: t.TempDir(), CacheDir: t.TempDir()}
	t.Setenv("RON1N_PORT", "70000")
	if _, err := ApplyEnv(Defaults(paths)); err == nil {
		t.Fatal("invalid legacy port was accepted")
	}
}
