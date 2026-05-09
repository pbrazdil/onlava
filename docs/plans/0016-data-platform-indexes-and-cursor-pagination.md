# Data Platform Indexes and Cursor Pagination

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

The data platform has metadata-defined objects/fields, real PostgreSQL tables/columns, transactional outbox, live updates, and trigger-backed direct-SQL capture. The next data-layer foundation is index metadata and stable cursor pagination.

The goal:

```text
metadata indexes
      |
      v
real PostgreSQL indexes
      |
      v
query compiler can paginate with keyset cursors
      |
      v
dashboard/data clients can scale beyond first-page queries
```

## Progress

- [ ] Create ExecPlan and link from `docs/plans/active.md`.
- [ ] Add index metadata tables.
- [ ] Add public data index API.
- [ ] Add physical index migrations.
- [ ] Add inspect data index output.
- [ ] Add keyset cursor pagination.
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
internal/objectstore/types.go
internal/objectstore/migrate.go
internal/objectstore/query.go
internal/datainspect/inspect.go
data/data.go
docs/schemas/onlava.inspect.data.v1.schema.json
```

`RecordPage` already has `NextCursor`; this plan should make it real.

## Interfaces and Dependencies

- Public API changes live in `github.com/pbrazdil/onlava/data`.
- Core implementation lives in `internal/objectstore`.
- Inspect output changes must update `docs/schemas/onlava.inspect.data.v1.schema.json`.
- PostgreSQL tests should use the existing test database/testcontainers path.

## Plan of Work

Add metadata and physical index creation first, then expose inspect output, then implement keyset cursor pagination. Keep index creation and cursor handling separate enough that either can be tested independently.

## Milestones

### Milestone 1: Index metadata

Add:

```text
onlava_data.indexes
onlava_data.index_fields
```

Represent:

```text
name
object_id
tenant_id
method: btree | gin
unique
where_deleted_at_null
fields
status
physical_name
```

### Milestone 2: Public API

Add:

```go
CreateIndex
ListIndexes
DropIndex or ArchiveIndex
```

Keep API small.

### Milestone 3: Physical index migrations

Support:

```text
btree scalar index
compound btree index
GIN index for multi_select text[]
optional partial index WHERE deleted_at IS NULL
```

Use migration rows and advisory locks.

### Milestone 4: Inspect output

`onlava inspect data` should show logical and physical index state.

### Milestone 5: Cursor pagination

Implement keyset pagination.

Rules:

```text
- no offset pagination
- always append id tie-breaker
- cursor encodes object, sort shape, and last values
- reject cursor if sort/object mismatch
```

## Concrete Steps

1. Add index metadata tables and bootstrap migrations.
2. Add data package request/response types and store methods.
3. Implement deterministic physical index names.
4. Create indexes through migration rows and advisory locks.
5. Extend inspect data output and schema.
6. Implement cursor encoding/decoding and query compiler predicates.
7. Add unit and PostgreSQL-backed tests.

## Validation and Acceptance

```sh
ONLAVA_TEST_DATABASE_URL=... go test ./internal/objectstore ./internal/datainspect
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- index metadata and physical index are created
- index drift is visible
- query returns NextCursor
- next page is stable
- changed sort rejects old cursor
- no SQL identifier injection
```

## Idempotence and Recovery

Repeated index creation with the same logical name should be predictable and should not create duplicate physical indexes. Failed index DDL must be reflected in migration state and inspect output.

## Artifacts and Notes

Expected artifacts include metadata migrations, data API additions, inspect schema updates, and objectstore integration tests.
