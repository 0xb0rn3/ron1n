package relay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/0xb0rn3/ron1n/internal/content"
	"github.com/0xb0rn3/ron1n/internal/host"
	"github.com/0xb0rn3/ron1n/internal/state"
)

func relayFixture(t *testing.T) (*host.Server, *state.Store) {
	t.Helper()
	root := t.TempDir()
	for _, name := range content.RequiredPSFree900 {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		value := []byte("fixture:" + name)
		if name == "goldhen.bin" {
			value = []byte("byte-exact-goldhen")
		}
		if err := os.WriteFile(full, value, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := content.Build(root, "psfree-lapse-900", "fixture", "abc", time.Now(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	store := state.New(t.TempDir())
	return &host.Server{Root: root, Manifest: manifest, State: store}, store
}

func TestProvisionAuthentication(t *testing.T) {
	t.Parallel()
	credentials := filepath.Join(t.TempDir(), "hosts.json")
	token, err := Provision(credentials, "home-host", false)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := LoadAuthenticator(credentials)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Authenticate("home-host", "Bearer "+token) {
		t.Fatal("valid host token was rejected")
	}
	if auth.Authenticate("home-host", "Bearer wrong-token-that-is-long-enough") {
		t.Fatal("invalid host token was accepted")
	}
	if _, err := Provision(credentials, "home-host", false); err == nil {
		t.Fatal("existing token was replaced without rotate")
	}
}

func TestRemoteDeliveryAcrossRelay(t *testing.T) {
	contentHost, store := relayFixture(t)
	credentials := filepath.Join(t.TempDir(), "hosts.json")
	token, err := Provision(credentials, "host-a", false)
	if err != nil {
		t.Fatal(err)
	}
	auth, _ := LoadAuthenticator(credentials)
	broker := NewBroker()
	relayHTTP, err := NewHTTPServer(ServerConfig{ExternalURL: "https://relay.example"}, auth, broker)
	if err != nil {
		t.Fatal(err)
	}
	testServer := httptest.NewServer(relayHTTP.Handler())
	defer testServer.Close()

	agent, err := NewAgent(AgentConfig{
		RelayURL: testServer.URL,
		HostID:   "host-a",
		Token:    token,
		Workers:  4,
	}, contentHost)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = agent.Run(ctx)
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		testServer.CloseClientConnections()
	}()

	_, capability, err := broker.CreateSession("host-a", contentHost.Manifest.BundleID, 5*time.Minute, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, testServer.URL+"/s/"+capability+"/goldhen.bin", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (PlayStation 4 9.00)")
	response, err := testServer.Client().Do(request)
	if err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "byte-exact-goldhen" {
		t.Fatalf("remote GET = %d %q", response.StatusCode, body)
	}
	var console state.ConsoleState
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		console, err = store.Console()
		if err == nil && console.Phase == "artifact-delivered" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if console.Phase != "artifact-delivered" || console.Transport != "relay" {
		t.Fatalf("remote completion = %#v", console)
	}

	rangeRequest, _ := http.NewRequest(http.MethodGet, testServer.URL+"/s/"+capability+"/goldhen.bin", nil)
	rangeRequest.Header.Set("Range", "bytes=0-3")
	rangeResponse, err := testServer.Client().Do(rangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	rangeBody, _ := io.ReadAll(rangeResponse.Body)
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != http.StatusPartialContent || string(rangeBody) != "byte" {
		t.Fatalf("remote range = %d %q", rangeResponse.StatusCode, rangeBody)
	}

}

func TestSessionExpiryRevocationAndMethodPolicy(t *testing.T) {
	contentHost, _ := relayFixture(t)
	credentials := filepath.Join(t.TempDir(), "hosts.json")
	_, _ = Provision(credentials, "host-a", false)
	auth, _ := LoadAuthenticator(credentials)
	broker := NewBroker()
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	broker.now = func() time.Time { return now }
	relayHTTP, _ := NewHTTPServer(ServerConfig{ExternalURL: "https://relay.example"}, auth, broker)
	testServer := httptest.NewServer(relayHTTP.Handler())
	defer testServer.Close()

	session, token, err := broker.CreateSession("host-a", contentHost.Manifest.BundleID, time.Minute, 1, 1024)
	if err != nil {
		t.Fatal(err)
	}
	post, _ := http.NewRequest(http.MethodPost, testServer.URL+"/s/"+token+"/", strings.NewReader("x"))
	response, err := testServer.Client().Do(post)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", response.StatusCode)
	}
	if !broker.Revoke("host-a", session.ID) {
		t.Fatal("session revoke failed")
	}
	response, err = testServer.Client().Get(testServer.URL + "/s/" + token + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("revoked session status = %d", response.StatusCode)
	}

	_, token, _ = broker.CreateSession("host-a", contentHost.Manifest.BundleID, time.Minute, 1, 1024)
	now = now.Add(2 * time.Minute)
	response, err = testServer.Client().Get(testServer.URL + "/s/" + token + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expired session status = %d", response.StatusCode)
	}
}

func TestListenerSecurity(t *testing.T) {
	t.Parallel()
	if err := ValidateListenerSecurity("0.0.0.0:9000", "http://relay.example", "", "", false); err == nil {
		t.Fatal("public plaintext listener was accepted")
	}
	if err := ValidateListenerSecurity("127.0.0.1:9000", "https://relay.example", "", "", false); err != nil {
		t.Fatalf("loopback reverse-proxy listener rejected: %v", err)
	}
}

func TestRelayClientRejectsCrossOriginAndDowngradeRedirects(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("https://relay.example/base")
	if err != nil {
		t.Fatal(err)
	}
	client := exactOriginClient(&http.Client{}, base)
	for _, target := range []string{
		"http://relay.example/redirect",
		"https://sub.relay.example/redirect",
		"https://relay.example:444/redirect",
	} {
		request, _ := http.NewRequest(http.MethodGet, target, nil)
		if err := client.CheckRedirect(request, nil); err == nil {
			t.Fatalf("unsafe redirect to %s was accepted", target)
		}
	}
	sameOrigin, _ := http.NewRequest(http.MethodGet, "https://relay.example:443/redirect", nil)
	if err := client.CheckRedirect(sameOrigin, nil); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
}

func TestSessionByteQuotaIsAtomicallyReserved(t *testing.T) {
	t.Parallel()
	broker := NewBroker()
	session, _, err := broker.CreateSession("host-a", "bundle-a", time.Minute, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type enqueueResult struct {
		delivery *Delivery
		err      error
	}
	firstEnqueue := make(chan enqueueResult, 1)
	go func() {
		delivery, err := broker.Enqueue(ctx, session, FetchRequest{Protocol: Protocol, ID: "request-one", SessionID: session.ID})
		firstEnqueue <- enqueueResult{delivery: delivery, err: err}
	}()
	if _, err := broker.Poll(ctx, "host-a"); err != nil {
		t.Fatal(err)
	}
	firstRespond := make(chan error, 1)
	go func() {
		firstRespond <- broker.Respond(ctx, "host-a", FetchResponse{Protocol: Protocol, ID: "request-one", Status: http.StatusOK, Body: []byte("1234")})
	}()
	first := <-firstEnqueue
	if first.err != nil {
		t.Fatal(first.err)
	}

	secondEnqueue := make(chan enqueueResult, 1)
	go func() {
		delivery, err := broker.Enqueue(ctx, session, FetchRequest{Protocol: Protocol, ID: "request-two", SessionID: session.ID})
		secondEnqueue <- enqueueResult{delivery: delivery, err: err}
	}()
	if _, err := broker.Poll(ctx, "host-a"); err != nil {
		t.Fatal(err)
	}
	if err := broker.Respond(ctx, "host-a", FetchResponse{Protocol: Protocol, ID: "request-two", Status: http.StatusOK, Body: []byte("5678")}); !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("concurrent quota overshoot = %v, want ErrSessionLimit", err)
	}
	if second := <-secondEnqueue; !errors.Is(second.err, ErrSessionLimit) {
		t.Fatalf("second enqueue = %v, want ErrSessionLimit", second.err)
	}
	first.delivery.Complete(nil)
	if err := <-firstRespond; err != nil {
		t.Fatal(err)
	}

	broker.mu.Lock()
	stored := broker.sessions[session.TokenHash]
	if stored.Bytes != 4 || stored.reserved != 0 || stored.inFlight != 0 {
		t.Fatalf("quota accounting = bytes:%d reserved:%d inflight:%d", stored.Bytes, stored.reserved, stored.inFlight)
	}
	broker.mu.Unlock()
}

func TestRemoteSessionCanBeRevokedByClient(t *testing.T) {
	t.Parallel()
	credentials := filepath.Join(t.TempDir(), "hosts.json")
	token, err := Provision(credentials, "host-a", false)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := LoadAuthenticator(credentials)
	if err != nil {
		t.Fatal(err)
	}
	broker := NewBroker()
	server, err := NewHTTPServer(ServerConfig{ExternalURL: "http://relay.example"}, auth, broker)
	if err != nil {
		t.Fatal(err)
	}
	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()
	result, err := CreateRemoteSession(context.Background(), testServer.URL, "host-a", token, "bundle-a", time.Minute, false, testServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	capability, err := url.Parse(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.Trim(capability.Path, "/"), "/")
	if len(parts) != 2 {
		t.Fatalf("capability path = %q", capability.Path)
	}
	if err := RevokeRemoteSession(context.Background(), testServer.URL, "host-a", token, result.ID, false, testServer.Client()); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.SessionForToken(parts[1]); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("revoked capability remained active: %v", err)
	}
}

func TestReadTokenRejectsUnsafeFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := filepath.Join(root, "relay.token")
	token := strings.Repeat("x", 43)
	if err := WriteToken(name, token); err != nil {
		t.Fatal(err)
	}
	if value, err := ReadToken(name); err != nil || value != token {
		t.Fatalf("generated token = %q, %v", value, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(name, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadToken(name); err == nil {
			t.Fatal("group/world-readable token was accepted")
		}
		if err := os.Chmod(name, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "relay-link.token")
	if err := os.Symlink(name, link); err == nil {
		if _, err := ReadToken(link); err == nil {
			t.Fatal("symlink token was accepted")
		}
	}
}
