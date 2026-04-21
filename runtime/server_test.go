package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPulseConfigEndpoint(t *testing.T) {
	SetAppConfig(AppConfig{Name: "onlvnext-o5o2", ListenAddr: "127.0.0.1:4000"})
	SetPublicBaseURL("https://api.onlv.localhost")

	server, err := newServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__pulse/config", nil)
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want %q", got, "application/json")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}

	var body struct {
		AppID      string `json:"appID"`
		APIBaseURL string `json:"apiBaseURL"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AppID != "onlvnext-o5o2" {
		t.Fatalf("appID = %q, want %q", body.AppID, "onlvnext-o5o2")
	}
	if body.APIBaseURL != "https://api.onlv.localhost" {
		t.Fatalf("apiBaseURL = %q, want %q", body.APIBaseURL, "https://api.onlv.localhost")
	}
}

func TestPlatformStatsEndpoint(t *testing.T) {
	SetAppConfig(AppConfig{Name: "onlvnext-o5o2", ListenAddr: "127.0.0.1:4000"})
	SetPublicBaseURL("https://api.onlv.localhost")

	server, err := newServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/platform.Stats", nil)
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want %q", got, "application/json")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}

	var body PlatformStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AppID != "onlvnext-o5o2" {
		t.Fatalf("appID = %q, want %q", body.AppID, "onlvnext-o5o2")
	}
	if body.APIBaseURL != "https://api.onlv.localhost" {
		t.Fatalf("apiBaseURL = %q, want %q", body.APIBaseURL, "https://api.onlv.localhost")
	}
	if body.Process.PID == 0 {
		t.Fatal("expected process pid")
	}
	if body.Go.Goroutines <= 0 {
		t.Fatal("expected goroutine count")
	}
	if body.Memory.CurrentHeap.Bytes == 0 {
		t.Fatal("expected current heap bytes")
	}
	if body.Disk.Path == "" {
		t.Fatal("expected disk path")
	}
	if body.Profiles.CPU != "https://api.onlv.localhost/debug/pprof/profile?seconds=30" {
		t.Fatalf("cpu profile URL = %q", body.Profiles.CPU)
	}
}

func TestDevPubSubClearEndpointRequiresTokenAndCallsRuntime(t *testing.T) {
	prevGetenv := osGetenv
	prevClearer := localPubSubClearer
	defer func() {
		osGetenv = prevGetenv
		localPubSubClearer = prevClearer
	}()
	osGetenv = func(key string) string {
		if key == "PULSE_DEV_REPORT_TOKEN" {
			return "secret"
		}
		return ""
	}
	called := false
	localPubSubClearer = func(context.Context) (any, error) {
		called = true
		return []map[string]any{{"name": "events"}}, nil
	}
	SetAppConfig(AppConfig{Name: "onlvnext-o5o2", ListenAddr: "127.0.0.1:4000"})

	server, err := newServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/__pulse/pubsub/clear", nil)
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unauthorized status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if called {
		t.Fatal("clearer called without token")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/__pulse/pubsub/clear", nil)
	req.Header.Set("Authorization", "Bearer secret")
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !called {
		t.Fatal("clearer not called")
	}
}

func TestPProfHeapEndpoint(t *testing.T) {
	server, err := newServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected heap profile body")
	}
}
