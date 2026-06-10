# Data Platform Public Contract

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

The `scenery.sh/data` package is currently beta. This plan hardens its public contract so apps and agents can depend on it safely.

The goal:

```text
beta data package
      |
      v
documented API
      |
      v
typed errors
      |
      v
compatibility tests
      |
      v
stable-ish v0 surface
```

## Progress

- [x] Audit public `data` package.
- [x] Add docs.
- [x] Add typed errors.
- [x] Add compatibility tests.
- [x] Add examples.
- [x] Update local contract.

## Surprises & Discoveries

- The public package is still mostly aliases over objectstore types. That is acceptable for beta, but docs now make the supported app-facing surface explicit.
- A lightweight wrapper error layer was enough for coarse app handling without changing the internal objectstore error flow.

## Decision Log

- Kept `scenery.sh/data` classified as beta, not stable, because relationships and saved views are still actively evolving.
- Added public `data.CodeOf(err)` and `*data.Error` for coarse error classification instead of a larger typed error hierarchy.
- Added a compile-only example package under `examples/data-platform` rather than a runnable sample requiring PostgreSQL credentials.

## Outcomes & Retrospective

- Added `docs/data-platform.md`, docs index/knowledge entries, public error codes, error classification tests, and a compiling example.
- Updated `docs/local-contract.md` to cover relation fields, saved views, and public data error handling.
- `go test -count=1 ./data ./examples/data-platform` and `go test -count=1 ./internal/objectstore` pass.

## Context and Orientation

Relevant files:

```text
data/data.go
internal/objectstore/*
docs/local-contract.md
docs/data-platform.md
testdata/apps/data-platform
```

## Scope

Stabilize:

```text
Store
Actor
Permissions
Object
Field
Record
Query
Filter
Sort
Event
Create/Update/Delete/Query APIs
ServeEvents
```

Do not stabilize:

```text
internal objectstore details
physical table names as app contract
migration internals
trigger internals
```

## Milestones

### Milestone 1: API audit

Identify awkward aliases and internal leaks.

### Milestone 2: Typed errors

Add coded errors for:

```text
object_not_found
field_not_found
invalid_filter
permission_denied
migration_failed
schema_drift
invalid_cursor
```

### Milestone 3: Docs

Add:

```text
docs/data-platform.md
examples/data-platform/
```

### Milestone 4: Compatibility tests

Snapshot public API examples and expected JSON behavior.

## Interfaces and Dependencies

- Stable or beta public API lives only under `scenery.sh/data`.
- Internal objectstore implementation details must remain internal.
- Error types should compose with existing scenery error conventions where possible.
- Examples must compile without private package imports.

## Plan of Work

Audit public types first, remove accidental internal leaks, then add typed errors and examples. Update the local contract only after tests prove the documented API.

## Concrete Steps

1. Inventory exported `data` identifiers.
2. Decide which identifiers are stable, beta, or internal leaks.
3. Add typed errors and mapping tests.
4. Add docs and examples.
5. Add compatibility tests.
6. Update `docs/local-contract.md`.

## Validation and Acceptance

```sh
go test ./data ./internal/objectstore
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

```text
- public data docs exist
- examples compile
- typed errors are exposed
- local-contract classifies stable/beta boundaries clearly
```

## Idempotence and Recovery

Public API cleanup should avoid churn in app-facing names. If a breaking change is needed, record it in the Decision Log and update examples in the same change.

## Artifacts and Notes

Expected artifacts include `docs/data-platform.md`, examples, typed error tests, and local-contract updates.
