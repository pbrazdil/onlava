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
	})

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
