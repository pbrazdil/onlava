package main

import (
	"errors"
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
		var silent *silentCLIError
		if !errors.As(err, &silent) {
			fmt.Fprintln(os.Stderr, err)
		}
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
	case "dev":
		return devCommand(args[1:])
	case "run":
		return runCommand(args[1:])
	case "version":
		return versionCommand(args[1:])
	case "build":
		return buildCommand(args[1:])
	case "psql":
		return psqlCommand(args[1:])
	case "check":
		return checkCommand(args[1:])
	case "harness":
		return harnessCommand(args[1:])
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
	return fmt.Errorf("usage:\n  pulse dev [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json] [--proxy] [--trust]\n  pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]\n  pulse version [--json]\n  pulse build [--app-root <path>] [-o <path>] [--db-studio]\n  pulse psql [--app-root <path>] [psql args...]\n  pulse check [--app-root <path>] [--json]\n  pulse harness [--app-root <path>] [--json] [--write]\n  pulse harness self [--repo-root <path>] [--json] [--write]\n  pulse inspect app|routes|services|endpoints|wire|build|paths|traces|metrics --json [--app-root <path>]\n  pulse inspect docs --json [--repo-root <path>]\n  pulse inspect traces --json [--service <name>] [--endpoint <name>] [--trace-id <id>] [--status ok|error] [--min-duration-ms <n>] [--since <duration>] [--limit <n>] [--slowest]\n  pulse inspect metrics --json [--service <name>] [--endpoint <name>] [--status ok|error] [--since <duration>] [--limit <n>]\n  pulse admin traces clear --json [--app-root <path>]\n  pulse admin pubsub clear --json [--app-root <path>]\n  pulse logs [--app-root <path>] [--limit <n>] [--stream all|stdout|stderr] [-f|--follow] [--jsonl|--json]\n  pulse test [--app-root <path>] [go test flags/packages...]\n  pulse gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]")
}

type silentCLIError struct {
	err error
}

func (e *silentCLIError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func devCommand(args []string) error {
	opts, err := parseDevArgs(args)
	if err != nil {
		return err
	}
	restore := configureDevProcessEnv(opts)
	defer restore()
	addr := resolveListenAddr(opts.Listen, opts.Port)
	return runWithWatchFunc(addr, opts.Verbose, opts.JSON, opts.AppRoot)
}

type devOptions struct {
	Listen  string
	Port    int
	Verbose bool
	JSON    bool
	AppRoot string
	Proxy   bool
	Trust   bool
}

func parseDevArgs(args []string) (devOptions, error) {
	opts := devOptions{Port: 4000}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			i++
			if i >= len(args) {
				return devOptions{}, fmt.Errorf("missing value for --port")
			}
			value, err := strconv.Atoi(args[i])
			if err != nil {
				return devOptions{}, fmt.Errorf("invalid port %q", args[i])
			}
			opts.Port = value
		case "--listen":
			i++
			if i >= len(args) {
				return devOptions{}, fmt.Errorf("missing value for --listen")
			}
			opts.Listen = args[i]
		case "--verbose", "-v":
			opts.Verbose = true
		case "--json":
			opts.JSON = true
		case "--proxy":
			opts.Proxy = true
		case "--trust":
			opts.Trust = true
			opts.Proxy = true
		case "--app-root":
			i++
			if i >= len(args) {
				return devOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		default:
			return devOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func configureDevProcessEnv(opts devOptions) func() {
	changes := map[string]string{}
	if opts.Proxy {
		changes["PULSE_LOCAL_PROXY"] = "1"
		if opts.Trust {
			changes["PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL"] = "0"
		} else {
			changes["PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL"] = "1"
		}
	}
	return applyTemporaryEnv(changes)
}

func applyTemporaryEnv(values map[string]string) func() {
	if len(values) == 0 {
		return func() {}
	}
	type previousValue struct {
		value string
		ok    bool
	}
	previous := make(map[string]previousValue, len(values))
	for key, value := range values {
		old, ok := os.LookupEnv(key)
		previous[key] = previousValue{value: old, ok: ok}
		_ = os.Setenv(key, value)
	}
	return func() {
		for key, old := range previous {
			if old.ok {
				_ = os.Setenv(key, old.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
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
