package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pulse.dev/internal/devdash"
)

func TestParseLogsArgs(t *testing.T) {
	opts, err := parseLogsArgs([]string{"--app-root", "/tmp/app", "--limit", "50", "--stream", "stderr", "--follow"})
	if err != nil {
		t.Fatalf("parseLogsArgs returned error: %v", err)
	}
	if opts.AppRoot != "/tmp/app" || opts.Limit != 50 || opts.Stream != "stderr" || !opts.Follow {
		t.Fatalf("unexpected logs options: %#v", opts)
	}
}

func TestRunPulseLogsReadsStoredOutput(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	t.Setenv("PULSE_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, "pulse.app", `{"name":"logsapp"}`)
	writeTestAppFile(t, root, "go.mod", "module example.com/logsapp\n\ngo 1.26.0\n")

	store, err := devdash.OpenStore(cacheRoot)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.UpsertApp(ctx, devdash.AppRecord{
		ID:         "logsapp",
		Name:       "logsapp",
		Root:       root,
		ListenAddr: "127.0.0.1:4000",
		Running:    true,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertApp: %v", err)
	}
	if err := store.WriteProcessOutput(ctx, devdash.ProcessOutput{
		AppID:  "logsapp",
		PID:    "123",
		Stream: "stdout",
		Output: []byte("first line\n"),
	}); err != nil {
		t.Fatalf("WriteProcessOutput stdout: %v", err)
	}
	if err := store.WriteProcessOutput(ctx, devdash.ProcessOutput{
		AppID:  "logsapp",
		PID:    "123",
		Stream: "stderr",
		Output: []byte("second line\n"),
	}); err != nil {
		t.Fatalf("WriteProcessOutput stderr: %v", err)
	}

	var buf bytes.Buffer
	if err := runPulseLogs(ctx, &buf, []string{"--app-root", root, "--limit", "10"}); err != nil {
		t.Fatalf("runPulseLogs returned error: %v", err)
	}
	if got := buf.String(); got != "first line\nsecond line\n" {
		t.Fatalf("logs output = %q", got)
	}
}

func TestRunPulseLogsFiltersStream(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	t.Setenv("PULSE_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, "pulse.app", `{"name":"logsapp"}`)
	writeTestAppFile(t, root, "go.mod", "module example.com/logsapp\n\ngo 1.26.0\n")

	store, err := devdash.OpenStore(cacheRoot)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.UpsertApp(ctx, devdash.AppRecord{
		ID:         "logsapp",
		Name:       "logsapp",
		Root:       root,
		ListenAddr: "127.0.0.1:4000",
		Running:    true,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertApp: %v", err)
	}
	for _, item := range []devdash.ProcessOutput{
		{AppID: "logsapp", PID: "123", Stream: "stdout", Output: []byte("out\n")},
		{AppID: "logsapp", PID: "123", Stream: "stderr", Output: []byte("err\n")},
	} {
		if err := store.WriteProcessOutput(ctx, item); err != nil {
			t.Fatalf("WriteProcessOutput: %v", err)
		}
	}

	var buf bytes.Buffer
	if err := runPulseLogs(ctx, &buf, []string{"--app-root", root, "--stream", "stderr"}); err != nil {
		t.Fatalf("runPulseLogs returned error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "err" {
		t.Fatalf("stderr logs output = %q", got)
	}
}
