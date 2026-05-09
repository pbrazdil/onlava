# Dashboard Slot-Layout Migration

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

The dashboard should gradually migrate from route-level className-heavy markup to onlava-owned slot layouts and primitives.

This is not a visual redesign. The goal is to reduce agent mistakes by moving page structure into named layouts:

```text
route file
  -> choose layout
  -> fill named slots
  -> avoid page-level grid/class soup
```

## Progress

- [ ] Create this ExecPlan and link from `docs/plans/active.md`.
- [ ] Count current UI static className warnings.
- [ ] Pick first routes to migrate.
- [ ] Add missing layout primitives.
- [ ] Migrate routes without visual redesign.
- [ ] Reduce warnings.
- [ ] Add route render tests.

## Surprises & Discoveries

Record discoveries here.

## Decision Log

Record decisions here.

## Outcomes & Retrospective

Fill when complete.

## Context and Orientation

Relevant files:

```text
docs/ui-agent-contract.md
cmd/onlava/harness_ui.go
ui/src/routes/*
ui/src/components/layouts/*
ui/src/components/primitives/*
```

## Interfaces and Dependencies

- Follow `docs/ui-agent-contract.md`.
- Reuse existing primitives and layouts before adding new ones.
- Avoid visual redesign and dependency churn.
- Keep route tests focused on render behavior and DOM markers.

## Plan of Work

Start with the current self-harness warning list, migrate the smallest routes first, and only tighten checks after warning counts are near zero. Each migration should preserve the current dashboard behavior.

## Milestones

### Milestone 1: Baseline warning count

Run self-harness and record current warnings.

Acceptance:

```text
- baseline warning count captured
- high-warning files identified
```

### Milestone 2: Migrate simple routes

Migrate two or three low-risk dashboard routes to `DashboardPage`.

Acceptance:

```text
- no visual intent changes
- warnings reduced
- tests pass
```

### Milestone 3: Migrate data/observability routes

Migrate API Explorer, traces, metrics, and data explorer routes into named layouts.

Acceptance:

```text
- route files mostly compose layouts/primitives
- no direct low-level imports
```

### Milestone 4: Make className warnings stricter

After warnings are close to zero, upgrade some warning categories to errors.

## Concrete Steps

1. Run `onlava harness self --json --write` and record the className warning baseline.
2. Pick two or three low-risk route files.
3. Add missing layout primitives only when an existing primitive cannot express the route.
4. Move route-level structure into named slots.
5. Add or update render tests.
6. Re-run the UI static check and record warning reduction.

## Validation and Acceptance

```sh
go test ./...
cd ui && bun run typecheck
cd ui && bun run test
cd ui && bun run build
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- className warnings reduced materially
- route files use layouts/primitives
- no dashboard visual redesign
- no forbidden imports
```

## Idempotence and Recovery

Each route migration should be independently revertible. If a migration causes visual or behavior regressions, revert that route while keeping any generally useful primitive only if it remains covered by tests.

## Artifacts and Notes

Track the warning baseline and final warning count in this plan's Progress or Outcomes section.
