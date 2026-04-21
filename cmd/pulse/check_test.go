package main

import (
	"context"
	"path/filepath"
	"testing"
)

func TestParseCheckArgs(t *testing.T) {
	got, err := parseCheckArgs([]string{"--app-root", "/tmp/app"})
	if err != nil {
		t.Fatalf("parseCheckArgs returned error: %v", err)
	}
	if got != "/tmp/app" {
		t.Fatalf("parseCheckArgs app root = %q", got)
	}
}

func TestRunPulseCheckCompilesApp(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	t.Setenv("PULSE_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, "pulse.app", `{"name":"checkapp"}`)
	writeTestAppFile(t, root, "go.mod", "module example.com/checkapp\n\ngo 1.26.0\n")
	writeTestAppFile(t, root, "svc/api.go", "package svc\n\nimport \"context\"\n\n//pulse:api public\nfunc Ping(context.Context) error { return nil }\n")

	restore := chdirForTest(t, root)
	defer restore()

	if err := runPulseCheck(context.Background(), nil); err != nil {
		t.Fatalf("runPulseCheck returned error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(cacheRoot, "build", "checkapp-*", "pulse-app"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected compiled workspace binary for pulse check")
	}
}
