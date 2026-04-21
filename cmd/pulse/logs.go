package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"pulse.dev/internal/app"
	"pulse.dev/internal/devdash"
)

type logsOptions struct {
	AppRoot string
	Limit   int
	Follow  bool
	Stream  string
}

func logsCommand(args []string) error {
	return runPulseLogs(context.Background(), os.Stdout, args)
}

func runPulseLogs(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseLogsArgs(args)
	if err != nil {
		return err
	}

	start, err := resolveAppRoot(opts.AppRoot)
	if err != nil {
		return err
	}
	appRoot, cfg, err := app.DiscoverRoot(start)
	if err != nil {
		return err
	}

	store, err := devdash.OpenStore(os.Getenv("PULSE_DEV_CACHE_DIR"))
	if err != nil {
		return err
	}
	defer store.Close()

	record, err := store.GetApp(ctx, cfg.Name)
	if err != nil {
		return fmt.Errorf("no local logs found for %q; run `pulse run` first", cfg.Name)
	}
	if record.Root != "" && record.Root != appRoot {
		return fmt.Errorf("local logs for %q belong to %s, not %s", cfg.Name, record.Root, appRoot)
	}

	items, err := store.ListProcessOutput(ctx, cfg.Name, opts.Limit)
	if err != nil {
		return err
	}
	lastID := int64(0)
	for _, item := range items {
		if item.ID > lastID {
			lastID = item.ID
		}
		if streamAllowed(opts.Stream, item.Stream) {
			if err := writeProcessOutput(stdout, item); err != nil {
				return err
			}
		}
	}

	if !opts.Follow {
		return nil
	}

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			items, err := store.ListProcessOutputSince(ctx, cfg.Name, lastID, 200)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.ID > lastID {
					lastID = item.ID
				}
				if streamAllowed(opts.Stream, item.Stream) {
					if err := writeProcessOutput(stdout, item); err != nil {
						return err
					}
				}
			}
		}
	}
}

func parseLogsArgs(args []string) (logsOptions, error) {
	opts := logsOptions{
		Limit:  200,
		Stream: "all",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--limit", "-n":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for %s", args[i-1])
			}
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				return logsOptions{}, fmt.Errorf("invalid limit %q", args[i])
			}
			opts.Limit = value
		case "--follow", "-f":
			opts.Follow = true
		case "--stream":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --stream")
			}
			opts.Stream = normalizeLogStream(args[i])
			if opts.Stream == "" {
				return logsOptions{}, fmt.Errorf("invalid stream %q", args[i])
			}
		default:
			return logsOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func normalizeLogStream(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "all", "":
		return "all"
	case "stdout":
		return "stdout"
	case "stderr":
		return "stderr"
	default:
		return ""
	}
}

func streamAllowed(filter, stream string) bool {
	return filter == "all" || filter == stream
}

func writeProcessOutput(w io.Writer, item devdash.ProcessOutput) error {
	_, err := w.Write(item.Output)
	return err
}
