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

- [ ] Create ExecPlan.
- [ ] Add saved view metadata tables.
- [ ] Add public API.
- [ ] Add query execution by view.
- [ ] Add dashboard integration.
- [ ] Add tests.

## Surprises & Discoveries

Record discoveries here.

## Decision Log

Record decisions here.

## Outcomes & Retrospective

Fill when complete.

## Context and Orientation

Relevant files:

```text
internal/objectstore/*
internal/datainspect/*
data/data.go
ui/src/features/data-explorer/*
docs/schemas/onlava.inspect.data.v1.schema.json
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
onlava_data.views
onlava_data.view_fields
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

- Public saved-view APIs belong in `github.com/pbrazdil/onlava/data`.
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
ONLAVA_TEST_DATABASE_URL=... go test ./internal/objectstore
cd ui && bun run typecheck && bun run test && bun run build
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
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
