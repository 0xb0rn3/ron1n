package relay

import "net/http"

const Protocol = 1

type FetchRequest struct {
	Protocol  int         `json:"protocol"`
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	BundleID  string      `json:"bundle_id"`
	Method    string      `json:"method"`
	Path      string      `json:"path"`
	Header    http.Header `json:"header,omitempty"`
	ClientIP  string      `json:"client_ip,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
}

type FetchResponse struct {
	Protocol int         `json:"protocol"`
	ID       string      `json:"id"`
	Status   int         `json:"status"`
	Header   http.Header `json:"header,omitempty"`
	Body     []byte      `json:"body,omitempty"`
}

type SessionRequest struct {
	BundleID    string `json:"bundle_id"`
	TTLSeconds  int    `json:"ttl_seconds"`
	MaxRequests int    `json:"max_requests,omitempty"`
	MaxBytes    int64  `json:"max_bytes,omitempty"`
}

type SessionResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}
