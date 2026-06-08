package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	appcfg "github.com/pbrazdil/onlava/internal/app"
	obs "github.com/pbrazdil/onlava/internal/observability"
)

type logsQueryOptions struct {
	AppRoot  string
	Session  string
	Query    string
	LogQL    string
	Since    time.Duration
	SinceRaw string
	Start    time.Time
	End      time.Time
	Limit    int
	Timeout  time.Duration
	Fields   []string
	JSONL    bool
}

type metricsQueryOptions struct {
	AppRoot  string
	Session  string
	PromQL   string
	Since    time.Duration
	SinceRaw string
	Start    time.Time
	End      time.Time
	Step     time.Duration
	Timeout  time.Duration
	Limit    int
	Instant  bool
}

type metricsCatalogOptions struct {
	AppRoot  string
	Session  string
	Match    string
	Since    time.Duration
	SinceRaw string
	Start    time.Time
	End      time.Time
	Limit    int
}

type inspectObservabilityResponse struct {
	SchemaVersion string             `json:"schema_version"`
	App           inspectAppRefLite  `json:"app"`
	Scope         obs.QueryScope     `json:"scope"`
	Backends      observabilityKinds `json:"backends"`
	Examples      []string           `json:"examples"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type inspectAppRefLite struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Root string `json:"root"`
}

type observabilityKinds struct {
	Logs    obs.QueryBackend `json:"logs"`
	Metrics obs.QueryBackend `json:"metrics"`
	Traces  obs.QueryBackend `json:"traces"`
}

func runLogsQueryCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseLogsQueryArgs(args)
	if err != nil {
		return err
	}
	scope, err := resolveQueryScope(ctx, opts.AppRoot, opts.Session)
	if err != nil {
		return err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	baseURL := ""
	if stack != nil {
		baseURL = stack.BaseURL("logs")
	}
	result, err := obs.QueryLogs(ctx, obs.LogsQuery{
		BaseURL: baseURL,
		Scope:   scope,
		Query:   opts.Query,
		Bounds:  queryBounds(opts.Since, opts.SinceRaw, opts.Start, opts.End),
		Limit:   opts.Limit,
		Timeout: opts.Timeout,
		Fields:  opts.Fields,
	})
	if err != nil {
		return err
	}
	if opts.JSONL {
		enc := json.NewEncoder(stdout)
		for _, entry := range result.Logs {
			if err := enc.Encode(entry); err != nil {
				return err
			}
		}
		return nil
	}
	return writeInspectJSON(stdout, result)
}

func runLogsTailCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseLogsTailArgs(args)
	if err != nil {
		return err
	}
	scope, err := resolveQueryScope(ctx, opts.AppRoot, opts.Session)
	if err != nil {
		return err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	if stack == nil || stack.BaseURL("logs") == "" {
		return fmt.Errorf("VictoriaLogs is unavailable")
	}
	enc := json.NewEncoder(stdout)
	return obs.TailLogs(ctx, obs.LogsQuery{
		BaseURL: stack.BaseURL("logs"),
		Scope:   scope,
		Query:   opts.Query,
		Bounds:  queryBounds(opts.Since, opts.SinceRaw, opts.Start, opts.End),
		Limit:   opts.Limit,
		Timeout: opts.Timeout,
		Fields:  opts.Fields,
	}, func(entry obs.LogEntry) error {
		return enc.Encode(entry)
	})
}

func runMetricsQueryCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseMetricsQueryArgs(args)
	if err != nil {
		return err
	}
	scope, err := resolveQueryScope(ctx, opts.AppRoot, opts.Session)
	if err != nil {
		return err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	baseURL := ""
	if stack != nil {
		baseURL = stack.BaseURL("metrics")
	}
	result, err := obs.QueryMetrics(ctx, obs.MetricsQuery{
		BaseURL: baseURL,
		Scope:   scope,
		PromQL:  opts.PromQL,
		Bounds:  queryBounds(opts.Since, opts.SinceRaw, opts.Start, opts.End),
		Step:    opts.Step,
		Instant: opts.Instant,
		Timeout: opts.Timeout,
		Limit:   opts.Limit,
	})
	if err != nil {
		return err
	}
	return writeInspectJSON(stdout, result)
}

func runMetricsLabelsCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseMetricsCatalogArgs(args, false)
	if err != nil {
		return err
	}
	scope, err := resolveQueryScope(ctx, opts.AppRoot, opts.Session)
	if err != nil {
		return err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	baseURL := ""
	if stack != nil {
		baseURL = stack.BaseURL("metrics")
	}
	result, err := obs.MetricsLabels(ctx, obs.MetricsCatalogQuery{
		BaseURL: baseURL,
		Scope:   scope,
		Bounds:  queryBounds(opts.Since, opts.SinceRaw, opts.Start, opts.End),
		Limit:   opts.Limit,
	})
	if err != nil {
		return err
	}
	return writeInspectJSON(stdout, result)
}

func runMetricsSeriesCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseMetricsCatalogArgs(args, true)
	if err != nil {
		return err
	}
	scope, err := resolveQueryScope(ctx, opts.AppRoot, opts.Session)
	if err != nil {
		return err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	baseURL := ""
	if stack != nil {
		baseURL = stack.BaseURL("metrics")
	}
	result, err := obs.MetricsSeries(ctx, obs.MetricsCatalogQuery{
		BaseURL: baseURL,
		Scope:   scope,
		Bounds:  queryBounds(opts.Since, opts.SinceRaw, opts.Start, opts.End),
		Match:   opts.Match,
		Limit:   opts.Limit,
	})
	if err != nil {
		return err
	}
	return writeInspectJSON(stdout, result)
}

func buildInspectObservabilityResponse(ctx context.Context, appRoot string, cfg appcfg.Config, session string) (inspectObservabilityResponse, error) {
	scope, err := resolveQueryScopeForApp(ctx, appRoot, cfg, session)
	if err != nil {
		return inspectObservabilityResponse{}, err
	}
	stack := resolveLogsVictoriaStackFunc(ctx, true)
	logsBase, metricsBase, tracesBase := "", "", ""
	if stack != nil {
		logsBase = stack.BaseURL("logs")
		metricsBase = stack.BaseURL("metrics")
		tracesBase = stack.BaseURL("traces")
	}
	resp := inspectObservabilityResponse{
		SchemaVersion: obs.InspectObservabilitySchema,
		App: inspectAppRefLite{
			ID:   cfg.AppID(),
			Name: cfg.Name,
			Root: appRoot,
		},
		Scope: scope,
		Backends: observabilityKinds{
			Logs:    obs.QueryBackend{Kind: "victorialogs", Dialect: "LogsQL", BaseURL: logsBase, Ready: logsBase != "", QueryPath: "/select/logsql/query", TailPath: "/select/logsql/tail"},
			Metrics: obs.QueryBackend{Kind: "victoriametrics", Dialect: "PromQL/MetricsQL", BaseURL: metricsBase, Ready: metricsBase != "", QueryPath: "/prometheus/api/v1/query_range"},
			Traces:  obs.QueryBackend{Kind: "victoriatraces", Dialect: "Jaeger query API", BaseURL: tracesBase, Ready: tracesBase != "", QueryPath: "/select/jaeger/api/traces"},
		},
		Examples: []string{
			"onlava logs query --json --since 15m --query 'error OR panic'",
			"onlava metrics query --json --since 15m --step 5s --promql 'max_over_time(onlava_request_duration_seconds[15m])'",
			"onlava metrics labels --json --since 1h",
		},
	}
	if logsBase == "" {
		resp.Warnings = append(resp.Warnings, "VictoriaLogs is unavailable")
	}
	if metricsBase == "" {
		resp.Warnings = append(resp.Warnings, "VictoriaMetrics is unavailable")
	}
	if tracesBase == "" {
		resp.Warnings = append(resp.Warnings, "VictoriaTraces is unavailable")
	}
	return resp, nil
}

func parseLogsQueryArgs(args []string) (logsQueryOptions, error) {
	opts := logsQueryOptions{Session: "current", Since: 15 * time.Minute, SinceRaw: "15m", Limit: 200, Timeout: 3 * time.Second}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
		case "--jsonl":
			opts.JSONL = true
		case "--app-root":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.Session = strings.TrimSpace(args[i])
			if opts.Session == "" {
				return logsQueryOptions{}, fmt.Errorf("invalid session %q", args[i])
			}
		case "--query":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --query")
			}
			opts.Query = strings.TrimSpace(args[i])
		case "--logql":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --logql")
			}
			opts.LogQL = strings.TrimSpace(args[i])
		case "--since":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --since")
			}
			duration, err := parsePositiveDuration(args[i], "since")
			if err != nil {
				return logsQueryOptions{}, err
			}
			opts.Since = duration
			opts.SinceRaw = args[i]
		case "--start":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --start")
			}
			start, err := parseQueryTime(args[i])
			if err != nil {
				return logsQueryOptions{}, err
			}
			opts.Start = start
		case "--end":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --end")
			}
			end, err := parseQueryTime(args[i])
			if err != nil {
				return logsQueryOptions{}, err
			}
			opts.End = end
		case "--limit", "-n":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for %s", args[i-1])
			}
			limit, err := parsePositiveInt(args[i], "limit")
			if err != nil {
				return logsQueryOptions{}, err
			}
			opts.Limit = limit
		case "--timeout":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --timeout")
			}
			timeout, err := parsePositiveDuration(args[i], "timeout")
			if err != nil {
				return logsQueryOptions{}, err
			}
			opts.Timeout = timeout
		case "--fields":
			i++
			if i >= len(args) {
				return logsQueryOptions{}, fmt.Errorf("missing value for --fields")
			}
			opts.Fields = splitCSV(args[i])
		default:
			return logsQueryOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if opts.LogQL != "" {
		return logsQueryOptions{}, fmt.Errorf("--logql is not supported yet; use native VictoriaLogs LogsQL with --query")
	}
	if opts.Query == "" {
		return logsQueryOptions{}, fmt.Errorf("missing required --query")
	}
	return opts, nil
}

func parseLogsTailArgs(args []string) (logsQueryOptions, error) {
	opts, err := parseLogsQueryArgs(args)
	if err != nil {
		return logsQueryOptions{}, err
	}
	opts.JSONL = true
	opts.Limit = 0
	return opts, nil
}

func parseMetricsQueryArgs(args []string) (metricsQueryOptions, error) {
	opts := metricsQueryOptions{Session: "current", Since: 15 * time.Minute, SinceRaw: "15m", Step: 5 * time.Second, Timeout: 3 * time.Second, Limit: 100}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
		case "--instant":
			opts.Instant = true
		case "--app-root":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.Session = strings.TrimSpace(args[i])
			if opts.Session == "" {
				return metricsQueryOptions{}, fmt.Errorf("invalid session %q", args[i])
			}
		case "--promql":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --promql")
			}
			opts.PromQL = strings.TrimSpace(args[i])
		case "--since":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --since")
			}
			duration, err := parsePositiveDuration(args[i], "since")
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.Since = duration
			opts.SinceRaw = args[i]
		case "--start":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --start")
			}
			start, err := parseQueryTime(args[i])
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.Start = start
		case "--end":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --end")
			}
			end, err := parseQueryTime(args[i])
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.End = end
		case "--step":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --step")
			}
			step, err := parsePositiveDuration(args[i], "step")
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.Step = step
		case "--timeout":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for --timeout")
			}
			timeout, err := parsePositiveDuration(args[i], "timeout")
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.Timeout = timeout
		case "--limit", "-n":
			i++
			if i >= len(args) {
				return metricsQueryOptions{}, fmt.Errorf("missing value for %s", args[i-1])
			}
			limit, err := parsePositiveInt(args[i], "limit")
			if err != nil {
				return metricsQueryOptions{}, err
			}
			opts.Limit = limit
		default:
			return metricsQueryOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if opts.PromQL == "" {
		return metricsQueryOptions{}, fmt.Errorf("missing required --promql")
	}
	return opts, nil
}

func parseMetricsCatalogArgs(args []string, requireMatch bool) (metricsCatalogOptions, error) {
	opts := metricsCatalogOptions{Session: "current", Since: time.Hour, SinceRaw: "1h", Limit: 1000}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
		case "--app-root":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--session":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.Session = strings.TrimSpace(args[i])
			if opts.Session == "" {
				return metricsCatalogOptions{}, fmt.Errorf("invalid session %q", args[i])
			}
		case "--match":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --match")
			}
			opts.Match = strings.TrimSpace(args[i])
		case "--since":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --since")
			}
			duration, err := parsePositiveDuration(args[i], "since")
			if err != nil {
				return metricsCatalogOptions{}, err
			}
			opts.Since = duration
			opts.SinceRaw = args[i]
		case "--start":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --start")
			}
			start, err := parseQueryTime(args[i])
			if err != nil {
				return metricsCatalogOptions{}, err
			}
			opts.Start = start
		case "--end":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for --end")
			}
			end, err := parseQueryTime(args[i])
			if err != nil {
				return metricsCatalogOptions{}, err
			}
			opts.End = end
		case "--limit", "-n":
			i++
			if i >= len(args) {
				return metricsCatalogOptions{}, fmt.Errorf("missing value for %s", args[i-1])
			}
			limit, err := parsePositiveInt(args[i], "limit")
			if err != nil {
				return metricsCatalogOptions{}, err
			}
			opts.Limit = limit
		default:
			return metricsCatalogOptions{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if requireMatch && opts.Match == "" {
		return metricsCatalogOptions{}, fmt.Errorf("missing required --match")
	}
	return opts, nil
}

func resolveQueryScope(ctx context.Context, appRootFlag, sessionFlag string) (obs.QueryScope, error) {
	start, err := resolveAppRoot(appRootFlag)
	if err != nil {
		return obs.QueryScope{}, err
	}
	appRoot, cfg, err := appcfg.DiscoverRoot(start)
	if err != nil {
		return obs.QueryScope{}, err
	}
	return resolveQueryScopeForApp(ctx, appRoot, cfg, sessionFlag)
}

func resolveQueryScopeForApp(ctx context.Context, appRoot string, cfg appcfg.Config, sessionFlag string) (obs.QueryScope, error) {
	sessionFlag = strings.TrimSpace(sessionFlag)
	if sessionFlag == "" {
		sessionFlag = "current"
	}
	scope := obs.QueryScope{
		AppID:       cfg.AppID(),
		AppRoot:     appRoot,
		AppRootHash: appRootHash(appRoot),
		Worktree:    appWorktreeName(appRoot),
		Enforced:    true,
	}
	if sessionFlag == "current" {
		session, err := currentAgentSessionForAppRoot(ctx, appRoot)
		if err != nil {
			return obs.QueryScope{}, err
		}
		scope.SessionID = session.SessionID
		scope.Branch = session.Branch
		if strings.TrimSpace(session.AppRoot) != "" {
			scope.AppRoot = session.AppRoot
			scope.AppRootHash = appRootHash(session.AppRoot)
			scope.Worktree = appWorktreeName(session.AppRoot)
		}
		return scope, nil
	}
	scope.SessionID = sessionFlag
	if client, err := localagent.DefaultClient(); err == nil {
		if sessions, err := client.List(ctx, appRoot); err == nil {
			for _, session := range sessions {
				if session.SessionID == sessionFlag {
					scope.Branch = session.Branch
					break
				}
			}
		}
	}
	return scope, nil
}

func queryBounds(since time.Duration, sinceRaw string, start, end time.Time) obs.TimeBounds {
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if start.IsZero() && since > 0 {
		start = end.Add(-since)
	}
	return obs.TimeBounds{Since: sinceRaw, Start: start.UTC(), End: end.UTC()}
}

func parsePositiveDuration(value, name string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s duration %q", name, value)
	}
	return duration, nil
}

func parsePositiveInt(value, name string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	return n, nil
}

func parseQueryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("invalid time %q", value)
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q; use RFC3339", value)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
