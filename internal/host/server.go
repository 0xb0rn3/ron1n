package host

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/0xb0rn3/ron1n/internal/content"
	"github.com/0xb0rn3/ron1n/internal/state"
	"github.com/0xb0rn3/ron1n/internal/version"
)

const DefaultMaxFileBytes int64 = 64 << 20

type Server struct {
	Root         string
	Manifest     content.Manifest
	State        *state.Store
	Transport    string
	MaxFileBytes int64
}

type RequestMeta struct {
	RequestID string
	Transport string
	ClientIP  string
	UserAgent string
}

type Prepared struct {
	Path     string
	Stage    string
	SHA256   string
	Expected int64
	Status   int
	Header   http.Header
	Body     []byte
	Method   string
	FullBody bool
}

func (server *Server) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/_ron1n/health" {
		server.serveHealth(w, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/_ron1n/") {
		http.NotFound(w, request)
		return
	}
	meta := RequestMeta{
		RequestID: newRequestID(),
		Transport: server.transport(),
		ClientIP:  clientIP(request.RemoteAddr),
		UserAgent: request.UserAgent(),
	}
	server.RecordRequested(meta, request.URL.Path)
	prepared, err := server.Prepare(request.Method, request.URL.Path, request.Header)
	if err != nil {
		prepared = errorResponse(request.Method, http.StatusServiceUnavailable, "verified content unavailable\n")
	}
	copyHeaders(w.Header(), prepared.Header)
	w.WriteHeader(prepared.Status)
	var writeErr error
	written := 0
	if request.Method != http.MethodHead && len(prepared.Body) > 0 {
		written, writeErr = w.Write(prepared.Body)
		if writeErr == nil && written != len(prepared.Body) {
			writeErr = io.ErrShortWrite
		}
	}
	server.RecordCompletion(meta, prepared, int64(written), writeErr)
}

func (server *Server) Prepare(method, requestPath string, headers http.Header) (Prepared, error) {
	if method != http.MethodGet && method != http.MethodHead {
		prepared := errorResponse(method, http.StatusMethodNotAllowed, "method not allowed\n")
		prepared.Header.Set("Allow", "GET, HEAD")
		return prepared, nil
	}
	name, err := content.Normalize(requestPath)
	if err != nil {
		return errorResponse(method, http.StatusBadRequest, "bad request path\n"), nil
	}
	entry, ok := server.Manifest.Lookup(name)
	if !ok {
		return errorResponse(method, http.StatusNotFound, "not found\n"), nil
	}
	if entry.Size > server.maxFileBytes() {
		return Prepared{}, fmt.Errorf("content exceeds configured limit: %s", name)
	}
	root, err := os.OpenRoot(server.Root)
	if err != nil {
		return Prepared{}, fmt.Errorf("open verified content root: %w", err)
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		return Prepared{}, fmt.Errorf("open verified content: %w", err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, server.maxFileBytes()+1))
	if err != nil {
		return Prepared{}, fmt.Errorf("read verified content: %w", err)
	}
	if int64(len(body)) > server.maxFileBytes() {
		return Prepared{}, fmt.Errorf("content exceeds configured limit: %s", name)
	}
	if int64(len(body)) != entry.Size {
		return Prepared{}, fmt.Errorf("content size changed after verification: %s", name)
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != entry.SHA256 {
		return Prepared{}, fmt.Errorf("content hash changed after verification: %s", name)
	}
	info, err := file.Stat()
	if err != nil {
		return Prepared{}, err
	}

	w := newBufferResponse()
	setSecurityHeaders(w.header)
	w.header.Set("Content-Type", entry.MIME)
	w.header.Set("ETag", `"sha256-`+entry.SHA256+`"`)
	w.header.Set("Accept-Ranges", "bytes")
	if path.Ext(name) == ".html" || path.Ext(name) == ".cache" {
		w.header.Set("Cache-Control", "no-cache")
	} else {
		w.header.Set("Cache-Control", "public, max-age=3600")
	}
	request := &http.Request{Method: method, Header: allowedRequestHeaders(headers)}
	http.ServeContent(w, request, name, info.ModTime(), bytes.NewReader(body))
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	prepared := Prepared{
		Path:     name,
		Stage:    entry.Stage,
		SHA256:   entry.SHA256,
		Expected: entry.Size,
		Status:   status,
		Header:   w.header.Clone(),
		Body:     w.body.Bytes(),
		Method:   method,
	}
	prepared.FullBody = method == http.MethodGet && status == http.StatusOK && int64(len(prepared.Body)) == entry.Size
	return prepared, nil
}

func (server *Server) RecordRequested(meta RequestMeta, requestPath string) {
	if server.State == nil {
		return
	}
	name, err := content.Normalize(requestPath)
	if err != nil {
		name = "[invalid]"
	}
	entry, ok := server.Manifest.Lookup(name)
	stage := "browser"
	expected := int64(0)
	if ok {
		stage = entry.Stage
		expected = entry.Size
	}
	_ = server.State.Record(state.Event{
		RequestID:     meta.RequestID,
		Transport:     choose(meta.Transport, server.transport()),
		IP:            meta.ClientIP,
		Path:          name,
		UserAgent:     meta.UserAgent,
		IsPS4:         IsPlayStation(meta.UserAgent),
		Stage:         stage,
		Phase:         "artifact-requested",
		ExpectedBytes: expected,
	})
}

func (server *Server) RecordCompletion(meta RequestMeta, prepared Prepared, written int64, transferErr error) {
	if server.State == nil {
		return
	}
	phase := "response-complete"
	if transferErr != nil {
		phase = "response-failed"
	} else if prepared.Method == http.MethodHead {
		phase = "head-complete"
	} else if prepared.Status == http.StatusPartialContent {
		phase = "artifact-partial"
	} else if prepared.FullBody && written == prepared.Expected {
		phase = "artifact-delivered"
	} else if prepared.Status >= 400 {
		phase = "response-failed"
	}
	errorText := ""
	if transferErr != nil {
		errorText = transferErr.Error()
	}
	_ = server.State.Record(state.Event{
		RequestID:     meta.RequestID,
		Transport:     choose(meta.Transport, server.transport()),
		IP:            meta.ClientIP,
		Path:          prepared.Path,
		UserAgent:     meta.UserAgent,
		IsPS4:         IsPlayStation(meta.UserAgent),
		Stage:         prepared.Stage,
		Phase:         phase,
		Status:        prepared.Status,
		Bytes:         written,
		ExpectedBytes: prepared.Expected,
		SHA256:        prepared.SHA256,
		Error:         errorText,
	})
}

func IsPlayStation(userAgent string) bool {
	value := strings.ToLower(userAgent)
	return strings.Contains(value, "playstation") || strings.Contains(value, "ps4")
}

func (server *Server) serveHealth(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setSecurityHeaders(w.Header())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	value := struct {
		Status   string `json:"status"`
		Version  string `json:"version"`
		BundleID string `json:"bundle_id"`
	}{Status: "ok", Version: version.Version, BundleID: server.Manifest.BundleID}
	b, _ := json.Marshal(value)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	if request.Method == http.MethodGet {
		_, _ = w.Write(b)
	}
}

func setSecurityHeaders(header http.Header) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Server", "ron1n/"+version.Version)
}

func allowedRequestHeaders(input http.Header) http.Header {
	output := make(http.Header)
	for _, name := range []string{"Range", "If-None-Match", "If-Modified-Since"} {
		if value := input.Values(name); len(value) > 0 {
			for _, item := range value {
				output.Add(name, item)
			}
		}
	}
	return output
}

func errorResponse(method string, status int, message string) Prepared {
	body := []byte(message)
	if method == http.MethodHead {
		body = nil
	}
	header := make(http.Header)
	setSecurityHeaders(header)
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Length", strconv.Itoa(len(message)))
	return Prepared{Path: "[none]", Stage: "browser", Status: status, Header: header, Body: body, Method: method}
}

func clientIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err == nil {
		return host
	}
	return remote
}

func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func choose(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (server *Server) transport() string {
	if server.Transport == "" {
		return "local"
	}
	return server.Transport
}

func (server *Server) maxFileBytes() int64 {
	if server.MaxFileBytes <= 0 {
		return DefaultMaxFileBytes
	}
	return server.MaxFileBytes
}

func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value)
}

type bufferResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferResponse() *bufferResponse      { return &bufferResponse{header: make(http.Header)} }
func (w *bufferResponse) Header() http.Header { return w.header }
func (w *bufferResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *bufferResponse) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(value)
}

var _ http.ResponseWriter = (*bufferResponse)(nil)
var _ = errors.Is
