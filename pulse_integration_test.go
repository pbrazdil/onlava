package pulse_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPulseRunBasicApp(t *testing.T) {
	repo := repoRoot(t)
	appDir := filepath.Join(repo, "testdata", "apps", "basic")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = os.Environ()
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

func TestPulseRunReloadsOnGoChanges(t *testing.T) {
	repo := repoRoot(t)
	sourceAppDir := filepath.Join(repo, "testdata", "apps", "basic")
	appDir := filepath.Join(t.TempDir(), "basic")
	copyDir(t, sourceAppDir, appDir)
	rewriteFixtureReplace(t, filepath.Join(appDir, "go.mod"), repo)

	port := freePort(t)
	addr := "127.0.0.1:" + port
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = os.Environ()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start pulse run: %v", err)
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
	repo := repoRoot(t)
	appDir := filepath.Join(repo, "testdata", "apps", "secrets")
	port := freePort(t)
	addr := "127.0.0.1:" + port
	binary := buildPulseBinary(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "run", "--listen", addr)
	cmd.Env = os.Environ()
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

func TestPulseBuildProducesRunnableBinary(t *testing.T) {
	repo := repoRoot(t)
	appDir := filepath.Join(repo, "testdata", "apps", "basic")
	pulseBinary := buildPulseBinary(t, repo)
	outputPath := filepath.Join(t.TempDir(), "basic-app")

	buildCmd := exec.Command(pulseBinary, "build", "-o", outputPath)
	buildCmd.Dir = appDir
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
	cmd.Env = append(os.Environ(), "PULSE_LISTEN_ADDR="+addr)
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

func buildPulseBinary(t *testing.T, repo string) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "pulse")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/pulse")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build pulse binary: %v\n%s", err, output)
	}
	return binPath
}

func stopPulseProcess(t *testing.T, cancel context.CancelFunc, cmd *exec.Cmd) {
	t.Helper()
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
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("server did not start: %s", url)
}

func waitForJSONResponse(t *testing.T, url string, wantStatus int, want map[string]any) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		var got map[string]any
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr == nil && resp.StatusCode == wantStatus && mapsEqual(got, want) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("response did not settle to %v at %s", want, url)
}

func postJSON(t *testing.T, url string, body any, headers map[string]string, wantStatus int, want map[string]any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	assertJSONResponse(t, req, wantStatus, want)
}

func getJSON(t *testing.T, url string, headers map[string]string, wantStatus int, want map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	assertJSONResponse(t, req, wantStatus, want)
}

func assertCORSPreflight(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodOptions, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5178")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected preflight status %d: %s", resp.StatusCode, body)
	}
	if got, want := resp.Header.Get("Access-Control-Allow-Origin"), "http://localhost:5178"; got != want {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodGet) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want it to include GET", got)
	}
	if got := strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers")); !strings.Contains(got, "authorization") || !strings.Contains(got, "content-type") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want authorization and content-type", got)
	}
	vary := resp.Header.Get("Vary")
	for _, want := range []string{"Origin", "Authorization", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
		if !strings.Contains(vary, want) {
			t.Fatalf("Vary = %q, want %q", vary, want)
		}
	}
}

func assertCORSActual(t *testing.T, url string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5178")
	req.Header.Set("Authorization", "Bearer token123")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected actual CORS status %d: %s", resp.StatusCode, body)
	}
	if got, want := resp.Header.Get("Access-Control-Allow-Origin"), "http://localhost:5178"; got != want {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	vary := resp.Header.Get("Vary")
	for _, want := range []string{"Origin", "Authorization"} {
		if !strings.Contains(vary, want) {
			t.Fatalf("Vary = %q, want %q", vary, want)
		}
	}
}

func assertJSONResponse(t *testing.T, req *http.Request, wantStatus int, want map[string]any) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, want %d: %s", resp.StatusCode, wantStatus, body)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !mapsEqual(got, want) {
		t.Fatalf("unexpected body: got=%v want=%v", got, want)
	}
}

func mapsEqual(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return false
		}
		if strings.TrimSpace(toString(gotValue)) != strings.TrimSpace(toString(wantValue)) {
			return false
		}
	}
	return true
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return strings.Split(ln.Addr().String(), ":")[1]
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(wd)
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	}); err != nil {
		t.Fatal(err)
	}
}

func rewriteFixtureReplace(t *testing.T, goModPath, repo string) {
	t.Helper()
	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "replace pulse.dev => ../../..", "replace pulse.dev => "+repo, 1)
	if updated == string(data) {
		t.Fatalf("expected fixture go.mod replace in %s", goModPath)
	}
	if err := os.WriteFile(goModPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}
