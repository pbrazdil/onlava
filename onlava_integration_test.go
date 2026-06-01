package onlava_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	onlavaruntime "github.com/pbrazdil/onlava/runtime"
	temporalclient "go.temporal.io/sdk/client"
)

var (
	buildOnlavaBinaryOnce sync.Once
	buildOnlavaBinaryPath string
	buildOnlavaBinaryErr  error
)

func TestMain(m *testing.M) {
	code := 0
	if err := prebuildOnlavaBinaryForSelectedTests(); err != nil {
		fmt.Fprintf(os.Stderr, "prebuild onlava binary: %v\n", err)
		code = 1
	} else {
		code = m.Run()
	}
	stopSharedTemporalDevServer()
	os.Exit(code)
}

func prebuildOnlavaBinaryForSelectedTests() error {
	if !shouldPrebuildOnlavaBinary() {
		return nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo := filepath.Clean(wd)
	buildOnlavaBinaryOnce.Do(func() {
		buildOnlavaBinaryPath, buildOnlavaBinaryErr = buildOnlavaBinaryForRepo(repo)
	})
	return buildOnlavaBinaryErr
}

func shouldPrebuildOnlavaBinary() bool {
	runFlag := flag.Lookup("test.run")
	if runFlag == nil {
		return true
	}
	pattern := strings.TrimSpace(runFlag.Value.String())
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return true
	}
	for _, name := range []string{
		"TestOnlavaServeRuntimeMatrix",
		"TestOnlavaServeStandardAuthDevBootstrap",
		"TestOnlavaServeProductionFailsForMissingSecrets",
		"TestOnlavaServePopulatesSecretsBeforeTemporalPackageDeclarations",
		"TestOnlavaServeExecutesCronJobs",
		"TestOnlavaBuiltBinaryIsHeadlessByDefault",
		"TestOnlavaDevDashboardNotificationsAndRoutes",
	} {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

func TestOnlavaServeRuntimeMatrix(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedSyntheticApp(t, "serve-runtime-matrix", map[string]string{
		"go.mod":       "module example.com/serveruntime\n\ngo 1.26.3\n\nrequire github.com/pbrazdil/onlava v0.0.0\n\nreplace github.com/pbrazdil/onlava => " + repo + "\n",
		".onlava.json": `{"name":"serveruntime"}`,
		".env":         "ServiceSecret=service-secret\nHelperSecret=helper-secret\n",
		"helper/helper.go": `package helper

var secrets struct {
	HelperSecret string
}

func Value() string {
	return secrets.HelperSecret
}
`,
		"globalmw/global.go": `package globalmw

import "github.com/pbrazdil/onlava/middleware"

//onlava:middleware global target=tag:global
func AddHeader(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	resp.Header().Set("X-Global-Middleware", "true")
	return resp
}
`,
		"service/api.go": `package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"example.com/serveruntime/helper"

	onlava "github.com/pbrazdil/onlava"
	onlavaauth "github.com/pbrazdil/onlava/auth"
	"github.com/pbrazdil/onlava/errs"
	"github.com/pbrazdil/onlava/middleware"
)

var secrets struct {
	ServiceSecret string
}

//onlava:service
type Service struct {
	Prefix string
}

func initService() (*Service, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cwd, ".onlava-init.marker"), []byte("started"), 0o644); err != nil {
		return nil, err
	}
	return &Service{Prefix: "hi"}, nil
}

type EchoRequest struct {
	Title  string ` + "`query:\"title\"`" + `
	Header string ` + "`header:\"X-Echo\"`" + `
	Body   string ` + "`json:\"body\"`" + `
}

type EchoResponse struct {
	Message string ` + "`json:\"message\"`" + `
}

//onlava:api public path=/echo/:name method=GET,POST
func (s *Service) Echo(ctx context.Context, name string, req *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{
		Message: s.Prefix + " " + name + " " + req.Title + " " + req.Header + " " + req.Body,
	}, nil
}

//onlava:api private
func (s *Service) Secret(ctx context.Context) (*EchoResponse, error) {
	return &EchoResponse{Message: "secret:" + s.Prefix}, nil
}

//onlava:api public
func (s *Service) CallPrivate(ctx context.Context) (*EchoResponse, error) {
	return s.Secret(ctx)
}

type AuthData struct {
	Role string ` + "`json:\"role\"`" + `
}

//onlava:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (onlavaauth.UID, *AuthData, error) {
	if token != "token123" {
		return "", nil, errs.B().Code(errs.Unauthenticated).Msg("bad token").Err()
	}
	return "user-1", &AuthData{Role: "admin"}, nil
}

type AuthEchoResponse struct {
	User string ` + "`json:\"user\"`" + `
	Role string ` + "`json:\"role\"`" + `
}

//onlava:api auth
func (s *Service) AuthEcho(ctx context.Context) (*AuthEchoResponse, error) {
	userID, ok := onlavaauth.UserID()
	if !ok {
		return nil, errs.B().Code(errs.Unauthenticated).Msg("missing auth").Err()
	}
	data := onlavaauth.Data().(*AuthData)
	return &AuthEchoResponse{User: string(userID), Role: data.Role}, nil
}

type StatusResponse struct {
	Message string ` + "`json:\"message\"`" + `
	Status  int    ` + "`onlava:\"httpstatus\"`" + `
}

//onlava:api public
func (s *Service) CustomStatus(ctx context.Context) (*StatusResponse, error) {
	return &StatusResponse{Message: "created", Status: 201}, nil
}

//onlava:api public raw path=/raw/*rest
func (s *Service) Raw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"path":   onlava.CurrentRequest().PathParams.Get("rest"),
		"method": onlava.CurrentRequest().Method,
	})
}

type SecretResponse struct {
	Service string ` + "`json:\"service\"`" + `
	Helper  string ` + "`json:\"helper\"`" + `
}

//onlava:api public path=/secrets method=GET
func (s *Service) Secrets(ctx context.Context) (*SecretResponse, error) {
	return &SecretResponse{
		Service: secrets.ServiceSecret,
		Helper:  helper.Value(),
	}, nil
}

type ctxKey struct{}

//onlava:middleware target=all
func (s *Service) InjectContext(req middleware.Request, next middleware.Next) middleware.Response {
	ctx := context.WithValue(req.Context(), ctxKey{}, "svc")
	return next(req.WithContext(ctx))
}

//onlava:api public tag:ctx tag:global
func (s *Service) Context(ctx context.Context) (*EchoResponse, error) {
	value, _ := ctx.Value(ctxKey{}).(string)
	return &EchoResponse{Message: value}, nil
}

//onlava:api private tag:rewrite
func (s *Service) Private(ctx context.Context) (*EchoResponse, error) {
	return &EchoResponse{Message: "handler"}, nil
}

//onlava:api public
func (s *Service) MiddlewareCallPrivate(ctx context.Context) (*EchoResponse, error) {
	return s.Private(ctx)
}

//onlava:api public tag:error
func (s *Service) Error(ctx context.Context) error {
	return nil
}

//onlava:api public raw path=/mw/raw/:id tag:raw
func (s *Service) MiddlewareRaw(w http.ResponseWriter, req *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": onlava.CurrentRequest().PathParams.Get("id"),
	})
}
`,
		"service/mw/mw.go": `package mw

import (
	service "example.com/serveruntime/service"

	"github.com/pbrazdil/onlava/errs"
	"github.com/pbrazdil/onlava/middleware"
)

//onlava:middleware target=tag:rewrite
func Rewrite(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	payload := resp.Payload.(*service.EchoResponse)
	payload.Message = "middleware:" + payload.Message
	return resp
}

//onlava:middleware target=tag:error
func Error(req middleware.Request, next middleware.Next) middleware.Response {
	return middleware.Response{
		Err: errs.B().Code(errs.Internal).Msg("middleware error").Err(),
	}
}

//onlava:middleware target=tag:raw
func RawHeader(req middleware.Request, next middleware.Next) middleware.Response {
	resp := next(req)
	resp.Header().Set("X-Raw-Middleware", "true")
	return resp
}
`,
	})
	markerPath := filepath.Join(appDir, ".onlava-init.marker")
	_ = os.Remove(markerPath)
	t.Cleanup(func() { _ = os.Remove(markerPath) })
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := sharedIntegrationCache(t)
	binary := buildOnlavaBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "serve", "--listen", addr)
	cmd.Env = append(onlavaServeEnv(repo, dashAddr, cacheDir), "ONLAVA_CORS_ALLOW_ORIGINS=http://localhost:5178")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start onlava serve: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	waitForFile(t, markerPath)

	postJSON(t, "http://"+addr+"/echo/Alice?title=Dr", map[string]string{"body": "body"}, map[string]string{"X-Echo": "hdr"}, http.StatusOK, map[string]any{"message": "hi Alice Dr hdr body"})
	getJSON(t, "http://"+addr+"/service.CallPrivate", nil, http.StatusOK, map[string]any{"message": "secret:hi"})
	getJSON(t, "http://"+addr+"/service.CustomStatus", nil, http.StatusCreated, map[string]any{"message": "created"})
	getJSON(t, "http://"+addr+"/service.AuthEcho", map[string]string{"Authorization": "Bearer token123"}, http.StatusOK, map[string]any{"user": "user-1", "role": "admin"})
	getJSON(t, "http://"+addr+"/raw/alpha/beta", nil, http.StatusOK, map[string]any{"path": "alpha/beta", "method": "GET"})
	assertCORSPreflight(t, "http://"+addr+"/service.AuthEcho")
	assertCORSActual(t, "http://"+addr+"/service.AuthEcho")
	getJSON(t, "http://"+addr+"/secrets", nil, http.StatusOK, map[string]any{
		"service": "service-secret",
		"helper":  "helper-secret",
	})
	assertJSONResponseWithHeaders(t, mustRequest(t, http.MethodGet, "http://"+addr+"/service.Context", nil), http.StatusOK, map[string]any{"message": "svc"}, map[string]string{
		"X-Global-Middleware": "true",
	})
	getJSON(t, "http://"+addr+"/service.MiddlewareCallPrivate", nil, http.StatusOK, map[string]any{"message": "middleware:handler"})
	getJSON(t, "http://"+addr+"/service.Error", nil, http.StatusInternalServerError, map[string]any{"code": "internal", "message": "middleware error"})
	assertJSONResponseWithHeaders(t, mustRequest(t, http.MethodGet, "http://"+addr+"/mw/raw/alpha", nil), http.StatusOK, map[string]any{"id": "alpha"}, map[string]string{
		"X-Raw-Middleware": "true",
	})
}

func TestOnlavaServeStandardAuthDevBootstrap(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedFixtureApp(t, repo, "standard-auth")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := sharedIntegrationCache(t)
	binary := buildOnlavaBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "serve", "--listen", addr)
	cmd.Env = onlavaServeEnv(repo, dashAddr, cacheDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start onlava serve: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/users/dev-bootstrap")
	token := postJSONForString(t, "http://"+addr+"/users/dev-bootstrap", map[string]string{
		"user_id":   "user-123",
		"tenant_id": "00000000-0000-0000-0000-000000000123",
	}, "token")
	getJSON(t, "http://"+addr+"/whoami", map[string]string{"Authorization": "Bearer " + token}, http.StatusOK, map[string]any{
		"user_id":   "user-123",
		"tenant_id": "00000000-0000-0000-0000-000000000123",
	})
}

func TestOnlavaServeProductionFailsForMissingSecrets(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedFixtureAppVariant(t, repo, "secrets", "missing-env", nil, []string{".env"})
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := sharedIntegrationCache(t)
	binary := buildOnlavaBinary(t, repo)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "serve", "--listen", addr, "--env", "production")
	cmd.Env = onlavaServeEnv(repo, dashAddr, cacheDir)
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("onlava serve --env production succeeded with missing secrets; output:\n%s", output)
	}
	if ctx.Err() != nil {
		t.Fatalf("onlava serve --env production timed out; output:\n%s", output)
	}
	got := string(output)
	for _, want := range []string{"missing required secrets for production"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q does not contain %q", got, want)
		}
	}
	if !strings.Contains(got, "ServiceSecret") && !strings.Contains(got, "HelperSecret") {
		t.Fatalf("output %q does not name a missing declared secret", got)
	}
}

func TestOnlavaServePopulatesSecretsBeforeTemporalPackageDeclarations(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedSyntheticApp(t, "temporalsecrets", map[string]string{
		"go.mod":       "module example.com/temporalsecrets\n\ngo 1.26.3\n\nrequire github.com/pbrazdil/onlava v0.0.0\n\nreplace github.com/pbrazdil/onlava => " + repo + "\n",
		".onlava.json": `{"name":"temporalsecrets"}`,
		".env":         "TestActivityTimeoutSeconds=10\n",
		"queue/api.go": `package queue

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/pbrazdil/onlava/temporal"
)

var secrets struct {
	TestActivityTimeoutSeconds string
}

type Input struct {
	ID string ` + "`json:\"id\"`" + `
}

type Response struct {
	TimeoutSeconds int    ` + "`json:\"timeout_seconds\"`" + `
	Secret         string ` + "`json:\"secret\"`" + `
}

var activity = temporal.NewActivity[*Input, temporal.Void]("queue.Handle/v1", temporal.ActivityConfig{
	TaskQueue:    "queue.go",
	StartToClose: time.Duration(parsePositiveInt(secrets.TestActivityTimeoutSeconds, 1)) * time.Second,
}, func(context.Context, *Input) (temporal.Void, error) {
	return temporal.Void{}, nil
})

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

//onlava:api public path=/concurrency method=GET
func Concurrency(ctx context.Context) (*Response, error) {
	return &Response{
		TimeoutSeconds: int(activity.Config().StartToClose / time.Second),
		Secret: secrets.TestActivityTimeoutSeconds,
	}, nil
}
`,
	})

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	cacheDir := sharedIntegrationCache(t)
	binary := buildOnlavaBinary(t, repo)
	temporalAddr := startTemporalDevServerForTest(t, filepath.Join(t.TempDir(), "temporal-cache"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var output strings.Builder
	cmd := exec.CommandContext(ctx, binary, "serve", "--listen", addr)
	cmd.Env = append(onlavaServeEnv(repo, dashAddr, cacheDir), "TEMPORAL_ADDRESS="+temporalAddr)
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start onlava serve: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTPWithProcessOutput(t, "http://"+addr+"/concurrency", &output)
	getJSON(t, "http://"+addr+"/concurrency", nil, http.StatusOK, map[string]any{
		"timeout_seconds": 10,
		"secret":          "10",
	})
}

func TestOnlavaServeExecutesCronJobs(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedFixtureApp(t, repo, "cron")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	apiCacheDir := sharedIntegrationCache(t)
	workerCacheDir := apiCacheDir
	temporalCacheDir := filepath.Join(t.TempDir(), "temporal-cache")
	statePath := filepath.Join(appDir, ".onlava-cron-state.json")
	_ = os.Remove(statePath)
	t.Cleanup(func() { _ = os.Remove(statePath) })
	binary := buildOnlavaBinary(t, repo)
	temporalAddr := startTemporalDevServerForTest(t, temporalCacheDir)
	commonEnv := []string{
		"TEMPORAL_ADDRESS=" + temporalAddr,
		"ONLAVA_BUILD_ID=test",
		"ONLAVA_TEMPORAL_TASK_QUEUE_PREFIX=onlava.cronapp",
		"ONLAVA_TEMPORAL_DEPLOYMENT_NAME=onlava-cronapp",
	}
	apiEnv := append(onlavaServeEnv(repo, dashAddr, apiCacheDir), commonEnv...)
	workerEnv := append(onlavaServeEnv(repo, dashAddr, workerCacheDir), commonEnv...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	var workerOutput strings.Builder
	workerCmd := exec.CommandContext(workerCtx, binary, "worker")
	workerCmd.Env = workerEnv
	workerCmd.Stdout = &workerOutput
	workerCmd.Stderr = &workerOutput
	workerCmd.Stdin = nil
	workerCmd.Dir = appDir
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start onlava worker: %v", err)
	}
	defer killOnlavaProcess(t, workerCancel, workerCmd)

	var apiOutput strings.Builder
	cmd := exec.CommandContext(ctx, binary, "serve", "--listen", addr)
	cmd.Env = apiEnv
	cmd.Stdout = &apiOutput
	cmd.Stderr = &apiOutput
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start onlava serve: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTPWithProcessOutput(t, "http://"+addr+"/cron/status", &apiOutput, &workerOutput)
	startTemporalCronWorkflow(t, temporalAddr, "onlava-cronapp.cron.onlava-tick")
	waitForCronStatus(t, "http://"+addr+"/cron/status")
}

type temporalCronInputForTest struct {
	AppID                string
	JobID                string
	ActivityName         string
	TaskQueue            string
	ActivityStartToClose time.Duration
	ActivityRetryPolicy  onlavaruntime.CronRetryPolicy
}

func startTemporalCronWorkflow(t *testing.T, temporalAddr, workflowID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := temporalclient.Dial(temporalclient.Options{HostPort: temporalAddr})
	if err != nil {
		t.Fatalf("dial temporal: %v", err)
	}
	defer client.Close()
	_, err = client.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "onlava.cronapp.cron.go",
	}, "onlava.cron.Invoke/v1", temporalCronInputForTest{
		AppID:                "cronapp",
		JobID:                "onlava-tick",
		ActivityName:         "onlava.cron.onlava-tick/v1",
		TaskQueue:            "onlava.cronapp.cron.go",
		ActivityStartToClose: time.Hour,
	})
	if err != nil {
		t.Fatalf("start temporal cron workflow %s: %v", workflowID, err)
	}
}

func TestOnlavaBuiltBinaryIsHeadlessByDefault(t *testing.T) {
	t.Parallel()

	repo := repoRoot(t)
	appDir := cachedSyntheticApp(t, "headless", map[string]string{
		"go.mod":       "module example.com/headless\n\ngo 1.26.3\n\nrequire github.com/pbrazdil/onlava v0.0.0\n\nreplace github.com/pbrazdil/onlava => " + repo + "\n",
		".onlava.json": `{"name":"headlessapp","proxy":{"api_host":"api.acme.localhost","console_host":"console.acme.localhost","frontends":{"web":{"host":"web.acme.localhost"}}}}`,
		"svc/api.go": `package svc

import "context"

type Response struct {
	Message string ` + "`json:\"message\"`" + `
}

//onlava:api public
func Ping(context.Context) (*Response, error) {
	return &Response{Message: "pong"}, nil
}
`,
	})
	onlavaBinary := buildOnlavaBinary(t, repo)
	cacheDir := sharedIntegrationCache(t)
	outputPath := filepath.Join(cacheDir, "bin", "headless-app")

	buildCmd := exec.Command(onlavaBinary, "build", "-o", outputPath)
	buildCmd.Dir = appDir
	buildCmd.Env = append(os.Environ(), "ONLAVA_DEV_CACHE_DIR="+cacheDir)
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("onlava build failed: %v\n%s", err, buildOutput)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("built binary missing: %v", err)
	}

	port := freePort(t)
	addr := "127.0.0.1:" + port
	httpsPort := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, outputPath)
	cmd.Env = append(
		os.Environ(),
		"ONLAVA_LISTEN_ADDR="+addr,
		"ONLAVA_LOCAL_PROXY_HTTPS_PORT="+httpsPort,
		"ONLAVA_LOCAL_PROXY_SKIP_TRUST_INSTALL=1",
		"ONLAVA_DEV_CACHE_DIR="+cacheDir,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start built app: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/svc.Ping")
	getJSON(t, "http://"+addr+"/svc.Ping", nil, http.StatusOK, map[string]any{"message": "pong"})
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+httpsPort, 50*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("built binary unexpectedly listened on local HTTPS proxy port %s", httpsPort)
	}
}

func TestOnlavaDevDashboardNotificationsAndRoutes(t *testing.T) {
	repo := repoRoot(t)
	serviceSource := `package service

import "context"

//onlava:service
type Service struct {
	Prefix string
}

func initService() (*Service, error) {
	return &Service{Prefix: "hi"}, nil
}

type Response struct {
	Message string ` + "`json:\"message\"`" + `
}

//onlava:api private
func (s *Service) Secret(ctx context.Context) (*Response, error) {
	return &Response{Message: "secret:" + s.Prefix}, nil
}

//onlava:api public
func (s *Service) CallPrivate(ctx context.Context) (*Response, error) {
	return s.Secret(ctx)
}
`
	appDir := cachedSyntheticApp(t, "devdashboard", map[string]string{
		"go.mod":         "module example.com/devdashboard\n\ngo 1.26.3\n\nrequire github.com/pbrazdil/onlava v0.0.0\n\nreplace github.com/pbrazdil/onlava => " + repo + "\n",
		".onlava.json":   `{"name":"basicapp","proxy":{"workspace":"ignored","api_host":"api.acme.localhost","console_host":"console.acme.localhost","frontends":{"web":{"host":"web.acme.localhost"}}}}`,
		".env":           "# Fixture environment intentionally empty.\n",
		"service/api.go": serviceSource,
	})
	unlockFixture := lockIntegrationFixtureMutation(t, appDir)
	t.Cleanup(unlockFixture)
	apiPath := filepath.Join(appDir, "service", "api.go")
	writeFileIfChanged(t, apiPath, serviceSource)
	t.Cleanup(func() {
		writeFileIfChanged(t, apiPath, serviceSource)
	})

	port := freePort(t)
	addr := "127.0.0.1:" + port
	dashAddr := "127.0.0.1:" + freePort(t)
	httpPort := freePort(t)
	httpsPort := freePort(t)
	cacheDir := sharedIntegrationCache(t)
	binary := buildOnlavaBinary(t, repo)

	frontendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	frontendAddr := frontendLn.Addr().String()
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

	cmd := exec.CommandContext(ctx, binary, "dev", "--listen", addr, "--proxy")
	cmd.Env = onlavaDevProxyEnv(repo, dashAddr, cacheDir, httpPort, httpsPort, frontendAddr)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start onlava dev: %v", err)
	}
	defer killOnlavaProcess(t, cancel, cmd)

	waitForHTTP(t, "http://"+addr+"/service.CallPrivate")
	waitForHTTP(t, "http://"+dashAddr+"/basicapp")

	client := insecureHTTPSClient()
	apiURL := "https://api.acme.localhost:" + httpsPort + "/service.CallPrivate"
	getJSONWithClient(t, client, apiURL, nil, http.StatusOK, map[string]any{"message": "secret:hi"})

	consoleURL := "https://console.acme.localhost:" + httpsPort + "/"
	waitForURL(t, client, consoleURL)
	resp, err := client.Get(consoleURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("unexpected console status %d", resp.StatusCode)
	}

	frontendURL := "https://web.acme.localhost:" + httpsPort + "/"
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

	wsConn, _, err := websocket.DefaultDialer.Dial("ws://"+dashAddr+"/__onlava", nil)
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
