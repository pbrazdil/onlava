# Pulse Local Contract

This document freezes the local developer and agent-facing contract for Pulse v0.

The goal is to make Pulse deterministic and inspectable:
- app shape is explicit
- CLI grammar is explicit
- machine-readable JSON outputs have versioned schemas
- generated and cache artifact locations are named, even where some paths are still reserved for upcoming work

If implementation and this document disagree, treat that as a bug.

## Status

Implemented now:
- `pulse.app`
- `pulse dev --json`
- `pulse run`
- `pulse version --json`
- `pulse check --json`
- `pulse harness --json`
- `pulse harness self --json`
- `pulse admin traces clear --json`
- `pulse admin pubsub clear --json`
- `pulse inspect app --json`
- `pulse inspect routes --json`
- `pulse inspect services --json`
- `pulse inspect endpoints --json`
- `pulse inspect wire --json`
- `pulse inspect build --json`
- `pulse inspect paths --json`
- `pulse inspect traces --json`
- `pulse inspect metrics --json`
- `pulse inspect docs --json`
- `pulse logs --jsonl`
- `.pulse/gen/app.json`
- `.pulse/gen/routes.json`
- `.pulse/gen/services.json`
- `.pulse/gen/endpoints.json`
- `.pulse/gen/wire/capabilities.json`
- `.pulse/gen/manifest.json`
- `.pulse/build/latest.json`
- `.pulse/harness/latest.json`
- `.pulse/harness/self-latest.json`

Reserved by contract, implementation pending:
- other `pulse admin ... --json` commands beyond `traces clear` and `pubsub clear`
- repo-local runtime and state manifests beyond `.pulse/build/latest.json`, `.pulse/gen/*`, and `.pulse/harness/latest.json`

Stable v0 surface:
- `pulse.app`
- `pulse run`
- `pulse build`
- `pulse version --json`
- `pulse check --json`
- `pulse inspect ... --json`
- `pulse logs --jsonl`
- `pulse test`
- `pulse gen client`
- typed/raw HTTP endpoints
- auth handler
- service struct initialization and shutdown
- private/internal calls
- secrets from process env and local `.env`
- basic traces and logs

Dev-only or beta surface:
- `pulse dev`
- dashboard and API Explorer
- DB Studio
- MCP server
- local HTTPS/frontend proxy
- trust-store installation
- Pub/Sub UI and queue controls
- cron UI
- Encore migration compatibility

Compatibility posture:
- Pulse-native syntax and imports are the stable API.
- Encore directives/imports may still be accepted for migration, but they are compatibility behavior and not the primary v0 API.

## `pulse.app`

Schema:
- [pulse.app.v1.schema.json](schemas/pulse.app.v1.schema.json)

Current shape:

```json
{
  "name": "myapp",
  "id": "myapp-dev",
  "proxy": {
    "workspace": "onlv",
    "api_host": "api.onlv.localhost",
    "console_host": "console.onlv.localhost",
    "mcp_host": "mcp.onlv.localhost",
    "frontend_host": "pulse.onlv.localhost"
  },
  "observability": {
    "logs": {
      "include_endpoints": [],
      "exclude_endpoints": []
    },
    "tracing": {
      "include_endpoints": [],
      "exclude_endpoints": []
    }
  }
}
```

Rules:
- `name` or `id` must be non-empty.
- If `name` is empty, Pulse falls back to `id`.
- `proxy` is optional.
- `observability` is optional.
- Unknown fields are currently ignored by Go JSON decoding, but they are not part of the frozen contract.

## CLI Grammar

Current implemented grammar:

```text
pulse dev [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json] [--proxy] [--trust]
pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]
pulse version [--json]
pulse build [--app-root <path>] [-o <path>] [--db-studio]
pulse check [--app-root <path>] [--json]
pulse harness [--app-root <path>] [--json] [--write]
pulse harness self [--repo-root <path>] [--json] [--write]
pulse inspect app|routes|services|endpoints|wire|build|paths|traces|metrics --json [--app-root <path>]
pulse inspect docs --json [--repo-root <path>]
pulse inspect traces --json [--service <name>] [--endpoint <name>] [--trace-id <id>] [--status ok|error] [--min-duration-ms <n>] [--since <duration>] [--limit <n>] [--slowest]
pulse inspect metrics --json [--service <name>] [--endpoint <name>] [--status ok|error] [--since <duration>] [--limit <n>]
pulse admin traces clear --json [--app-root <path>]
pulse admin pubsub clear --json [--app-root <path>]
pulse logs [--app-root <path>] [--limit <n>] [--stream all|stdout|stderr] [-f|--follow] [--jsonl|--json]
pulse test [--app-root <path>] [go test flags/packages...]
pulse gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]
```

Frozen `inspect` rules:
- `pulse inspect` requires a subject.
- `pulse inspect` currently requires `--json`.
- `--app-root` is optional. When omitted, Pulse walks upward from the current working directory to find `pulse.app`.
- `traces` and `metrics` prefer local VictoriaTraces reads when those sidecars are available, and fall back to the Pulse dashboard SQLite store. If no local state exists, they return valid JSON with a warning and empty result sets.
- `--since` accepts Go duration strings such as `15m`, `1h`, or `24h`.
- `--min-duration-ms` filters root traces by duration in milliseconds.
- `--status` accepts `ok` or `error`.
- `metrics` defaults to `--since 24h` and `--limit 10000` so agents get useful local summaries without scanning unbounded history.
- `docs` inspects the Pulse repo knowledge base, not a target Pulse app. It accepts `--repo-root` and otherwise walks upward to the `module pulse.dev` repo root.

Command split:

- `pulse dev` starts the local development platform: app process, dashboard, MCP endpoint, DB Studio when configured, file watching, and rebuild/restart supervision.
- `pulse dev` also starts local VictoriaMetrics, VictoriaLogs, and VictoriaTraces sidecars by default when their binaries can be found or downloaded. SQLite dashboard storage remains active for parity and fallback.
- `pulse dev --proxy` enables the local HTTPS/frontend proxy.
- `pulse dev --proxy --trust` allows local trust-store installation. Without `--trust`, the proxy skips trust installation.
- `pulse run` builds once and starts the app runtime headlessly. It does not start the dashboard, MCP server, local proxy, DB Studio, frontend proxy, or file watcher.
- `pulse build` produces the deployable binary and remains the preferred deployment artifact path.
- Generated app binaries are headless by default. `pulse build --db-studio` is an explicit opt-in for the DB Studio integration.

Runtime safety:

- `pulse run` and generated binaries do not expose dev/admin endpoints by default.
- Dev/admin endpoints such as `/__pulse/config`, `/__pulse/pubsub/clear`, `/platform.Stats`, and `/debug/pprof/*` are enabled only for the development child process launched by `pulse dev` or when `PULSE_DEV_ENDPOINTS=1` is set explicitly.
- Runtime CORS reflection is enabled in dev endpoint mode. Outside dev mode, CORS origins must be explicitly allowlisted with `PULSE_CORS_ALLOW_ORIGINS`.
- Build workspaces skip local secret and machine artifacts such as `.env`, `.env.*`, `.git`, `.pulse`, `node_modules`, `.DS_Store`, `__MACOSX`, and `coverage`.

Local observability:

- Pulse keeps SQLite observability writes active in `pulse dev`.
- When Victoria sidecars are available, Pulse also exports OTLP protobuf to:
  - VictoriaMetrics: `/opentelemetry/v1/metrics`
  - VictoriaLogs: `/insert/opentelemetry/v1/logs`
  - VictoriaTraces: `/insert/opentelemetry/v1/traces`
- Dashboard trace reads and `pulse inspect traces|metrics --json` prefer Victoria data and fall back to SQLite data.
- Victoria sidecars are supervised by `pulse dev`, store data under `.pulse/victoria/` by default, and are stopped with the dev supervisor.
- `PULSE_DEV_VICTORIA=0` disables Victoria sidecars. `PULSE_DEV_VICTORIA_DOWNLOAD=0` disables automatic binary downloads.

Secrets and environment:

- Process environment always wins over values loaded from local files.
- The stable runtime path reads `.env` from the app root for local secret population when a value is not already present in the process environment.
- `pulse dev` passes local file values into the child process before Go package initialization so package-level declarations can read them through `os.Getenv`.
- `pulse dev` loads `.env` first and `.env.local` second. `.env.local` overrides `.env` only for keys that are not already present in the parent process environment.
- `.env`, `.env.*`, and secret-bearing local files are not copied into build workspaces.

Implemented `dev --json` rules:

```text
pulse dev --json
```

- output is JSONL
- each line conforms to `pulse.run.event.v1`
- human-readable console output is suppressed in this mode
- child stdout/stderr are emitted as structured `process.output` events instead of raw terminal writes

Implemented `check --json` rules:

```text
pulse check --json
```

- output is a single JSON document
- output conforms to `pulse.check.result.v1`
- success returns `ok: true` and an empty `diagnostics` array
- failure returns `ok: false` and structured diagnostics
- diagnostics may include `stage`, `file`, `line`, `column`, `severity`, `message`, and `suggested_action`

Implemented `harness --json` rules:

```text
pulse harness --json
pulse harness --json --write
```

- output is a single JSON document
- output conforms to `pulse.harness.result.v1`
- it composes `pulse check --json` and the stable `pulse inspect ... --json` surfaces
- success returns `ok: true`
- failure returns `ok: false`, per-step errors, diagnostics, and `next_actions`
- `--write` persists the same result to `.pulse/harness/latest.json`

Implemented `harness self --json` rules:

```text
pulse harness self --json
pulse harness self --json --write
```

- output is a single JSON document
- output conforms to `pulse.harness.self.v1`
- it validates the Pulse repo itself instead of a target app
- it runs docs knowledge validation, `pulse inspect docs --json`, architecture checks, Go package tests for the CLI, dev dashboard store, and runtime, dashboard UI typecheck/build, DB Studio UI typecheck/build, UI freshness checks, `go install ./cmd/pulse`, and installed binary freshness checks
- architecture checks fail on unapproved direct dependencies, forbidden framework imports, CLI package boundary violations, missing generated/vendored ignore markers, non-fixture `encore.gen.go`, and non-generated source files over 2500 lines
- architecture checks warn on non-generated source files over 1000 lines, cgo imports, `.DS_Store` artifacts, and compatibility imports outside known migration paths
- `--write` persists the same result to `.pulse/harness/self-latest.json`

Implemented `logs --jsonl` rules:

```text
pulse logs --jsonl
pulse logs --json
```

- `--json` is an alias for `--jsonl`
- output is JSONL
- each line conforms to `pulse.logs.event.v1`
- one JSON object is emitted per stored process-output chunk
- human-readable raw output remains the default when neither flag is used

Reserved grammar:

```text
pulse admin <subcommand> --json ...
```

Implemented `admin --json` rules:
- current supported commands are `traces clear` and `pubsub clear`
- output conforms to `pulse.admin.result.v1`
- `pubsub clear` requires a running Pulse dashboard/supervisor because it tunnels through the supervisor RPC surface

Any additional admin subcommands are reserved contract surfaces and should produce versioned JSON when implemented.

## Artifact Locations

### Current implemented locations

Use `pulse inspect paths --json` as the source of truth.

Today Pulse uses:
- app config: `<app-root>/pulse.app`
- cache root:
  - `$PULSE_DEV_CACHE_DIR`, if set
  - otherwise OS user cache + `/pulse`
- build workspace: `<cache-root>/build/<sanitized-app-name>-<hash>`
- built app binary: `<workspace>/pulse-app`
- build state: `<workspace>/.pulse-build-state.json`

### Stable repo-local locations

Implemented now:

```text
<app-root>/.pulse/
  gen/
    app.json
    routes.json
    services.json
    endpoints.json
    wire/
      capabilities.json
    manifest.json
  build/
    latest.json
  harness/
    latest.json
    self-latest.json
```

Reserved for upcoming work:

```text
<app-root>/.pulse/
  state/
  logs/
```

Rules:
- `app.json`, `routes.json`, `services.json`, and `endpoints.json` mirror the current `pulse inspect ... --json` outputs for those subjects
- `wire/capabilities.json` mirrors `pulse inspect wire --json` and the runtime `GET /_wire/capabilities` response
- `manifest.json` ties the generated inspect artifacts to schema versions, stable artifact paths, and deterministic content hashes
- `build/latest.json` is the stable repo-local pointer to the latest prepared or compiled build workspace
- `harness/latest.json` is the stable repo-local pointer to the latest agent validation run
- `harness/self-latest.json` is the stable repo-local pointer to the latest Pulse repo validation run
- agents can use either `pulse inspect ... --json` or the corresponding `.pulse/gen/*.json` files
- future implementation should conform to these locations instead of inventing a different layout

## JSON Schemas

Implemented now:
- [pulse.inspect.app.v1.schema.json](schemas/pulse.inspect.app.v1.schema.json)
- [pulse.inspect.routes.v1.schema.json](schemas/pulse.inspect.routes.v1.schema.json)
- [pulse.inspect.services.v1.schema.json](schemas/pulse.inspect.services.v1.schema.json)
- [pulse.inspect.endpoints.v1.schema.json](schemas/pulse.inspect.endpoints.v1.schema.json)
- [pulse.inspect.traces.v1.schema.json](schemas/pulse.inspect.traces.v1.schema.json)
- [pulse.inspect.metrics.v1.schema.json](schemas/pulse.inspect.metrics.v1.schema.json)
- [pulse.inspect.docs.v1.schema.json](schemas/pulse.inspect.docs.v1.schema.json)
- [pulse.docs.index.v1.schema.json](schemas/pulse.docs.index.v1.schema.json)
- [pulse.wire.capabilities.v1.schema.json](schemas/pulse.wire.capabilities.v1.schema.json)
- [pulse.inspect.build.v1.schema.json](schemas/pulse.inspect.build.v1.schema.json)
- [pulse.inspect.paths.v1.schema.json](schemas/pulse.inspect.paths.v1.schema.json)
- [pulse.gen.manifest.v1.schema.json](schemas/pulse.gen.manifest.v1.schema.json)
- [pulse.build.latest.v1.schema.json](schemas/pulse.build.latest.v1.schema.json)
- [pulse.run.event.v1.schema.json](schemas/pulse.run.event.v1.schema.json)
- [pulse.check.result.v1.schema.json](schemas/pulse.check.result.v1.schema.json)
- [pulse.harness.result.v1.schema.json](schemas/pulse.harness.result.v1.schema.json)
- [pulse.harness.self.v1.schema.json](schemas/pulse.harness.self.v1.schema.json)
- [pulse.logs.event.v1.schema.json](schemas/pulse.logs.event.v1.schema.json)
- [pulse.admin.result.v1.schema.json](schemas/pulse.admin.result.v1.schema.json)
- [pulse.version.v1.schema.json](schemas/pulse.version.v1.schema.json)

Reserved now:
- future command-specific admin schemas if `pulse.admin.result.v1` becomes too generic

Schema rules:
- top-level schema field is `schema_version`
- schema names are versioned strings like `pulse.inspect.app.v1`
- additive fields are allowed in future versions only by introducing a new schema version when needed
- consumers should match on `schema_version`, not on command name alone

## Examples

### `pulse inspect app --json`

```json
{
  "schema_version": "pulse.inspect.app.v1",
  "app": {
    "name": "billing",
    "id": "billing-dev",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app",
    "module_path": "example.com/billing"
  },
  "config": {
    "name": "billing",
    "id": "billing-dev",
    "proxy": {
      "workspace": "billing",
      "api_host": "api.billing.localhost",
      "console_host": "console.billing.localhost",
      "mcp_host": "mcp.billing.localhost",
      "frontend_host": "pulse.billing.localhost"
    },
    "observability": {
      "logs": {
        "include_endpoints": [],
        "exclude_endpoints": []
      },
      "tracing": {
        "include_endpoints": [],
        "exclude_endpoints": []
      }
    }
  },
  "counts": {
    "packages": 3,
    "services": 2,
    "endpoints": 7,
    "middleware": 1,
    "auth_handler": 1
  },
  "services": [
    "auth",
    "users"
  ],
  "auth_handler": {
    "service": "auth",
    "name": "AuthHandler"
  }
}
```

### `pulse inspect build --json`

```json
{
  "schema_version": "pulse.inspect.build.v1",
  "app": {
    "name": "billing",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app"
  },
  "build": {
    "workspace_dir": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789",
    "binary_path": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789/pulse-app",
    "workspace_exists": true,
    "binary_exists": true,
    "build_state_path": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789/.pulse-build-state.json",
    "build_state_exists": true,
    "build_state_version": "2",
    "dependency_fingerprint": "abc123",
    "graph_fingerprint": "def456",
    "metadata_present": true,
    "api_encoding_present": true,
    "source_file_count": 24,
    "generated_file_count": 6
  }
}
```

### `pulse inspect endpoints --json`

```json
{
  "schema_version": "pulse.inspect.endpoints.v1",
  "app": {
    "name": "billing",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app"
  },
  "endpoints": [
    {
      "id": "users.Get",
      "service": "users",
      "endpoint": "Get",
      "access": "public",
      "raw": false,
      "path": "/users/:id",
      "methods": ["GET"],
      "has_payload": true,
      "wire": {
        "available": true,
        "schema_hash": "abc123",
        "path": "/_wire/users.Get"
      }
    }
  ],
  "wire": {
    "wire_schema_hash": "def456",
    "available": 1,
    "unsupported": 0
  }
}
```

### `pulse inspect wire --json`

`pulse inspect wire --json` returns the same hidden generated-client capability document served at `GET /_wire/capabilities`. It is intended for generated clients and agents that need to know whether the JSON transport or binary transport will be used for each logical endpoint.

### `pulse inspect traces --json`

Use this when an agent needs concrete local traces without scraping the dashboard UI.

Example:

```text
pulse inspect traces --json --endpoint SyncGet --min-duration-ms 2000 --since 1h --slowest
```

Example output:

```json
{
  "schema_version": "pulse.inspect.traces.v1",
  "app": {
    "name": "billing",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app"
  },
  "query": {
    "app_id": "billing",
    "limit": 100,
    "since": "1h0m0s",
    "endpoint": "SyncGet",
    "min_duration_ms": 2000,
    "sort": "duration_desc",
    "available_filters": ["--service", "--endpoint", "--trace-id", "--status ok|error", "--min-duration-ms", "--since", "--limit", "--slowest"]
  },
  "traces": [
    {
      "trace_id": "trace-1",
      "span_id": "span-1",
      "kind": "RPC",
      "status": "ok",
      "service": "sync",
      "endpoint": "SyncGet",
      "started_at": "2026-04-27T13:00:00Z",
      "duration_ms": 2310,
      "duration_nanos": 2310000000
    }
  ]
}
```

### `pulse inspect metrics --json`

Use this when an agent needs a metrics-style rollup over locally captured traces and logs.

Example:

```text
pulse inspect metrics --json --service sync --since 15m
```

Example output:

```json
{
  "schema_version": "pulse.inspect.metrics.v1",
  "app": {
    "name": "billing",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app"
  },
  "query": {
    "app_id": "billing",
    "limit": 10000,
    "since": "15m0s",
    "service": "sync",
    "sort": "started_at_desc",
    "available_filters": ["--service", "--endpoint", "--trace-id", "--status ok|error", "--min-duration-ms", "--since", "--limit", "--slowest"]
  },
  "summary": {
    "trace_count": 12,
    "error_count": 1,
    "error_rate": 0.08333333333333333,
    "event_count": 34,
    "log_count": 9,
    "avg_duration_ms": 120.4,
    "min_duration_ms": 3.1,
    "max_duration_ms": 520.7,
    "p50_duration_ms": 88.2,
    "p95_duration_ms": 500.1
  },
  "services": [],
  "endpoints": [],
  "logs": [],
  "meta": {
    "trace_metric_limit": 10000
  }
}
```

### `pulse inspect docs --json`

Use this when an agent needs to understand the repo knowledge base before making changes.

Source files:

- [docs/index.md](index.md)
- [docs/knowledge.json](knowledge.json)
- [docs/plans/active.md](plans/active.md)
- [docs/plans/completed.md](plans/completed.md)
- [docs/tech-debt.md](tech-debt.md)

Example:

```text
pulse inspect docs --json
```

Example output:

```json
{
  "schema_version": "pulse.inspect.docs.v1",
  "repo": {
    "root": "/repo/pulse",
    "module_path": "pulse.dev",
    "go_mod_path": "/repo/pulse/go.mod"
  },
  "summary": {
    "document_count": 9,
    "missing_count": 0,
    "review_due_count": 0,
    "stale_count": 0,
    "quality": {
      "A": 4,
      "B": 5
    }
  },
  "documents": [
    {
      "path": "docs/local-contract.md",
      "title": "Pulse Local Contract",
      "owner": "Pulse runtime",
      "status": "active",
      "quality": "A",
      "freshness": "current",
      "last_reviewed": "2026-04-27",
      "review_after": "2026-05-27",
      "summary": "Frozen local developer and agent-facing contract.",
      "tags": ["contract", "cli", "agents", "schemas"],
      "exists": true,
      "review_due": false,
      "stale": false
    }
  ],
  "plans": {
    "active": {
      "path": "docs/plans/active.md",
      "exists": true
    },
    "completed": {
      "path": "docs/plans/completed.md",
      "exists": true
    }
  },
  "tech_debt": {
    "path": "docs/tech-debt.md",
    "exists": true
  }
}
```
