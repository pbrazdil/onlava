package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	"github.com/pbrazdil/onlava/internal/app"
	"github.com/pbrazdil/onlava/internal/devdash"
)

type logsOptions struct {
	AppRoot  string
	Limit    int
	Follow   bool
	Stream   string
	Session  string
	JSONL    bool
	Source   string
	Kind     string
	Level    string
	Grep     string
	Since    time.Duration
	SinceRaw string
	TUI      bool
	Backend  string
}

const (
	logsBackendAuto     = "auto"
	logsBackendSQLite   = "sqlite"
	logsBackendVictoria = "victoria"
)

type logsEvent struct {
	SchemaVersion string `json:"schema_version"`
	App           struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Root string `json:"root"`
	} `json:"app"`
	ID        int64                 `json:"id"`
	Time      string                `json:"time"`
	SessionID string                `json:"session_id,omitempty"`
	Source    devdash.DevSource     `json:"source"`
	Level     string                `json:"level"`
	Message   string                `json:"message"`
	Fields    json.RawMessage       `json:"fields,omitempty"`
	Raw       string                `json:"raw,omitempty"`
	Parse     devdash.DevEventParse `json:"parse"`
}

type logsCompareOptions struct {
	AppRoot  string
	Session  string
	Limit    int
	BackendA string
	BackendB string
	JSON     bool
}

type logsCompareResult struct {
	Equal      bool                  `json:"equal"`
	BackendA   string                `json:"backend_a"`
	BackendB   string                `json:"backend_b"`
	CountA     int                   `json:"count_a"`
	CountB     int                   `json:"count_b"`
	Mismatches []logsCompareMismatch `json:"mismatches,omitempty"`
}

type logsCompareMismatch struct {
	Index int    `json:"index"`
	ID    int64  `json:"id,omitempty"`
	Field string `json:"field"`
	A     any    `json:"a,omitempty"`
	B     any    `json:"b,omitempty"`
}

func logsCommand(args []string) error {
	if len(args) > 0 && args[0] == "compare" {
		return runOnlavaLogsCompare(context.Background(), os.Stdout, args[1:])
	}
	return runOnlavaLogsFunc(context.Background(), os.Stdout, args)
}

var runOnlavaLogsFunc = runOnlavaLogs

func attachCommand(args []string) error {
	opts, err := parseLogsArgs(args)
	if err != nil {
		return err
	}
	if opts.TUI {
		return runOnlavaConsoleOrFallback(context.Background(), os.Stdin, os.Stdout, opts)
	}
	logArgs, err := attachLogArgs(args)
	if err != nil {
		return err
	}
	return runOnlavaLogsFunc(context.Background(), os.Stdout, logArgs)
}

func consoleCommand(args []string) error {
	opts, err := parseLogsArgs(args)
	if err != nil {
		return err
	}
	opts.TUI = true
	return runOnlavaConsoleOrFallback(context.Background(), os.Stdin, os.Stdout, opts)
}

func attachLogArgs(args []string) ([]string, error) {
	opts, err := parseLogsArgs(args)
	if err != nil {
		return nil, err
	}
	return logArgsFromOptions(opts, true), nil
}

func logArgsFromOptions(opts logsOptions, follow bool) []string {
	if opts.Session == "" {
		opts.Session = "current"
	}
	out := []string{"--session", opts.Session, "--limit", strconv.Itoa(opts.Limit), "--stream", opts.Stream}
	if follow {
		out = append([]string{"--follow"}, out...)
	}
	if opts.AppRoot != "" {
		out = append(out, "--app-root", opts.AppRoot)
	}
	if opts.JSONL {
		out = append(out, "--jsonl")
	}
	if opts.Source != "" {
		out = append(out, "--source", opts.Source)
	}
	if opts.Kind != "" {
		out = append(out, "--kind", opts.Kind)
	}
	if opts.Level != "" {
		out = append(out, "--level", opts.Level)
	}
	if opts.Grep != "" {
		out = append(out, "--grep", opts.Grep)
	}
	if opts.SinceRaw != "" {
		out = append(out, "--since", opts.SinceRaw)
	}
	if opts.Backend != "" && opts.Backend != logsBackendAuto {
		out = append(out, "--backend", opts.Backend)
	}
	return out
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

	devQuery := logsDevEventQuery(opts, appID, sessionID)
	hasStructuredEvents, err := logsHaveStructuredEvents(ctx, store, appID, sessionID)
	if err != nil {
		return err
	}
	backend := normalizeLogsBackend(opts.Backend)
	eventBackend, err := selectDevEventBackend(ctx, store, opts)
	if err != nil {
		return err
	}
	if backend == logsBackendVictoria || hasStructuredEvents || logsRequireStructuredEvents(opts) || (eventBackend.BackendName() == logsBackendVictoria && opts.Follow) {
		devItems, err := eventBackend.ListDevEvents(ctx, devQuery)
		if err != nil {
			if backend == logsBackendAuto && eventBackend.BackendName() == logsBackendVictoria {
				eventBackend = sqliteDevEventBackend{store: store}
				devItems, err = eventBackend.ListDevEvents(ctx, devQuery)
			}
		}
		if err == nil && backend == logsBackendAuto && eventBackend.BackendName() == logsBackendVictoria && !opts.Follow && hasStructuredEvents && len(devItems) == 0 {
			eventBackend = sqliteDevEventBackend{store: store}
			devItems, err = eventBackend.ListDevEvents(ctx, devQuery)
		}
		if err != nil {
			return err
		}
		return followDevEventBackend(ctx, stdout, eventBackend, appID, appRoot, sessionID, opts, devItems)
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

func runOnlavaLogsCompare(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseLogsCompareArgs(args)
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
	backendA, err := selectNamedDevEventBackend(ctx, store, opts.BackendA)
	if err != nil {
		return err
	}
	backendB, err := selectNamedDevEventBackend(ctx, store, opts.BackendB)
	if err != nil {
		return err
	}
	query := devdash.DevEventQuery{
		AppID:     appID,
		SessionID: sessionID,
		Limit:     opts.Limit,
	}
	eventsA, err := backendA.ListDevEvents(ctx, query)
	if err != nil {
		return err
	}
	eventsB, err := backendB.ListDevEvents(ctx, query)
	if err != nil {
		return err
	}
	result := compareDevEventBackends(backendA.BackendName(), backendB.BackendName(), appID, appRoot, eventsA, eventsB)
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if result.Equal {
		_, err = fmt.Fprintf(stdout, "dev-event backends match: %s=%d %s=%d\n", result.BackendA, result.CountA, result.BackendB, result.CountB)
		return err
	}
	if _, err := fmt.Fprintf(stdout, "dev-event backends differ: %s=%d %s=%d mismatches=%d\n", result.BackendA, result.CountA, result.BackendB, result.CountB, len(result.Mismatches)); err != nil {
		return err
	}
	for _, mismatch := range result.Mismatches {
		if _, err := fmt.Fprintf(stdout, "[%d] id=%d %s: %v != %v\n", mismatch.Index, mismatch.ID, mismatch.Field, mismatch.A, mismatch.B); err != nil {
			return err
		}
	}
	return nil
}

func parseLogsCompareArgs(args []string) (logsCompareOptions, error) {
	opts := logsCompareOptions{
		Limit:    500,
		Session:  "current",
		BackendA: logsBackendSQLite,
		BackendB: logsBackendVictoria,
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return logsCompareOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return logsCompareOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.Session = strings.TrimSpace(args[i])
			if opts.Session == "" {
				return logsCompareOptions{}, fmt.Errorf("invalid session %q", args[i])
			}
		case "--limit", "-n":
			i++
			if i >= len(args) {
				return logsCompareOptions{}, fmt.Errorf("missing value for %s", args[i-1])
			}
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				return logsCompareOptions{}, fmt.Errorf("invalid limit %q", args[i])
			}
			opts.Limit = value
		case "--backend-a":
			i++
			if i >= len(args) {
				return logsCompareOptions{}, fmt.Errorf("missing value for --backend-a")
			}
			opts.BackendA = normalizeLogsBackend(args[i])
			if opts.BackendA == "" || opts.BackendA == logsBackendAuto {
				return logsCompareOptions{}, fmt.Errorf("invalid backend %q", args[i])
			}
		case "--backend-b":
			i++
			if i >= len(args) {
				return logsCompareOptions{}, fmt.Errorf("missing value for --backend-b")
			}
			opts.BackendB = normalizeLogsBackend(args[i])
			if opts.BackendB == "" || opts.BackendB == logsBackendAuto {
				return logsCompareOptions{}, fmt.Errorf("invalid backend %q", args[i])
			}
		case "--json":
			opts.JSON = true
		default:
			return logsCompareOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func selectNamedDevEventBackend(ctx context.Context, store *devdash.Store, backend string) (devEventBackend, error) {
	return selectDevEventBackend(ctx, store, logsOptions{Backend: backend})
}

func compareDevEventBackends(nameA, nameB, appName, appRoot string, eventsA, eventsB []devdash.DevEvent) logsCompareResult {
	result := logsCompareResult{
		Equal:    true,
		BackendA: nameA,
		BackendB: nameB,
		CountA:   len(eventsA),
		CountB:   len(eventsB),
	}
	limit := len(eventsA)
	if len(eventsB) < limit {
		limit = len(eventsB)
	}
	for i := 0; i < limit; i++ {
		a := devConsoleEventJSON(appName, appRoot, eventsA[i])
		b := devConsoleEventJSON(appName, appRoot, eventsB[i])
		if !reflect.DeepEqual(a, b) {
			result.Mismatches = append(result.Mismatches, firstDevEventMismatch(i, a, b))
		}
	}
	if len(eventsA) != len(eventsB) {
		result.Mismatches = append(result.Mismatches, logsCompareMismatch{
			Index: limit,
			Field: "count",
			A:     len(eventsA),
			B:     len(eventsB),
		})
	}
	if len(result.Mismatches) > 20 {
		result.Mismatches = result.Mismatches[:20]
	}
	result.Equal = len(result.Mismatches) == 0
	return result
}

func firstDevEventMismatch(index int, a, b logsEvent) logsCompareMismatch {
	id := a.ID
	if id == 0 {
		id = b.ID
	}
	switch {
	case a.ID != b.ID:
		return logsCompareMismatch{Index: index, ID: id, Field: "id", A: a.ID, B: b.ID}
	case a.Time != b.Time:
		return logsCompareMismatch{Index: index, ID: id, Field: "time", A: a.Time, B: b.Time}
	case a.SessionID != b.SessionID:
		return logsCompareMismatch{Index: index, ID: id, Field: "session_id", A: a.SessionID, B: b.SessionID}
	case !reflect.DeepEqual(a.Source, b.Source):
		return logsCompareMismatch{Index: index, ID: id, Field: "source", A: a.Source, B: b.Source}
	case a.Level != b.Level:
		return logsCompareMismatch{Index: index, ID: id, Field: "level", A: a.Level, B: b.Level}
	case a.Message != b.Message:
		return logsCompareMismatch{Index: index, ID: id, Field: "message", A: a.Message, B: b.Message}
	case string(a.Fields) != string(b.Fields):
		return logsCompareMismatch{Index: index, ID: id, Field: "fields", A: string(a.Fields), B: string(b.Fields)}
	case a.Raw != b.Raw:
		return logsCompareMismatch{Index: index, ID: id, Field: "raw", A: a.Raw, B: b.Raw}
	case !reflect.DeepEqual(a.Parse, b.Parse):
		return logsCompareMismatch{Index: index, ID: id, Field: "parse", A: a.Parse, B: b.Parse}
	default:
		return logsCompareMismatch{Index: index, ID: id, Field: "event", A: a, B: b}
	}
}

func logsDevEventQuery(opts logsOptions, appID, sessionID string) devdash.DevEventQuery {
	query := devdash.DevEventQuery{
		AppID:     appID,
		SessionID: sessionID,
		SourceID:  opts.Source,
		Kind:      opts.Kind,
		Level:     opts.Level,
		Stream:    opts.Stream,
		Grep:      opts.Grep,
		Limit:     opts.Limit,
	}
	if opts.Since > 0 {
		query.Since = time.Now().Add(-opts.Since)
	}
	return query
}

func logsHaveStructuredEvents(ctx context.Context, store *devdash.Store, appID, sessionID string) (bool, error) {
	items, err := store.ListDevEvents(ctx, devdash.DevEventQuery{AppID: appID, SessionID: sessionID, Limit: 1})
	return len(items) > 0, err
}

func logsRequireStructuredEvents(opts logsOptions) bool {
	return opts.Source != "" || opts.Kind != "" || opts.Level != "" || opts.Grep != "" || opts.Since > 0
}

func parseLogsArgs(args []string) (logsOptions, error) {
	opts := logsOptions{
		Limit:   200,
		Stream:  "all",
		Backend: logsBackendAuto,
	}
	if backend := strings.TrimSpace(os.Getenv("ONLAVA_LOGS_BACKEND")); backend != "" {
		normalized := normalizeLogsBackend(backend)
		if normalized == "" {
			return logsOptions{}, fmt.Errorf("invalid logs backend %q", backend)
		}
		opts.Backend = normalized
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
		case "--tui":
			opts.TUI = true
		case "--backend":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --backend")
			}
			opts.Backend = normalizeLogsBackend(args[i])
			if opts.Backend == "" {
				return logsOptions{}, fmt.Errorf("invalid backend %q", args[i])
			}
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
		case "--source":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --source")
			}
			opts.Source = strings.TrimSpace(args[i])
			if opts.Source == "" {
				return logsOptions{}, fmt.Errorf("invalid source %q", args[i])
			}
		case "--kind":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --kind")
			}
			opts.Kind = strings.ToLower(strings.TrimSpace(args[i]))
			if opts.Kind == "" {
				return logsOptions{}, fmt.Errorf("invalid kind %q", args[i])
			}
		case "--level":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --level")
			}
			opts.Level = normalizeLogLevel(args[i])
			if opts.Level == "" {
				return logsOptions{}, fmt.Errorf("invalid level %q", args[i])
			}
		case "--grep":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --grep")
			}
			opts.Grep = strings.TrimSpace(args[i])
			if opts.Grep == "" {
				return logsOptions{}, fmt.Errorf("invalid grep %q", args[i])
			}
		case "--since":
			i++
			if i >= len(args) {
				return logsOptions{}, fmt.Errorf("missing value for --since")
			}
			duration, err := time.ParseDuration(args[i])
			if err != nil || duration <= 0 {
				return logsOptions{}, fmt.Errorf("invalid since duration %q", args[i])
			}
			opts.Since = duration
			opts.SinceRaw = args[i]
		default:
			return logsOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func normalizeLogsBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return logsBackendAuto
	case "sqlite", "sql":
		return logsBackendSQLite
	case "victoria", "victorialogs", "vl":
		return logsBackendVictoria
	default:
		return ""
	}
}

func resolveLogsVictoriaStack(ctx context.Context, allowDefault bool) *victoriaStack {
	agentCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if client, err := localagent.DefaultClient(); err == nil {
		if substrate, err := client.GetSubstrate(agentCtx, localagent.SubstrateVictoria); err == nil {
			if stack := victoriaStackFromSubstrate(substrate); stack != nil {
				return stack
			}
		}
	}
	if allowDefault {
		return defaultVictoriaQueryStack()
	}
	return nil
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

func normalizeLogLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "trace":
		return "debug"
	case "info", "information", "notice":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error", "err":
		return "error"
	case "fatal", "panic":
		return "fatal"
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
	source := devdash.DevSource{ID: "process:" + item.PID, Kind: "process", Name: "process", PID: item.PID, Stream: item.Stream}
	event := devdash.DevEventFromOutput(appName, item.SessionID, source, item.Output, item.CreatedAt)
	event.ID = item.ID
	return writeDevEventJSONL(w, appName, appRoot, event)
}

func writeDevEventOutput(w io.Writer, appName, appRoot string, item devdash.DevEvent, jsonl bool) error {
	if jsonl {
		return writeDevEventJSONL(w, appName, appRoot, item)
	}
	text := item.Raw
	if text == "" {
		text = item.Message
	}
	if text == "" {
		return nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, err := io.WriteString(w, text)
	return err
}

func writeDevEventJSONL(w io.Writer, appName, appRoot string, item devdash.DevEvent) error {
	event := logsEvent{
		SchemaVersion: devdash.DevEventSchemaVersion,
		ID:            item.ID,
		Time:          item.CreatedAt.UTC().Format(time.RFC3339Nano),
		SessionID:     item.SessionID,
		Source:        item.Source,
		Level:         item.Level,
		Message:       item.Message,
		Raw:           item.Raw,
		Parse:         item.Parse,
	}
	if len(item.Fields) > 0 && string(item.Fields) != "{}" {
		event.Fields = item.Fields
	}
	event.App.ID = appName
	event.App.Name = appName
	event.App.Root = appRoot
	enc := json.NewEncoder(w)
	return enc.Encode(event)
}
