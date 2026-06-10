# Data Platform Relationships

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

The data platform currently has a `relation` field type as a UUID-like storage primitive. This plan turns relation fields into first-class object relationships.

The goal:

```text
relation metadata
      |
      v
foreign keys / join tables
      |
      v
relationship-aware queries
      |
      v
CRM-style object graph
```

## Progress

- [x] Create ExecPlan and link from active.md.
- [x] Design relationship metadata.
- [x] Implement many-to-one.
- [x] Implement inverse one-to-many metadata.
- [x] Implement many-to-many join tables.
- [x] Add relationship filters and selects.
- [x] Add tests.

## Surprises & Discoveries

- The existing `relation_object_id` field metadata column was enough for the first public shape; relation details fit in field settings without adding another metadata table.
- The query compiler could support one-hop many-to-one relation paths with metadata-resolved scalar subqueries, avoiding a broader join-planner refactor in this pass.

## Decision Log

- Default relation kind is `many_to_one`; default delete behavior is `restrict`.
- `many_to_one` relation fields create a real UUID column and PostgreSQL foreign key.
- `many_to_many` relation fields create a real join table and inspectable metadata, but record-level many-to-many mutation helpers remain outside the stable app-facing contract.
- Relation path queries are limited to one-hop many-to-one paths such as `company.name` for now.

## Outcomes & Retrospective

- Completed relationship metadata, many-to-one FK DDL, many-to-many join-table DDL, one-hop relation query paths, inspect output, and PostgreSQL-backed tests.
- `go test ./...`, UI typecheck, and UI tests pass after the relationship changes.
- The next relationship expansion should add ergonomic record mutation helpers for many-to-many fields and deeper relationship paths if product work needs them.

## Context and Orientation

Relevant files:

```text
internal/objectstore/types.go
internal/objectstore/migrate.go
internal/objectstore/query.go
internal/datainspect/inspect.go
data/data.go
docs/schemas/scenery.inspect.data.v1.schema.json
testdata/apps/data-platform
```

## Scope

Initial relationship kinds:

```text
many_to_one
one_to_many inverse metadata
many_to_many
```

Delete behavior:

```text
restrict
set_null
cascade only if explicitly requested later
```

Default should be `restrict`.

## Milestones

### Milestone 1: Relationship metadata

Extend field settings or add relation tables.

Capture:

```text
source object
target object
relation kind
inverse field name
on_delete behavior
join table name for many-to-many
```

### Milestone 2: Many-to-one foreign keys

For `many_to_one`, create UUID column and FK.

### Milestone 3: Many-to-many join tables

Create join table:

```text
source_record_id
target_record_id
tenant_id
created_at
```

### Milestone 4: Query support

Support simple relationship paths:

```text
company.name
deal.company.name
```

Filters:

```json
{"op":"eq","field":"company.stage","value":"customer"}
```

### Milestone 5: Inspect and tests

Show relation state in inspect data.

## Interfaces and Dependencies

- Relation APIs should live in `scenery.sh/data`.
- Physical relationship structures are owned by `internal/objectstore`.
- Inspect data must expose relationship state without making physical names the app-facing contract.
- PostgreSQL foreign keys and join tables are required for the first real relationship implementation.

## Plan of Work

Implement many-to-one first because it maps cleanly to a UUID column and foreign key. Then add inverse metadata and many-to-many join tables. Add relationship-aware query support only after the physical structures and inspect output are tested.

## Concrete Steps

1. Design relation settings and public request types.
2. Add migration support for many-to-one foreign keys.
3. Add inverse metadata.
4. Add many-to-many join table creation.
5. Extend query compiler path resolution.
6. Extend inspect data output and tests.

## Validation and Acceptance

```sh
SCENERY_TEST_DATABASE_URL=... go test ./internal/objectstore
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

```text
- relation fields create correct physical structures
- FK constraints work
- many-to-many join table works
- relationship queries are parameterized
- inspect data shows relation metadata
```

## Idempotence and Recovery

Repeated relation migrations should be safe and should verify existing foreign keys/join tables. Destructive relation changes should fail clearly unless an explicit conservative migration path exists.

## Artifacts and Notes

Expected artifacts include relation metadata types, migration tests, query compiler tests, and inspect schema updates.
