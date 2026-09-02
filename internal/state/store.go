package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultMaxLogBytes int64 = 5 << 20

type Event struct {
	Timestamp     int64  `json:"ts"`
	RequestID     string `json:"request_id,omitempty"`
	Transport     string `json:"transport"`
	IP            string `json:"ip,omitempty"`
	Path          string `json:"path"`
	UserAgent     string `json:"ua,omitempty"`
	IsPS4         bool   `json:"is_ps4"`
	Stage         string `json:"stage"`
	Phase         string `json:"phase"`
	Status        int    `json:"status,omitempty"`
	Bytes         int64  `json:"bytes,omitempty"`
	ExpectedBytes int64  `json:"expected_bytes,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Error         string `json:"error,omitempty"`
}

type ConsoleState struct {
	Timestamp     int64  `json:"ts"`
	IP            string `json:"ip,omitempty"`
	Path          string `json:"path"`
	UserAgent     string `json:"ua,omitempty"`
	IsPS4         bool   `json:"is_ps4"`
	Stage         string `json:"stage"`
	Phase         string `json:"phase,omitempty"`
	Transport     string `json:"transport,omitempty"`
	Status        int    `json:"status,omitempty"`
	Bytes         int64  `json:"bytes,omitempty"`
	ExpectedBytes int64  `json:"expected_bytes,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
}

type Store struct {
	dir         string
	maxLogBytes int64
	mu          sync.Mutex
}

func New(dir string) *Store {
	return &Store{dir: dir, maxLogBytes: defaultMaxLogBytes}
}

func (store *Store) Record(event Event) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	event.Path = redactPath(event.Path)
	event.UserAgent = sanitize(event.UserAgent, 256)
	event.Error = sanitize(event.Error, 512)
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := store.rotateIfNeeded(); err != nil {
		return err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(store.eventsPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	if _, err := logFile.Write(append(encoded, '\n')); err != nil {
		logFile.Close()
		return fmt.Errorf("append event log: %w", err)
	}
	if err := logFile.Close(); err != nil {
		return err
	}

	if !event.IsPS4 {
		return nil
	}
	console := ConsoleState{
		Timestamp:     event.Timestamp,
		IP:            event.IP,
		Path:          event.Path,
		UserAgent:     event.UserAgent,
		IsPS4:         event.IsPS4,
		Stage:         event.Stage,
		Phase:         event.Phase,
		Transport:     event.Transport,
		Status:        event.Status,
		Bytes:         event.Bytes,
		ExpectedBytes: event.ExpectedBytes,
		RequestID:     event.RequestID,
	}
	return store.writeConsole(console)
}

func (store *Store) Console() (ConsoleState, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	b, err := os.ReadFile(store.consolePath())
	if err != nil {
		return ConsoleState{}, err
	}
	var value ConsoleState
	if err := json.Unmarshal(b, &value); err != nil {
		return ConsoleState{}, fmt.Errorf("decode console state: %w", err)
	}
	return value, nil
}

func (store *Store) Recent(maxAge time.Duration, now time.Time) (ConsoleState, bool, error) {
	value, err := store.Console()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConsoleState{}, false, nil
		}
		return ConsoleState{}, false, err
	}
	if value.Timestamp <= 0 || now.Sub(time.Unix(value.Timestamp, 0)) > maxAge {
		return value, false, nil
	}
	return value, true, nil
}

func (store *Store) Events(limit int) ([]Event, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	file, err := os.Open(store.eventsPath())
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if limit < 1 {
		limit = 100
	}
	result := make([]Event, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if len(result) == limit {
			copy(result, result[1:])
			result[len(result)-1] = event
		} else {
			result = append(result, event)
		}
	}
	return result, scanner.Err()
}

func (store *Store) eventsPath() string  { return filepath.Join(store.dir, "events.log") }
func (store *Store) consolePath() string { return filepath.Join(store.dir, "console.json") }

func (store *Store) writeConsole(value ConsoleState) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(store.dir, ".console-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, store.consolePath())
}

func (store *Store) rotateIfNeeded() error {
	info, err := os.Stat(store.eventsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Size() < store.maxLogBytes {
		return nil
	}
	rotated := store.eventsPath() + ".1"
	_ = os.Remove(rotated)
	return os.Rename(store.eventsPath(), rotated)
}

func sanitize(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	if len(value) > limit {
		value = value[:limit]
	}
	return value
}

func redactPath(value string) string {
	parts := strings.Split(value, "/")
	for i := range parts {
		if i > 0 && parts[i-1] == "s" && len(parts[i]) >= 20 {
			parts[i] = "[session]"
		}
	}
	return sanitize(strings.Join(parts, "/"), 512)
}
