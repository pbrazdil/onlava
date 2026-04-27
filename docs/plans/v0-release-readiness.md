# Pulse v0 Release Readiness

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan follows the standard in [../../PLANS.md](../../PLANS.md). It is based on [../PRD-3-release.md](../PRD-3-release.md), but this file is self-contained so an agent can execute it without prior chat context.

## Purpose / Big Picture

Pulse is close to being a useful local-first runtime, but the current repository mixes stable app runtime behavior with development-platform behavior. The first production-ready release should be intentionally smaller, more boring, and easier to validate.

The goal of this plan is to freeze a reliable v0 contract. Stable v0 should include the app config file, runtime commands, build artifacts, typed/raw HTTP endpoints, auth handler, service initialization and shutdown, private/internal calls, secrets from environment and `.env`, basic logs/traces, and machine-readable CLI outputs. Development conveniences such as dashboard, DB Studio, local HTTPS proxy, trust-store installation, MCP, Pub/Sub UI, cron UI, and Encore migration compatibility should be labeled dev-only, beta, or explicitly compatibility-mode until their contracts are hardened.

The outcome should be observable from a clean checkout. A contributor should be able to run the documented release validation sequence and prove that the CLI builds, tests pass, generated artifacts are deterministic, stable APIs match docs, dev/admin endpoints are not exposed on the public app listener, secrets are not copied into build caches, and release archives do not contain local machine artifacts.

## Progress

- [x] (2026-04-27 16:34Z) Created this ExecPlan from `docs/PRD-3-release.md`.
- [ ] Define the stable v0 surface and mark everything else dev-only, beta, or compatibility-mode.
- [ ] Complete or reference the `pulse dev` / headless `pulse run` split from [devrun-command-split.md](devrun-command-split.md).
- [ ] Fix clean-checkout release reproducibility, including UI embed/build expectations.
- [ ] Move dev/admin/pprof endpoints off the public app router or gate them behind explicit local-only behavior.
- [ ] Make local HTTPS proxy and trust-store installation opt-in.
- [ ] Decide and document Pulse-native versus Encore compatibility behavior.
- [ ] Centralize secrets and `.env` loading rules.
- [ ] Restrict build workspace copying so secrets and local artifacts are not persisted in caches.
- [ ] Add response JSON semantics tests for tags, omitted fields, embedded structs, and custom marshalers.
- [ ] Align CLI usage, docs, schemas, and implementation.
- [ ] Run the release validation sequence and record the results.

## Surprises & Discoveries

No implementation discoveries yet.

Known audit findings from the PRD:

- `pulse run` previously started development supervisor behavior, including dashboard, DB Studio, local HTTPS proxy, MCP, and file watching.
- Generated app binaries could carry dev-platform behavior through `pulse.dev/runtimeapp`.
- `runtime/server.go` mounted dev/admin/platform/pprof endpoints on the app router.
- Local HTTPS proxy and trust-store behavior were enabled by default in development paths.
- The repo had conflicting guidance about strict Pulse-only behavior versus Encore compatibility support.
- The build workspace copied arbitrary app files, which risks copying `.env` and other local files into cache.
- Response encoding did not fully match normal `encoding/json` semantics for tags such as `json:"-"` and `omitempty`.

## Decision Log

- Decision: Freeze a narrow stable v0 contract instead of freezing the whole current feature set.
  Rationale: The runtime, dev supervisor, dashboard, proxy, DB Studio, Pub/Sub, cron, MCP, and migration compatibility are interwoven. A smaller stable surface reduces production risk.
  Date/Author: 2026-04-27 / Codex

- Decision: Treat the command split in [devrun-command-split.md](devrun-command-split.md) as a release-readiness dependency, not a duplicate workstream.
  Rationale: `pulse dev` versus headless `pulse run` is the highest-leverage boundary and already has its own detailed ExecPlan.
  Date/Author: 2026-04-27 / Codex

- Decision: Stable v0 should prefer Pulse-native behavior, with any Encore support made explicit as compatibility mode or migration tooling.
  Rationale: Hidden compatibility makes APIs harder to freeze and contradicts the repository’s strict Pulse naming goal.
  Date/Author: 2026-04-27 / Codex

- Decision: Dev/admin features should not live on the public app listener by default.
  Rationale: Users may bind apps to `0.0.0.0`; exposing pprof, platform stats, Pub/Sub clear, or dev config endpoints there is unsafe.
  Date/Author: 2026-04-27 / Codex

## Outcomes & Retrospective

Not yet completed.

## Context and Orientation

The release-readiness source audit is stored in `docs/PRD-3-release.md`. It recommends not freezing the current feature set as-is. It names the main risk as the mixing of app runtime, development supervisor, dashboard, local HTTPS proxy, DB Studio, Pub/Sub, cron, MCP, and Encore compatibility.

The CLI dispatcher lives in `cmd/pulse/main.go`. The stable commands to freeze for v0 are expected to be `pulse run`, `pulse build`, `pulse check --json`, `pulse inspect ... --json`, `pulse logs --jsonl`, `pulse test`, and `pulse gen client`. `pulse dev` is the development-platform command after the command split.

The current development supervisor lives in `cmd/pulse/dev_supervisor.go`. It owns dashboard, DB Studio, local proxy, MCP/dashboard endpoints, app child process lifecycle, file watching integration, process output capture, and dashboard state.

The file watcher lives in `cmd/pulse/watch.go`. It has historically watched only selected files such as `pulse.app`, `.go`, `.cpp`, and `.h`, which may miss build-affecting files like `go.mod`, `go.sum`, `.env`, and `.env.local`.

The generated runtime entry point and build workspace logic live under `internal/build` and `internal/codegen`. Release readiness depends on deterministic generated artifacts and safe build workspace copying.

The public runtime server lives in `runtime/server.go`. Audit findings say this currently mounts app APIs and dev/admin endpoints on the same listener. The v0 app listener should serve user APIs by default. Dev/admin surfaces should move to a local supervisor listener, a CLI-only path, or an explicitly enabled local admin listener.

The local proxy lives under `internal/localproxy`. It uses embedded Caddy and can install local trust roots. The release contract must make this opt-in and clearly development-only.

The dashboard UI source lives in `ui/` and is embedded by the CLI through `pulse.dev/ui`. A clean release build must either include built `ui/dist` assets, generate them in the release process, or avoid requiring them for headless/stable builds.

Terms used in this plan:

- Stable v0 means the supported behavior that users and agents can rely on without beta labels.
- Dev-only means a feature is useful in `pulse dev` but not part of the production-like runtime contract.
- Beta means a feature can ship but its behavior is not frozen yet.
- Compatibility mode means support for Encore syntax/imports or migration behavior that is explicitly documented and tested rather than accidental.
- Public app listener means the HTTP listener that serves user application endpoints.
- Admin/dev listener means a local-only or explicitly enabled listener for diagnostics, pprof, Pub/Sub controls, dashboard reporting, or platform operations.

## Milestones

Milestone 1 defines the release contract. At the end of this milestone, `docs/local-contract.md`, `AGENTS.md`, command usage, and docs index agree on the stable v0 commands, stable runtime features, beta/dev-only features, and compatibility posture.

Milestone 2 completes the runtime/dev boundary. At the end of this milestone, `pulse dev` owns the development platform and headless `pulse run` starts only the app runtime. This milestone is complete when the acceptance criteria in [devrun-command-split.md](devrun-command-split.md) are satisfied.

Milestone 3 removes release build blockers. At the end of this milestone, a clean checkout can run `go install ./cmd/pulse` without missing embedded assets. Release docs state the required Go version and the Bun/UI build expectations. Release packaging excludes `.DS_Store`, `__MACOSX`, caches, and other local artifacts.

Milestone 4 hardens public-router safety. At the end of this milestone, pprof, `/__pulse/config`, `/__pulse/pubsub/clear`, platform stats, dashboard reporting, and other dev/admin endpoints are not mounted on the public app listener by default. If any remain, they are explicitly gated, documented, tested, and safe for local-only use.

Milestone 5 centralizes configuration and secrets. At the end of this milestone, one loader owns process environment, `.env`, and `.env.local` precedence. Development may warn for missing secrets, while production-like run/build paths fail early for missing required secrets unless explicitly configured otherwise.

Milestone 6 makes build artifacts safe and deterministic. At the end of this milestone, build workspace copying includes only files needed to compile and run the app. `.env`, `.env.*`, `.git`, `.pulse` runtime state, `node_modules`, editor files, caches, and local artifacts are excluded unless explicitly required and documented.

Milestone 7 fixes framework semantics before freeze. At the end of this milestone, response encoding honors expected Go JSON behavior for `json:"-"`, `omitempty`, embedded structs, pointers, headers, `pulse:"httpstatus"`, and custom marshalers, or documents any deliberate custom behavior with tests.

Milestone 8 runs the release gate. At the end of this milestone, the full release validation checklist passes and `Outcomes & Retrospective` records exact command results.

## Plan of Work

Start by updating the release contract before changing behavior. The repo should have one canonical local contract that says what is stable, what is beta, what is dev-only, and what is compatibility-mode. Use `docs/local-contract.md` as the canonical document, and keep `cmd/pulse/main.go` usage text aligned with it.

Next, finish the command split work tracked by [devrun-command-split.md](devrun-command-split.md). Do not make other release-hardening work depend on a dev supervisor hidden inside `pulse run`.

Then handle clean-checkout reproducibility. Verify whether `ui/dist` and other embedded assets are required for `go install ./cmd/pulse`. If they are required, choose one explicit release strategy: commit built assets, generate them in release packaging, or move embedding behind a development build boundary. The strategy must be documented and validated from a clean checkout.

After reproducibility, audit runtime routes. Move dev/admin endpoints out of `runtime/server.go` public routing by default. If dashboard or development reporting needs endpoints, keep them on the dashboard/supervisor server. If pprof is needed, expose it only through an explicit local admin mode.

Then make local HTTPS proxy and trust installation opt-in. `pulse dev` may support `--proxy` and a separate explicit trust flag. Do not surprise users by mutating system trust stores. `pulse run` should never install trust roots.

Next, decide Encore compatibility. Either remove compatibility from stable paths for v0 or label it explicitly and add tests/docs. Relevant areas include directive parsing, import rewriting, generated compatibility aliases, dashboard branding rewrites, and public symbols that still mention Encore.

Then centralize `.env` and secrets loading. Replace duplicate parsers and loaders in runtime, supervisor, DB Studio, and tests with one package-level implementation. Document precedence and mode-specific missing-secret behavior.

Then lock down build workspace copying. Replace broad file copying with an allowlist plus explicit asset inclusion behavior. Add tests that prove `.env`, `.env.local`, `.git`, `.pulse`, `node_modules`, `.DS_Store`, and `__MACOSX` are excluded.

Finally, add the response encoding tests and fix semantics before declaring the runtime contract stable.

## Concrete Steps

Work from the repository root:

    cd /Users/petrbrazdil/Repos/pulse

List current command and runtime boundary references:

    rg -n "case \"run\"|case \"dev\"|runCommand|devCommand|runWithWatch|newDevSupervisor|runtimeapp|localproxy|pprof|platform.Stats|pubsub/clear|__pulse/config" cmd internal runtime

Check clean checkout build assumptions:

    go install ./cmd/pulse
    test -f ui/dist/index.html
    test -f dbstudio/dist/index.html

Update the canonical contract:

    $EDITOR docs/local-contract.md AGENTS.md docs/index.md

Use the command split plan:

    $EDITOR docs/plans/devrun-command-split.md

Audit public runtime routes:

    rg -n "__pulse/config|pubsub/clear|platform.Stats|debug/pprof|Access-Control-Allow-Origin|Access-Control-Allow-Credentials" runtime cmd internal

Audit Encore compatibility:

    rg -n "encore|Encore|encore.dev|//encore" --glob '!encore/**' --glob '!cmd/pulse/devdash_static/**'

Audit build workspace copying:

    rg -n "WalkDir|isSourceFile|copy|\\.env|node_modules|DS_Store|__MACOSX" internal/build cmd internal

Audit secrets loaders:

    rg -n "LoadDotEnv|\\.env|secrets|DatabaseURL|PULSE_ENV|PULSE_MODE" cmd internal runtime

Add or update tests for the release blockers:

    go test ./cmd/pulse ./internal/build ./runtime

Run the full validation gate:

    go test ./...
    go install ./cmd/pulse
    pulse inspect docs --json --repo-root /Users/petrbrazdil/Repos/pulse
    pulse harness self --json --write

If `/Users/petrbrazdil/Repos/onlv` is used, keep it read-only:

    go -C /Users/petrbrazdil/Repos/pulse run ./cmd/pulse inspect app --json --app-root /Users/petrbrazdil/Repos/onlv
    go -C /Users/petrbrazdil/Repos/pulse run ./cmd/pulse check --json --app-root /Users/petrbrazdil/Repos/onlv

Record exact command results in `Outcomes & Retrospective` before marking this plan complete.

## Validation and Acceptance

Release readiness is accepted when all of these are true:

- A clean checkout can run `go install ./cmd/pulse`.
- `go test ./...` passes.
- `go test -race ./...` has either passed or has documented exclusions for tests where race mode is impractical.
- `pulse harness self --json --write` passes.
- `pulse inspect docs --json --repo-root /Users/petrbrazdil/Repos/pulse` reports no missing or stale documents.
- CLI usage text and `docs/local-contract.md` describe the same commands.
- `pulse version --json` exists or the lack of version command is explicitly deferred before release.
- Stable, beta, dev-only, and compatibility-mode features are labeled in docs.
- `pulse run` is headless and production-like.
- `pulse dev` owns dashboard, DB Studio, local HTTPS proxy, frontend proxy, MCP, file watching, and development-only UI.
- The public app listener does not expose pprof, platform stats, Pub/Sub clear, dashboard report endpoints, or arbitrary credentialed CORS by default.
- Local HTTPS proxy and trust-store installation are opt-in.
- `.env`, `.env.local`, `.git`, `.pulse` runtime state, `node_modules`, `.DS_Store`, and `__MACOSX` are not copied into build workspaces or release archives.
- Response encoding tests cover `json:"-"`, `omitempty`, embedded structs, pointer fields, header fields, `pulse:"httpstatus"`, and custom marshalers.
- The release validation sequence is documented in `Outcomes & Retrospective`.

## Idempotence and Recovery

This plan should be executed in small, independently testable slices. Each milestone should leave the repo buildable. If a risky change fails, revert only that change and keep completed hardening work.

Do not delete development functionality while moving it behind `pulse dev`. The recovery path for a broken command split is to keep `pulse dev` on the existing supervisor path and continue narrowing `pulse run` separately.

Do not silently remove Encore compatibility while app migrations still depend on it. If compatibility is removed from stable v0, provide a documented transition path or explicit compatibility mode.

When changing build workspace copying, expect some apps to rely on embedded assets. Preserve required app assets through explicit inclusion rules or clear diagnostics rather than broad copying.

When moving dev/admin endpoints, keep local dashboard functionality working by routing dashboard traffic through the supervisor/dashboard server instead of the public app listener.

## Artifacts and Notes

Primary source:

    docs/PRD-3-release.md

Related active ExecPlan:

    docs/plans/devrun-command-split.md

Validation artifact:

    .pulse/harness/self-latest.json

Release-stable generated artifacts:

    .pulse/gen/app.json
    .pulse/gen/routes.json
    .pulse/gen/services.json
    .pulse/gen/manifest.json
    .pulse/build/latest.json

Stable v0 candidates:

    pulse.app
    pulse run
    pulse build
    pulse check --json
    pulse inspect ... --json
    pulse logs --jsonl
    pulse test
    pulse gen client
    typed/raw HTTP endpoints
    auth handler
    service struct initialization and shutdown
    private/internal calls
    secrets from env/.env
    basic traces/logs

Dev-only or beta candidates:

    dashboard
    DB Studio
    local HTTPS proxy
    trust-store installation
    MCP server
    Pub/Sub UI
    cron UI
    Encore migration compatibility
    source rewrite/direct-call behavior unless made inspectable

## Interfaces and Dependencies

No new external dependency is expected for this plan. Prefer the Go standard library and existing Pulse packages.

Expected CLI interfaces to freeze:

    pulse dev [development flags]
    pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [--env <name>] [--log-format text|json]
    pulse build [--app-root <path>] [-o <path>] [--db-studio]
    pulse check --json [--app-root <path>]
    pulse inspect app|routes|services|endpoints|wire|build|paths|traces|metrics|docs --json
    pulse logs --jsonl [--app-root <path>]
    pulse test [--app-root <path>] [go test flags/packages...]
    pulse gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]

Expected documentation interfaces:

    docs/local-contract.md
    docs/index.md
    docs/knowledge.json
    docs/plans/active.md
    docs/tech-debt.md

Expected implementation areas:

    cmd/pulse/main.go
    cmd/pulse/watch.go
    cmd/pulse/dev_supervisor.go
    cmd/pulse/build.go
    runtime/server.go
    runtime/secrets.go
    runtime/encode.go
    runtime/decode.go
    internal/build
    internal/codegen
    internal/localproxy
    internal/dbstudio
