# Data Platform Search

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

Dynamic records need fast text search across selected fields. This plan adds metadata-driven search without introducing a search service.

The first implementation should use PostgreSQL:

```text
searchable fields
      |
      v
generated/search vector column or maintained search document
      |
      v
query search operator
      |
      v
dashboard search box
```

## Progress

- [x] Create ExecPlan.
- [x] Decide search storage model.
- [x] Add metadata for searchable fields.
- [x] Add search indexes.
- [x] Add query compiler support.
- [x] Add dashboard search.
- [x] Add tests.

## Surprises & Discoveries

- A generic search document table keeps the first implementation smaller than per-object generated columns and avoids reworking physical object-table DDL every time a searchable field is added.
- Direct SQL edits are not enough to keep search documents fresh in this version. Normal data mutation APIs update search documents transactionally; trigger-backed search refresh can close the direct-SQL gap later.

## Decision Log

- Use `onlava_data.search_documents` with a GIN index on a `tsvector` document. This gives one indexed PostgreSQL search path for all dynamic objects without adding a search service.
- Maintain search documents in the mutation layer for `CreateRecord`, `UpdateRecord`, and `DeleteRecord`, in the same transaction as the record write and outbox row.
- Add field metadata columns `is_searchable` and `search_weight` instead of storing search configuration only in opaque settings JSON.
- Add an object-wide `search` filter operator and a public `data.Search(...)` helper. Field-scoped search is intentionally not part of this pass.

## Outcomes & Retrospective

- Implemented searchable field metadata, search document storage, GIN indexing, query compiler support, live-event search matching, inspect output, public helper, and Data Explorer search input.
- The first version is useful and indexed through normal data writes, but direct SQL changes need a later rebuild or trigger-backed refresh path to keep search documents current.

## Context and Orientation

Relevant files:

```text
internal/objectstore/query.go
internal/objectstore/migrate.go
internal/datainspect/inspect.go
data/data.go
ui/src/features/data-explorer/*
```

## Milestones

### Milestone 1: Search metadata

Add field/object metadata:

```text
is_searchable
search_weight
```

### Milestone 2: Physical search storage

Choose one:

```text
generated tsvector column
maintained tsvector column via mutation layer
materialized search table
```

Record decision.

### Milestone 3: Query support

Add query operator:

```json
{"op":"search","value":"acme"}
```

### Milestone 4: Dashboard

Add search input in Data Explorer.

## Interfaces and Dependencies

- Use PostgreSQL full-text search before considering any external search dependency.
- Search metadata belongs to the data platform, not generated build artifacts.
- Dashboard search should call the same data query path as API clients.

## Plan of Work

Choose and document the search storage model, add metadata and indexes, then add query compiler support and dashboard search UI.

## Concrete Steps

1. Decide generated column, maintained column, or search table.
2. Add searchable metadata.
3. Add migrations and indexes.
4. Add query operator validation and SQL compilation.
5. Add inspect output if needed.
6. Add Data Explorer search input.
7. Add PostgreSQL-backed tests.

## Validation and Acceptance

```sh
ONLAVA_TEST_DATABASE_URL=... go test ./internal/objectstore
cd ui && bun run typecheck && bun run build
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- searchable fields contribute to search
- search is indexed
- no SQL injection
- dashboard search works
```

## Idempotence and Recovery

Search index creation should be idempotent. Rebuilding search documents should be safe to retry and should not mutate source record fields.

## Artifacts and Notes

Expected artifacts include metadata migrations, query compiler tests, and Data Explorer UI updates.
