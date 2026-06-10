# Generated Data Client

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

scenery already generates TypeScript clients for app endpoints. The data platform should eventually have a typed helper client for dynamic data APIs exposed by apps or the dashboard.

The goal:

```text
data API endpoint model
      |
      v
generated TypeScript helpers
      |
      v
safer dashboard and app clients
```

## Progress

- [ ] Create ExecPlan.
- [ ] Decide client generation boundary.
- [ ] Add data client helpers.
- [ ] Add tests.
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
internal/clientgen/*
data/data.go
runtime/client generation fixtures
testdata/apps/data-platform
```

## Scope

Generate helper types for:

```text
Record
Query
Filter
Sort
CreateObjectRequest
CreateFieldRequest
CreateRecordRequest
QueryRecordsRequest
Event
```

Do not attempt fully typed per-object field models yet.

## Milestones

### Milestone 1: Generic data client

Add generic helpers:

```ts
client.data.queryRecords(...)
client.data.createRecord(...)
client.data.events(...)
```

### Milestone 2: Filter builders

Generate or ship TS equivalents:

```ts
eq()
gte()
contains()
and()
or()
not()
```

### Milestone 3: SSE event helper

Add event subscription helper.

## Interfaces and Dependencies

- Use existing TypeScript client generation conventions.
- Do not generate per-tenant or per-object static field models in this plan.
- SSE helpers should parse existing data event payloads rather than inventing a new event format.

## Plan of Work

Add generic data helper types first, then filter builders, then SSE helpers. Keep generated output boring and compatible with existing client generation tests.

## Concrete Steps

1. Identify the generated client insertion point.
2. Add generic data request/response types.
3. Add filter/sort builder helpers.
4. Add SSE event subscription helper.
5. Add clientgen tests and fixture output.
6. Update docs.

## Validation and Acceptance

```sh
go test ./internal/clientgen ./...
cd ui && bun run typecheck && bun run build
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

```text
- generated client supports generic data operations
- filter helpers typecheck
- SSE helper can parse data events
- no per-object codegen yet
```

## Idempotence and Recovery

Regenerating clients should be deterministic. If helper output changes, update golden files in the same change.

## Artifacts and Notes

Expected artifacts include generated TypeScript helpers, golden tests, and fixture updates.
