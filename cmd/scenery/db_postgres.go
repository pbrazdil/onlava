package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	localagent "scenery.sh/internal/agent"
	"scenery.sh/internal/envpolicy"
)

type dbPostgresOptions struct {
	Command string
	AppRoot string
	JSON    bool
}

func dbPostgresCommand(args []string) error {
	return runDBPostgresCommand(context.Background(), os.Stdout, args)
}

func runDBPostgresCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseDBPostgresArgs(args)
	if err != nil {
		return err
	}
	switch opts.Command {
	case "install", "start":
		return runDBPostgresStart(ctx, stdout, opts)
	case "status":
		return runDBPostgresStatus(ctx, stdout, opts)
	case "logs":
		return runDBPostgresLogs(ctx, stdout, opts)
	case "restart":
		if err := runDBPostgresStop(ctx, io.Discard, opts); err != nil && !localagent.IsNotFound(err) {
			return err
		}
		return runDBPostgresStart(ctx, stdout, opts)
	case "stop", "uninstall":
		return runDBPostgresStop(ctx, stdout, opts)
	default:
		return fmt.Errorf("unknown db postgres command %q", opts.Command)
	}
}

func parseDBPostgresArgs(args []string) (dbPostgresOptions, error) {
	if len(args) == 0 {
		return dbPostgresOptions{}, fmt.Errorf("usage: scenery db postgres install|start|status|logs|stop|restart|uninstall [--json] [--app-root <path>]")
	}
	opts := dbPostgresOptions{Command: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.JSON = true
		case "--app-root":
			i++
			if i >= len(args) {
				return dbPostgresOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		default:
			return dbPostgresOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	switch opts.Command {
	case "install", "start", "status", "logs", "stop", "restart", "uninstall":
	default:
		return dbPostgresOptions{}, fmt.Errorf("unknown db postgres command %q", opts.Command)
	}
	return opts, nil
}

func runDBPostgresStart(ctx context.Context, stdout io.Writer, opts dbPostgresOptions) error {
	_, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	agent, err := localagent.Ensure(ctx)
	if err != nil {
		return err
	}
	env, err := envWithManagedPostgresAdminURL(ctx, cfg, envpolicy.Environ(), agent)
	if err != nil {
		return err
	}
	adminURL, _ := lookupEnvValue(env, devPostgresAdminURLEnv)
	substrate, err := agent.GetSubstrate(ctx, localagent.SubstratePostgres)
	if err != nil {
		return err
	}
	result := map[string]any{
		"schema_version": "scenery.db.postgres.status.v1",
		"ok":             true,
		"provider":       postgresBranchProviderName,
		"status":         substrate.Status,
		"admin_url":      adminURL,
		"substrate":      substrate,
	}
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "Postgres %s\n", substrate.Status)
	return nil
}

func runDBPostgresStatus(ctx context.Context, stdout io.Writer, opts dbPostgresOptions) error {
	agent, err := localagent.Ensure(ctx)
	if err != nil {
		return err
	}
	substrate, err := agent.GetSubstrate(ctx, localagent.SubstratePostgres)
	if err != nil {
		if localagent.IsNotFound(err) {
			result := map[string]any{
				"schema_version": "scenery.db.postgres.status.v1",
				"ok":             false,
				"provider":       postgresBranchProviderName,
				"status":         "not_started",
				"message":        "managed Postgres substrate is not recorded",
			}
			if opts.JSON {
				return writeInspectJSON(stdout, result)
			}
			fmt.Fprintln(stdout, "Postgres not_started")
			return nil
		}
		return err
	}
	result := map[string]any{
		"schema_version": "scenery.db.postgres.status.v1",
		"ok":             substrate.Status == "ready",
		"provider":       postgresBranchProviderName,
		"status":         substrate.Status,
		"substrate":      substrate,
	}
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "Postgres %s\n", substrate.Status)
	return nil
}

func runDBPostgresLogs(ctx context.Context, stdout io.Writer, opts dbPostgresOptions) error {
	agent, err := localagent.Ensure(ctx)
	if err != nil {
		return err
	}
	substrate, err := agent.GetSubstrate(ctx, localagent.SubstratePostgres)
	if err != nil {
		return err
	}
	logPath := strings.TrimSpace(substrate.Endpoints["log"])
	if logPath == "" {
		return fmt.Errorf("managed Postgres log path is not recorded")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	_, err = stdout.Write(data)
	return err
}

func runDBPostgresStop(ctx context.Context, stdout io.Writer, opts dbPostgresOptions) error {
	agent, err := localagent.Ensure(ctx)
	if err != nil {
		return err
	}
	substrate, err := agent.DeleteSubstrate(ctx, localagent.SubstratePostgres)
	if err != nil {
		if localagent.IsNotFound(err) {
			if opts.JSON {
				return writeInspectJSON(stdout, map[string]any{
					"schema_version": "scenery.db.postgres.status.v1",
					"ok":             true,
					"provider":       postgresBranchProviderName,
					"status":         "not_started",
				})
			}
			fmt.Fprintln(stdout, "Postgres not_started")
			return nil
		}
		return err
	}
	if pid := firstPositiveInt(substrate.OwnerPID, substrate.Owner.PID); pid > 0 {
		if err := killProcessIDTree(pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
	}
	if opts.JSON {
		return writeInspectJSON(stdout, map[string]any{
			"schema_version": "scenery.db.postgres.status.v1",
			"ok":             true,
			"provider":       postgresBranchProviderName,
			"status":         "stopped",
			"substrate":      substrate,
		})
	}
	fmt.Fprintln(stdout, "Postgres stopped")
	return nil
}
