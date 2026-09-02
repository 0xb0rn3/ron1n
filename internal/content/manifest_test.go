package content

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"/", "index.html", true},
		{"/cache.html", "cache.html", true},
		{"fonts\\LiberationMono-Regular.ttf", "fonts/LiberationMono-Regular.ttf", true},
		{"module/../goldhen.bin", "", false},
		{"../secret", "", false},
		{".git/config", "", false},
		{"a/.git/config", "", false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			got, err := Normalize(test.input)
			if (err == nil) != test.ok || got != test.want {
				t.Fatalf("Normalize(%q) = %q, %v; want %q, ok=%v", test.input, got, err, test.want, test.ok)
			}
		})
	}
}

func TestBuildSignVerify(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range RequiredPSFree900 {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	manifest, err := Build(root, "psfree-lapse-900", "test", "abc", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFiles(root, manifest, now); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Sign(manifest, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(envelope, public); err != nil {
		t.Fatal(err)
	}
	envelope.Manifest.Files[0].Size++
	if err := VerifySignature(envelope, public); err == nil {
		t.Fatal("tampered manifest signature was accepted")
	}
}

func TestBuildRejectsSymlink(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("symlink creation may require Windows privileges")
	}
	root := t.TempDir()
	if err := os.Symlink("outside", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root, "static", "test", "abc", time.Now(), time.Time{}); err == nil {
		t.Fatal("symlink bundle was accepted")
	}
}

func TestMIMEAndStageCompatibility(t *testing.T) {
	t.Parallel()
	if got := MIME("psfree_lapse.cache"); got != "text/cache-manifest; charset=utf-8" {
		t.Fatalf("unexpected cache MIME: %s", got)
	}
	if got := MIME("lapse.mjs"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("unexpected module MIME: %s", got)
	}
	if got := Stage("goldhen.bin"); got != "goldhen-payload-served" {
		t.Fatalf("unexpected GoldHEN stage: %s", got)
	}
}
