---
name: onlava
description: Use when building, running, debugging, inspecting, validating, or generating clients for onlava applications. onlava is a Go-native service runtime and CLI using .onlava.json, //onlava directives, typed endpoints, local dev supervision, MCP, logs, traces, metrics, psql, Temporal/cron workers, and TypeScript client generation.
---

# onlava

onlava is a Go-native service runtime and local development platform. Applications are ordinary Go modules with a `.onlava.json` app root and `//onlava:` directives in Go source.

This skill is for agents working inside onlava apps. Install it with:

```sh
npx skills add https://github.com/pbrazdil/onlava
```

This installs the agent skill, not the `onlava` CLI. The `onlava` binary must also be available on `PATH`.

Read next when you need detail:

- `docs/agent-guide.md` for agent workflows, MCP, and client-app integration.
- `docs/local-contract.md` for exact CLI grammar, JSON schemas, generated artifact paths, and stable/beta labels.
- `docs/app-development-cookbook.md` for practical app recipes.
- `docs/ui-agent-contract.md` before dashboard or app UI work.
- app-local `AGENTS.md` for product-specific rules in client repositories.

## Agent Rules

Prefer machine-readable onlava surfaces over prose:

```sh
onlava check --json
onlava inspect app --json
onlava inspect routes --json
onlava inspect endpoints --json
onlava inspect wire --json
onlava logs --jsonl --limit 200
onlava harness --json --write
```

Use `onlava dev` for local development and debugging. Use `onlava run` only for headless API execution. Use `onlava worker` for worker-only cron/Temporal processes.

The skill gives shared onlava knowledge. It is not enough for a product app by itself. Client apps should also keep a small app-local `AGENTS.md` with app root, frontend roots, generated client paths, required env names, validation commands, and product invariants.

## Mental Model

- `.onlava.json` marks the app root and names/configures the app.
- Go source is the app model. onlava discovers services, APIs, auth handlers, middleware, Temporal declarations, cron jobs, and generated-client capabilities from code.
- `onlava dev` starts supervised local development: API child process, rebuild/restart loop, dashboard, API Explorer, MCP endpoint, logs, traces, metrics, managed dev services, Grafana/Victoria sidecars when available, and optional frontend/proxy routing.
- `onlava run` builds once and starts a headless API-role server. It does not start dashboard, MCP, proxy, watch mode, or dev/admin endpoints.
- `onlava worker` builds once and starts a worker-role process for cron and native Temporal workers.
- Public and auth endpoints are externally reachable. Private endpoints are internal-only and called through generated helpers.
- Typed endpoints decode path params, query params, headers, cookies, and JSON bodies into Go values, then encode typed responses.
- Generated internal calls preserve routing, private access, auth context, tracing, and error semantics.

## Minimal App

Create `.onlava.json`:

```json
{"name":"hello"}
```

Create `go.mod`:

```go
module example.com/hello

go 1.26.0

require github.com/pbrazdil/onlava v0.0.0
```

Create `service/api.go`:

```go
package service

import "context"

type HelloResponse struct {
	Message string `json:"message"`
}

//onlava:api public path=/hello/:name method=GET
func Hello(ctx context.Context, name string) (*HelloResponse, error) {
	return &HelloResponse{Message: "hello " + name}, nil
}
```

Validate and run:

```sh
onlava check --json
onlava run
curl http://127.0.0.1:4000/hello/world
```

## Directives And Signatures

```go
//onlava:api public|auth|private [raw] [path=/...] [method=GET,POST]
//onlava:service
//onlava:authhandler
```

Typed API shape:

```go
func Endpoint(ctx context.Context, pathParam string, req *Request) (*Response, error)
```

Raw API shape:

```go
func Endpoint(w http.ResponseWriter, req *http.Request)
```

Route defaults:

- Default path is `/<service>.<Endpoint>`.
- Typed endpoint default methods are `GET,POST` when no payload exists.
- Typed endpoint default method is `POST` when a payload exists.
- Raw endpoint default method is wildcard.

Struct tags:

- Request decoding: `json`, `header`, `query`, `qs`, `cookie`.
- onlava tags: `onlava:"optional"` and `onlava:"httpstatus"`.

## Public Go Packages

- `github.com/pbrazdil/onlava`: app metadata and `CurrentRequest()`.
- `github.com/pbrazdil/onlava/auth`: request auth helpers such as `UserID()`, `Data()`, `CurrentAuthData()`, and standard auth.
- `github.com/pbrazdil/onlava/errs`: coded errors and HTTP status mapping.
- `github.com/pbrazdil/onlava/middleware`: middleware request/response types.
- `github.com/pbrazdil/onlava/temporal`: beta workflow/activity declarations and start helpers for the onlava-managed Temporal runtime.
- `github.com/pbrazdil/onlava/cron`: cron job declarations.
- `github.com/pbrazdil/onlava/pgxpool`: pgx pool wrapper with onlava DB tracing.
- `github.com/pbrazdil/onlava/et`: endpoint and service mocking helpers for tests.

## Auth

Use `//onlava:authhandler` for custom request authentication. Auth handlers may be package functions or service methods.

Standard auth can be enabled in `.onlava.json`:

```json
{
  "name": "hello",
  "auth": {
    "enabled": true,
    "database_url_env": "DatabaseURL",
    "dev_bootstrap": { "enabled": true }
  }
}
```

Standard auth registers `/auth/*` endpoints plus local `/users/dev-bootstrap`, stores DB-backed state in PostgreSQL schema `onlava_auth`, and exposes `*auth.AuthData`.

Inside auth-protected endpoints:

```go
import "github.com/pbrazdil/onlava/auth"

userID, ok := auth.UserID()
data := auth.Data()
standardData, ok := auth.CurrentAuthData()
```

Use `//onlava:api auth` for externally reachable endpoints that require auth. Use `//onlava:api private` for internal-only endpoints.

## Local Development

Start the full local development platform:

```sh
onlava dev
```

Common modes:

```sh
onlava dev --json
onlava dev --detach
onlava attach
onlava down
onlava dev --session feature-a
onlava dev --new-session
onlava dev --port 4000 --listen 127.0.0.1
```

Use `onlava dev --json` for JSONL events. Child stdout/stderr become structured `process.output` events.

Use `onlava dev --detach` when a local agent session should keep running after the command returns. Use `onlava attach` to follow current session logs and `onlava down` to stop the selected session.

Local startup expects an app-root `.env` for `onlava dev`, local `onlava run`, and local `onlava worker`. `.env.local` is optional and overrides `.env` only where the parent process environment has not already set a key. `onlava run --env production` can rely on process environment instead.

## MCP

`onlava dev` exposes a development MCP server over SSE. The startup banner prints `MCP SSE URL`; agent-router sessions also expose a session-scoped MCP route.

Use MCP for interactive local app work when a dev session is running:

- list services, middleware, auth handlers, and cron jobs
- call endpoints through the dev runtime
- read recent traces and trace spans
- inspect discovered development database metadata and run local SQL queries
- read selected source files
- report referenced env names and availability without printing values

Treat MCP as a dev convenience surface. For stable automation, prefer `onlava inspect ... --json`, `onlava logs --jsonl`, schemas, and harness outputs.

## Debugging

First checks:

```sh
onlava check --json
onlava inspect app --json
onlava inspect routes --json
onlava inspect endpoints --json
onlava inspect paths --json
onlava logs --limit 200
onlava logs --jsonl --limit 200
```

Traces and metrics:

```sh
onlava inspect traces --json --session current --since 15m --slowest
onlava inspect traces --json --trace-id <trace-id>
onlava inspect metrics --json --session current --since 1h
```

Harness snapshot:

```sh
onlava harness --json --write
```

Dashboard browser checks:

```sh
onlava harness ui --json
onlava harness ui --json --headed
```

## Database And psql

Prefer the PRD-facing DB command:

```sh
onlava db psql [psql args...]
```

Managed session database commands:

```sh
onlava db reset
onlava db drop
onlava db snapshot create <name>
onlava db snapshot restore <name>
```

`onlava psql` is a legacy beta spelling kept as a helper. Prefer `onlava db psql` in new docs and scripts.

## TypeScript Client Generation

Generate a TypeScript client:

```sh
onlava gen client --lang typescript --output ./src/onlava-client.ts
```

Inspect route and wire support first:

```sh
onlava inspect endpoints --json
onlava inspect wire --json
```

Regenerate committed clients after endpoint, request/response, auth, or wire-capability changes.

## Command Reference

```text
onlava dev [--port <n>] [--listen <addr>] [--app-root <path>] [--session <id>|--new-session] [-v|--verbose] [--json] [--proxy] [--trust] [--detach]
onlava attach [--app-root <path>] [--session current|<id>] [--limit <n>] [--stream all|stdout|stderr] [--jsonl|--json]
onlava agent [--socket <path>] [--router-listen <addr>] [--router-tls|--router-http] [--trust] [--json]
onlava agent restart [--socket <path>] [--router-listen <addr>] [--router-tls|--router-http] [--trust] [--json]
onlava status --json [--app-root <path>] [--session <id>]
onlava down [--app-root <path>] [--session <id>] [--db] [--state] [--all]
onlava prune --older-than <duration> [--app-root <path>] [--json]
onlava run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]
onlava worker [--task-queue <name>[,<name>...]]... [--app-root <path>] [--env <name>] [--log-format text|json]
onlava worker bindings [--app-root <path>] [--out <dir>] [--json]
onlava worker typescript [--task-queue <name>[,<name>...]]... [--runtime bun|node] [--app-root <path>] [--generate-only]
onlava temporal deployment set-current --build-id <id> [--deployment <name>] [--ignore-missing-task-queues] [--allow-no-pollers] [--app-root <path>] [--json]
onlava temporal deployment ramp --build-id <id> --percentage <0-100> [--deployment <name>] [--ignore-missing-task-queues] [--allow-no-pollers] [--app-root <path>] [--json]
onlava temporal deployment drain --build-id <id> [--deployment <name>] [--force] [--app-root <path>] [--json]
onlava version [--json]
onlava build [--app-root <path>] [-o <path>]
onlava check [--app-root <path>] [--json]
onlava db psql [--app-root <path>] [psql args...]
onlava db reset [--app-root <path>]
onlava db drop [--app-root <path>]
onlava db snapshot create|restore <name> [--app-root <path>]
onlava harness [--app-root <path>] [--json] [--write]
onlava harness self [--repo-root <path>] [--json] [--write]
onlava harness ui --json [--app-root <path>] [--dashboard-url <url>] [--headed] [--write]
onlava inspect app|routes|services|endpoints|wire|build|paths|temporal|traces|metrics --json [--app-root <path>]
onlava inspect docs --json [--repo-root <path>]
onlava logs [--app-root <path>] [--session current|<id>] [--limit <n>] [--stream all|stdout|stderr] [-f|--follow] [--jsonl|--json]
onlava test [--app-root <path>] [go test flags/packages...]
onlava gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]
onlava psql [--app-root <path>] [psql args...]
```

## Validation Before Finishing

For most app changes:

```sh
onlava check --json
go test ./...
```

For broader app changes:

```sh
onlava harness --json --write
```

For onlava repo changes:

```sh
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```
