package state

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRecordAndRecent(t *testing.T) {
	t.Parallel()
	store := New(t.TempDir())
	now := time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC)
	event := Event{
		Timestamp: now.Unix(),
		Transport: "local",
		IP:        "192.0.2.4",
		Path:      "goldhen.bin",
		UserAgent: "PlayStation 4\nspoof",
		IsPS4:     true,
		Stage:     "goldhen-payload-served",
		Phase:     "artifact-delivered",
		Status:    200,
		Bytes:     10,
	}
	if err := store.Record(event); err != nil {
		t.Fatal(err)
	}
	got, recent, err := store.Recent(30*time.Second, now.Add(29*time.Second))
	if err != nil || !recent {
		t.Fatalf("Recent() = %#v, %v, %v", got, recent, err)
	}
	if strings.Contains(got.UserAgent, "\n") {
		t.Fatal("control character was not sanitized")
	}
	_, recent, err = store.Recent(30*time.Second, now.Add(31*time.Second))
	if err != nil || recent {
		t.Fatalf("stale state was considered recent: %v, %v", recent, err)
	}
}

func TestNonPS4DoesNotReplaceConsole(t *testing.T) {
	t.Parallel()
	store := New(t.TempDir())
	if err := store.Record(Event{IsPS4: false, Path: "index.html", Stage: "browser"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.consolePath()); !os.IsNotExist(err) {
		t.Fatalf("non-PS4 request created console state: %v", err)
	}
}

func TestSessionPathRedaction(t *testing.T) {
	t.Parallel()
	store := New(t.TempDir())
	secret := "abcdefghijklmnopqrstuvwxyz012345"
	if err := store.Record(Event{Path: "/s/" + secret + "/goldhen.bin"}); err != nil {
		t.Fatal(err)
	}
	events, err := store.Events(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || strings.Contains(events[0].Path, secret) {
		t.Fatalf("session token leaked: %#v", events)
	}
}
