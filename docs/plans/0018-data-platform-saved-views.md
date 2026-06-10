# Data Platform Saved Views

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

CRM-like products need reusable views: table columns, filters, sorts, grouping, and display preferences. This plan adds metadata-backed saved views for data objects.

The goal:

```text
object
  -> saved views
  -> columns / filters / sorts / layout state
  -> dashboard and clients reuse the same query shape
```

## Progress

- [x] Create ExecPlan.
- [x] Add saved view metadata tables.
- [x] Add public API.
- [x] Add query execution by view.
- [x] Add dashboard integration.
- [x] Add tests.

## Surprises & Discoveries

- Saved view query validation could reuse the existing objectstore query compiler directly, including relation path validation from the previous plan.
- Inspect output is the simplest dashboard bridge for saved views; the Data Explorer can select a view without a new dashboard RPC method.

## Decision Log

- Stored view columns in `scenery_data.view_fields` as metadata-resolved field names, including relation paths, rather than field IDs only.
- Kept the first public API name-based per object: `CreateView`, `UpdateView`, `ListViews`, `DeleteView`, and `QueryView`.
- Reserved `kanban` and `calendar` view types in metadata, but the first UI treats saved views as table query shapes.

## Outcomes & Retrospective

- Completed metadata tables, public data package aliases/methods, query-by-view execution, inspect output, and Data Explorer view selection.
- Invalid saved view columns/filters fail before persistence through the existing query compiler.
- `go test -count=1 ./internal/objectstore`, `go test -count=1 ./internal/datainspect ./data`, and `cd ui && bun run typecheck` pass for this slice.

## Context and Orientation

Relevant files:

```text
internal/objectstore/*
internal/datainspect/*
data/data.go
ui/src/features/data-explorer/*
docs/schemas/scenery.inspect.data.v1.schema.json
```

## Scope

Support initial view types:

```text
table
kanban reserved but not implemented
calendar reserved but not implemented
```

View metadata:

```text
name
object_id
tenant_id
columns
filter
sort
limit
visibility
owner_id
```

## Milestones

### Milestone 1: Metadata tables

Add:

```text
scenery_data.views
scenery_data.view_fields
```

### Milestone 2: API

Add:

```go
CreateView
UpdateView
ListViews
DeleteView
QueryView
```

### Milestone 3: Dashboard integration

Data Explorer can select saved views.

### Milestone 4: Validation

View filters must be validated using same query compiler rules.

## Interfaces and Dependencies

- Public saved-view APIs belong in `scenery.sh/data`.
- Query execution should reuse the existing query compiler.
- Dashboard integration should be optional and should not block API tests.
- Saved view metadata must not require a CRM-specific role model.

## Plan of Work

Add saved view metadata and APIs first, prove query-by-view in Go tests, then wire the Data Explorer to list and select views.

## Concrete Steps

1. Add metadata tables and bootstrap migrations.
2. Add public create/list/update/delete/query APIs.
3. Validate stored filters and sorts through the query compiler.
4. Add inspect output if useful for debugging.
5. Add Data Explorer view selection.
6. Add PostgreSQL-backed tests.

## Validation and Acceptance

```sh
SCENERY_TEST_DATABASE_URL=... go test ./internal/objectstore
cd ui && bun run typecheck && bun run test && bun run build
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

```text
- saved views persist
- query by saved view works
- invalid filters fail clearly
- dashboard can list/select views
```

## Idempotence and Recovery

View creation should be transactional. Invalid view filters must fail before persistence. Deleting a view should not delete records.

## Artifacts and Notes

Expected artifacts include metadata migrations, public data API tests, and Data Explorer UI updates.
