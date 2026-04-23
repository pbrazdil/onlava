package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"pulse.dev/internal/stdlog"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	stdlog.Install(os.Stderr)
	log.SetFlags(log.LstdFlags)
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "build":
		return buildCommand(args[1:])
	case "psql":
		return psqlCommand(args[1:])
	case "check":
		return checkCommand(args[1:])
	case "inspect":
		return inspectCommand(args[1:])
	case "admin":
		return adminCommand(args[1:])
	case "logs":
		return logsCommand(args[1:])
	case "test":
		return testCommand(args[1:])
	case "gen":
		return genCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usageError() error {
	return fmt.Errorf("usage:\n  pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json]\n  pulse build [--app-root <path>] [-o <path>] [--db-studio]\n  pulse psql [--app-root <path>] [psql args...]\n  pulse check [--app-root <path>]\n  pulse inspect app|routes|services|build|paths --json [--app-root <path>]\n  pulse admin traces clear --json [--app-root <path>]\n  pulse admin pubsub clear --json [--app-root <path>]\n  pulse logs [--app-root <path>] [--limit <n>] [--stream all|stdout|stderr] [-f|--follow]\n  pulse test [--app-root <path>] [go test flags/packages...]\n  pulse gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]")
}

func runCommand(args []string) error {
	opts, err := parseRunArgs(args)
	if err != nil {
		return err
	}
	addr := resolveListenAddr(opts.Listen, opts.Port)
	return runWithWatch(addr, opts.Verbose, opts.JSON, opts.AppRoot)
}

type runOptions struct {
	Listen  string
	Port    int
	Verbose bool
	JSON    bool
	AppRoot string
}

func parseRunArgs(args []string) (runOptions, error) {
	opts := runOptions{Port: 4000}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			i++
			if i >= len(args) {
				return runOptions{}, fmt.Errorf("missing value for --port")
			}
			value, err := strconv.Atoi(args[i])
			if err != nil {
				return runOptions{}, fmt.Errorf("invalid port %q", args[i])
			}
			opts.Port = value
		case "--listen":
			i++
			if i >= len(args) {
				return runOptions{}, fmt.Errorf("missing value for --listen")
			}
			opts.Listen = args[i]
		case "--verbose", "-v":
			opts.Verbose = true
		case "--json":
			opts.JSON = true
		case "--app-root":
			i++
			if i >= len(args) {
				return runOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		default:
			return runOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func resolveListenAddr(listen string, port int) string {
	if listen == "" {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	if _, _, err := net.SplitHostPort(listen); err == nil {
		return listen
	}
	return net.JoinHostPort(listen, strconv.Itoa(port))
}

func resolveAppRoot(start string) (string, error) {
	if start == "" {
		return ".", nil
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	return abs, nil
}
