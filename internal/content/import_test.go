package content

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGitHubRepository(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"kmeps4/PSFree", "https://github.com/kmeps4/PSFree.git"} {
		owner, repo, err := ParseGitHubRepository(value)
		if err != nil || owner != "kmeps4" || repo != "PSFree" {
			t.Fatalf("ParseGitHubRepository(%q) = %q, %q, %v", value, owner, repo, err)
		}
	}
	for _, value := range []string{"https://example.com/a/b", "a/b/c", "../bad"} {
		if _, _, err := ParseGitHubRepository(value); err == nil {
			t.Fatalf("unsafe repository accepted: %q", value)
		}
	}
}

func TestImportArchive(t *testing.T) {
	t.Parallel()
	files := make(map[string][]byte)
	for _, name := range RequiredPSFree900 {
		files[name] = []byte("fixture:" + name)
	}
	archive := tarGzipFixture(t, files, false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	result, err := importArchive(context.Background(), server.URL, "owner/repo", "0123456789012345678901234567890123456789", ImportOptions{
		DestinationRoot: t.TempDir(),
		Profile:         "psfree-lapse-900",
		Client:          server.Client(),
		Now:             time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(result.Directory, "goldhen.bin")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFiles(result.Directory, result.Envelope.Manifest, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestImportRejectsArchiveSymlink(t *testing.T) {
	t.Parallel()
	archive := tarGzipFixture(t, map[string][]byte{"index.html": []byte("x")}, true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	_, err := importArchive(context.Background(), server.URL, "owner/repo", "abcdef", ImportOptions{
		DestinationRoot: t.TempDir(),
		Profile:         "static",
		Client:          server.Client(),
	})
	if err == nil {
		t.Fatal("archive symlink was accepted")
	}
}

func tarGzipFixture(t *testing.T, files map[string][]byte, symlink bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "repo-root/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	for name, value := range files {
		header := &tar.Header{Name: "repo-root/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(value))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(value); err != nil {
			t.Fatal(err)
		}
	}
	if symlink {
		if err := tarWriter.WriteHeader(&tar.Header{Name: "repo-root/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
