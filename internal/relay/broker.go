package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrHostOffline    = errors.New("host request queue is full")
	ErrRequestGone    = errors.New("request is no longer active")
	ErrSessionInvalid = errors.New("session is invalid or expired")
	ErrSessionLimit   = errors.New("session limit reached")
)

const (
	maxSessionInFlight = 8
	maxSessionsPerHost = 128
	maxSessionsGlobal  = 4096
)

type Session struct {
	ID          string
	HostID      string
	BundleID    string
	TokenHash   string
	ExpiresAt   time.Time
	MaxRequests int
	MaxBytes    int64
	Requests    int
	Bytes       int64
	inFlight    int
	reserved    int64
}

type pending struct {
	hostID    string
	request   FetchRequest
	response  chan FetchResponse
	ack       chan error
	cancelled chan struct{}
	once      sync.Once
	session   string
	reserved  int64
	responded bool
	cancelErr error
}

func (p *pending) cancel(err error) {
	p.once.Do(func() {
		p.cancelErr = err
		close(p.cancelled)
	})
}

func (p *pending) cancellationError() error {
	if p.cancelErr != nil {
		return p.cancelErr
	}
	return ErrRequestGone
}

type Delivery struct {
	Response FetchResponse
	broker   *Broker
	pending  *pending
	once     sync.Once
}

func (delivery *Delivery) Complete(err error) {
	delivery.once.Do(func() {
		delivery.broker.finish(delivery.pending.request.ID, err == nil)
		select {
		case delivery.pending.ack <- err:
		default:
		}
	})
}

type Broker struct {
	mu       sync.Mutex
	hosts    map[string]chan *pending
	pending  map[string]*pending
	sessions map[string]*Session
	byID     map[string]string
	now      func() time.Time
}

func NewBroker() *Broker {
	return &Broker{
		hosts:    make(map[string]chan *pending),
		pending:  make(map[string]*pending),
		sessions: make(map[string]*Session),
		byID:     make(map[string]string),
		now:      time.Now,
	}
}

func (broker *Broker) CreateSession(hostID, bundleID string, ttl time.Duration, maxRequests int, maxBytes int64) (Session, string, error) {
	if !hostIDPattern.MatchString(hostID) || bundleID == "" {
		return Session{}, "", errors.New("host ID and bundle ID are required")
	}
	if ttl < time.Minute || ttl > 24*time.Hour {
		return Session{}, "", errors.New("session TTL must be between one minute and 24 hours")
	}
	if maxRequests == 0 {
		maxRequests = 512
	}
	if maxBytes == 0 {
		maxBytes = 64 << 20
	}
	if maxRequests < 1 || maxRequests > 10000 || maxBytes < 1 || maxBytes > 4<<30 {
		return Session{}, "", errors.New("session request or byte limit is out of range")
	}
	token, err := randomToken(32)
	if err != nil {
		return Session{}, "", err
	}
	id, err := randomToken(12)
	if err != nil {
		return Session{}, "", err
	}
	digest := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(digest[:])
	session := &Session{
		ID:          id,
		HostID:      hostID,
		BundleID:    bundleID,
		TokenHash:   hash,
		ExpiresAt:   broker.now().Add(ttl),
		MaxRequests: maxRequests,
		MaxBytes:    maxBytes,
	}
	broker.mu.Lock()
	broker.removeExpiredLocked(broker.now())
	if len(broker.sessions) >= maxSessionsGlobal {
		broker.mu.Unlock()
		return Session{}, "", errors.New("relay session capacity reached")
	}
	hostSessions := 0
	for _, active := range broker.sessions {
		if active.HostID == hostID {
			hostSessions++
		}
	}
	if hostSessions >= maxSessionsPerHost {
		broker.mu.Unlock()
		return Session{}, "", errors.New("host session capacity reached")
	}
	broker.sessions[hash] = session
	broker.byID[id] = hash
	broker.mu.Unlock()
	return *session, token, nil
}

func (broker *Broker) Revoke(hostID, sessionID string) bool {
	broker.mu.Lock()
	hash, ok := broker.byID[sessionID]
	if !ok {
		broker.mu.Unlock()
		return false
	}
	session := broker.sessions[hash]
	if session == nil || session.HostID != hostID {
		broker.mu.Unlock()
		return false
	}
	delete(broker.byID, sessionID)
	delete(broker.sessions, hash)
	var cancelled []*pending
	for id, item := range broker.pending {
		if item.request.SessionID == sessionID {
			delete(broker.pending, id)
			cancelled = append(cancelled, item)
		}
	}
	broker.mu.Unlock()
	for _, item := range cancelled {
		item.cancel(ErrSessionInvalid)
	}
	return true
}

func (broker *Broker) SessionForToken(token string) (Session, error) {
	digest := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(digest[:])
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.removeExpiredLocked(broker.now())
	session := broker.sessions[hash]
	if session == nil || !broker.now().Before(session.ExpiresAt) {
		return Session{}, ErrSessionInvalid
	}
	return *session, nil
}

func (broker *Broker) Enqueue(ctx context.Context, session Session, request FetchRequest) (*Delivery, error) {
	broker.mu.Lock()
	stored := broker.sessions[session.TokenHash]
	if stored == nil || stored.ID != session.ID || !broker.now().Before(stored.ExpiresAt) {
		broker.mu.Unlock()
		return nil, ErrSessionInvalid
	}
	if stored.Requests >= stored.MaxRequests || stored.Bytes+stored.reserved >= stored.MaxBytes || stored.inFlight >= maxSessionInFlight {
		broker.mu.Unlock()
		return nil, ErrSessionLimit
	}
	stored.Requests++
	stored.inFlight++
	queue := broker.hosts[session.HostID]
	if queue == nil {
		queue = make(chan *pending, 128)
		broker.hosts[session.HostID] = queue
	}
	p := &pending{
		hostID:    session.HostID,
		request:   request,
		response:  make(chan FetchResponse, 1),
		ack:       make(chan error, 1),
		cancelled: make(chan struct{}),
		session:   session.TokenHash,
	}
	broker.pending[request.ID] = p
	broker.mu.Unlock()

	select {
	case queue <- p:
	case <-ctx.Done():
		broker.cancel(request.ID, ctx.Err())
		return nil, ctx.Err()
	default:
		broker.cancel(request.ID, ErrHostOffline)
		return nil, ErrHostOffline
	}

	select {
	case response := <-p.response:
		return &Delivery{Response: response, broker: broker, pending: p}, nil
	case <-ctx.Done():
		broker.cancel(request.ID, ctx.Err())
		return nil, ctx.Err()
	case <-p.cancelled:
		return nil, p.cancellationError()
	}
}

func (broker *Broker) Poll(ctx context.Context, hostID string) (FetchRequest, error) {
	broker.mu.Lock()
	queue := broker.hosts[hostID]
	if queue == nil {
		queue = make(chan *pending, 128)
		broker.hosts[hostID] = queue
	}
	broker.mu.Unlock()
	for {
		select {
		case p := <-queue:
			select {
			case <-p.cancelled:
				continue
			default:
				return p.request, nil
			}
		case <-ctx.Done():
			return FetchRequest{}, ctx.Err()
		}
	}
}

func (broker *Broker) Respond(ctx context.Context, hostID string, response FetchResponse) error {
	broker.mu.Lock()
	p := broker.pending[response.ID]
	if p == nil || p.hostID != hostID {
		broker.mu.Unlock()
		return ErrRequestGone
	}
	if p.responded {
		broker.mu.Unlock()
		return ErrRequestGone
	}
	session := broker.sessions[p.session]
	responseBytes := int64(len(response.Body))
	if session == nil || !broker.now().Before(session.ExpiresAt) {
		broker.removePendingLocked(p)
		broker.mu.Unlock()
		p.cancel(ErrSessionInvalid)
		return ErrSessionInvalid
	}
	if responseBytes > session.MaxBytes-session.Bytes-session.reserved {
		broker.removePendingLocked(p)
		broker.mu.Unlock()
		p.cancel(ErrSessionLimit)
		return ErrSessionLimit
	}
	p.responded = true
	p.reserved = responseBytes
	session.reserved += responseBytes
	broker.mu.Unlock()
	select {
	case p.response <- response:
	case <-p.cancelled:
		return p.cancellationError()
	case <-ctx.Done():
		broker.cancel(response.ID, ctx.Err())
		return ctx.Err()
	}
	select {
	case err := <-p.ack:
		return err
	case <-p.cancelled:
		return p.cancellationError()
	case <-ctx.Done():
		broker.cancel(response.ID, ctx.Err())
		return ctx.Err()
	}
}

func (broker *Broker) cancel(requestID string, reason error) {
	broker.mu.Lock()
	p := broker.pending[requestID]
	if p != nil {
		broker.removePendingLocked(p)
	}
	broker.mu.Unlock()
	if p != nil {
		p.cancel(reason)
	}
}

func (broker *Broker) finish(requestID string, delivered bool) {
	broker.mu.Lock()
	p := broker.pending[requestID]
	if p != nil {
		if session := broker.sessions[p.session]; session != nil {
			session.reserved -= p.reserved
			if delivered {
				session.Bytes += p.reserved
			}
			if session.inFlight > 0 {
				session.inFlight--
			}
		}
	}
	delete(broker.pending, requestID)
	broker.removeExpiredLocked(broker.now())
	broker.mu.Unlock()
}

func (broker *Broker) removePendingLocked(p *pending) {
	delete(broker.pending, p.request.ID)
	if session := broker.sessions[p.session]; session != nil {
		session.reserved -= p.reserved
		if session.inFlight > 0 {
			session.inFlight--
		}
	}
}

func (broker *Broker) removeExpiredLocked(now time.Time) {
	for hash, session := range broker.sessions {
		if !now.Before(session.ExpiresAt) && session.inFlight == 0 {
			delete(broker.sessions, hash)
			delete(broker.byID, session.ID)
		}
	}
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
