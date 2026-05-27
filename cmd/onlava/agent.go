package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	"github.com/pbrazdil/onlava/internal/app"
)

type agentOptions struct {
	SocketPath string
	RouterAddr string
	RouterTLS  bool
	RouterHTTP bool
	Trust      bool
	JSON       bool
}

type statusOptions struct {
	AppRoot   string
	SessionID string
	JSON      bool
}

type downOptions struct {
	AppRoot   string
	SessionID string
}

func agentCommand(args []string) error {
	if len(args) > 0 && args[0] == "restart" {
		return agentRestartCommand(args[1:])
	}
	opts, err := parseAgentArgs(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if opts.JSON {
		paths, err := localagent.DefaultPaths()
		if err != nil {
			return err
		}
		if opts.SocketPath != "" {
			paths.SocketPath = opts.SocketPath
		}
		routerTLS := opts.effectiveRouterTLS()
		routerScheme := "http"
		if routerTLS {
			routerScheme = "https"
		}
		fmt.Fprintf(os.Stdout, "{\"type\":\"agent.start\",\"socket_path\":%q,\"router_addr\":%q,\"router_scheme\":%q}\n", paths.SocketPath, firstNonEmpty(opts.RouterAddr, localagent.RouterAddrFromEnv()), routerScheme)
	}
	dashboardAddr, err := freeLoopbackAddr()
	if err != nil {
		return err
	}
	server, err := localagent.NewServer(localagent.RunOptions{
		SocketPath:   opts.SocketPath,
		RouterAddr:   opts.RouterAddr,
		RouterTLS:    opts.effectiveRouterTLS(),
		InstallTrust: opts.Trust,
		DashboardBackend: localagent.Backend{
			Network: "tcp",
			Addr:    dashboardAddr,
		},
		JSON: opts.JSON,
	})
	if err != nil {
		return err
	}
	dashboard, err := startAgentDashboard(ctx, server, dashboardAddr)
	if err != nil {
		_ = server.Close()
		return err
	}
	defer dashboard.Close()
	return server.Run(ctx)
}

func agentRestartCommand(args []string) error {
	opts, err := parseAgentArgs(args)
	if err != nil {
		return err
	}
	paths, err := localagent.DefaultPaths()
	if err != nil {
		return err
	}
	if opts.SocketPath != "" {
		paths.SocketPath = filepath.Clean(opts.SocketPath)
		paths.RunDir = filepath.Dir(paths.SocketPath)
	}
	client := localagent.NewClient(paths.SocketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oldHealth, running := currentAgentHealth(ctx, client)
	if running && oldHealth.PID > 0 {
		if err := signalAgentPID(oldHealth.PID); err != nil {
			return fmt.Errorf("stop onlava agent pid %d: %w", oldHealth.PID, err)
		}
		if err := waitForAgentStop(ctx, client, oldHealth.PID); err != nil {
			return err
		}
	}
	if err := localagent.StartProcess(paths, localagent.StartOptions{
		RouterAddr: opts.RouterAddr,
		RouterTLS:  opts.effectiveRouterTLS(),
		RouterHTTP: opts.RouterHTTP,
		Trust:      opts.Trust,
	}); err != nil {
		return err
	}
	health, err := waitForAgentStart(ctx, client, oldHealth.PID)
	if err != nil {
		return err
	}
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"schema_version": "onlava.agent.restart.v1",
			"old_pid":        oldHealth.PID,
			"pid":            health.PID,
			"socket_path":    health.SocketPath,
			"router_addr":    health.RouterAddr,
			"router_scheme":  health.RouterScheme,
		})
	}
	fmt.Fprintf(os.Stdout, "restarted onlava agent")
	if health.PID > 0 {
		fmt.Fprintf(os.Stdout, " (pid %d)", health.PID)
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

func parseAgentArgs(args []string) (agentOptions, error) {
	var opts agentOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--socket":
			i++
			if i >= len(args) {
				return agentOptions{}, fmt.Errorf("missing value for --socket")
			}
			opts.SocketPath = args[i]
		case "--router-listen":
			i++
			if i >= len(args) {
				return agentOptions{}, fmt.Errorf("missing value for --router-listen")
			}
			opts.RouterAddr = args[i]
		case "--router-tls":
			opts.RouterTLS = true
			opts.RouterHTTP = false
		case "--router-http":
			opts.RouterHTTP = true
			opts.RouterTLS = false
		case "--trust":
			opts.Trust = true
			opts.RouterTLS = true
			opts.RouterHTTP = false
		case "--json":
			opts.JSON = true
		default:
			return agentOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func (opts agentOptions) effectiveRouterTLS() bool {
	if opts.Trust || opts.RouterTLS {
		return true
	}
	if opts.RouterHTTP {
		return false
	}
	return localagent.RouterTLSDefault()
}

func currentAgentHealth(ctx context.Context, client *localagent.Client) (localagent.HealthResponse, bool) {
	health, err := client.Health(ctx)
	return health, err == nil
}

func signalAgentPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func waitForAgentStop(ctx context.Context, client *localagent.Client, pid int) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		health, err := client.Health(ctx)
		if err != nil || health.PID != pid {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for onlava agent pid %d to stop: %w", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForAgentStart(ctx context.Context, client *localagent.Client, oldPID int) (localagent.HealthResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		health, err := client.Health(ctx)
		if err == nil && (oldPID == 0 || health.PID != oldPID) {
			return health, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			if lastErr == nil {
				lastErr = ctx.Err()
			}
			return localagent.HealthResponse{}, fmt.Errorf("timed out waiting for restarted onlava agent: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func statusCommand(args []string) error {
	opts, err := parseStatusArgs(args)
	if err != nil {
		return err
	}
	client, err := localagent.DefaultClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	appRoot := ""
	if opts.AppRoot != "" {
		appRoot, err = resolveStatusAppRoot(opts.AppRoot)
		if err != nil {
			return err
		}
	}
	sessions, err := client.List(ctx, appRoot)
	if err != nil {
		return err
	}
	if opts.SessionID != "" {
		filtered := sessions[:0]
		for _, session := range sessions {
			if session.SessionID == opts.SessionID {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}
	if opts.JSON {
		health, _ := client.Health(ctx)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"schema_version": "onlava.agent.status.v1",
			"agent":          health,
			"sessions":       sessions,
		})
	}
	for _, session := range sessions {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", session.SessionID, session.Status, session.AppRoot)
	}
	return nil
}

func parseStatusArgs(args []string) (statusOptions, error) {
	opts := statusOptions{JSON: false}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.JSON = true
		case "--app-root":
			i++
			if i >= len(args) {
				return statusOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return statusOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.SessionID = args[i]
		default:
			return statusOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func downCommand(args []string) error {
	opts, err := parseDownArgs(args)
	if err != nil {
		return err
	}
	client, err := localagent.DefaultClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		appRoot, err := resolveStatusAppRoot(opts.AppRoot)
		if err != nil {
			return err
		}
		sessions, err := client.List(ctx, appRoot)
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no onlava agent session found for %s", appRoot)
		}
		sessionID = sessions[0].SessionID
	}
	session, err := client.Delete(ctx, sessionID, true)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "stopped onlava session %s\n", session.SessionID)
	return nil
}

func parseDownArgs(args []string) (downOptions, error) {
	var opts downOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return downOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return downOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.SessionID = args[i]
		default:
			return downOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func resolveStatusAppRoot(value string) (string, error) {
	start := strings.TrimSpace(value)
	if start == "" {
		start = "."
	}
	root, _, err := app.DiscoverRoot(start)
	if err == nil {
		return root, nil
	}
	if value != "" {
		abs, absErr := filepath.Abs(value)
		if absErr != nil {
			return "", errors.Join(err, absErr)
		}
		return filepath.Clean(abs), nil
	}
	return "", err
}
