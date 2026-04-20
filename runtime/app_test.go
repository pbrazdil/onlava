package runtime

import (
	"bytes"
	"strings"
	"testing"

	"pulse.dev/internal/localproxy"
)

func TestPrintRuntimeBanner(t *testing.T) {
	SetAppConfig(AppConfig{Name: "testapp", ListenAddr: "127.0.0.1:4000"})

	var out bytes.Buffer
	printRuntimeBanner(&out, "127.0.0.1:4000", localproxy.Routes{
		APIURL:      "https://api.test.localhost",
		ConsoleURL:  "https://console.test.localhost",
		MCPBaseURL:  "https://mcp.test.localhost",
		FrontendURL: "https://pulse.test.localhost",
	}, "http://127.0.0.1:4002")

	text := out.String()
	for _, want := range []string{
		"Pulse development server running!",
		"Your API is running at:",
		"https://api.test.localhost",
		"Development Dashboard URL:",
		"https://console.test.localhost",
		"MCP SSE URL:",
		"https://mcp.test.localhost/sse?appID=testapp",
		"Pulse App URL:",
		"https://pulse.test.localhost",
		"Drizzle Studio URL:",
		"http://127.0.0.1:4002",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("banner %q missing %q", text, want)
		}
	}
}

func TestLaunchedBySupervisor(t *testing.T) {
	t.Setenv("PULSE_DEV_SUPERVISOR", "1")
	if !launchedBySupervisor() {
		t.Fatal("expected launchedBySupervisor to be true")
	}
}

func TestSupervisorParentMonitorShouldCancel(t *testing.T) {
	tests := []struct {
		name    string
		initial int
		current int
		want    bool
	}{
		{name: "same parent", initial: 123, current: 123, want: false},
		{name: "reparented to pid1", initial: 123, current: 1, want: true},
		{name: "reparented elsewhere", initial: 123, current: 456, want: true},
		{name: "initial pid1 ignored", initial: 1, current: 1, want: false},
		{name: "invalid current ignored", initial: 123, current: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supervisorParentMonitorShouldCancel(tt.initial, tt.current); got != tt.want {
				t.Fatalf("supervisorParentMonitorShouldCancel(%d, %d) = %v, want %v", tt.initial, tt.current, got, tt.want)
			}
		})
	}
}
