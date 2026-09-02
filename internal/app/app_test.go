package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xb0rn3/ron1n/internal/catalog"
)

func TestVersionAndHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	application := New(&stdout, &stderr)
	if code := application.Run(context.Background(), []string{"version"}); code != 0 {
		t.Fatalf("version exit = %d", code)
	}
	if stdout.String() != "0.0.1zoro\n" {
		t.Fatalf("version = %q", stdout.String())
	}
	stdout.Reset()
	if code := application.Run(context.Background(), []string{"help"}); code != 0 || !strings.Contains(stdout.String(), "cross-network relay") {
		t.Fatalf("help exit/content = %d %q", code, stdout.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := New(&stdout, &stderr).Run(context.Background(), []string{"nope"}); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unknown command error = %q", stderr.String())
	}
}

func TestSourcesJSON(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := New(&stdout, &stderr).Run(context.Background(), []string{"sources", "--json"}); code != 0 {
		t.Fatalf("sources exit = %d: %s", code, stderr.String())
	}
	var decoded []catalog.Entry
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) < 10 {
		t.Fatalf("ecosystem catalog too small: %d", len(decoded))
	}
}
