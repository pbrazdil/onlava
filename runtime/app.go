package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"pulse.dev/internal/dbstudio"
	"pulse.dev/internal/localproxy"
	"pulse.dev/pubsub"
)

func ListenAddrFromEnv() string {
	if value := os.Getenv("PULSE_LISTEN_ADDR"); value != "" {
		return value
	}
	return "127.0.0.1:4000"
}

func Main(cfg AppConfig) error {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ListenAddrFromEnv()
	}
	SetAppConfig(cfg)
	stopReporting := startDevelopmentReporting(cfg)
	defer stopReporting()
	FlushMissingSecretsWarnings()

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	stopSupervisorMonitor := startSupervisorParentMonitor(cancelRun)
	defer stopSupervisorMonitor()

	server, err := newServer(cfg.ListenAddr)
	if err != nil {
		return err
	}
	if err := InitializeServices(); err != nil {
		return err
	}
	stopPubSub, err := pubsub.StartLocalRuntime(runCtx, pubsub.LocalRuntimeConfig{AppID: cfg.Name})
	if err != nil {
		return err
	}
	scheduler := startCronScheduler(runCtx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	var routes localproxy.Routes
	dbStudioURL := ""
	var dbStudioMu sync.Mutex
	var dbStudio *dbstudio.Instance
	if !launchedBySupervisor() {
		proxy, err := startLocalHTTPSProxy(cfg)
		if err != nil {
			slog.Warn("local HTTPS proxy unavailable", "err", err)
		} else if proxy != nil {
			defer func() {
				_ = proxy.Close()
			}()
			routes = proxy.Routes()
			if routes.APIURL != "" {
				SetPublicBaseURL(routes.APIURL)
			}
		}
		if cfg.EnableDBStudio {
			root := appRootFromEnvOrCWD()
			studioCfg, ok, err := dbstudio.Discover(root)
			if err != nil {
				slog.Warn("db studio unavailable", "err", err)
			} else if ok {
				dbStudioURL = dbstudio.DefaultURL(dbstudio.DefaultPort)
				go func() {
					inst, startErr := dbstudio.Start(runCtx, dbstudio.Options{
						AppRoot: root,
						AppID:   cfg.Name,
						Config:  studioCfg,
						Port:    dbstudio.DefaultPort,
						Stdout:  osStdout(),
						Stderr:  osStderr(),
					})
					if startErr != nil {
						slog.Warn("db studio unavailable", "err", startErr)
						return
					}
					if inst == nil {
						return
					}
					dbStudioMu.Lock()
					dbStudio = inst
					dbStudioMu.Unlock()
				}()
			}
		}
		defer func() {
			dbStudioMu.Lock()
			inst := dbStudio
			dbStudioMu.Unlock()
			_ = inst.Close()
		}()
		printRuntimeBanner(osStdout(), cfg.ListenAddr, routes, dbStudioURL)
	}

	logTrace(context.Background(), fmt.Sprintf("registered %d API endpoints", len(listEndpoints())))
	logTrace(context.Background(), "listening for incoming HTTP requests")

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-sigCtx.Done()
		cancelRun()
	}()

	select {
	case <-runCtx.Done():
		cancelRun()
		pubsubCtx, pubsubCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pubsubCancel()
		if err := stopPubSub(pubsubCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		cronCtx, cronCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cronCancel()
		if err := scheduler.Stop(cronCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		cancelRun()
		pubsubCtx, pubsubCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pubsubCancel()
		if stopErr := stopPubSub(pubsubCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return stopErr
		}
		cronCtx, cronCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cronCancel()
		if stopErr := scheduler.Stop(cronCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return stopErr
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func launchedBySupervisor() bool {
	return os.Getenv("PULSE_DEV_SUPERVISOR") == "1"
}

func startLocalHTTPSProxy(cfg AppConfig) (*localproxy.Proxy, error) {
	if os.Getenv("PULSE_LOCAL_PROXY") == "0" {
		return nil, nil
	}
	workspace := cfg.Workspace
	if workspace == "" {
		workspace = localproxy.DiscoverWorkspace(mustGetwd(), cfg.Name)
	}
	proxyCfg := localproxy.BuildConfig(localproxy.Config{
		Workspace:         workspace,
		APIHost:           cfg.ProxyAPIHost,
		ConsoleHost:       cfg.ProxyConsoleHost,
		MCPHost:           cfg.ProxyMCPHost,
		FrontendHost:      cfg.ProxyFrontendHost,
		APIUpstream:       cfg.ListenAddr,
		DashboardUpstream: strings.TrimSpace(os.Getenv("PULSE_DEV_DASHBOARD_ADDR")),
		FrontendUpstream:  localproxy.DiscoverFrontendUpstream(mustGetwd()),
	})
	if proxyCfg.Workspace == "" && proxyCfg.APIHost == "" {
		return nil, nil
	}
	return localproxy.Start(proxyCfg)
}

func printRuntimeBanner(out io.Writer, listenAddr string, routes localproxy.Routes, dbStudioURL string) {
	if out == nil {
		return
	}
	apiURL := "http://" + listenAddr
	if routes.APIURL != "" {
		apiURL = routes.APIURL
	}

	lines := []string{
		"",
		"  Pulse development server running!",
		"",
		fmt.Sprintf("  %-26s  %s", "Your API is running at:", apiURL),
	}
	if routes.ConsoleURL != "" {
		lines = append(lines, fmt.Sprintf("  %-26s  %s", "Development Dashboard URL:", routes.ConsoleURL))
	}
	if routes.MCPBaseURL != "" {
		lines = append(lines, fmt.Sprintf("  %-26s  %s", "MCP SSE URL:", routes.MCPBaseURL+"/sse?appID="+Meta().AppID))
	}
	if routes.FrontendURL != "" {
		lines = append(lines, fmt.Sprintf("  %-26s  %s", "Pulse App URL:", routes.FrontendURL))
	}
	if dbStudioURL != "" {
		lines = append(lines, fmt.Sprintf("  %-26s  %s", "Drizzle Studio URL:", dbStudioURL))
	}
	lines = append(lines, "")
	for _, line := range lines {
		_, _ = fmt.Fprintln(out, line)
	}
}

func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

var osStdout = func() io.Writer { return os.Stdout }

func appRootFromEnvOrCWD() string {
	if root := strings.TrimSpace(os.Getenv("PULSE_APP_ROOT")); root != "" {
		return root
	}
	return mustGetwd()
}
