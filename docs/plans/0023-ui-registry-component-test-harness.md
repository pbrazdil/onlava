# UI Registry Component Test Harness

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

The onlava UI registry should not only exist; every primitive and layout should have a small test fixture proving that it renders, exposes DOM markers, and can be used by agents.

The goal:

```text
registry item
      |
      v
component/layout
      |
      v
render test
      |
      v
future browser harness marker
```

## Progress

- [ ] Create ExecPlan.
- [ ] Add registry validation tests.
- [ ] Add component render tests.
- [ ] Add layout slot tests.
- [ ] Add docs.

## Surprises & Discoveries

Record discoveries here.

## Decision Log

Record decisions here.

## Outcomes & Retrospective

Fill when complete.

## Context and Orientation

Relevant files:

```text
ui/registry/onlava/*.json
ui/scripts/onlava-shadcn.mjs
ui/src/components/primitives/*
ui/src/components/layouts/*
cmd/onlava/harness_ui.go
```

## Scope

Test:

```text
Button
Input
Select
Card
Dialog
DashboardPage
DataExplorerLayout
ProductSurface
FilterPill
SidebarItem
```

## Milestones

### Milestone 1: Registry validation

Ensure every registry item points to existing files and valid targets.

### Milestone 2: Primitive render tests

Use Testing Library.

### Milestone 3: Layout slot tests

Assert:

```text
data-onlava-ui
data-slot
slot contents render
optional slots behave correctly
```

### Milestone 4: Wrapper tests

Test `onlava-shadcn.mjs` rejection behavior without requiring network.

## Interfaces and Dependencies

- Prefer existing UI test tools already in `ui/package.json`.
- Registry tests should share expectations with self-harness where possible.
- Browser-level checks belong to the browser harness; this plan focuses on component render tests.

## Plan of Work

Start by asserting registry metadata points to real components, then add render tests for primitives and slot layouts, then cover wrapper rejection behavior.

## Concrete Steps

1. Add registry metadata tests.
2. Add primitive render tests.
3. Add layout slot tests.
4. Add wrapper tests for allowed and rejected arguments.
5. Update docs if the component contract changes.

## Validation and Acceptance

```sh
cd ui && bun run test
cd ui && bun run typecheck
cd ui && bun run build
go test ./cmd/onlava
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- registry items are test-covered
- layouts render all required markers
- wrapper rejection behavior is tested
- optional slots behave correctly
```

## Idempotence and Recovery

Tests should not install registry items into the real working tree. Use temporary directories for wrapper behavior.

## Artifacts and Notes

Expected artifacts include UI tests and any small test fixtures needed for wrapper execution.
