package host

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xb0rn3/ron1n/internal/content"
	"github.com/0xb0rn3/ron1n/internal/state"
)

func fixtureServer(t *testing.T) (*Server, *state.Store, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	files := make(map[string][]byte)
	for _, name := range content.RequiredPSFree900 {
		value := []byte("fixture-body:" + name)
		if name == "goldhen.bin" {
			value = []byte("0123456789abcdef")
		}
		files[name] = value
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, value, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := content.Build(root, "psfree-lapse-900", "fixture", "fixture", time.Now(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	store := state.New(t.TempDir())
	return &Server{Root: root, Manifest: manifest, State: store}, store, files
}

func TestByteExactAndCompatibilityMIME(t *testing.T) {
	server, _, files := fixtureServer(t)
	for _, name := range []string{"index.html", "psfree_lapse.cache", "lapse.mjs", "goldhen.bin"} {
		request := httptest.NewRequest(http.MethodGet, "/"+name, nil)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK || string(response.Body.Bytes()) != string(files[name]) {
			t.Fatalf("GET %s = %d %q", name, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/psfree_lapse.cache", nil))
	if got := response.Header().Get("Content-Type"); got != "text/cache-manifest; charset=utf-8" {
		t.Fatalf("cache MIME = %q", got)
	}
}

func TestGoldHENCompletionSemantics(t *testing.T) {
	server, store, _ := fixtureServer(t)
	ua := "Mozilla/5.0 (PlayStation 4 9.00)"

	head := httptest.NewRequest(http.MethodHead, "/goldhen.bin", nil)
	head.Header.Set("User-Agent", ua)
	server.ServeHTTP(httptest.NewRecorder(), head)
	console, err := store.Console()
	if err != nil {
		t.Fatal(err)
	}
	if console.Phase != "head-complete" {
		t.Fatalf("HEAD recorded as %q", console.Phase)
	}

	missing := httptest.NewRequest(http.MethodGet, "/not-goldhen.bin", nil)
	missing.Header.Set("User-Agent", ua)
	server.ServeHTTP(httptest.NewRecorder(), missing)
	console, _ = store.Console()
	if console.Status != http.StatusNotFound || console.Phase != "response-failed" {
		t.Fatalf("404 completion = %#v", console)
	}

	request := httptest.NewRequest(http.MethodGet, "/goldhen.bin", nil)
	request.Header.Set("User-Agent", ua)
	server.ServeHTTP(httptest.NewRecorder(), request)
	console, _ = store.Console()
	if console.Phase != "artifact-delivered" || console.Bytes != 16 {
		t.Fatalf("full body completion = %#v", console)
	}
}

func TestSingleRangeIsPartial(t *testing.T) {
	server, store, _ := fixtureServer(t)
	request := httptest.NewRequest(http.MethodGet, "/goldhen.bin", nil)
	request.Header.Set("User-Agent", "PlayStation 4")
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("range = %d %q", response.Code, response.Body.String())
	}
	console, _ := store.Console()
	if console.Phase != "artifact-partial" {
		t.Fatalf("range recorded as %q", console.Phase)
	}
}

func TestFailedWriterIsNotDelivered(t *testing.T) {
	server, store, _ := fixtureServer(t)
	request := httptest.NewRequest(http.MethodGet, "/goldhen.bin", nil)
	request.Header.Set("User-Agent", "PlayStation 4")
	server.ServeHTTP(&failingWriter{header: make(http.Header)}, request)
	console, _ := store.Console()
	if console.Phase != "response-failed" {
		t.Fatalf("failed transfer recorded as %q", console.Phase)
	}
}

func TestPrivateStatusAndTraversal(t *testing.T) {
	server, _, _ := fixtureServer(t)
	for _, target := range []string{"/_ron1n/status", "/.git/config", "/../secret"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code == http.StatusOK {
			t.Fatalf("unexpected access to %s", target)
		}
	}
	health := httptest.NewRecorder()
	server.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/_ron1n/health", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), "0.0.1zoro") {
		t.Fatalf("health = %d %q", health.Code, health.Body.String())
	}
}

type failingWriter struct {
	header http.Header
	status int
}

func (w *failingWriter) Header() http.Header    { return w.header }
func (w *failingWriter) WriteHeader(status int) { w.status = status }
func (w *failingWriter) Write(value []byte) (int, error) {
	return 0, errors.New("client disconnected")
}
