# CLI Observability Query Surface

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

Scenery already owns the local observability substrate for app sessions:
VictoriaLogs for logs, VictoriaMetrics for metrics, VictoriaTraces for traces,
and Grafana as an optional visual surface. Agents should not need external tool
bridges, Grafana scraping, raw Victoria ports, or query-string rewriting to
answer basic debugging questions such as "what errors happened in this session?"
or "which request duration series is spiking?"

This plan adds a first-class CLI query surface under existing observability
nouns:

```sh
scenery inspect observability --json
scenery logs query --json --since 15m --query 'error OR panic'
scenery logs tail --query 'error'
scenery metrics query --json --since 15m --step 5s --promql 'max_over_time(scenery_request_duration_seconds[15m])'
scenery metrics labels --json --since 1h
scenery metrics series --json --match 'scenery_request_duration_seconds'
```

The observable behavior is that a contributor can run those commands from an
app root, get bounded JSON scoped to the current app session by default, and see
the exact applied scope echoed in the output. The CLI owns session resolution,
query bounds, backend discovery, and stable response schemas. Victoria remains
the current implementation detail, not the user-facing integration API.

## Progress

- [x] 2026-06-08: Created ExecPlan `0067-cli-observability-query.md` from the requested CLI-only observability brief.
- [x] 2026-06-08: Added shared observability query scope records and VictoriaLogs/VictoriaMetrics query clients in `internal/observability`.
- [x] 2026-06-08: Added CLI commands for `inspect observability`, LogsQL query/tail, and PromQL/MetricsQL query/catalog commands.
- [x] 2026-06-08: Added parser and httptest coverage for required flags, LogQL rejection, backend paths, scope parameters, and normalized JSON shape.
- [x] 2026-06-08: Updated docs, schemas, harness schema inventories, knowledge metadata, and agent instructions for the new CLI surface.

## Surprises & Discoveries

- 2026-06-08: `scenery inspect docs --json` reports one review-due document, `docs/ui-agent-contract.md`. This plan is CLI/runtime work and should preserve that signal without mixing unrelated UI gardening into the observability query work.
- 2026-06-08: `docs/local-contract.md` currently lists `scenery traces list --json`, `scenery metrics list --json`, and `scenery logs --jsonl` as observability surfaces. The new query commands must update that contract rather than living only in code or this plan.
- 2026-06-08: The current code has `cmd/scenery/observability_commands.go` for `traces list`, `traces clear`, and `metrics list`; `cmd/scenery/logs.go` owns the existing structured dev log reader; and `cmd/scenery/victoria_query.go` already contains VictoriaTraces query helpers.
- 2026-06-08: Current Victoria docs name the log query language LogsQL and expose `/select/logsql/query` plus `/select/logsql/tail`. VictoriaMetrics exposes PromQL/MetricsQL instant and range endpoints under `/prometheus/api/v1/query` and `/prometheus/api/v1/query_range`, and documents `extra_label` for enforced metrics scoping.
- 2026-06-08: Existing structured dev-event exports use `scenery_app_id` and `scenery_session_id`, while OTLP log exports use dotted fields such as `scenery.application_id` and `scenery.session_id`. The LogsQL scope filter accepts both app/session spellings and uses the root-hash filter where available.

## Decision Log

- 2026-06-08: Put the new commands under existing `logs`, `metrics`, and `inspect` nouns instead of adding top-level `promql`, `logql`, or `observability` commands. This matches the current CLI shape and makes the feature discoverable to agents already using `scenery logs`, `scenery metrics`, and `scenery inspect`.
- 2026-06-08: Name the logs flag `--query` for native LogsQL and reserve `--logql` only for a future explicit translate-or-reject path. VictoriaLogs is not Loki, so the CLI must not silently pretend Loki LogQL is native.
- 2026-06-08: Enforce app/session/worktree scope through backend query parameters, not by editing user query strings. Use VictoriaLogs `extra_filters` and VictoriaMetrics `extra_label` where available.
- 2026-06-08: Default to `--session current`, bounded time ranges, bounded limits, and JSON output for query commands. Unscoped cross-session queries require an explicit friction flag and are not part of the default agent path.
- 2026-06-08: Keep shared backend and scope logic outside `cmd/scenery` where practical. CLI files parse flags and print responses; shared query/discovery behavior belongs under an internal package such as `internal/observability`.

## Outcomes & Retrospective

Completed on 2026-06-08.

Shipped:

- `scenery inspect observability --json [--session current|<id>]` with backend readiness, dialects, examples, warnings, and echoed enforced scope.
- `scenery logs query` and `scenery logs tail` over VictoriaLogs LogsQL, with bounded defaults, JSON/JSONL output, LogQL rejection, and backend-enforced `extra_filters`.
- `scenery metrics query`, `metrics labels`, and `metrics series` over VictoriaMetrics PromQL/MetricsQL APIs, with bounded defaults and repeated `extra_label` scope parameters.
- Versioned schemas for the new discovery, log query/tail, and metrics query/catalog envelopes.
- Docs and agent guidance updates in `docs/local-contract.md`, `docs/agent-guide.md`, `SKILL.md`, and `docs/app-development-cookbook.md`.
- Targeted tests for parser behavior, backend request shape, scope isolation parameters, catalog decoding, and result normalization.

Validation:

- `go test ./internal/observability ./cmd/scenery` passed during implementation.
- `go test ./...` passed.
- `go install ./cmd/scenery` passed.
- `scenery inspect docs --json` passed with the expected review-due UI contract signal.
- `scenery harness self --json --write` passed after restoring the pre-existing missing 0066 active ExecPlan file and installing existing UI dependencies.

## Context and Orientation

Start with these files and surfaces:

- `cmd/scenery/main.go` for top-level CLI dispatch and usage text.
- `cmd/scenery/observability_commands.go` for existing `traces` and `metrics` command routing.
- `cmd/scenery/logs.go` for existing `scenery logs` flag parsing, session resolution, and structured dev-event output.
- `cmd/scenery/inspect.go` and `cmd/scenery/inspect_observability.go` for `inspect` subject routing and the current trace/metrics summary response shapes.
- `cmd/scenery/victoria.go`, `cmd/scenery/victoria_query.go`, and `cmd/scenery/grafana.go` for the Victoria component specs, local default ports, and current query helper style.
- `internal/agent` for current session records, session IDs, route/worktree metadata, and substrate records.
- `internal/devdash` for current dev-event, log, metric, and trace persistence models.
- `docs/local-contract.md` for CLI grammar, JSON schemas, artifact paths, stability labels, and public contract language.
- `docs/agent-guide.md`, `SKILL.md`, and `docs/app-development-cookbook.md` for agent-facing workflows that should prefer CLI JSON over Grafana or raw substrate ports.
- `docs/schemas/` for stable JSON schema files when new response contracts are added.
- `docs/knowledge.json` for active ExecPlan indexing until deterministic plan indexing exists.

Relevant current CLI contract before this plan:

```text
scenery traces list --json [--session current|<id>] [--service <name>] [--endpoint <name>] [--trace-id <id>] [--status ok|error] [--min-duration-ms <n>] [--since <duration>] [--limit <n>] [--slowest]
scenery metrics list --json [--session current|<id>] [--service <name>] [--endpoint <name>] [--status ok|error] [--since <duration>] [--limit <n>]
scenery logs [--app-root <path>] [--session current|<id>] [--limit <n>] [--stream all|stdout|stderr] [--source <id>] [--kind <kind>] [--level <level>] [--grep <text>] [--since <duration>] [--backend auto|victoria] [-f|--follow] [--jsonl|--json]
```

This plan extends that surface. It does not remove the existing summary/list
commands, and it does not make raw Victoria URLs part of the public contract.

## Milestones

1. Discovery Contract: add `scenery inspect observability --json` so agents can
   discover backend readiness, dialects, examples, and enforced scope for the
   selected app/session.
2. Shared Scope and Clients: add an internal query package that resolves app
   root, app ID, current session, root hash, worktree, branch, backend URLs, and
   applies scope through Victoria query parameters.
3. Logs Query Surface: add `scenery logs query` and `scenery logs tail` with
   LogsQL input, bounded time/limit defaults, JSON and JSONL output, field
   selection, and explicit diagnostics for unsupported `--logql`.
4. Metrics Query Surface: add `scenery metrics query`, `metrics labels`, and
   `metrics series` for bounded PromQL/MetricsQL queries and catalog reads.
5. Contract and Validation: add schemas, tests, self-harness coverage, and docs
   updates that prove session isolation and keep the CLI contract stable.

## Plan of Work

First add a small internal scope model that can be reused by logs, metrics, and
inspection commands. The scope model should discover `.scenery.json` from cwd or
`--app-root`, resolve `--session current` through the local agent/session
registry, compute the app root hash using the same value emitted into existing
observability records, and expose a JSON object that every query response echoes.

Next add backend discovery and client functions. Reuse the existing Victoria
stack shape where it fits, but keep new code focused: logs queries call
VictoriaLogs, metrics queries call VictoriaMetrics, and inspect reports backend
readiness without forcing a query. The caller should not need to know default
ports or Victoria endpoint paths.

Then layer CLI parsing on top. Preserve existing `scenery logs ...` behavior by
dispatching only when the first argument is a new subcommand such as `query` or
`tail`. Preserve existing `scenery metrics list ...` behavior while adding
`query`, `labels`, and `series`. Add `inspect observability` as a new inspect
subject rather than overloading the existing `inspect traces` and `inspect
metrics` compatibility errors.

Finally update contracts and validation. Query response JSON is a public agent
surface, so schema files, `docs/local-contract.md`, `docs/agent-guide.md`,
`SKILL.md`, examples, and self-harness expectations must move together.

## Concrete Steps

1. Add an internal package such as `internal/observability` with a `QueryScope`
   type:

   ```go
   type QueryScope struct {
       AppID       string `json:"app_id"`
       SessionID   string `json:"session_id"`
       AppRoot     string `json:"app_root"`
       AppRootHash string `json:"app_root_hash"`
       Worktree    string `json:"worktree,omitempty"`
       Branch      string `json:"branch,omitempty"`
   }
   ```

2. Implement scope resolution from `--app-root` or cwd plus `--session
   current|<id>`. Reuse existing app discovery and local agent helpers where
   possible. The default for every query command is `--session current`.
3. Define request/response types for logs query, logs tail, metrics query,
   metrics labels, metrics series, and inspect observability. Response
   `schema_version` values should be:
   - `scenery.inspect.observability.v1`
   - `scenery.logs.query.v1`
   - `scenery.logs.tail.entry.v1` for self-describing streaming JSONL records.
   - `scenery.metrics.query.v1`
   - `scenery.metrics.labels.v1`
   - `scenery.metrics.series.v1`
4. Implement VictoriaLogs query support against `/select/logsql/query`. Pass
   `query`, `start`, `end`, `limit`, and `timeout` using HTTP parameters. Apply
   app/session scope through `extra_filters`, not by modifying the LogsQL text.
   Do not require app root hash for logs unless the backend row schema is known
   to include it.
5. Implement VictoriaLogs tail support against `/select/logsql/tail` with the
   same scope behavior. Ensure cancellation propagates from `context.Context`
   and the command exits cleanly on interrupt.
6. Normalize log result records into stable fields:
   `time`, `level`, `source`, `message`, `fields`, `trace_id`, `span_id`, and
   raw backend fields where needed under a bounded `raw` object.
7. Implement VictoriaMetrics instant and range query support. Use
   `/prometheus/api/v1/query` when `--instant` is set, otherwise
   `/prometheus/api/v1/query_range` with `--since` or `--start/--end` plus
   `--step`.
8. Apply metrics scope through repeated `extra_label` parameters such as
   `scenery_app=<app_id>`, `scenery_session_id=<session_id>`, and
   `scenery_app_root_hash=<hash>`.
9. Implement `metrics labels` through `/prometheus/api/v1/labels` and
   `metrics series` through `/prometheus/api/v1/series`, including `start`,
   `end`, `match[]`, `limit`, and the same `extra_label` scope.
10. Add `scenery inspect observability --json [--app-root <path>] [--session
    current|<id>]`. Output backend kinds, dialects, readiness, query examples,
    and the exact enforced scope.
11. Change `logsCommand` routing so `scenery logs query ...` and `scenery logs
    tail ...` dispatch to new handlers, while plain `scenery logs ...` continues
    to call the existing dev-event reader.
12. Change `metricsCommand` routing so `query`, `labels`, and `series` dispatch
    to new handlers, while `list` preserves current behavior and a missing
    subcommand continues to produce a helpful usage error.
13. Add parser tests for every new subcommand, default, required flag, invalid
    duration, invalid limit, and unknown flag.
14. Add client tests with `httptest.Server` for VictoriaLogs and VictoriaMetrics
    requests. Assert endpoint paths, query parameters, time bounds, repeated
    scope parameters, timeout, and response normalization.
15. Add command tests that prove existing `scenery logs --jsonl`,
    `scenery logs --follow`, and `scenery metrics list --json` behavior is not
    broken by subcommand dispatch.
16. Add a self-harness check or fixture-backed test for session isolation. The
    acceptance artifact should live at `.scenery/harness/observability/latest.json`
    and report booleans such as `logs_scoped`, `metrics_scoped`,
    `cross_session_log_leak`, and `cross_session_metric_leak`.
17. Add JSON schemas under `docs/schemas/` for the new stable response shapes
    and wire them into schema validation where current self-harness expects
    schema references.
18. Update `docs/local-contract.md` with command grammar, defaults, schema
    versions, scope semantics, unscoped-query friction, artifact paths, and
    stability labels.
19. Update `docs/agent-guide.md`, `SKILL.md`, and
    `docs/app-development-cookbook.md` so agents use the CLI query surface for
    logs and metrics instead of Grafana scraping or raw Victoria URLs.
20. Update `docs/knowledge.json` for the new schemas, any changed docs, and this
    active ExecPlan entry.

## Validation and Acceptance

Acceptance criteria:

- `scenery inspect observability --json` returns
  `scenery.inspect.observability.v1` with logs, metrics, and traces backend
  readiness; dialects; examples; selected app/session; and enforced scope.
- `scenery logs query --json --since 15m --limit 100 --query 'error OR panic'`
  returns `scenery.logs.query.v1`, uses VictoriaLogs LogsQL, applies scope through
  `extra_filters`, and returns bounded normalized log entries.
- `scenery logs tail --query 'error'` streams only scoped log entries for the
  selected app session and exits cleanly on cancellation.
- `scenery logs query --logql '{app=\"demo\"} |= \"error\"'` either performs an
  explicit, tested translation or rejects with a diagnostic that names LogsQL as
  the native dialect. It must not silently send Loki LogQL to VictoriaLogs.
- `scenery metrics query --json --since 15m --step 5s --promql
  'max_over_time(scenery_request_duration_seconds[15m])'` returns
  `scenery.metrics.query.v1`, uses range query semantics by default, applies
  scope through `extra_label`, and echoes query bounds and scope.
- `scenery metrics query --json --instant --promql 'up'` uses instant query
  semantics and still applies scope.
- `scenery metrics labels --json --since 1h` and `scenery metrics series --json
  --match 'scenery_request_duration_seconds'` return bounded scoped catalog data.
- Existing `scenery logs`, `scenery logs --follow`, `scenery logs --jsonl`,
  `scenery metrics list --json`, `scenery traces list --json`, and `scenery traces
  clear --json` behavior remains compatible.
- No query command runs unscoped by default. Any future unscoped mode requires
  an explicit flag pair such as `--unscoped --i-know-this-crosses-sessions` and
  echoes `scope.enforced=false`.
- Documentation, schemas, tests, and harness output agree on the public JSON
  shapes and command grammar.

Validation commands:

```sh
go test ./...
go install ./cmd/scenery
scenery inspect docs --json
scenery harness self --json --write
```

When practical after implementation, also run a fixture or temporary app session
to validate the runtime behavior:

```sh
scenery up --detach --session obs-a --app-root <fixture-app-a>
scenery up --detach --session obs-b --app-root <fixture-app-b>
scenery inspect observability --json --session obs-a --app-root <fixture-app-a>
scenery logs query --json --session obs-a --query 'unique-from-b' --app-root <fixture-app-a>
scenery metrics query --json --session obs-a --promql 'scenery_request_duration_seconds' --app-root <fixture-app-a>
scenery harness self --json --write
```

The cross-session log query should return zero entries for values emitted only
by `obs-b`. Every returned metrics series for `obs-a` must carry only the
`obs-a` scope. If a local environment cannot start detached app sessions, record
the skipped command and run the unit/httptest coverage instead.

## Idempotence and Recovery

All new query commands are read-only. Re-running them should not mutate app,
session, or substrate state. `logs tail` opens a streaming read and should close
without cleanup beyond context cancellation.

Schema and docs updates are ordinary tracked files. If implementation stops
halfway, keep this plan's `Progress`, `Surprises & Discoveries`, and `Decision
Log` current so another agent can resume from the last completed milestone.

The self-harness observability artifact is generated evidence under `.scenery/`
and must not be committed. Re-running the harness can replace it safely.

If a Victoria component is not running or has no data, commands should return
valid JSON with a warning and empty result sets where that matches the existing
observability list behavior. Hard failures should be reserved for invalid flags,
unresolvable app roots, unresolvable sessions, unavailable required backends, or
malformed backend responses.

## Artifacts and Notes

Generated or written evidence:

- `.scenery/harness/observability/latest.json` for self-harness session-isolation
  evidence.
- Existing `.scenery/harness/self-latest.json` for full self-harness evidence.

External references checked while writing this plan:

- VictoriaLogs Querying:
  `https://docs.victoriametrics.com/victorialogs/querying/`
- VictoriaLogs LogQL to LogsQL conversion:
  `https://docs.victoriametrics.com/victorialogs/logql-to-logsql/`
- VictoriaMetrics API examples:
  `https://docs.victoriametrics.com/victoriametrics/url-examples/`
- VictoriaMetrics single-node reference for `extra_label`:
  `https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/`

Current local review-due signal:

- `docs/ui-agent-contract.md` is review-due as of 2026-06-08, but this plan does
  not touch dashboard UI. Handle that in a separate doc-gardening change unless
  future implementation work crosses UI boundaries.

## Interfaces and Dependencies

Public CLI additions:

```text
scenery inspect observability --json [--app-root <path>] [--session current|<id>]

scenery logs query [--app-root <path>] [--session current|<id>] --query <logsql> [--logql <logql>] [--since <duration>] [--start <time>] [--end <time>] [--limit <n>] [--timeout <duration>] [--fields <csv>] [--json|--jsonl]
scenery logs tail [--app-root <path>] [--session current|<id>] --query <logsql> [--since <duration>] [--timeout <duration>] [--fields <csv>] [--jsonl]

scenery metrics query [--app-root <path>] [--session current|<id>] --promql <query> [--instant] [--since <duration>] [--start <time>] [--end <time>] [--step <duration>] [--timeout <duration>] [--limit <n>] [--json]
scenery metrics labels [--app-root <path>] [--session current|<id>] [--since <duration>] [--start <time>] [--end <time>] [--limit <n>] [--json]
scenery metrics series [--app-root <path>] [--session current|<id>] --match <selector> [--since <duration>] [--start <time>] [--end <time>] [--limit <n>] [--json]
```

Default values:

- `--session current`
- logs query `--since 15m`, `--limit 200`, `--timeout 3s`, `--json`
- metrics query `--since 15m`, `--step 5s`, `--timeout 3s`, `--json`
- metrics labels/series `--since 1h`, bounded `--limit`

Backend dependencies:

- VictoriaLogs native dialect is LogsQL. Query endpoints are
  `/select/logsql/query` and `/select/logsql/tail`.
- VictoriaMetrics accepts PromQL/MetricsQL at
  `/prometheus/api/v1/query` and `/prometheus/api/v1/query_range`, with catalog
  endpoints `/prometheus/api/v1/labels` and `/prometheus/api/v1/series`.
- Scenery applies log scope through VictoriaLogs `extra_filters` and metric scope
  through VictoriaMetrics `extra_label`.

Proposed internal API shape:

```go
func Inspect(ctx context.Context, scope QueryScope, stack VictoriaStack) (*InspectResult, error)
func QueryLogs(ctx context.Context, stack VictoriaStack, q LogsQuery) (*LogsQueryResult, error)
func TailLogs(ctx context.Context, stack VictoriaStack, q LogsQuery, emit func(LogEntry) error) error
func QueryMetrics(ctx context.Context, stack VictoriaStack, q MetricsQuery) (*MetricsQueryResult, error)
func MetricsLabels(ctx context.Context, stack VictoriaStack, q MetricsCatalogQuery) (*MetricsLabelsResult, error)
func MetricsSeries(ctx context.Context, stack VictoriaStack, q MetricsCatalogQuery) (*MetricsSeriesResult, error)
```

Keep implementation dependency growth at zero unless tests show a clear reason
for a small parser or streaming helper. The Go standard library should be
enough for flag parsing, HTTP requests, JSON/JSONL decoding, and time bounds.
