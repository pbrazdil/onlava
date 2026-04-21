package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRootAcceptsLegacyID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pulse.app"), []byte(`{"id":"legacy-app","proxy":{"workspace":"onlv","api_host":"api.onlv.localhost","console_host":"console.onlv.localhost","mcp_host":"mcp.onlv.localhost","frontend_host":"pulse.onlv.localhost"},"observability":{"logs":{"exclude_endpoints":["sync.*"]},"tracing":{"include_endpoints":["tenants.Config"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root, cfg, err := DiscoverRoot(dir)
	if err != nil {
		t.Fatalf("DiscoverRoot returned error: %v", err)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
	if cfg.Name != "legacy-app" {
		t.Fatalf("cfg.Name = %q, want %q", cfg.Name, "legacy-app")
	}
	if cfg.Proxy.Workspace != "onlv" {
		t.Fatalf("cfg.Proxy.Workspace = %q, want %q", cfg.Proxy.Workspace, "onlv")
	}
	if cfg.Proxy.APIHost != "api.onlv.localhost" {
		t.Fatalf("cfg.Proxy.APIHost = %q, want %q", cfg.Proxy.APIHost, "api.onlv.localhost")
	}
	if len(cfg.Observability.Logs.ExcludeEndpoints) != 1 || cfg.Observability.Logs.ExcludeEndpoints[0] != "sync.*" {
		t.Fatalf("cfg.Observability.Logs.ExcludeEndpoints = %v", cfg.Observability.Logs.ExcludeEndpoints)
	}
	if len(cfg.Observability.Tracing.IncludeEndpoints) != 1 || cfg.Observability.Tracing.IncludeEndpoints[0] != "tenants.Config" {
		t.Fatalf("cfg.Observability.Tracing.IncludeEndpoints = %v", cfg.Observability.Tracing.IncludeEndpoints)
	}
}

func TestDiscoverRootRequiresNameOrID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pulse.app"), []byte(`{"build":{"cgo_enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := DiscoverRoot(dir)
	if err == nil {
		t.Fatal("DiscoverRoot returned nil error")
	}
	if got, want := err.Error(), "pulse.app must define a non-empty name or id"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
