# Browser UI Harness

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

scenery should make dashboard behavior legible to agents through a browser harness. The self-harness currently checks Go, schemas, docs, and static UI architecture. It does not yet open the dashboard in a browser, assert DOM markers, collect screenshots, capture console errors, or report failed network requests.

The goal is:

```text
scenery harness ui --json
        |
        v
start/reuse local app + dashboard
        |
        v
visit core routes
        |
        v
return screenshots, console logs, network failures, DOM marker status
```

## Progress

- [x] (2026-05-09) Created this ExecPlan and linked it from `docs/plans/active.md`.
- [x] (2026-05-09) Decided on `chromedp` for the first browser runner.
- [x] (2026-05-09) Added `scenery harness ui --json`.
- [x] (2026-05-09) Added core dashboard route visits for home, API Explorer, service catalog, traces, Data Explorer, and DB Explorer.
- [x] (2026-05-09) Captured route screenshots under `.scenery/harness/ui/screenshots/`.
- [x] (2026-05-09) Captured console errors and failed network requests as JSONL artifacts.
- [x] (2026-05-09) Emitted `scenery.harness.ui.v1` JSON.
- [x] (2026-05-09) Added schema, local-contract docs, and command tests.

## Surprises & Discoveries

- The dashboard already has stable `AppShell` and `DataExplorerLayout` markers, so the first browser harness can verify useful route health without adding route-specific markers everywhere.
- The harness can use a temporary `scenery dev --json` subprocess with an isolated dashboard port. That keeps the command app-facing without wiring browser checks into the long-lived dev supervisor.

## Decision Log

- Decision: use `chromedp` rather than Playwright or a shell wrapper for the first pass.
  Rationale: it keeps the browser runner inside Go, captures screenshots/console/network events directly, and avoids introducing a Node-side browser test harness into the CLI path.
- Decision: keep `scenery harness ui --json` out of the default `scenery harness self --json --write` path.
  Rationale: browser discovery is heavier and more environment-sensitive than static and build checks; users and agents should invoke it explicitly.
- Decision: allow `--dashboard-url` for reuse of an existing dashboard.
  Rationale: it makes debugging a running local session fast and avoids restarting apps when the dashboard is already up.

## Outcomes & Retrospective

The first UI harness is intentionally narrow but real: it can start or reuse a dashboard, visit core routes in a browser, assert DOM markers, capture screenshots, record console errors and network failures, and return a versioned JSON result. Future work can add richer route-specific markers and visual diffs without changing the command shape.

## Context and Orientation

Relevant files:

```text
PLAN.md
docs/local-contract.md
cmd/scenery/*harness*
ui/src/components/layouts/*
ui/src/routes/*
docs/schemas/*
```

`PLAN.md` identifies Browser and UI Harness as a major agent-first capability.

## Decision Constraints

Keep dependencies minimal. If adding Playwright or another browser dependency, document the concrete payoff.

Possible approaches:

```text
Option A: Playwright dependency
  Pros: robust browser automation, screenshots, console/network events.
  Cons: dependency weight.

Option B: chromedp
  Pros: Go-native.
  Cons: browser install/discovery complexity.

Option C: external browser command wrapper
  Pros: simple start.
  Cons: less portable.
```

Record the decision before implementation.

## Interfaces and Dependencies

- Add a versioned `scenery.harness.ui.v1` JSON surface.
- Keep the command separate from default `scenery harness self` until the browser path is proven reliable.
- If a browser automation dependency is added, record the rationale and update dependency checks.
- Reuse dashboard DOM markers from scenery layouts.

## Plan of Work

Define the JSON contract first, then add the CLI command and a minimal route runner. Capture screenshots, console errors, and failed network requests only after the command can reliably start or connect to a dashboard.

## Milestones

### Milestone 1: JSON schema

Add:

```text
docs/schemas/scenery.harness.ui.v1.schema.json
```

Shape:

```json
{
  "schema_version": "scenery.harness.ui.v1",
  "ok": true,
  "routes": [],
  "artifacts": [],
  "diagnostics": []
}
```

### Milestone 2: CLI command

Add:

```sh
scenery harness ui --json [--repo-root <path>] [--app-root <path>] [--headed]
```

### Milestone 3: Route checks

Visit:

```text
dashboard home
API Explorer
service catalog
traces
data explorer
DB Explorer route
```

Assert stable DOM markers:

```text
data-scenery-ui="AppShell"
data-scenery-ui="DashboardPage"
data-scenery-ui="DataExplorerLayout"
```

### Milestone 4: Artifacts

Write artifacts under:

```text
.scenery/harness/ui/
  screenshots/
  console.jsonl
  network.jsonl
```

### Milestone 5: Optional self-harness integration

Do not add browser harness to the default fast self-harness immediately. Add optional wiring or document how to run it.

## Concrete Steps

1. Add the JSON schema and local-contract entry.
2. Add `scenery harness ui --json` CLI parsing.
3. Implement dashboard URL discovery or startup rules.
4. Visit the first route and assert a DOM marker.
5. Add screenshots and console/network collection.
6. Add tests around JSON output and failure diagnostics.

## Validation and Acceptance

```sh
go test ./cmd/scenery
go install ./cmd/scenery
scenery harness ui --json --app-root testdata/apps/basic
scenery harness self --json --write
```

Acceptance criteria:

```text
- broken dashboard route fails with route name and screenshot path
- console errors are reported
- failed network requests are reported
- JSON schema is documented
- command is not part of default heavy path unless explicitly enabled
```

## Idempotence and Recovery

The harness should write artifacts under a deterministic `.scenery/harness/ui/` path and may overwrite previous local UI harness artifacts. A failed route check should not leave child processes running.

## Artifacts and Notes

Expected artifacts include `docs/schemas/scenery.harness.ui.v1.schema.json`, command tests, screenshots, console/network logs, and documentation updates.
