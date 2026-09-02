package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xb0rn3/ron1n/internal/host"
)

type AgentConfig struct {
	RelayURL  string
	HostID    string
	Token     string
	Workers   int
	AllowHTTP bool
	Client    *http.Client
}

type Agent struct {
	config AgentConfig
	host   *host.Server
}

func NewAgent(config AgentConfig, contentHost *host.Server) (*Agent, error) {
	if contentHost == nil {
		return nil, errors.New("content host is required")
	}
	base, err := validateAgentURL(config.RelayURL, config.AllowHTTP)
	if err != nil {
		return nil, err
	}
	if !hostIDPattern.MatchString(config.HostID) || len(config.Token) < 32 {
		return nil, errors.New("valid host ID and relay token are required")
	}
	if config.Workers == 0 {
		config.Workers = 4
	}
	if config.Workers < 1 || config.Workers > 8 {
		return nil, errors.New("agent workers must be between 1 and 8")
	}
	config.RelayURL = strings.TrimRight(base.String(), "/")
	if config.Client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConnsPerHost = config.Workers + 2
		transport.ResponseHeaderTimeout = 90 * time.Second
		config.Client = &http.Client{Transport: transport}
	}
	config.Client = exactOriginClient(config.Client, base)
	contentHost.Transport = "relay"
	return &Agent{config: config, host: contentHost}, nil
}

func (agent *Agent) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, agent.config.Workers)
	for worker := 0; worker < agent.config.Workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := agent.worker(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

func (agent *Agent) worker(ctx context.Context) error {
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		fetch, ok, err := agent.poll(ctx)
		if err != nil {
			timer := time.NewTimer(backoff + time.Duration(rand.IntN(500))*time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		if !ok {
			continue
		}
		agent.handle(ctx, fetch)
	}
}

func (agent *Agent) poll(ctx context.Context) (FetchRequest, bool, error) {
	endpoint := agent.config.RelayURL + "/v1/agent/poll?host_id=" + url.QueryEscape(agent.config.HostID) + "&wait_seconds=25"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return FetchRequest{}, false, err
	}
	agent.authorize(request)
	response, err := agent.config.Client.Do(request)
	if err != nil {
		return FetchRequest{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return FetchRequest{}, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return FetchRequest{}, false, responseError(response)
	}
	var fetch FetchRequest
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fetch); err != nil {
		return FetchRequest{}, false, fmt.Errorf("decode relay request: %w", err)
	}
	if fetch.Protocol != Protocol || fetch.ID == "" || fetch.BundleID != agent.host.Manifest.BundleID {
		return FetchRequest{}, false, errors.New("relay request has an invalid protocol, ID, or bundle")
	}
	if fetch.Method != http.MethodGet && fetch.Method != http.MethodHead {
		return FetchRequest{}, false, errors.New("relay requested an unsupported method")
	}
	return fetch, true, nil
}

func (agent *Agent) handle(ctx context.Context, fetch FetchRequest) {
	meta := host.RequestMeta{
		RequestID: fetch.ID,
		Transport: "relay",
		ClientIP:  fetch.ClientIP,
		UserAgent: fetch.UserAgent,
	}
	agent.host.RecordRequested(meta, fetch.Path)
	prepared, prepareErr := agent.host.Prepare(fetch.Method, fetch.Path, fetch.Header)
	if prepareErr != nil {
		prepared = host.Prepared{
			Path:   "[integrity-error]",
			Stage:  "browser",
			Status: http.StatusServiceUnavailable,
			Header: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:   []byte("verified content unavailable\n"),
			Method: fetch.Method,
		}
	}
	response := FetchResponse{
		Protocol: Protocol,
		ID:       fetch.ID,
		Status:   prepared.Status,
		Header:   prepared.Header,
		Body:     prepared.Body,
	}
	if int64(len(response.Body)) > DefaultMaxResponseBytes {
		prepared = host.Prepared{
			Path:   prepared.Path,
			Stage:  prepared.Stage,
			Status: http.StatusRequestEntityTooLarge,
			Header: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:   []byte("artifact exceeds the remote relay response limit\n"),
			Method: fetch.Method,
		}
		response.Status = prepared.Status
		response.Header = prepared.Header
		response.Body = prepared.Body
	}
	err := agent.respond(ctx, response)
	written := int64(len(prepared.Body))
	if fetch.Method == http.MethodHead {
		written = 0
	}
	if prepareErr != nil && err == nil {
		err = prepareErr
	}
	agent.host.RecordCompletion(meta, prepared, written, err)
}

func (agent *Agent) respond(ctx context.Context, response FetchResponse) error {
	b, err := json.Marshal(response)
	if err != nil {
		return err
	}
	endpoint := agent.config.RelayURL + "/v1/agent/respond?host_id=" + url.QueryEscape(agent.config.HostID)
	requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	agent.authorize(request)
	request.Header.Set("Content-Type", "application/json")
	responseHTTP, err := agent.config.Client.Do(request)
	if err != nil {
		return err
	}
	defer responseHTTP.Body.Close()
	if responseHTTP.StatusCode != http.StatusNoContent {
		return responseError(responseHTTP)
	}
	return nil
}

func (agent *Agent) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+agent.config.Token)
	request.Header.Set("User-Agent", "ron1n-agent/"+strconv.Itoa(Protocol))
}

func CreateRemoteSession(ctx context.Context, relayURL, hostID, token, bundleID string, ttl time.Duration, allowHTTP bool, client *http.Client) (SessionResponse, error) {
	base, err := validateAgentURL(relayURL, allowHTTP)
	if err != nil {
		return SessionResponse{}, err
	}
	if !hostIDPattern.MatchString(hostID) || len(token) < 32 || bundleID == "" {
		return SessionResponse{}, errors.New("valid host ID, relay token, and bundle ID are required")
	}
	if ttl < time.Minute || ttl > 24*time.Hour {
		return SessionResponse{}, errors.New("session TTL must be between one minute and 24 hours")
	}
	input := SessionRequest{BundleID: bundleID, TTLSeconds: int(ttl / time.Second)}
	b, _ := json.Marshal(input)
	endpoint := strings.TrimRight(base.String(), "/") + "/v1/agent/sessions?host_id=" + url.QueryEscape(hostID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return SessionResponse{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	client = exactOriginClient(client, base)
	response, err := client.Do(request)
	if err != nil {
		return SessionResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return SessionResponse{}, responseError(response)
	}
	var result SessionResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&result); err != nil {
		return SessionResponse{}, err
	}
	return result, nil
}

func RevokeRemoteSession(ctx context.Context, relayURL, hostID, token, sessionID string, allowHTTP bool, client *http.Client) error {
	base, err := validateAgentURL(relayURL, allowHTTP)
	if err != nil {
		return err
	}
	if !hostIDPattern.MatchString(hostID) || len(token) < 32 {
		return errors.New("valid host ID and relay token are required")
	}
	if sessionID == "" || strings.ContainsAny(sessionID, "/\\") {
		return errors.New("valid session ID is required")
	}
	endpoint := strings.TrimRight(base.String(), "/") + "/v1/agent/sessions/" + url.PathEscape(sessionID) + "?host_id=" + url.QueryEscape(hostID)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	client = exactOriginClient(client, base)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return responseError(response)
	}
	return nil
}

func exactOriginClient(input *http.Client, base *url.URL) *http.Client {
	client := *input
	previous := input.CheckRedirect
	expected := origin(base)
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if origin(request.URL) != expected {
			return errors.New("relay redirect rejected: exact origin required")
		}
		if previous != nil {
			return previous(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &client
}

func origin(value *url.URL) string {
	port := value.Port()
	if port == "" {
		switch strings.ToLower(value.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return strings.ToLower(value.Scheme) + "://" + strings.ToLower(value.Hostname()) + ":" + port
}

func validateAgentURL(value string, allowHTTP bool) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(value, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("relay URL must include scheme and host")
	}
	if base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("relay URL may not include credentials, a query, or a fragment")
	}
	if base.Scheme == "https" {
		return base, nil
	}
	if base.Scheme != "http" {
		return nil, errors.New("relay URL must use https")
	}
	host := base.Hostname()
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if !allowHTTP && !loopback {
		return nil, errors.New("remote relay requires https; use --allow-http only for development")
	}
	return base, nil
}

func responseError(response *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(b))
	if message == "" {
		message = response.Status
	}
	return fmt.Errorf("relay returned HTTP %d: %s", response.StatusCode, message)
}
