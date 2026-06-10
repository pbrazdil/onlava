package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	localagent "scenery.sh/internal/agent"
	"scenery.sh/internal/app"
)

func TestManagedFrontendCommandUsesViteLocalBin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFrontendPackage(t, root, `{"scripts":{"dev":"vite"}}`)
	bin := writeFrontendBin(t, root, "vite")
	cmd, args, err := managedFrontendCommand(root, "49231", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != bin {
		t.Fatalf("command = %q, want %q", cmd, bin)
	}
	wantArgs := []string{"--host", "127.0.0.1", "--port", "49231"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestManagedFrontendCommandUsesAstroLocalBin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFrontendPackage(t, root, `{"scripts":{"dev":"astro dev"}}`)
	bin := writeFrontendBin(t, root, "astro")
	cmd, args, err := managedFrontendCommand(root, "49232", "blog.main-test.local.dev")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != bin {
		t.Fatalf("command = %q, want %q", cmd, bin)
	}
	wantArgs := []string{"dev", "--host", "127.0.0.1", "--port", "49232", "--allowed-hosts", "blog.main-test.local.dev"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestManagedFrontendPackageManagerUsesWorkspaceParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"packageManager":"bun@1.3.11"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFrontendPackage(t, appRoot, `{"scripts":{"dev":"custom-dev"}}`)
	if got := managedFrontendPackageManager(appRoot); got != "bun" {
		t.Fatalf("package manager = %q, want bun", got)
	}
}

func TestFrontendDevEnvIncludesSessionRoutes(t *testing.T) {
	t.Parallel()

	env := frontendDevEnv([]string{"EXISTING=1"}, "/repo/app", "127.0.0.1:49231", localagent.Session{
		SessionID: "main-abc123",
		Routes: map[string]string{
			localagent.RouteAPI: "http://api.main-abc123.local.dev:9440/",
			"electric":          "http://electric.main-abc123.local.dev:9440/",
			"web":               "http://web.main-abc123.local.dev:9440/",
		},
	}, "web")
	for _, want := range []string{
		"EXISTING=1",
		"HOST=127.0.0.1",
		"PORT=49231",
		"SCENERY_APP_ROOT=/repo/app",
		"SCENERY_SESSION_ID=main-abc123",
		"SCENERY_API_BASE_URL=http://api.main-abc123.local.dev:9440/",
		"VITE_API_BASE_URL=http://api.main-abc123.local.dev:9440/",
		"SCENERY_ELECTRIC_URL=http://electric.main-abc123.local.dev:9440/",
		"VITE_ELECTRIC_URL=http://electric.main-abc123.local.dev:9440/",
		"__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS=web.main-abc123.local.dev",
	} {
		if !containsString(env, want) {
			t.Fatalf("frontendDevEnv() missing %q in %s", want, strings.Join(env, "\n"))
		}
	}
}

func TestManagedFrontendAllowedHostFromRouteNamespace(t *testing.T) {
	t.Parallel()

	session := localagent.Session{
		SessionID: "main-abc123",
		RouteNamespace: localagent.RouteNamespace{
			BaseDomain: "onlv.dev",
			Hosts: map[string]string{
				"blog": "blog.onlv.dev",
			},
		},
	}
	if got, want := managedFrontendAllowedHost(session, "blog"), "blog.main-abc123.onlv.dev"; got != want {
		t.Fatalf("allowed host = %q, want %q", got, want)
	}
	if got, want := managedFrontendAllowedHost(session, "pulse"), "pulse.main-abc123.onlv.dev"; got != want {
		t.Fatalf("fallback allowed host = %q, want %q", got, want)
	}
}

func TestManagedFrontendBackendsRequiresExplicitSharedUpstream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := app.Config{
		Proxy: app.ProxyConfig{
			Frontends: map[string]app.FrontendConfig{
				"web": {
					Root:     "apps/web",
					Upstream: "127.0.0.1:5173",
				},
			},
		},
	}
	_, _, err := managedFrontendBackendsForSession(context.Background(), root, cfg, nil, localagent.Session{
		SessionID: "main-test",
		StateRoot: filepath.Join(root, ".scenery", "sessions", "main-test"),
	})
	if err == nil {
		t.Fatal("expected managed frontend fallback error")
	}
	if !strings.Contains(err.Error(), "allow_shared_upstream") {
		t.Fatalf("error = %q, want allow_shared_upstream guidance", err)
	}
}

func TestManagedFrontendBackendsAllowsExplicitSharedUpstream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := app.Config{
		Proxy: app.ProxyConfig{
			Frontends: map[string]app.FrontendConfig{
				"web": {
					Root:                "apps/web",
					Upstream:            "127.0.0.1:5173",
					AllowSharedUpstream: true,
				},
			},
		},
	}
	backends, processes, err := managedFrontendBackendsForSession(context.Background(), root, cfg, nil, localagent.Session{
		SessionID: "main-test",
		StateRoot: filepath.Join(root, ".scenery", "sessions", "main-test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 0 {
		t.Fatalf("processes = %d, want 0", len(processes))
	}
	if got := backends["web"]; got.Network != "tcp" || got.Addr != "127.0.0.1:5173" {
		t.Fatalf("web backend = %+v", got)
	}
}

func TestManagedFrontendFailsFastWhenChildExitsBeforeReady(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	appRoot := filepath.Join(root, "app")
	frontendRoot := filepath.Join(appRoot, "apps", "web")
	writeFrontendPackage(t, frontendRoot, `{"scripts":{"dev":"vite"}}`)
	writeFrontendBinWithScript(t, frontendRoot, "vite", "echo frontend-boom\nexit 7\n")

	start := time.Now()
	_, _, err := managedFrontendBackendsForSession(context.Background(), appRoot, app.Config{
		Name: "demo",
		Proxy: app.ProxyConfig{
			Frontends: map[string]app.FrontendConfig{
				"web": {Root: "apps/web"},
			},
		},
	}, nil, localagent.Session{
		SessionID: "main-test",
		StateRoot: filepath.Join(root, "state"),
	})
	if err == nil {
		t.Fatal("managedFrontendBackendsForSession returned nil error, want early exit")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("managed frontend failure took %s, want early exit before timeout", elapsed)
	}
	got := err.Error()
	if !strings.Contains(got, "frontend web exited before becoming ready") || !strings.Contains(got, "frontend-boom") {
		t.Fatalf("error = %q, want early exit with output tail", got)
	}
}

func writeFrontendPackage(t *testing.T, root, data string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFrontendBin(t *testing.T, root, name string) string {
	t.Helper()
	return writeFrontendBinWithScript(t, root, name, "exit 0\n")
}

func writeFrontendBinWithScript(t *testing.T, root, name, script string) string {
	t.Helper()
	dir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}
