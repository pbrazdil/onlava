package pulse_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var (
	buildPulseBinaryOnce sync.Once
	buildPulseBinaryPath string
	buildPulseBinaryErr  error
)

func TestPulseRunBasicApp(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := copyFixtureApp(t, repo, "basic")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = pulseRunEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer func() {
		cancel()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for pulse process to exit")
		}
	}()

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")

	postJSON(t, "http://"+addr+"/echo/Alice?title=Dr", map[string]string{"body": "body"}, map[string]string{"X-Echo": "hdr"}, http.StatusOK, map[string]any{"message": "hi Alice Dr hdr body"})
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})
	getJSON(t, "http://"+addr+"/service.CustomStatus", nil, http.StatusCreated, map[string]any{"message": "created"})
	getJSON(t, "http://"+addr+"/service.AuthEcho", map[string]string{"Authorization": "Bearer token123"}, http.StatusOK, map[string]any{"user": "user-1", "role": "admin"})
	getJSON(t, "http://"+addr+"/raw/alpha/beta", nil, http.StatusOK, map[string]any{"path": "alpha/beta", "method": "GET"})
	assertCORSPreflight(t, "http://"+addr+"/service.AuthEcho")
	assertCORSActual(t, "http://"+addr+"/service.AuthEcho")
}

func TestPulseDevReloadsOnGoChanges(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	sourceAppDir := filepath.Join(repo, "testdata", "apps", "basic")
	appDir := filepath.Join(t.TempDir(), "basic")
	copyDir(t, sourceAppDir, appDir)
	rewriteFixtureReplace(t, filepath.Join(appDir, "go.mod"), repo)

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "dev", "--listen", addr)
	cmd.Env = pulseDevEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse dev: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})

	apiPath := filepath.Join(appDir, "service", "api.go")
	data, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), `Prefix: "hi"`, `Prefix: "bye"`, 1)
	if updated == string(data) {
		t.Fatal("failed to update test fixture source")
	}
	if err := os.WriteFile(apiPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForJSONResponse(t, "http://"+addr+"/service.CallPrivate", http.StatusOK, map[string]any{"message": "secret:bye"})
}

func TestPulseRunLoadsSecretsFromDotEnv(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := copyFixtureApp(t, repo, "secrets")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = pulseRunEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/secrets")
	getJSON(t, "http://"+addr+"/secrets", nil, http.StatusOK, map[string]any{
		"service": "service-secret",
		"helper":  "helper-secret",
	})
}

func TestPulseRunPopulatesSecretsBeforePubSubPackageDeclarations(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := filepath.Join(t.TempDir(), "pubsubsecrets")
	writeFile(t, filepath.Join(appDir, "go.mod"), "module example.com/pubsubsecrets\n\ngo 1.26.0\n\nrequire pulse.dev v0.0.0\n\nreplace pulse.dev => "+repo+"\n")
	writeFile(t, filepath.Join(appDir, "pulse.app"), `{"name":"pubsubsecrets"}`)
	writeFile(t, filepath.Join(appDir, ".env"), "TestQueueConcurrency=10\n")
	writeFile(t, filepath.Join(appDir, "queue", "api.go"), `package queue

import (
	"context"
	"strconv"
	"strings"

	"pulse.dev/pubsub"
)

var secrets struct {
	TestQueueConcurrency string
}

type Event struct {
	ID string `+"`json:\"id\"`"+`
}

type Response struct {
	MaxConcurrency int    `+"`json:\"max_concurrency\"`"+`
	Secret         string `+"`json:\"secret\"`"+`
}

var topic = pubsub.NewTopic[*Event]("events", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

var maxConcurrency = parseConcurrency(secrets.TestQueueConcurrency, 1)

var sub = pubsub.NewSubscription(topic, "sub", pubsub.SubscriptionConfig[*Event]{
	Handler: func(ctx context.Context, msg *Event) error { return nil },
	MaxConcurrency: maxConcurrency,
})

func parseConcurrency(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

//pulse:api public path=/concurrency method=GET
func Concurrency(ctx context.Context) (*Response, error) {
	return &Response{
		MaxConcurrency: sub.Config().MaxConcurrency,
		Secret: secrets.TestQueueConcurrency,
	}, nil
}
`)

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = pulseRunEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/concurrency")
	getJSON(t, "http://"+addr+"/concurrency", nil, http.StatusOK, map[string]any{
		"max_concurrency": 10,
		"secret":          "10",
	})
}

func TestPulseRunInitializesServiceStructsAtStartup(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := filepath.Join(t.TempDir(), "serviceinit")
	markerPath := filepath.Join(t.TempDir(), "init.marker")
	writeFile(t, filepath.Join(appDir, "go.mod"), "module example.com/serviceinit\n\ngo 1.26.0\n\nrequire pulse.dev v0.0.0\n\nreplace pulse.dev => "+repo+"\n")
	writeFile(t, filepath.Join(appDir, "pulse.app"), `{"name":"serviceinit"}`)
	writeFile(t, filepath.Join(appDir, "svc", "api.go"), `package svc

import (
	"context"
	"os"
)

//pulse:service
type Service struct{}

func initService() (*Service, error) {
	if path := os.Getenv("PULSE_INIT_MARKER"); path != "" {
		if err := os.WriteFile(path, []byte("started"), 0o644); err != nil {
			return nil, err
		}
	}
	return &Service{}, nil
}

//pulse:api public
func (s *Service) Hello(ctx context.Context) error { return nil }
`)

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = append(pulseRunEnv(repo, dashAddr, cacheDir), "PULSE_INIT_MARKER="+markerPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForFile(t, markerPath)
}

func TestPulseRunMiddlewareApp(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := copyFixtureApp(t, repo, "middleware")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = pulseRunEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.Context")
	assertJSONResponseWithHeaders(t, mustRequest(t, http.MethodGet, "http://"+addr+"/service.Context", nil), http.StatusOK, map[string]any{"message": "svc"}, map[string]string{
		"X-Global-Middleware": "true",
	})
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "middleware:handler"})
	getJSON(t, "http://"+addr+"/service.Error", nil, http.StatusInternalServerError, map[string]any{"code": "internal", "message": "middleware error"})
	assertJSONResponseWithHeaders(t, mustRequest(t, http.MethodGet, "http://"+addr+"/raw/alpha", nil), http.StatusOK, map[string]any{"id": "alpha"}, map[string]string{
		"X-Raw-Middleware": "true",
	})
}

func TestPulseRunExecutesCronJobs(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := copyFixtureApp(t, repo, "cron")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = pulseRunEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/cron/status")
	waitForCronStatus(t, "http://"+addr+"/cron/status")
}

func TestPulseBuildProducesRunnableBinary(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := copyFixtureApp(t, repo, "basic")
	pulseBinary := buildPulseBinary(t, repo)
	outputPath := filepath.Join(t.TempDir(), "basic-app")
	cacheDir := filepath.Join(t.TempDir(), "cache")

	buildCmd := exec.Command(pulseBinary, "build", "-o", outputPath)
	buildCmd.Dir = appDir
	buildCmd.Env = append(os.Environ(), "PULSE_DEV_CACHE_DIR="+cacheDir)
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pulse build failed: %v\n%s", err, buildOutput)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("built binary missing: %v", err)
	}

	port := freePort(t)
	addr := "127.0.0.1:" + port
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, outputPath)
	cmd.Env = append(os.Environ(), "PULSE_LISTEN_ADDR="+addr, "PULSE_LOCAL_PROXY=0", "PULSE_DEV_CACHE_DIR="+cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start built app: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})
}

func TestPulseDevServesHTTPSHostnames(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	sourceAppDir := filepath.Join(repo, "testdata", "apps", "basic")
	appDir := filepath.Join(t.TempDir(), "basic")
	copyDir(t, sourceAppDir, appDir)
	rewriteFixtureReplace(t, filepath.Join(appDir, "go.mod"), repo)
	writePulseApp(t, appDir, `{"name":"basicapp","proxy":{"workspace":"ignored","api_host":"api.onlv.localhost","console_host":"console.onlv.localhost","mcp_host":"mcp.onlv.localhost","frontend_host":"pulse.onlv.localhost"}}`)
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	httpPort := freePort(t)
	httpsPort := freePort(t)
	frontendPort := freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	frontendLn, err := net.Listen("tcp", "127.0.0.1:"+frontendPort)
	if err != nil {
		t.Fatal(err)
	}
	defer frontendLn.Close()
	frontendSrv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = io.WriteString(w, "frontend ok")
		}),
	}
	defer frontendSrv.Close()
	go func() { _ = frontendSrv.Serve(frontendLn) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "dev", "--listen", addr)
	cmd.Env = pulseDevProxyEnv(repo, dashAddr, cacheDir, httpPort, httpsPort, "127.0.0.1:"+frontendPort)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse dev: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")

	client := insecureHTTPSClient()
	apiURL := "https://api.onlv.localhost:" + httpsPort + "/service.CallPrivate"
	getJSONWithClient(t, client, apiURL, nil, http.StatusOK, map[string]any{"message": "secret:hi"})

	consoleURL := "https://console.onlv.localhost:" + httpsPort + "/"
	waitForURL(t, client, consoleURL)
	resp, err := client.Get(consoleURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("unexpected console status %d", resp.StatusCode)
	}

	mcpURL := "https://mcp.onlv.localhost:" + httpsPort + "/sse?app=basicapp"
	waitForURL(t, client, mcpURL)
	resp, err = client.Get(mcpURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected mcp status %d", resp.StatusCode)
	}

	frontendURL := "https://pulse.onlv.localhost:" + httpsPort + "/"
	waitForURL(t, client, frontendURL)
	resp, err = client.Get(frontendURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "frontend ok" {
		t.Fatalf("unexpected frontend response status=%d body=%q", resp.StatusCode, body)
	}
}

func TestPulseBuiltBinaryIsHeadlessByDefault(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	sourceAppDir := filepath.Join(repo, "testdata", "apps", "basic")
	appDir := filepath.Join(t.TempDir(), "basic")
	copyDir(t, sourceAppDir, appDir)
	rewriteFixtureReplace(t, filepath.Join(appDir, "go.mod"), repo)
	writePulseApp(t, appDir, `{"name":"basicapp","proxy":{"api_host":"api.onlv.localhost","console_host":"console.onlv.localhost","mcp_host":"mcp.onlv.localhost","frontend_host":"pulse.onlv.localhost"}}`)
	pulseBinary := buildPulseBinary(t, repo)
	outputPath := filepath.Join(t.TempDir(), "basic-app")
	cacheDir := filepath.Join(t.TempDir(), "cache")

	buildCmd := exec.Command(pulseBinary, "build", "-o", outputPath)
	buildCmd.Dir = appDir
	buildCmd.Env = append(os.Environ(), "PULSE_DEV_CACHE_DIR="+cacheDir)
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pulse build failed: %v\n%s", err, buildOutput)
	}

	port := freePort(t)
	addr := "127.0.0.1:" + port
	httpsPort := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, outputPath)
	cmd.Env = append(
		os.Environ(),
		"PULSE_LISTEN_ADDR="+addr,
		"PULSE_LOCAL_PROXY_HTTPS_PORT="+httpsPort,
		"PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL=1",
		"PULSE_DEV_CACHE_DIR="+cacheDir,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start built app: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})
	client := insecureHTTPSClient()
	client.Timeout = 300 * time.Millisecond
	resp, err := client.Get("https://api.onlv.localhost:" + httpsPort + "/service.CallPrivate")
	if err == nil {
		resp.Body.Close()
		t.Fatalf("built binary unexpectedly served local HTTPS proxy on %s", httpsPort)
	}
}

func TestPulseDevDashboardNotificationsAndMCP(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	sourceAppDir := filepath.Join(repo, "testdata", "apps", "basic")
	appDir := filepath.Join(t.TempDir(), "basic")
	copyDir(t, sourceAppDir, appDir)
	rewriteFixtureReplace(t, filepath.Join(appDir, "go.mod"), repo)

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "dev", "--listen", addr)
	cmd.Env = pulseDevEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse dev: %v", err)
	}
	defer stopPulseProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	waitForHTTP(t, "http://"+dashAddr+"/basicapp")

	wsConn, _, err := websocket.DefaultDialer.Dial("ws://"+dashAddr+"/__pulse", nil)
	if err != nil {
		t.Fatalf("dial dashboard websocket: %v", err)
	}
	defer wsConn.Close()

	version := wsCall(t, wsConn, 1, "version", map[string]any{})
	if toString(version["version"]) == "" {
		t.Fatalf("dashboard version response missing version: %#v", version)
	}

	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})
	waitForWSMethods(t, wsConn, 10*time.Second, "trace/new")

	mcp := openMCPClient(t, dashAddr, "basicapp")
	defer mcp.Close()

	initResp := mcp.Call(t, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "pulse-test",
			"version": "0.0.0",
		},
		"capabilities": map[string]any{},
	})
	if toString(toMap(initResp["serverInfo"])["name"]) != "pulse-mcp" {
		t.Fatalf("unexpected mcp initialize response: %#v", initResp)
	}

	toolsResp := mcp.Call(t, 2, "tools/list", map[string]any{})
	toolNames := mcpToolNames(toSlice(toolsResp["tools"]))
	for _, want := range []string{"get_services", "get_traces", "call_endpoint"} {
		if !toolNames[want] {
			t.Fatalf("mcp tools missing %q: %#v", want, toolsResp)
		}
	}

	servicesResp := mcp.CallTool(t, 3, "get_services", map[string]any{})
	services := servicesResp["structuredContent"]
	if !strings.Contains(fmt.Sprint(services), "service") {
		t.Fatalf("unexpected get_services response: %#v", servicesResp)
	}

	tracesResp := waitForMCPToolResult(t, 10*time.Second, func() map[string]any {
		return mcp.CallTool(t, 4, "get_traces", map[string]any{"limit": 10})
	})
	if !strings.Contains(fmt.Sprint(tracesResp["structuredContent"]), "trace_id") {
		t.Fatalf("unexpected get_traces response: %#v", tracesResp)
	}

	apiPath := filepath.Join(appDir, "service", "api.go")
	data, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), `Prefix: "hi"`, `Prefix: "dashboard"`, 1)
	if updated == string(data) {
		t.Fatal("failed to update test fixture source")
	}
	if err := os.WriteFile(apiPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForWSMethods(t, wsConn, 15*time.Second, "process/compile-start", "process/output", "process/reload")
	waitForJSONResponse(t, "http://"+addr+"/service.CallPrivate", http.StatusOK, map[string]any{"message": "secret:dashboard"})
}
