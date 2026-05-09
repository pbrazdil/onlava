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

- [ ] Create ExecPlan and link from active.md.
- [ ] Design relationship metadata.
- [ ] Implement many-to-one.
- [ ] Implement inverse one-to-many metadata.
- [ ] Implement many-to-many join tables.
- [ ] Add relationship filters and selects.
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

- Relation APIs should live in `github.com/pbrazdil/onlava/data`.
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
ONLAVA_TEST_DATABASE_URL=... go test ./internal/objectstore
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
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
