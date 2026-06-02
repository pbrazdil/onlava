package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appcfg "github.com/pbrazdil/onlava/internal/app"
)

type fakeDoctorResourceProbe struct {
	runtime doctorRuntimeInfo
	memory  doctorMemoryInfo
	memErr  error
	disks   map[string]doctorDiskInfo
	diskErr error
}

func (p fakeDoctorResourceProbe) Runtime() doctorRuntimeInfo {
	if p.runtime.GOOS == "" {
		return doctorRuntimeInfo{GOOS: "linux", GOARCH: "amd64", NumCPU: 4}
	}
	return p.runtime
}

func (p fakeDoctorResourceProbe) Memory(context.Context) (doctorMemoryInfo, error) {
	if p.memErr != nil {
		return doctorMemoryInfo{}, p.memErr
	}
	if p.memory.TotalBytes == 0 {
		return doctorMemoryInfo{TotalBytes: 8 * 1024 * 1024 * 1024}, nil
	}
	return p.memory, nil
}

func (p fakeDoctorResourceProbe) Disk(_ context.Context, path string) (doctorDiskInfo, error) {
	if p.diskErr != nil {
		return doctorDiskInfo{}, p.diskErr
	}
	if disk, ok := p.disks[path]; ok {
		return disk, nil
	}
	abs, _ := filepath.Abs(path)
	return doctorDiskInfo{Path: abs, FreeBytes: 10 * 1024 * 1024 * 1024, TotalBytes: 20 * 1024 * 1024 * 1024}, nil
}

func fakeDoctorDeps(t *testing.T) doctorProbeDeps {
	t.Helper()
	tools := map[string]string{
		"go":      "/bin/go",
		"bun":     "/bin/bun",
		"psql":    "/bin/psql",
		"pg_dump": "/bin/pg_dump",
		"docker":  "/bin/docker",
		"atlas":   "/bin/atlas",
		"sqlc":    "/bin/sqlc",
		"git":     "/bin/git",
	}
	versions := map[string]string{
		"go version":        "go version go1.26.3 linux/amd64",
		"bun --version":     "1.2.3",
		"psql --version":    "psql (PostgreSQL) 18.0",
		"pg_dump --version": "pg_dump (PostgreSQL) 18.0",
		"docker --version":  "Docker version 29.0.0",
		"atlas version":     "atlas version v0.38.0",
		"sqlc version":      "v1.30.0",
		"git --version":     "git version 2.52.0",
	}
	return doctorProbeDeps{
		LookPath: func(file string) (string, error) {
			if path, ok := tools[file]; ok {
				return path, nil
			}
			return "", os.ErrNotExist
		},
		RunCommand: func(_ context.Context, name string, args ...string) ([]byte, error) {
			key := filepath.Base(name) + " " + strings.Join(args, " ")
			if out, ok := versions[key]; ok {
				return []byte(out), nil
			}
			return nil, errors.New("unexpected command " + key)
		},
		ResourceProbe: fakeDoctorResourceProbe{},
		Getwd:         func() (string, error) { return "/workspace", nil },
		CacheRoot:     func() (string, error) { return "/cache/onlava", nil },
		DiscoverApp: func(start string) (doctorAppInfo, appcfg.Config, bool, error) {
			return doctorAppInfo{}, appcfg.Config{}, false, errors.New("no app")
		},
	}
}

func TestParseDoctorArgs(t *testing.T) {
	t.Parallel()

	opts, err := parseDoctorArgs([]string{"--app-root", "/tmp/app", "--json"})
	if err != nil {
		t.Fatalf("parseDoctorArgs returned error: %v", err)
	}
	if opts.AppRoot != "/tmp/app" || !opts.JSON {
		t.Fatalf("opts = %+v", opts)
	}
	if _, err := parseDoctorArgs([]string{"--app-root"}); err == nil || err.Error() != "missing value for --app-root" {
		t.Fatalf("missing --app-root error = %v", err)
	}
	if _, err := parseDoctorArgs([]string{"--bad"}); err == nil || err.Error() != `unknown flag "--bad"` {
		t.Fatalf("unknown flag error = %v", err)
	}
}

func TestRunOnlavaDoctorJSONReportsRequiredFailure(t *testing.T) {
	t.Parallel()

	deps := fakeDoctorDeps(t)
	deps.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	var out bytes.Buffer
	err := runOnlavaDoctorWithDeps(context.Background(), &out, []string{"--json"}, deps)
	var silent *silentCLIError
	if !errors.As(err, &silent) {
		t.Fatalf("expected silent error, got %v", err)
	}
	var payload doctorResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != doctorSchemaVersion || payload.OK || payload.Summary.Errors != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	goCheck := doctorCheckByID(payload.Checks, "tool.go")
	if goCheck.Status != doctorStatusError || goCheck.Severity != doctorSeverityRequired {
		t.Fatalf("go check = %+v", goCheck)
	}
}

func TestRunOnlavaDoctorDiscoversAppSensitiveChecks(t *testing.T) {
	t.Parallel()

	deps := fakeDoctorDeps(t)
	deps.Getwd = func() (string, error) { return "/apps/demo", nil }
	deps.DiscoverApp = func(start string) (doctorAppInfo, appcfg.Config, bool, error) {
		if start != "/apps/demo" {
			t.Fatalf("discover start = %q", start)
		}
		cfg := appcfg.Config{
			Name: "demo",
			ID:   "demo-id",
			Proxy: appcfg.ProxyConfig{Frontends: map[string]appcfg.FrontendConfig{
				"web": {Host: "web.demo.localhost"},
			}},
			Temporal: appcfg.TemporalConfig{Enabled: true, TypeScript: appcfg.TemporalTypeScript{Enabled: true}},
			Dev: appcfg.DevConfig{Services: map[string]appcfg.DevServiceConfig{
				"postgres": {Kind: "postgres", Image: "postgres:18"},
			}},
			Generators: appcfg.GeneratorsConfig{SQLC: appcfg.SQLCGeneratorConfig{
				Schemas: []appcfg.SQLCGeneratorSchema{{AtlasSource: "db/schema.hcl", SQLCSchema: "db/schema.sql"}},
			}},
		}
		return doctorAppInfo{Root: "/apps/demo", ConfigPath: "/apps/demo/.onlava.json", Name: "demo", ID: "demo-id"}, cfg, true, nil
	}

	var out bytes.Buffer
	if err := runOnlavaDoctorWithDeps(context.Background(), &out, []string{"--json"}, deps); err != nil {
		t.Fatalf("runOnlavaDoctorWithDeps: %v\n%s", err, out.String())
	}
	var payload doctorResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.App == nil || payload.App.Name != "demo" || payload.App.ID != "demo-id" {
		t.Fatalf("app = %+v", payload.App)
	}
	for _, id := range []string{"tool.bun", "tool.docker", "tool.atlas", "tool.sqlc"} {
		if got := doctorCheckByID(payload.Checks, id); got.ID == "" || got.Status != doctorStatusOK {
			t.Fatalf("%s check = %+v", id, got)
		}
	}
}

func TestRunOnlavaDoctorExplicitAppRootFailureIsError(t *testing.T) {
	t.Parallel()

	deps := fakeDoctorDeps(t)
	deps.DiscoverApp = func(string) (doctorAppInfo, appcfg.Config, bool, error) {
		return doctorAppInfo{}, appcfg.Config{}, false, errors.New("no .onlava.json found")
	}
	var out bytes.Buffer
	err := runOnlavaDoctorWithDeps(context.Background(), &out, []string{"--app-root", "/missing", "--json"}, deps)
	var silent *silentCLIError
	if !errors.As(err, &silent) {
		t.Fatalf("expected silent error, got %v", err)
	}
	var payload doctorResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	appCheck := doctorCheckByID(payload.Checks, "app.root")
	if appCheck.Status != doctorStatusError || appCheck.Severity != doctorSeverityRequired {
		t.Fatalf("app.root check = %+v", appCheck)
	}
}

func TestDoctorResourceThresholds(t *testing.T) {
	t.Parallel()

	deps := fakeDoctorDeps(t)
	deps.ResourceProbe = fakeDoctorResourceProbe{
		memory: doctorMemoryInfo{TotalBytes: 1536 * 1024 * 1024},
		disks: map[string]doctorDiskInfo{
			"/workspace":    {Path: "/workspace", FreeBytes: 3 * 1024 * 1024 * 1024, TotalBytes: 20 * 1024 * 1024 * 1024},
			"/cache/onlava": {Path: "/cache/onlava", FreeBytes: 700 * 1024 * 1024, TotalBytes: 20 * 1024 * 1024 * 1024},
		},
	}
	resp := buildDoctorResponse(context.Background(), doctorOptions{}, deps)
	if got := doctorCheckByID(resp.Checks, "resource.memory"); got.Status != doctorStatusError {
		t.Fatalf("memory check = %+v", got)
	}
	if got := doctorCheckByID(resp.Checks, "resource.disk.cwd"); got.Status != doctorStatusWarn {
		t.Fatalf("cwd disk check = %+v", got)
	}
	if got := doctorCheckByID(resp.Checks, "resource.disk.cache_root"); got.Status != doctorStatusError {
		t.Fatalf("cache disk check = %+v", got)
	}
}

func TestGoToolchainVersionParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		output string
		want   doctorGoVersion
		ok     bool
	}{
		{output: "go version go1.26.3 darwin/arm64", want: doctorGoVersion{Major: 1, Minor: 26, Patch: 3}, ok: true},
		{output: "go version go1.27 linux/amd64", want: doctorGoVersion{Major: 1, Minor: 27}, ok: true},
		{output: "go version devel go1.28-abc linux/amd64", want: doctorGoVersion{Major: 1, Minor: 28}, ok: true},
		{output: "not go", ok: false},
	}
	for _, tt := range tests {
		got, ok := parseGoToolchainVersion(tt.output)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("parseGoToolchainVersion(%q) = %+v,%v want %+v,%v", tt.output, got, ok, tt.want, tt.ok)
		}
	}
	if (doctorGoVersion{Major: 1, Minor: 25, Patch: 9}).compare(doctorGoVersion{Major: 1, Minor: 26}) >= 0 {
		t.Fatal("old Go version compared as supported")
	}
}

func TestDoctorTextRendering(t *testing.T) {
	t.Parallel()

	resp := doctorResponse{
		SchemaVersion: doctorSchemaVersion,
		OK:            true,
		Summary:       doctorSummary{OK: 1, Warnings: 1},
		Checks: []doctorCheck{
			{ID: "os.runtime", Status: doctorStatusOK, Message: "linux/amd64"},
			{ID: "tool.bun", Status: doctorStatusWarn, Message: "bun not found"},
		},
	}
	var out bytes.Buffer
	if err := writeDoctorText(&out, resp); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"onlava doctor", "ok      os.runtime", "warn    tool.bun", "summary: 1 ok, 1 warnings, 0 errors, 0 skipped"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestDoctorSchemaValidatesSyntheticPayload(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join(repoRootForTest(t), "docs", "schemas", "onlava.doctor.result.v1.schema.json")
	payload := buildHarnessDoctorSchemaPayload(buildVersionResponse())
	if diagnostics := validateHarnessJSONSchemaFile(schemaPath, payload); len(diagnostics) != 0 {
		t.Fatalf("doctor schema diagnostics = %+v", diagnostics)
	}
}

func doctorCheckByID(checks []doctorCheck, id string) doctorCheck {
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	return doctorCheck{}
}
