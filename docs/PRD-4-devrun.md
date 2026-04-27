Yes. That is the right boundary.

I would define the product like this:

pulse dev
  Development experience.
  Interactive, forgiving, feature-rich.
pulse run
  Production/runtime experience.
  Headless, deterministic, minimal, safe.
pulse build
  Deployment artifact creation.
  Produces the thing you actually ship.

The important nuance: pulse run can be production-grade, but pulse build should still be the preferred production deployment primitive. In other words, production should usually be:

pulse build
./dist/my-app

or inside a container:

RUN pulse build --out /app/server
CMD ["/app/server"]

But pulse run should be safe enough that this is also reasonable for simpler deployments:

pulse run

provided it does not start dev-only systems.

Recommended command split

pulse dev

This should become what current pulse run mostly is today.

It can include:

file watching
automatic rebuild/restart
dashboard
API explorer
traces UI
DB Studio
local HTTPS proxy
frontend proxy
MCP server
local Pub/Sub controls
local cron controls
pretty logs
.env and .env.local loading
relaxed local-only defaults

Example:

pulse dev
pulse dev --dashboard
pulse dev --proxy
pulse dev --db-studio
pulse dev --frontend http://localhost:5173

This command may be magical. That is fine. Developers expect convenience here.

pulse run

This should run the application, not the development platform.

It should not include by default:

dashboard
DB Studio
local HTTPS proxy
trust-store installation
MCP
frontend proxy
file watching
admin UI
pprof
debug endpoints
open WebSocket origins
relaxed credentialed CORS

It should include:

one deterministic app startup
production-like config loading
strict secret validation
structured logs
graceful shutdown
health/readiness support
stable exit codes
signal handling
PORT/listen support
no mutation of local machine trust stores

Example:

pulse run
pulse run --listen :8080
pulse run --env production
pulse run --log-format json

I would avoid this:

pulse run --dashboard
pulse run --watch
pulse run --db-studio

Those should be pulse dev concerns.

What I would do to the current implementation

The current behavior should effectively be renamed:

current pulse run  ->  pulse dev
new pulse run      ->  headless runtime command

That means the current dashboard/proxy/supervisor startup path should move behind pulse dev.

From the previous audit, these are the key implementation changes:

cmd/pulse/watch.go
  likely becomes the basis for pulse dev, not production pulse run
cmd/pulse/dev_supervisor.go
  should only be used by pulse dev
runtimeapp/app.go
  should not cause generated production app binaries to start dev services
runtime/app.go
  should not automatically start standalone dev services in production mode
internal/localproxy
  should only be reachable through pulse dev --proxy or similar
internal/dbstudio
  should only be reachable through pulse dev / dashboard
runtime/server.go
  should not mount dev/admin/pprof endpoints on the public app router by default

My preferred final CLI contract

I would make the stable contract something like this:

pulse dev

Starts the full local developer environment.

pulse run

Runs the app in production-like mode from the current project. It may compile once, then run. No dashboard. No proxy. No watching.

pulse build

Builds a deployable artifact.

pulse check --json

Validates the app without running it.

pulse inspect routes --json
pulse inspect services --json
pulse inspect config --json

Machine-readable introspection.

pulse logs --jsonl

Structured log stream for automation.

pulse test

Runs Pulse-aware tests.

pulse gen client

Generates clients.

Should pulse run be used directly in production?

I would support it, but not make it the only recommended path.

There are two valid production models.

Model A: build artifact, then run binary

Best for serious deployments.

pulse build --out dist/server
dist/server

This is the cleanest production story because the runtime machine does not need the full Pulse CLI, source tree, dashboard assets, or build toolchain.

Model B: pulse run in production

Acceptable for simple platforms if pulse run is truly headless.

pulse run --listen :8080

But this only works if pulse run has these guarantees:

does not start dev services
does not install certificates
does not open dashboard ports
does not watch files
does not require Bun/UI assets
does not mutate source tree unexpectedly
does not copy .env into build caches
fails fast on missing required secrets
honors SIGTERM/SIGINT correctly
exits with meaningful non-zero codes

If those guarantees are not true, then pulse run should not be marketed as production-ready yet.

Suggested modes

I would avoid relying only on environment variables, but support them.

pulse dev
# implies PULSE_MODE=development
pulse run
# implies PULSE_MODE=production-like or runtime
pulse run --env production
# strict production behavior
pulse run --env staging
# production behavior, staging config

Environment variables can still override behavior:

PULSE_ENV=production pulse run
PULSE_LOG_FORMAT=json pulse run
PORT=8080 pulse run

But command choice should carry the main meaning.

Strong recommendation

Do not make pulse run mean “dev server with production option.”

Make it mean:

run the app

And make pulse dev mean:

run the app plus the local development platform

That gives you a much cleaner first production-ready release. It also makes the feature freeze easier:

Stable:
  pulse run
  pulse build
  pulse check
  pulse inspect
  pulse logs
  pulse test
  core runtime
Beta/dev:
  pulse dev dashboard
  DB Studio
  local HTTPS proxy
  MCP
  Pub/Sub UI
  cron UI

So yes: pulse dev for development including dashboard, DB Studio, proxy, etc.; pulse run for production-like app execution; pulse build for the artifact you deploy.
