## Verdict

Pulse should stop chasing Encore parity until it has a stable **local execution contract**. The biggest risk is not that it has too many features; it is that dev-platform conveniences are leaking into the generated app, the public runtime, the app HTTP server, and dashboard-private transports.

The clean v0 shape should be:

```text
pulse run       = deterministic app build/watch/supervise, headless by default
pulse dev       = dashboard + proxy + DB Studio + richer local tooling
pulse inspect   = machine-readable app/build/runtime introspection
pulse admin     = explicit local mutations: clear queues, query DB, clear traces
pulse gen       = deterministic generated clients/artifacts
```

I reviewed the uploaded bundle. One caveat: `internal/build/build.go` is referenced by `cmd/pulse/*` but is absent from the bundle, so build-layer comments are based on its callers and the missing artifact itself. `ui/src/router.tsx` is present in the zip/manifest but did not materialize in the extracted tree; router notes are from the archived file contents.

---

## The highest-leverage change: split “runtime” from “dev platform”

Right now `pulse run` is already doing more than “run the app.” The supervisor starts dashboard, DB Studio, and the local proxy as part of normal startup (`cmd/pulse/dev_supervisor.go:193-206`). Generated apps also import `_ "pulse.dev/runtimeapp"` (`internal/codegen/generator.go:185-200`), and `runtimeapp` imports `internal/dbstudio` and `internal/localproxy` (`runtimeapp/app.go:12-19`). That means the generated app binary can carry dev-platform concerns.

That is the architectural line I would redraw immediately.

**New rule:** the generated app binary should run the app and nothing else. The supervisor owns dev services. Directly executing a built app should not start Caddy, DB Studio, frontend proxying, dashboard helpers, or local trust-store behavior.

Concretely:

```text
pulse run
  parse -> codegen -> build -> start app -> watch -> logs/events
  no dashboard unless --dashboard
  no proxy unless --proxy
  no DB Studio unless --db-studio

pulse dev
  everything rich and interactive:
  dashboard, proxy, DB Studio, API explorer, traces, Pub/Sub UI, cron UI
```

This one split makes Pulse more agent-friendly, easier to test, easier to reason about, and less surprising for first adopters.

---

## What I would cut, simplify, or redesign now

### 1. Cut default local HTTPS/proxy behavior

The local proxy is too powerful to be default-on. `localproxy.Enabled()` defaults to true (`internal/localproxy/proxy.go:63-65`), `SkipInstallTrust()` defaults to false (`internal/localproxy/proxy.go:75-77`), and the supervisor auto-discovers a workspace name before starting the proxy (`cmd/pulse/dev_supervisor.go:735-762`). Caddy is also imported directly by the proxy package (`internal/localproxy/proxy.go:17-23`) and started/stopped process-globally (`internal/localproxy/proxy.go:136-176`).

I would change this to:

```text
default:
  no local HTTPS proxy
  no trust-store installation
  no frontend reverse proxy

opt in:
  pulse dev --proxy
  pulse dev --proxy --trust
  pulse proxy trust
```

Also consider making the default proxy a small `httputil.ReverseProxy`-style dev proxy and moving Caddy behind a build tag, sidecar, or plugin package. FEEDBACK already identified embedded Caddy as process-global, listener-owning, trust-store-capable, and imported into the build graph (`FEEDBACK.md:465-509`). That concern still holds.

### 2. Move admin/dev endpoints off the app’s public router

The app server currently registers dev/admin-ish endpoints on the public router: `/__pulse/config`, `/__pulse/pubsub/clear`, `/platform.Stats`, and `/debug/pprof/*` (`runtime/server.go:68-140`). Pub/Sub clear is token-protected, but it is still mounted on the app HTTP server (`runtime/server.go:82-103`), and the dashboard calls it via the app address (`cmd/pulse/dashboard.go:621-685`).

I would move these to a supervisor-owned admin listener or local socket:

```text
app listener:
  only user APIs

admin listener:
  /v0/status
  /v0/routes
  /v0/traces
  /v0/pubsub/clear
  /v0/pprof/*, opt-in only
```

Agents should not need to discover hidden magic endpoints on the app port. They should call `pulse inspect` or `pulse admin`.

### 3. Stop rewriting user endpoint declarations if possible

Codegen currently mutates endpoint declarations and emits rewritten source into the build workspace (`internal/codegen/generator.go:35-48`, `internal/codegen/generator.go:103-109`). Endpoint wrappers rename the implementation and keep the original function name as a runtime-routed wrapper (`internal/codegen/generator.go:487-548`).

That gives Encore-like intra-app call semantics, but it is hostile to agents and debugging:

```text
user reads source A
compiler runs generated/re-written source B
stack traces and mocks point at wrapper behavior C
```

For v0, I would prefer boring Go semantics:

```go
// Direct Go call = direct Go call.
resp, err := Foo(ctx, req)

// Pulse RPC semantics = explicit generated client.
resp, err := clients.MyService.Foo(ctx, req)
```

If you keep source rewriting, make it transparent and inspectable:

```text
pulse inspect rewrites --json
pulse diff generated
.pulse/gen/rewrite-map.json
```

But my stronger recommendation is to remove the wrapper/direct-call magic before anyone depends on it.

### 4. Defer auto-start DB Studio

DB Studio is useful, but it should not be part of basic app startup. The supervisor currently tries to start it during `Start()` (`cmd/pulse/dev_supervisor.go:193-206`, `cmd/pulse/dev_supervisor.go:690-729`). The dashboard backend also exposes DB query/transaction operations directly (`cmd/pulse/dashboard.go:802-889`).

Change this to explicit commands:

```text
pulse db studio
pulse admin db query --readonly --json
pulse admin db exec --confirm --json
```

For v0, the dashboard can show schema/read-only data only after an explicit DB capability is enabled.

### 5. Narrow the dashboard before it becomes an accidental API

The dashboard server currently serves SPA assets, GraphQL, WS JSON-RPC, an Encore-compatible WS path, dev report ingestion, MCP SSE/message routes, DB operations, API calls, traces, Pub/Sub, onboarding, telemetry, and editor opening from one backend (`cmd/pulse/dashboard.go:45-66`, `cmd/pulse/dashboard.go:333-507`). That is already a platform API, but without a clean boundary.

The dashboard should be a consumer of stable local APIs, not the place where stable local APIs emerge accidentally.

I would make the dashboard read from the same contracts agents use:

```text
pulse inspect app --json
pulse inspect routes --json
pulse inspect traces --json
pulse inspect pubsub --json
pulse admin pubsub clear --json
pulse admin db query --json
```

Then the dashboard backend can remain private and replaceable.

### 6. Remove Encore compatibility leakage

AGENTS.md says strict Pulse naming, no `encore.dev/...`, no `//encore:*`, no compatibility layer (`AGENTS.md:83-87`). But codegen recognizes `encore.dev/pubsub` and `encore.dev/cron` imports (`internal/codegen/generator.go:337-351`, `internal/codegen/generator.go:384-399`). The UI also carries Encore-shaped routes like `envs/local/api` and legacy redirects (`ui/src/router.tsx:46-67`, `ui/src/router.tsx:112-146`), plus “Cloud Dashboard” ghost UI (`ui/src/components/layout.tsx:206-219`).

I would cut compatibility affordances now unless you explicitly decide Pulse is an Encore migration layer. They confuse humans and agents because they imply a broader contract than you intend to support.

---

## The 3–5 public surfaces to freeze early

### 1. `pulse.app`

Freeze this early. It is the root of every other behavior.

Current config has `name`, `id`, `proxy`, and `observability` (`internal/app/root.go:11-35`). I would redesign before freezing:

```json
{
  "name": "example",
  "runtime": {
    "port": 4000
  },
  "dev": {
    "dashboard": true,
    "proxy": false,
    "dbStudio": false,
    "pprof": false
  },
  "observability": {
    "logs": {
      "includeEndpoints": [],
      "excludeEndpoints": []
    },
    "tracing": {
      "includeEndpoints": [],
      "excludeEndpoints": []
    }
  }
}
```

Avoid putting too much infra into `pulse.app` v0. Freeze names, defaults, and feature toggles; leave cloud/deploy concepts out.

### 2. Directive syntax and service model

Freeze:

```go
//pulse:api public|auth|private [raw] [path=/...] [method=GET,POST]
//pulse:service
//pulse:authhandler
```

These are already in AGENTS.md (`AGENTS.md:15-25`). Agents can edit source reliably if directives are simple, local, and declarative. Do not add clever directive aliases, compatibility variants, or hidden defaults.

### 3. CLI command grammar and JSON schemas

This is the most agent-important surface.

Current CLI is hand-rolled and small (`cmd/pulse/main.go:26-90`), which is good, but it lacks machine-readable contracts. Freeze a small command grammar now:

```text
pulse run [--json] [--no-watch] [--dashboard] [--proxy] [--db-studio]
pulse dev [--json] [--proxy] [--db-studio]
pulse check [--json]
pulse test [--json] [go test args...]
pulse build [--json] [-o path]
pulse inspect app|routes|services|graph|build|state|paths --json
pulse admin pubsub clear --json
pulse admin traces clear --json
pulse logs --jsonl
pulse gen client --lang typescript --output path
```

The JSON schemas matter more than the human text. Agents will depend on them.

### 4. Generated artifact layout and manifests

Freeze the generated file layout before generated code conventions spread.

Recommended:

```text
.pulse/
  gen/
    manifest.json
    app.json
    routes.json
    services.json
    openapi.json
    client/
  build/
    latest.json
  state/
    run.json
  logs/
    app.jsonl
```

`manifest.json` should include source hash, Pulse version, Go version, generated files with hashes, service list, endpoint list, feature flags, and build workspace path. No secrets.

Generated Go implementation details should **not** be public. Generated artifact locations and JSON manifests should be.

### 5. User-facing Go packages

Freeze only the packages users should import:

```text
pulse.dev
pulse.dev/auth
pulse.dev/errs
pulse.dev/middleware
pulse.dev/pubsub
pulse.dev/cron
pulse.dev/testkit
```

Do **not** freeze `pulse.dev/runtime` as a user API. It currently exposes registration/config/global runtime pieces (`runtime/registry.go:134-244`) because generated code needs them. Move generated-code-only APIs to something explicit like:

```text
pulse.dev/runtime/wire
```

Document it as generated-code-only. It must be public enough for Go import rules, but not advertised as an application API.

---

## Highest-risk coupling by area

### `cmd/pulse`

`cmd/pulse` is becoming the entire product kernel. `devSupervisor` owns root discovery, build, dashboard store, DB Studio, local proxy, app process lifecycle, status, report tokens, output capture, and metadata (`cmd/pulse/dev_supervisor.go:41-61`). `RebuildAndRestart` mixes parse/build/codegen/compile/start/dashboard notification/status updates in one method (`cmd/pulse/dev_supervisor.go:209-351`).

Refactor around explicit subsystems:

```text
Runner
  owns app process lifecycle

Builder
  owns parse/codegen/build/cache

DevServices
  owns dashboard/proxy/dbstudio

EventBus
  owns structured events for CLI, UI, agents
```

`pulse run --json` should consume the same event stream as the dashboard.

Also, `.env.local` is checked but not loaded: `appEnvWithDotEnv` loads `.env` (`cmd/pulse/dev_supervisor.go:467-491`), while `validateLocalSecretsFiles` merely stats `.env` and `.env.local` (`cmd/pulse/dev_supervisor.go:590-598`). Either load `.env.local` with documented precedence or remove the implication.

### `internal/build`

The uploaded bundle is missing `internal/build/build.go`, but `cmd/pulse/build.go`, `cmd/pulse/check.go`, `cmd/pulse/test.go`, and `cmd/pulse/dev_supervisor.go` all import it. That is a hard review finding: the build package is central enough that it needs to be a formal contract.

I would define this package around a stable build manifest:

```go
type Result struct {
    BuildID        string
    SourceHash     string
    AppRoot        string
    WorkspaceDir   string
    BinaryPath     string
    GeneratedFiles []GeneratedFile
    Diagnostics    []Diagnostic
}
```

And persist it as:

```text
.pulse/build/latest.json
```

Agents should never need to infer the current build workspace by reading temp directories.

Also note that `go.mod` pins `go 1.26.0` (`go.mod:3`). In offline agent environments, toolchain auto-download can fail before source analysis even starts. Either document the exact toolchain requirement or make the repo work cleanly with `GOTOOLCHAIN=local`.

### `internal/codegen`

Main risks:

1. AST mutation and rewritten source (`internal/codegen/generator.go:35-48`, `internal/codegen/generator.go:103-109`).
2. Generated main imports dev-platform runtimeapp (`internal/codegen/generator.go:185-200`).
3. Config, secrets, proxy, DB Studio, and observability are embedded into runtime config (`internal/codegen/generator.go:208-237`).
4. Compatibility detection accepts `encore.dev/...` imports (`internal/codegen/generator.go:337-351`, `internal/codegen/generator.go:384-399`).
5. Secrets population appears in generated early config and registration code (`internal/codegen/generator.go:88-100`, `internal/codegen/generator.go:561-585`), which deserves a single documented initialization path.

Codegen should produce deterministic manifests first, Go glue second. For agents, the JSON model is the contract; generated Go is an implementation detail.

### `runtime`

`runtime.Main` starts dev reporting, service initialization, Pub/Sub, cron, HTTP server, optional standalone dev services, logging, signal handling, and shutdown (`runtime/app.go:23-98`). That is too much for a core runtime.

Split it:

```text
runtime/core
  HTTP server, registry, endpoint invocation, service lifecycle

runtime/features/pubsub
runtime/features/cron
runtime/features/observability

runtime/dev
  reporting hooks, standalone dev integration, pprof/admin
```

The public server should not mount pprof, stats, and Pub/Sub admin endpoints by default (`runtime/server.go:49-140`). Also, the global mock API is convenient but reflection-heavy and global (`runtime/mock.go:24-81`). Move test ergonomics into `pulse.dev/testkit`.

### `internal/localproxy`

The local proxy is currently default-enabled, Caddy-backed, and capable of trust-store behavior (`internal/localproxy/proxy.go:63-77`, `internal/localproxy/proxy.go:136-176`). It also hardcodes frontend discovery around `apps/pulse/vite.config.*` (`internal/localproxy/proxy.go:95-124`), which feels project-specific rather than framework-level.

Make it an explicit dev service:

```text
pulse dev --proxy
pulse proxy status --json
pulse proxy trust
pulse proxy untrust
```

And make routes inspectable:

```json
{
  "enabled": true,
  "trusted": false,
  "routes": {
    "api": "https://api.example.localhost",
    "console": "https://console.example.localhost"
  }
}
```

### `ui`

The UI is already broader than a first local dev dashboard: API Explorer, Service Catalog, Traces, DB Explorer, Pub/Sub, Cron (`ui/src/components/layout.tsx:7-14`), plus ghost nav items like Infra, Flow, Snippets (`ui/src/components/layout.tsx:16-24`) and Cloud Dashboard affordances (`ui/src/components/layout.tsx:206-219`). Router paths also carry legacy/Encore-ish shapes (`ui/src/router.tsx:46-67`, `ui/src/router.tsx:112-146`).

I would narrow v0 UI to three areas:

```text
Status
  app state, compile diagnostics, routes, process/logs

Requests
  API explorer, traces

Data
  DB, Pub/Sub, cron, only if enabled
```

Remove ghost/dead affordances. They make the product feel bigger than it is and create false contracts for agents scraping or navigating the UI.

Also, compile errors are currently rendered as a raw string banner (`ui/src/components/layout.tsx:239-242`). Replace with structured diagnostics from `pulse check --json`: file, line, column, severity, message, suggested action.

---

## Concrete additions for agent-friendliness

### 1. Add `pulse inspect`

This should be the main agent interface.

```text
pulse inspect app --json
pulse inspect routes --json
pulse inspect services --json
pulse inspect graph --json
pulse inspect build --json
pulse inspect state --json
pulse inspect paths --json
pulse inspect generated --json
```

Example:

```json
{
  "schemaVersion": "pulse.inspect.app.v1",
  "app": {
    "name": "billing",
    "root": "/repo"
  },
  "services": [
    {
      "name": "payments",
      "package": "./payments",
      "endpoints": ["Charge", "Refund"]
    }
  ],
  "features": {
    "pubsub": true,
    "cron": false,
    "dbStudio": false,
    "proxy": false
  }
}
```

### 2. Add structured `pulse run --json`

Agents need event streams, not terminal prose.

```jsonl
{"type":"run.start","appRoot":"/repo","addr":"127.0.0.1:4000"}
{"type":"build.start","sourceHash":"..."}
{"type":"build.ok","buildId":"...","binary":"/.../app"}
{"type":"process.start","pid":12345}
{"type":"route.ready","url":"http://127.0.0.1:4000"}
{"type":"log","stream":"stdout","message":"..."}
{"type":"build.error","diagnostics":[{"file":"svc/api.go","line":12,"column":3,"message":"..."}]}
```

This can power both CLI users and the dashboard.

### 3. Add `pulse paths --json`

Every local-first tool eventually needs this.

```json
{
  "schemaVersion": "pulse.paths.v1",
  "appRoot": "/repo",
  "cacheDir": "/Users/me/Library/Caches/pulse/billing",
  "stateDir": "/repo/.pulse/state",
  "generatedDir": "/repo/.pulse/gen",
  "logsDir": "/repo/.pulse/logs",
  "latestBuildManifest": "/repo/.pulse/build/latest.json"
}
```

Agents should never guess where logs, generated files, dashboard state, or build workspaces live.

### 4. Add explicit admin commands

Move hidden UI/backend mutations into stable commands:

```text
pulse admin pubsub status --json
pulse admin pubsub clear --topic <topic> --json
pulse admin traces list --json
pulse admin traces clear --json
pulse admin db query --readonly --json
pulse admin db exec --confirm --json
pulse admin process stop --json
```

This also lets the dashboard dogfood the same API.

### 5. Add `pulse doctor --json`

Useful checks:

```text
pulse.app valid
Go toolchain available
generated dir writable
cache dir writable
port available
proxy disabled/enabled
trust-store status
DB URL present/missing
.env / .env.local load order
stale dashboard process
build package/cache state
```

### 6. Add a real `pulse.dev/testkit`

The current runtime mock layer is global and reflection-based (`runtime/mock.go:24-81`). It is useful, but not enough.

Recommended API shape:

```go
func TestCharge(t *testing.T) {
    app := pulsetest.New(t)

    app.MockEndpoint(payments.GetBalance, func(ctx context.Context, req payments.BalanceReq) (payments.BalanceResp, error) {
        return payments.BalanceResp{Amount: 100}, nil
    })

    resp, err := app.Call(payments.Charge, payments.ChargeReq{Amount: 50})
    require.NoError(t, err)
    require.Equal(t, "ok", resp.Status)
}
```

Properties agents need:

```text
per-test isolation
automatic cleanup
parallel-test safe
typed generated mocks
no hidden global state leaks
deterministic fake clocks for cron
deterministic Pub/Sub test driver
```

### 7. Generate a stable route/service manifest

For example:

```json
{
  "schemaVersion": "pulse.routes.v1",
  "routes": [
    {
      "service": "payments",
      "endpoint": "Charge",
      "access": "auth",
      "raw": false,
      "method": "POST",
      "path": "/payments/charge",
      "requestType": "payments.ChargeRequest",
      "responseType": "payments.ChargeResponse",
      "source": {
        "file": "payments/api.go",
        "line": 24
      }
    }
  ]
}
```

This is better than expecting agents to parse Go comments, dashboard RPC payloads, or generated code.

### 8. Make watch inputs explicit

The watcher currently tracks `pulse.app`, `.go`, `.cpp`, and `.h`, while skipping `encore.gen.go` (`cmd/pulse/watch.go:299-311`). It does not obviously watch `.env`, `.env.local`, `go.mod`, `go.sum`, or other config that affects builds/runtime.

Add:

```text
pulse inspect watch --json
```

And include:

```text
pulse.app
go.mod
go.sum
.env
.env.local
*.go
configured extra files
```

Then agents know whether an edit requires restart, rebuild, or no action.

---

## What to defer or narrow

### Defer

```text
automatic local HTTPS trust
auto-start DB Studio
frontend reverse proxy discovery
Pub/Sub charts
queue clearing UI
pprof-on-public-router
cloud dashboard affordances
MCP as a first-class public surface
Encore compatibility routes/imports
dashboard GraphQL/WS contracts as public APIs
```

### Keep

```text
typed HTTP endpoints
auth/private/public/raw endpoint model
source parser
deterministic codegen
local run/watch
check/test/build/gen client
basic logs/traces
secrets from .env with documented precedence
minimal dashboard status/API explorer
```

### Narrow

```text
Pub/Sub:
  keep declaration/runtime if already working
  move clear/status to admin commands
  defer rich charts

Cron:
  keep simple local runner
  expose inspectable schedule/status
  defer dashboard complexity

Middleware:
  keep if syntax/API is already stable
  document order and generated manifest representation

DB Studio:
  on-demand only
  no startup dependency for pulse run
```

---

## v0 clean-slate roadmap

### Phase 0: Contract reset

Goal: make Pulse boring, deterministic, and inspectable.

Do this first:

```text
1. Split pulse run and pulse dev.
2. Remove runtimeapp import from generated main by default.
3. Make proxy, trust, DB Studio, dashboard explicit dev services.
4. Move app admin/dev endpoints behind supervisor/admin API.
5. Add pulse paths --json.
6. Add pulse inspect app/routes/services/state --json.
7. Add pulse check --json diagnostics.
8. Define .pulse/ generated/state/log layout.
9. Update AGENTS.md into a real v0 SPEC.md.
```

This phase should break APIs freely.

### Phase 1: Agent contract layer

Goal: make agents first-class users.

Add:

```text
pulse run --json event stream
pulse inspect graph/build/generated --json
pulse logs --jsonl
pulse doctor --json
pulse admin pubsub/traces/db/process --json
stable generated manifest schemas
structured compile/runtime diagnostics
single shared .env parser and documented precedence
```

Also add `pulse.dev/testkit` and generated typed mocks.

### Phase 2: Reintroduce rich dev platform as plugins/capabilities

Goal: bring back the nice UX without coupling it to core execution.

Reintroduce:

```text
dashboard consuming inspect/admin APIs
DB Studio on demand
local HTTPS proxy opt-in
Pub/Sub UI
Cron UI
trace explorer
API explorer
MCP bridge over inspect/admin schemas
```

The dashboard should never be the only way to do an operation. Every meaningful operation should have a CLI/JSON equivalent.

---

## My strongest recommendation

Do not freeze the current shape. Freeze a smaller, more boring one.

Pulse’s durable value should be:

```text
local-first Go app model
deterministic generated artifacts
predictable runtime
excellent CLI/JSON introspection
explicit dev services
```

The current code is close to being useful, but the boundaries are inverted: dashboard/proxy/DB Studio/admin behavior is creeping into runtime and generated apps. Invert that now. Make the app runtime small, make dev services explicit, and make machine-readable contracts the source of truth.
