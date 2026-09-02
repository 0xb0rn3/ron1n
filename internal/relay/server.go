package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xb0rn3/ron1n/internal/version"
)

const (
	DefaultMaxResponseBytes int64 = 8 << 20
	hardMaxResponseBytes    int64 = 64 << 20
	defaultRequestTimeout         = 75 * time.Second
	maxRateEntries                = 10000
)

type ServerConfig struct {
	ExternalURL      string
	MaxResponseBytes int64
	RequestTimeout   time.Duration
	RatePerMinute    int
}

type HTTPServer struct {
	config ServerConfig
	auth   *Authenticator
	broker *Broker
	rates  *rateLimiter
}

func NewHTTPServer(config ServerConfig, auth *Authenticator, broker *Broker) (*HTTPServer, error) {
	if auth == nil || broker == nil {
		return nil, errors.New("relay authenticator and broker are required")
	}
	external, err := url.Parse(strings.TrimRight(config.ExternalURL, "/"))
	if err != nil || external.Scheme == "" || external.Host == "" || external.User != nil || external.RawQuery != "" || external.Fragment != "" {
		return nil, errors.New("relay external URL must include a scheme and host")
	}
	if external.Scheme != "https" && external.Scheme != "http" {
		return nil, errors.New("relay external URL must use http or https")
	}
	config.ExternalURL = external.String()
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if config.MaxResponseBytes > hardMaxResponseBytes {
		return nil, errors.New("relay response limit may not exceed 64 MiB")
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.RatePerMinute <= 0 {
		config.RatePerMinute = 180
	}
	return &HTTPServer{config: config, auth: auth, broker: broker, rates: newRateLimiter(config.RatePerMinute)}, nil
}

func (server *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ron1n/health", server.health)
	mux.HandleFunc("/v1/agent/poll", server.authenticated(server.poll))
	mux.HandleFunc("/v1/agent/respond", server.authenticated(server.respond))
	mux.HandleFunc("/v1/agent/sessions", server.authenticated(server.createSession))
	mux.HandleFunc("/v1/agent/sessions/", server.authenticated(server.revokeSession))
	mux.HandleFunc("/s/", server.publicFetch)
	return securityMiddleware(mux)
}

type agentHandler func(http.ResponseWriter, *http.Request, string)

func (server *HTTPServer) authenticated(next agentHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		hostID := request.URL.Query().Get("host_id")
		if !server.auth.Authenticate(hostID, request.Header.Get("Authorization")) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="ron1n-relay"`)
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, request, hostID)
	}
}

func (server *HTTPServer) poll(w http.ResponseWriter, request *http.Request, hostID string) {
	if request.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	wait := 25 * time.Second
	if value := request.URL.Query().Get("wait_seconds"); value != "" {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds < 1 || seconds > 30 {
			writeJSONError(w, http.StatusBadRequest, "wait_seconds must be between 1 and 30")
			return
		}
		wait = time.Duration(seconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(request.Context(), wait)
	defer cancel()
	fetch, err := server.broker.Poll(ctx, hostID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSONError(w, http.StatusRequestTimeout, "poll cancelled")
		return
	}
	writeJSON(w, http.StatusOK, fetch)
}

func (server *HTTPServer) respond(w http.ResponseWriter, request *http.Request, hostID string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, server.config.MaxResponseBytes*2)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var response FetchResponse
	if err := decoder.Decode(&response); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid response envelope")
		return
	}
	if response.Protocol != Protocol || response.ID == "" || response.Status < 200 || response.Status > 599 {
		writeJSONError(w, http.StatusBadRequest, "invalid response metadata")
		return
	}
	if int64(len(response.Body)) > server.config.MaxResponseBytes {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "response exceeds relay limit")
		return
	}
	response.Header = allowedResponseHeaders(response.Header)
	ctx, cancel := context.WithTimeout(request.Context(), server.config.RequestTimeout)
	defer cancel()
	if err := server.broker.Respond(ctx, hostID, response); err != nil {
		switch {
		case errors.Is(err, ErrSessionLimit):
			writeJSONError(w, http.StatusTooManyRequests, "session byte limit reached")
		case errors.Is(err, ErrSessionInvalid):
			writeJSONError(w, http.StatusGone, "session expired or revoked")
		case errors.Is(err, ErrRequestGone):
			writeJSONError(w, http.StatusGone, "browser request is no longer active")
		case errors.Is(err, context.DeadlineExceeded):
			writeJSONError(w, http.StatusGatewayTimeout, "browser delivery timed out")
		default:
			writeJSONError(w, http.StatusBadGateway, "browser delivery failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (server *HTTPServer) createSession(w http.ResponseWriter, request *http.Request, hostID string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, 32<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input SessionRequest
	if err := decoder.Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid session request")
		return
	}
	ttl := time.Duration(input.TTLSeconds) * time.Second
	session, token, err := server.broker.CreateSession(hostID, input.BundleID, ttl, input.MaxRequests, input.MaxBytes)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	response := SessionResponse{
		ID:        session.ID,
		URL:       server.config.ExternalURL + "/s/" + token + "/",
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	}
	writeJSON(w, http.StatusCreated, response)
}

func (server *HTTPServer) revokeSession(w http.ResponseWriter, request *http.Request, hostID string) {
	if request.Method != http.MethodDelete {
		methodNotAllowed(w, "DELETE")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/v1/agent/sessions/")
	if id == "" || strings.Contains(id, "/") {
		writeJSONError(w, http.StatusBadRequest, "invalid session ID")
		return
	}
	if !server.broker.Revoke(hostID, id) {
		writeJSONError(w, http.StatusNotFound, "session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (server *HTTPServer) publicFetch(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	ip := remoteIP(request.RemoteAddr)
	if !server.rates.Allow(ip, time.Now()) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	remainder := strings.TrimPrefix(request.URL.Path, "/s/")
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 0 || len(parts[0]) < 32 {
		http.NotFound(w, request)
		return
	}
	session, err := server.broker.SessionForToken(parts[0])
	if err != nil {
		http.NotFound(w, request)
		return
	}
	requestPath := "/"
	if len(parts) == 2 && parts[1] != "" {
		requestPath += parts[1]
	}
	requestID, err := randomToken(12)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "request initialization failed")
		return
	}
	fetch := FetchRequest{
		Protocol:  Protocol,
		ID:        requestID,
		SessionID: session.ID,
		BundleID:  session.BundleID,
		Method:    request.Method,
		Path:      requestPath,
		Header:    allowedBrowserHeaders(request.Header),
		ClientIP:  ip,
		UserAgent: truncate(request.UserAgent(), 256),
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.config.RequestTimeout)
	defer cancel()
	delivery, err := server.broker.Enqueue(ctx, session, fetch)
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionLimit):
			writeJSONError(w, http.StatusTooManyRequests, "session limit reached")
		case errors.Is(err, ErrSessionInvalid):
			http.NotFound(w, request)
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrHostOffline):
			writeJSONError(w, http.StatusGatewayTimeout, "host is unavailable")
		default:
			writeJSONError(w, http.StatusBadGateway, "relay request failed")
		}
		return
	}
	response := delivery.Response
	if response.Protocol != Protocol || response.ID != requestID {
		delivery.Complete(errors.New("agent response did not match request"))
		writeJSONError(w, http.StatusBadGateway, "invalid host response")
		return
	}
	copyHeaders(w.Header(), response.Header)
	if request.Method == http.MethodGet {
		if response.Status == http.StatusNoContent || response.Status == http.StatusNotModified {
			w.Header().Del("Content-Length")
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(response.Body)))
		}
	}
	w.Header().Set("CDN-Cache-Control", "no-store")
	w.Header().Set("Surrogate-Control", "no-store")
	w.WriteHeader(response.Status)
	var writeErr error
	if request.Method != http.MethodHead && len(response.Body) > 0 {
		written, err := w.Write(response.Body)
		if err != nil {
			writeErr = err
		} else if written != len(response.Body) {
			writeErr = io.ErrShortWrite
		}
	}
	delivery.Complete(writeErr)
}

func (server *HTTPServer) health(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	value := map[string]string{"status": "ok", "version": version.Version}
	if request.Method == http.MethodHead {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func allowedBrowserHeaders(input http.Header) http.Header {
	return copyAllowed(input, []string{"Range", "If-None-Match", "If-Modified-Since"})
}

func allowedResponseHeaders(input http.Header) http.Header {
	return copyAllowed(input, []string{
		"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag",
		"Last-Modified", "Cache-Control", "X-Content-Type-Options", "X-Frame-Options",
		"Referrer-Policy", "Cross-Origin-Resource-Policy",
	})
}

func copyAllowed(input http.Header, names []string) http.Header {
	output := make(http.Header)
	for _, name := range names {
		for _, value := range input.Values(name) {
			output.Add(name, truncate(value, 1024))
		}
	}
	return output
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Server", "ron1n-relay/"+version.Version)
		next.ServeHTTP(w, request)
	})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "json encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		destination.Del(name)
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func remoteIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err == nil {
		return host
	}
	return remote
}

func truncate(value string, max int) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
	if len(value) > max {
		return value[:max]
	}
	return value
}

type rateEntry struct {
	window time.Time
	count  int
}

type rateLimiter struct {
	mu    sync.Mutex
	limit int
	items map[string]rateEntry
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, items: make(map[string]rateEntry)}
}

func (limiter *rateLimiter) Allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, exists := limiter.items[key]
	if !exists && len(limiter.items) >= maxRateEntries {
		for item, value := range limiter.items {
			if now.Sub(value.window) >= 2*time.Minute {
				delete(limiter.items, item)
			}
		}
		if len(limiter.items) >= maxRateEntries {
			return false
		}
	}
	if entry.window.IsZero() || now.Sub(entry.window) >= time.Minute {
		entry = rateEntry{window: now}
	}
	if entry.count >= limiter.limit {
		return false
	}
	entry.count++
	limiter.items[key] = entry
	return true
}

func ValidateListenerSecurity(listen, externalURL, certFile, keyFile string, allowInsecure bool) error {
	external, err := url.Parse(externalURL)
	if err != nil || external.Scheme == "" || external.Host == "" || external.User != nil || external.RawQuery != "" || external.Fragment != "" {
		return errors.New("external URL must include a scheme and host")
	}
	if external.Scheme != "https" && external.Scheme != "http" {
		return errors.New("external URL must use http or https")
	}
	if external.Scheme != "https" && !allowInsecure {
		return errors.New("public relay URL must use https; use --allow-insecure-public only for development")
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	loopback := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if !loopback && (certFile == "" || keyFile == "") && !allowInsecure {
		return errors.New("plaintext relay may bind only to loopback; provide TLS or explicitly enable development mode")
	}
	if (certFile == "") != (keyFile == "") {
		return errors.New("both TLS certificate and key are required")
	}
	return nil
}
