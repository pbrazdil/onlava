package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCheckArgs(t *testing.T) {
	opts, err := parseCheckArgs([]string{"--app-root", "/tmp/app", "--json"})
	if err != nil {
		t.Fatalf("parseCheckArgs returned error: %v", err)
	}
	if opts.AppRoot != "/tmp/app" {
		t.Fatalf("parseCheckArgs app root = %q", opts.AppRoot)
	}
	if !opts.JSON {
		t.Fatal("expected --json to be true")
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

	var out bytes.Buffer
	if err := runPulseCheck(context.Background(), &out, nil); err != nil {
		t.Fatalf("runPulseCheck returned error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "pulse: check ok" {
		t.Fatalf("stdout = %q", out.String())
	}

	matches, err := filepath.Glob(filepath.Join(cacheRoot, "build", "checkapp-*", "pulse-app"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected compiled workspace binary for pulse check")
	}
}

func TestRunPulseCheckJSONSuccess(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	t.Setenv("PULSE_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, "pulse.app", `{"name":"checkjson","id":"check-id"}`)
	writeTestAppFile(t, root, "go.mod", "module example.com/checkjson\n\ngo 1.26.0\n")
	writeTestAppFile(t, root, "svc/api.go", "package svc\n\nimport \"context\"\n\n//pulse:api public\nfunc Ping(context.Context) error { return nil }\n")

	restore := chdirForTest(t, root)
	defer restore()

	var out bytes.Buffer
	if err := runPulseCheck(context.Background(), &out, []string{"--json"}); err != nil {
		t.Fatalf("runPulseCheck(--json) returned error: %v", err)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		App           struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"app"`
		Diagnostics []any `json:"diagnostics"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(success): %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != "pulse.check.result.v1" || !payload.OK {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.App.Name != "checkjson" || payload.App.ID != "check-id" {
		t.Fatalf("app = %+v", payload.App)
	}
	if len(payload.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want empty", payload.Diagnostics)
	}
}

func TestRunPulseCheckJSONCompileFailure(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	t.Setenv("PULSE_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, "pulse.app", `{"name":"checkfail"}`)
	writeTestAppFile(t, root, "go.mod", "module example.com/checkfail\n\ngo 1.26.0\n")
	writeTestAppFile(t, root, "svc/api.go", "package svc\n\nimport \"context\"\n\n//pulse:api public\nfunc Ping(context.Context) error { return MissingSymbol }\n")

	restore := chdirForTest(t, root)
	defer restore()

	var out bytes.Buffer
	err := runPulseCheck(context.Background(), &out, []string{"--json"})
	var silent *silentCLIError
	if !errors.As(err, &silent) {
		t.Fatalf("expected silentCLIError, got %v", err)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Diagnostics   []struct {
			Stage           string `json:"stage"`
			Severity        string `json:"severity"`
			File            string `json:"file"`
			Line            int    `json:"line"`
			Column          int    `json:"column"`
			Message         string `json:"message"`
			SuggestedAction string `json:"suggested_action"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(failure): %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != "pulse.check.result.v1" || payload.OK {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics, got none: %s", out.String())
	}
	first := payload.Diagnostics[0]
	if first.Stage != "compile" || first.Severity != "error" {
		t.Fatalf("first diagnostic = %+v", first)
	}
	if first.File != "svc/api.go" || first.Line == 0 {
		t.Fatalf("expected file/line in diagnostic, got %+v", first)
	}
	if !strings.Contains(first.Message, "undefined: MissingSymbol") {
		t.Fatalf("message = %q", first.Message)
	}
	if first.SuggestedAction == "" {
		t.Fatal("expected suggested action")
	}
}
