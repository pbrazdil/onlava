package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	"github.com/pbrazdil/onlava/internal/app"
	"github.com/pbrazdil/onlava/internal/devdash"
)

type logsOptions struct {
	AppRoot string
	Limit   int
	Follow  bool
	Stream  string
	Session string
	JSONL   bool
}

type logsEvent struct {
	SchemaVersion string `json:"schema_version"`
	App           struct {
		Name string `json:"name"`
		Root string `json:"root"`
	} `json:"app"`
	ID        int64  `json:"id"`
	SessionID string `json:"session_id,omitempty"`
	PID       string `json:"pid"`
	Stream    string `json:"stream"`
	Output    string `json:"output"`
	CreatedAt string `json:"created_at"`
}

func logsCommand(args []string) error {
	return runOnlavaLogsFunc(context.Background(), os.Stdout, args)
}

var runOnlavaLogsFunc = runOnlavaLogs

func attachCommand(args []string) error {
	logArgs, err := attachLogArgs(args)
	if err != nil {
		return err
	}
	return runOnlavaLogsFunc(context.Background(), os.Stdout, logArgs)
}

func attachLogArgs(args []string) ([]string, error) {
	opts, err := parseLogsArgs(args)
	if err != nil {
		return nil, err
	}
	if opts.Session == "" {
		opts.Session = "current"
	}
	out := []string{"--follow", "--session", opts.Session, "--limit", strconv.Itoa(opts.Limit), "--stream", opts.Stream}
	if opts.AppRoot != "" {
		out = append(out, "--app-root", opts.AppRoot)
	}
	if opts.JSONL {
		out = append(out, "--jsonl")
	}
	return out, nil
}

func runOnlavaLogs(ctx context.Context, stdout io.Writer, args []string) error {
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
	appID := cfg.AppID()
	sessionID, err := resolveLogsSessionID(ctx, opts.Session, appRoot)
	if err != nil {
		return err
	}

	store, err := openDevdashStore()
	if err != nil {
		return err
	}
	defer store.Close()

	record, sessionRecord, err := devdashAppRecordForSession(ctx, store, appID, sessionID)
	if err != nil {
		return fmt.Errorf("no local logs found for %q; run `onlava run` first", appID)
	}
	if !sessionRecord && sessionID == "" && record.Root != "" && record.Root != appRoot {
		return fmt.Errorf("local logs for %q belong to %s, not %s", appID, record.Root, appRoot)
	}

	items, err := store.ListProcessOutputForSession(ctx, appID, sessionID, opts.Limit)
	if err != nil {
		return err
	}
	lastID := int64(0)
	for _, item := range items {
		if item.ID > lastID {
			lastID = item.ID
		}
		if streamAllowed(opts.Stream, item.Stream) {
			if err := writeProcessOutput(stdout, appID, appRoot, item, opts.JSONL); err != nil {
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
			items, err := store.ListProcessOutputSinceForSession(ctx, appID, sessionID, lastID, 200)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.ID > lastID {
					lastID = item.ID
				}
				if streamAllowed(opts.Stream, item.Stream) {
					if err := writeProcessOutput(stdout, appID, appRoot, item, opts.JSONL); err != nil {
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
		case "--jsonl", "--json":
			opts.JSONL = true
		case "--stream":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --stream")
			}
			opts.Stream = normalizeLogStream(args[i])
			if opts.Stream == "" {
				return logsOptions{}, fmt.Errorf("invalid stream %q", args[i])
			}
		case "--session":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.Session = strings.TrimSpace(args[i])
			if opts.Session == "" {
				return logsOptions{}, fmt.Errorf("invalid session %q", args[i])
			}
		default:
			return logsOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func resolveLogsSessionID(ctx context.Context, value, appRoot string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if value != "current" {
		return value, nil
	}
	client, err := localagent.DefaultClient()
	if err != nil {
		return "", err
	}
	sessions, err := client.List(ctx, appRoot)
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no onlava agent session found for %s", appRoot)
	}
	return sessions[0].SessionID, nil
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

func writeProcessOutput(w io.Writer, appName, appRoot string, item devdash.ProcessOutput, jsonl bool) error {
	if jsonl {
		return writeLogsJSONL(w, appName, appRoot, item)
	}
	_, err := w.Write(item.Output)
	return err
}

func writeLogsJSONL(w io.Writer, appName, appRoot string, item devdash.ProcessOutput) error {
	event := logsEvent{
		SchemaVersion: "onlava.logs.event.v1",
		ID:            item.ID,
		SessionID:     item.SessionID,
		PID:           item.PID,
		Stream:        item.Stream,
		Output:        string(item.Output),
		CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	event.App.Name = appName
	event.App.Root = appRoot
	enc := json.NewEncoder(w)
	return enc.Encode(event)
}
