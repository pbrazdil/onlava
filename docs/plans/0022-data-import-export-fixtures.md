# Data Import, Export, and Fixtures

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

Dynamic data needs reproducible fixtures and safe import/export. This plan adds local development tools for exporting object metadata/records and importing them into another data tenant.

The goal:

```text
export metadata + records
      |
      v
portable JSON
      |
      v
import into tenant
      |
      v
fixture apps and tests become easier
```

## Progress

- [x] Create ExecPlan.
- [x] Design export format.
- [x] Implement export.
- [x] Implement import.
- [x] Add fixture support.
- [x] Add tests.

## Surprises & Discoveries

- The portable bundle should avoid physical table names, column names, field IDs, and relation join-table names. Those belong to the target tenant's migration history and are regenerated on import.
- Reusing existing mutation paths inside an outer transaction works with pgx nested transactions/savepoints and keeps migrations, outbox rows, search documents, and validation behavior consistent.

## Decision Log

- Use `onlava.data.export.v1` as the portable JSON schema.
- Export logical objects, fields/options, indexes, saved views, and records.
- Import recreates metadata through existing create APIs and creates new record IDs; the response returns `record_id_map` for exported ID to imported ID reconciliation.
- Import writes normal outbox rows and publishes imported record events only after the outer import transaction commits.
- Re-importing metadata is idempotent when the requested shape matches; records are appended as new records.

## Outcomes & Retrospective

- Implemented `ExportTenant` and `ImportTenant` in objectstore and the public `data.Store`.
- Added schema, fixture bundle, fixture API endpoints, docs, and PostgreSQL-backed round-trip coverage.
- The first import/export surface is intentionally local-development sized, not a bulk ETL system.

## Context and Orientation

Relevant files:

```text
data/data.go
internal/objectstore/*
testdata/apps/data-platform
docs/schemas/*
```

## Scope

Export:

```text
tenant key
objects
fields
field options
views if implemented
records
```

Import:

```text
create objects/fields
insert records
optionally preserve IDs
optionally remap IDs
```

Non-goals:

```text
large-scale ETL
streaming import
external SaaS importers
CSV UI
```

## Milestones

### Milestone 1: JSON format

Add schema:

```text
docs/schemas/onlava.data.export.v1.schema.json
```

### Milestone 2: Export API

Add `ExportTenant` or `ExportObject`.

### Milestone 3: Import API

Add `ImportTenant`.

### Milestone 4: Fixture integration

Use exported data in `testdata/apps/data-platform`.

## Interfaces and Dependencies

- Export/import APIs should live in the public `data` package only if they are intended for app use.
- JSON schema should be versioned before fixtures depend on it.
- Import should use existing object/field/record mutation paths unless there is a documented reason not to.

## Plan of Work

Define the portable JSON format first, then implement export, then import, then fixture usage. Keep record ID preservation/remapping explicit.

## Concrete Steps

1. Add `onlava.data.export.v1` schema.
2. Add export types and implementation.
3. Add import types and implementation.
4. Decide outbox behavior for import.
5. Add round-trip tests.
6. Use the format in a fixture.

## Validation and Acceptance

```sh
ONLAVA_TEST_DATABASE_URL=... go test ./internal/objectstore
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- export/import round trip works
- schema validates
- IDs can be remapped
- outbox behavior is explicit and tested
```

## Idempotence and Recovery

Imports should run in transactions and should either complete or leave the tenant unchanged. Re-import behavior must be documented: fail on conflict, upsert, or create a new tenant.

## Artifacts and Notes

Expected artifacts include schema, export/import tests, and fixture data files.
