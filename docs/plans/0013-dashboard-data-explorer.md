# Dashboard Data Explorer

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

onlava should expose the native data platform through the local dashboard. The first UI is not a full CRM and not a polished product builder. It is a developer-facing data explorer that lets humans and agents inspect tenants, objects, fields, records, migrations, triggers, and outbox events.

The goal is to prove:

```text
data platform exists
        |
        v
dashboard can inspect/query it
        |
        v
agents can compose the UI from onlava layouts/primitives instead of ad hoc shadcn/Tailwind code
```

This plan should use the onlava UI contract: primitives from `ui/src/components/primitives`, layouts from `ui/src/components/layouts`, and stable DOM markers for future browser harness checks.

## Progress

- [x] (2026-05-09) Created this ExecPlan and linked it from `docs/plans/active.md`.
- [ ] Read `docs/ui-agent-contract.md`, `docs/local-contract.md`, `internal/objectstore`, `internal/datainspect`, and current dashboard routes.
- [ ] Add dashboard route for data explorer.
- [ ] Add data inspect/query API bridge.
- [ ] Render tenants, objects, fields, migrations, trigger state, and outbox summary.
- [ ] Add record query table.
- [ ] Add outbox event tail.
- [ ] Add tests and validation.

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
ui/src/components/layouts/DataExplorerLayout.tsx
ui/src/components/primitives/*
internal/objectstore/*
internal/datainspect/*
cmd/onlava/*inspect*
docs/schemas/onlava.inspect.data.v1.schema.json
testdata/apps/data-platform
```

Existing `onlava inspect data --json --database-url <postgres-url>` reports data tenants, objects, fields, migrations, outbox, and trigger state. The dashboard should reuse that knowledge where possible instead of inventing a divergent data-inspection model.

## Interfaces and Dependencies

- Reuse existing dashboard routing and data-fetching patterns.
- Prefer `internal/datainspect` output shapes over a new dashboard-only metadata model.
- Use onlava UI primitives/layouts only; do not add direct shadcn, Radix, or low-level styling imports in route files.
- No new external UI dependencies are expected for the first data explorer.

## Scope

Build a local development dashboard feature:

```text
/data or /data-explorer dashboard route
tenant list
object list
field list
migration/outbox/trigger state
record query table
outbox event tail
```

Non-goals:

```text
full CRM
field editor
object builder
kanban/pipeline UI
bulk import/export
reporting
workflow automation
dynamic GraphQL
external deployment surface
```

## Milestones

### Milestone 1: Route and shell

Add a dashboard route that uses `DataExplorerLayout`.

The route should show placeholder panels first:

```tsx
<DataExplorerLayout
  title="Data"
  objectList={<ObjectList />}
  toolbar={<DataToolbar />}
  table={<RecordTable />}
  inspector={<ObjectInspector />}
  eventStream={<OutboxEventTail />}
/>
```

Acceptance:

```text
- route renders
- stable data-onlava-ui markers exist
- no direct shadcn/vendor imports
- no new long className soup outside layouts/primitives
```

### Milestone 2: Data inspect bridge

Add a dashboard API or local fetch path that can retrieve data inspect output.

Prefer reusing `internal/datainspect` semantics.

Acceptance:

```text
- dashboard can show metadata schema and records schema names
- dashboard can list tenants
- dashboard can list objects and fields
- errors are readable
```

### Milestone 3: Record query table

Add a simple query UI for one tenant/object.

Support:

```text
select object
display records
limit
simple filter text or JSON filter
refresh
```

Acceptance:

```text
- can query records from data-platform fixture
- selected object fields render as columns
- empty state is clear
```

### Milestone 4: Migration/outbox/trigger inspector

Show:

```text
schema_version
physical_table
outbox_triggers_enabled
outbox_trigger_present
latest migrations
latest outbox seq
unpublished count
```

Acceptance:

```text
- inspector shows object infrastructure state
- trigger mismatch is visible
```

### Milestone 5: Outbox event tail

Show latest outbox events for selected tenant/object.

Acceptance:

```text
- event list updates after refresh
- event payload is readable
- no user-record dump unless explicitly selected
```

## Plan of Work

Land the route shell first, then wire data inspect output, then add record querying and outbox inspection. Keep each step usable and testable so the dashboard can render even when no data database is configured.

## Concrete Steps

1. Add route file under `ui/src/routes` or the existing router structure.
2. Add a feature folder:

   ```text
   ui/src/features/data-explorer/
     DataExplorerPage.tsx
     ObjectList.tsx
     RecordTable.tsx
     ObjectInspector.tsx
     OutboxEventTail.tsx
     dataExplorerClient.ts
   ```

3. Use `DataExplorerLayout`.
4. Reuse `Button`, `Input`, `Select`, `Card`, and layout primitives.
5. Add fetch helpers.
6. Add dashboard tests for route render and core markers.
7. Run validation.

## Validation and Acceptance

```sh
go test ./...
go install ./cmd/onlava
cd ui && bun run typecheck
cd ui && bun run test
cd ui && bun run build
onlava harness self --json --write
```

Acceptance criteria:

```text
- data explorer route renders
- route uses onlava layout/primitives
- no forbidden imports
- data inspect information is visible
- records can be queried for fixture data
- self-harness UI static checks pass
```

## Idempotence and Recovery

The route and feature components should be safe to reload without mutating data. Refresh actions should only query. If dashboard data inspection fails because no PostgreSQL URL is available, show a readable empty/error state and keep the rest of the dashboard usable.

## Artifacts and Notes

Expected artifacts are source files under `ui/src/features/data-explorer`, a dashboard route, tests, and the updated `.onlava/harness/self-latest.json` snapshot.
