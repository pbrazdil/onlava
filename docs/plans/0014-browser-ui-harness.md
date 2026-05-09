# Browser UI Harness

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

onlava should make dashboard behavior legible to agents through a browser harness. The self-harness currently checks Go, schemas, docs, and static UI architecture. It does not yet open the dashboard in a browser, assert DOM markers, collect screenshots, capture console errors, or report failed network requests.

The goal is:

```text
onlava harness ui --json
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

- [ ] Create this ExecPlan and link from `docs/plans/active.md`.
- [ ] Decide browser runner approach.
- [ ] Add `onlava harness ui --json`.
- [ ] Visit dashboard home and core routes.
- [ ] Capture screenshots.
- [ ] Capture console errors and failed network requests.
- [ ] Emit versioned JSON.
- [ ] Add docs/schema/tests.

## Surprises & Discoveries

Record discoveries here.

## Decision Log

Record decisions here.

## Outcomes & Retrospective

Fill when complete.

## Context and Orientation

Relevant files:

```text
PLAN.md
docs/local-contract.md
cmd/onlava/*harness*
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

- Add a versioned `onlava.harness.ui.v1` JSON surface.
- Keep the command separate from default `onlava harness self` until the browser path is proven reliable.
- If a browser automation dependency is added, record the rationale and update dependency checks.
- Reuse dashboard DOM markers from onlava layouts.

## Plan of Work

Define the JSON contract first, then add the CLI command and a minimal route runner. Capture screenshots, console errors, and failed network requests only after the command can reliably start or connect to a dashboard.

## Milestones

### Milestone 1: JSON schema

Add:

```text
docs/schemas/onlava.harness.ui.v1.schema.json
```

Shape:

```json
{
  "schema_version": "onlava.harness.ui.v1",
  "ok": true,
  "routes": [],
  "artifacts": [],
  "diagnostics": []
}
```

### Milestone 2: CLI command

Add:

```sh
onlava harness ui --json [--repo-root <path>] [--app-root <path>] [--headed]
```

### Milestone 3: Route checks

Visit:

```text
dashboard home
API Explorer
traces
metrics
data explorer
DB Studio route if configured
```

Assert stable DOM markers:

```text
data-onlava-ui="AppShell"
data-onlava-ui="DashboardPage"
data-onlava-ui="DataExplorerLayout"
```

### Milestone 4: Artifacts

Write artifacts under:

```text
.onlava/harness/ui/
  screenshots/
  console.jsonl
  network.jsonl
```

### Milestone 5: Optional self-harness integration

Do not add browser harness to the default fast self-harness immediately. Add optional wiring or document how to run it.

## Concrete Steps

1. Add the JSON schema and local-contract entry.
2. Add `onlava harness ui --json` CLI parsing.
3. Implement dashboard URL discovery or startup rules.
4. Visit the first route and assert a DOM marker.
5. Add screenshots and console/network collection.
6. Add tests around JSON output and failure diagnostics.

## Validation and Acceptance

```sh
go test ./cmd/onlava
go install ./cmd/onlava
onlava harness ui --json --app-root testdata/apps/basic
onlava harness self --json --write
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

The harness should write artifacts under a deterministic `.onlava/harness/ui/` path and may overwrite previous local UI harness artifacts. A failed route check should not leave child processes running.

## Artifacts and Notes

Expected artifacts include `docs/schemas/onlava.harness.ui.v1.schema.json`, command tests, screenshots, console/network logs, and documentation updates.
