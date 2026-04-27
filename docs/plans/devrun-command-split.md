# Split `pulse dev` From Headless `pulse run`

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan follows the standard in [../../PLANS.md](../../PLANS.md). It is based on [../PRD-4-devrun.md](../PRD-4-devrun.md), but this file is self-contained so an agent can execute it without prior chat context.

## Purpose / Big Picture

Pulse currently treats `pulse run` as the local development command. It starts the app plus development-only systems such as the dashboard, local HTTPS proxy, frontend proxy, MCP server, DB Studio, file watching, rebuild/restart supervision, and relaxed local defaults. That is convenient for development, but it makes `pulse run` ambiguous and risky as the production-like command.

After this change, developers get a clear command split. `pulse dev` starts the full local development platform. `pulse run` starts the application in a headless, production-like mode with one deterministic startup, no dashboard, no proxy, no file watching, no DB Studio, no local certificate mutation, strict signal handling, and predictable logs. `pulse build` remains the preferred deployment artifact path.

The result is observable from the command line. Running `pulse dev --app-root /path/to/app` should show the existing development URLs and live-reload behavior. Running `pulse run --app-root /path/to/app --listen :8080` should only start the app listener and should not bind dashboard/proxy/DB Studio ports or require frontend dashboard assets.

## Progress

- [x] (2026-04-27 16:12Z) Created this ExecPlan from `docs/PRD-4-devrun.md`.
- [x] (2026-04-27 14:24Z) Inventory current `pulse run` behavior and tests that assume development behavior.
- [x] (2026-04-27 14:24Z) Add the `pulse dev` command as an alias for the current development supervisor path.
- [x] (2026-04-27 14:25Z) Implement a new headless `pulse run` path that builds once and starts the app without dev services.
- [x] (2026-04-27 14:25Z) Update generated app/runtime behavior so built binaries and headless run do not start dev-only services by default.
- [x] (2026-04-27 14:29Z) Update CLI docs, local contract, help text, and tests.
- [x] (2026-04-27 14:31Z) Validate against fixture apps and `/Users/petrbrazdil/Repos/onlv` in read-only mode.

## Surprises & Discoveries

- Existing generated mains imported `pulse.dev/runtimeapp` unconditionally, so built app binaries could start local proxy/DB Studio behavior outside the CLI. Headless behavior required changing codegen, not only command dispatch.
- The integration suite had several development-platform expectations under `pulse run`, especially reloads, dashboard/MCP, and HTTPS proxy hostnames. Those tests now belong to `pulse dev`.
- Headless `pulse run` still needs a parent-death monitor for its app child, otherwise force-killing the CLI can leave an orphaned app process.

## Decision Log

- Decision: `pulse dev` owns dashboard, file watching, local HTTPS proxy, frontend proxy, MCP, DB Studio, API explorer, traces UI, local Pub/Sub controls, cron controls, and pretty development logs.
  Rationale: These are development-platform features. Keeping them out of `pulse run` makes the runtime command safe and predictable.
  Date/Author: 2026-04-27 / Codex

- Decision: `pulse run` should be production-like and headless, but `pulse build` remains the preferred deployment primitive.
  Rationale: A built binary is the cleanest production artifact because the deployment machine does not need the Pulse CLI, source tree, dashboard UI, Bun, or build tooling.
  Date/Author: 2026-04-27 / Codex

- Decision: The initial migration should preserve today’s developer behavior through `pulse dev` before changing `pulse run`.
  Rationale: This avoids breaking the current local workflow while making room for the new runtime contract.
  Date/Author: 2026-04-27 / Codex

- Decision: Headless app children get a parent monitor without being marked as dev-supervisor-launched.
  Rationale: `pulse run` should still clean up its app child if the CLI parent dies, while preserving headless banners and avoiding dev reporting/proxy behavior.
  Date/Author: 2026-04-27 / Codex

- Decision: `runtimeapp` local proxy startup is gated behind explicit standalone development mode.
  Rationale: `pulse build --db-studio` may intentionally include runtimeapp for DB Studio, but that should not implicitly bring back the local HTTPS/frontend proxy.
  Date/Author: 2026-04-27 / Codex

## Outcomes & Retrospective

- `pulse dev` now owns the previous development supervisor path, including dashboard, MCP, proxy, DB Studio, watching, rebuilds, and JSONL development events.
- `pulse run` now builds once and starts the app binary headlessly with development-only flags rejected.
- Generated app mains no longer import `pulse.dev/runtimeapp` by default, so `pulse build` outputs are headless unless DB Studio is explicitly enabled.
- Validation passed: focused command/codegen/runtime tests, selected fixture integration tests, `go test ./...`, `go install ./cmd/pulse`, `pulse harness self --json --write`, and read-only `pulse inspect app --json --app-root /Users/petrbrazdil/Repos/onlv`.

## Context and Orientation

The command dispatcher lives in `cmd/pulse/main.go`. Today it recognizes `run`, `build`, `psql`, `check`, `harness`, `inspect`, `admin`, `logs`, `test`, and `gen`. The current `runCommand` parses `--port`, `--listen`, `--app-root`, `--verbose`, and `--json`, then calls `runWithWatch`.

The current development loop lives in `cmd/pulse/watch.go`. `runWithWatch` discovers the app root from `pulse.app`, installs signal handling, starts a parent monitor, scans watched files, creates a `devSupervisor`, starts it, performs the initial build/restart, then watches files and rebuilds on changes.

The development supervisor lives in `cmd/pulse/dev_supervisor.go`. It owns the app child process, dashboard server, WebSocket/MCP/report endpoints, SQLite dev state, local proxy, DB Studio, rebuild notifications, process output capture, app metadata, API explorer calls, and dashboard status.

The dashboard server lives primarily in `cmd/pulse/dashboard.go`. It should remain a development feature behind `pulse dev`.

The local HTTPS and frontend reverse proxy code lives under `internal/localproxy` and is started from the development supervisor. It should not run from headless `pulse run`.

The DB Studio integration lives under `internal/dbstudio` and is started from the development supervisor. It should not run from headless `pulse run` unless a future explicit production flag is added and documented.

The build command lives in `cmd/pulse/build.go`. It already produces a deployable binary and supports `--db-studio`. This remains the preferred production deployment path.

The generated runtime entry point is created by the build pipeline under `internal/build` and codegen packages. Generated app binaries must continue to serve app endpoints, cron, Pub/Sub, runtime logs, and graceful shutdown behavior, but they must not implicitly start dashboard/proxy/dev-only systems.

Terminology used in this plan:

- Development platform means the convenience systems around the app: dashboard, proxy, API explorer, traces UI, MCP, DB Studio, live reload, and local controls.
- Headless runtime means only the app server and runtime primitives needed by the app itself, without browser UI, local machine certificate management, or file watching.
- Dev supervisor means the parent process in `cmd/pulse/dev_supervisor.go` that manages development services and the app child process.

## Milestones

Milestone 1 preserves current behavior under `pulse dev`. At the end of this milestone, `pulse dev` should accept the same flags as today’s `pulse run` and call the existing `runWithWatch` path. `pulse run` can still temporarily behave as before until Milestone 2 lands. The acceptance proof is `pulse dev --app-root /Users/petrbrazdil/Repos/onlv` starting the dashboard/proxy/app exactly like current `pulse run`.

Milestone 2 introduces a headless runtime path for `pulse run`. At the end of this milestone, `pulse run` should compile once and start the generated app binary without starting `devSupervisor`, dashboard, local proxy, DB Studio, MCP, file watching, or dashboard UI package installation. The acceptance proof is that `pulse run --app-root <fixture> --listen 127.0.0.1:4080` serves endpoints and exits cleanly on SIGINT/SIGTERM while dashboard port `9401`, DB Studio port `4002`, and local HTTPS proxy domains are not bound by Pulse.

Milestone 3 hardens production-like behavior. At the end of this milestone, `pulse run` should support strict secret validation, stable exit codes, structured log options, `PORT`/listen behavior, and health/readiness behavior if those primitives exist. The acceptance proof is a test fixture and a documented command transcript showing missing required secrets fail fast in production mode while local development remains forgiving under `pulse dev`.

Milestone 4 updates docs, inspect output, and harness coverage. At the end of this milestone, `AGENTS.md`, `docs/local-contract.md`, `docs/index.md`, command usage, and tests describe the new split. The acceptance proof is `pulse harness self --json --write` passing and `pulse inspect docs --json` showing no stale or missing docs.

## Plan of Work

First, add command plumbing in `cmd/pulse/main.go`. Add a new `dev` case that calls a new `devCommand` function. Rename or wrap the existing `runCommand` behavior so the file-watching path is owned by `devCommand`. Update usage text to show `pulse dev` as the development command and `pulse run` as the headless runtime command.

Second, preserve backward compatibility intentionally during the transition. If changing `pulse run` immediately would break tests or workflows, add clear transitional tests that document the old behavior only where needed, then remove or update them before completing this plan. Do not leave `pulse run` as a hidden alias for `pulse dev` at the end of the plan.

Third, implement a headless app runner. This runner should discover the app root, load `pulse.app`, call the existing build pipeline once, and start the compiled app binary directly. It should reuse the existing process lifecycle helpers where safe, but it must not instantiate `devSupervisor`, dashboard server, dashboard store, local proxy, DB Studio, file watcher, or MCP server. It should forward stdout/stderr to the terminal, propagate SIGINT/SIGTERM to the child process, enforce shutdown timeouts, and return the child exit code as a meaningful CLI error.

Fourth, audit generated app startup. Ensure generated binaries only start app runtime services. If development services are currently triggered by generated runtime flags or environment variables, gate them behind explicit development mode injected only by `pulse dev`.

Fifth, split flags. `pulse dev` keeps current flags such as `--port`, `--listen`, `--app-root`, `--verbose`, and `--json`. `pulse run` supports `--listen`, `--port`, `--app-root`, `--env`, and `--log-format` if implemented in this milestone. Avoid adding `--dashboard`, `--watch`, `--db-studio`, or proxy flags to `pulse run`; those belong to `pulse dev`.

Sixth, update tests. Existing tests for parseRunArgs should be split so `parseDevArgs` covers the current dev flags and `parseRunArgs` covers the new headless flags. Add integration-style tests that assert `pulse dev` calls the watcher path and `pulse run` does not. Where practical, test behavior through function seams rather than spawning long-running processes.

Seventh, update docs and generated contracts. `docs/local-contract.md` must describe `pulse dev`, headless `pulse run`, and `pulse build`. `AGENTS.md` should prefer `pulse dev` for local development and `pulse run` for production-like execution. If inspect/build artifacts expose command metadata, update them only if there is a concrete schema change.

## Concrete Steps

Work from the repository root:

    cd /Users/petrbrazdil/Repos/pulse

Inspect the current command path:

    rg -n "case \"run\"|runCommand|runWithWatch|newDevSupervisor" cmd/pulse

Add `devCommand` in `cmd/pulse/main.go` or a new `cmd/pulse/dev.go`. Move today’s `runCommand` behavior to `devCommand`:

    pulse dev [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json]

Create a new headless `runCommand` implementation:

    pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]

The exact `--env` and `--log-format` flags may land in a later milestone if the first headless runner keeps scope smaller, but the parser should reject dev-only flags such as `--dashboard`, `--watch`, `--db-studio`, and proxy flags.

Add or update tests:

    go test ./cmd/pulse -run 'TestParse.*Run|TestParse.*Dev|TestRun'

Run the full validation:

    go test ./...
    go install ./cmd/pulse
    pulse harness self --json --write

Smoke-test development behavior against the onlv app without modifying it:

    go -C /Users/petrbrazdil/Repos/pulse run ./cmd/pulse dev --app-root /Users/petrbrazdil/Repos/onlv

Expected observation: the command starts the development dashboard, local URLs, file watching, and pretty development logs as today’s `pulse run` does. Stop it with Ctrl+C and confirm it exits.

Smoke-test headless behavior against the onlv app without modifying it:

    go -C /Users/petrbrazdil/Repos/pulse run ./cmd/pulse run --app-root /Users/petrbrazdil/Repos/onlv --listen 127.0.0.1:4080

Expected observation: the command serves the app on `127.0.0.1:4080`, does not print dashboard/proxy/DB Studio URLs, does not start file watching, and exits on Ctrl+C. If the app requires external services or secrets, use a smaller fixture app for the acceptance test and record the onlv limitation in `Surprises & Discoveries`.

## Validation and Acceptance

The command contract is accepted when these behaviors are true:

- `pulse dev` starts the development platform: app server, dashboard, MCP endpoint, local proxy if configured, DB Studio when `DatabaseURL` is configured, file watching, rebuild/restart, and pretty development logs.
- `pulse run` starts only the app runtime and app primitives. It does not bind dashboard port `9401`, DB Studio port `4002`, local proxy ports, or frontend proxy domains.
- `pulse run` does not install or trust certificates.
- `pulse run` does not require Bun, `ui/dist`, `dbstudio/dist`, or dashboard assets.
- `pulse run` does not watch files or rebuild after startup.
- `pulse run` handles SIGINT and SIGTERM reliably and exits after the child process exits.
- `pulse build` behavior remains unchanged except for documentation clarifying it as the preferred deployment artifact command.
- `go test ./...` passes.
- `go install ./cmd/pulse` succeeds and the installed `pulse` binary is fresh.
- `pulse harness self --json --write` succeeds.

Expected help text should include:

    pulse dev [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json]
    pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]
    pulse build [--app-root <path>] [-o <path>] [--db-studio]

## Idempotence and Recovery

All changes should be additive until tests prove the split works. Keep the existing development path callable while adding `pulse dev`, then move `pulse run` to the new path after `pulse dev` is verified.

If the headless runner fails during implementation, leave `pulse dev` working and document the failure in `Surprises & Discoveries`. Do not remove existing dev-supervisor behavior until the replacement command is verified.

If local ports are left occupied during manual testing, use the existing process-lifecycle tests and startup guards as references. Avoid requiring manual `kill -9` as part of normal operation.

If `/Users/petrbrazdil/Repos/onlv` cannot run because of local environment requirements, switch to a minimal fixture app for automated acceptance and note the onlv limitation here.

## Artifacts and Notes

Source PRD summary:

    pulse dev is the interactive development experience.
    pulse run is the headless production-like runtime command.
    pulse build produces the deployment artifact.

Current implementation fact:

    cmd/pulse/main.go runCommand currently calls runWithWatch.
    cmd/pulse/watch.go runWithWatch currently creates newDevSupervisor.
    cmd/pulse/dev_supervisor.go currently owns dashboard, proxy, DB Studio, MCP, rebuilds, and app child lifecycle.

Validation artifacts should be written by:

    pulse harness self --json --write

The expected artifact is:

    .pulse/harness/self-latest.json

## Interfaces and Dependencies

No new external dependency is expected. Use the Go standard library and existing Pulse packages.

Expected public CLI interface:

    pulse dev [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json]
    pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]
    pulse build [--app-root <path>] [-o <path>] [--db-studio]

Expected internal interfaces may include:

    func devCommand(args []string) error
    func parseDevArgs(args []string) (devOptions, error)
    func runCommand(args []string) error
    func parseRunArgs(args []string) (runOptions, error)
    func runHeadless(addr string, opts runOptions) error

Keep `devSupervisor` private to the development command. The headless runner should not import or instantiate dashboard, DB Studio, MCP, or local proxy types.
