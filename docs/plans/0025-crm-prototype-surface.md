# CRM Prototype Surface

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

After the data platform, data explorer, indexes, saved views, and relationships exist, scenery can host a small CRM-like prototype to validate the architecture against real workflows.

This is not a Twenty clone. It is a fixture/product prototype proving scenery-native dynamic data can support CRM surfaces.

The goal:

```text
dynamic objects
      |
      v
companies / people / deals
      |
      v
saved views
      |
      v
record detail
      |
      v
live updates
```

## Progress

- [ ] Create ExecPlan.
- [ ] Define prototype fixture app.
- [ ] Add default objects.
- [ ] Add list/detail UI.
- [ ] Add live update demonstration.
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
testdata/apps/data-platform
testdata/apps/crm-prototype
data/data.go
ui/src/features/data-explorer/*
docs/plans/0017-data-platform-relationships.md
docs/plans/0018-data-platform-saved-views.md
```

## Scope

Objects:

```text
company
person
deal
activity
```

Views:

```text
companies table
people table
deals pipeline table
```

UI:

```text
list view
record detail inspector
basic create/update form
live event tail
```

Non-goals:

```text
full CRM
email/calendar sync
workflow automation
reporting
billing
production SaaS UI
```

## Milestones

### Milestone 1: Fixture app

Add:

```text
testdata/apps/crm-prototype
```

### Milestone 2: Seed data

Use data import/export or programmatic setup.

### Milestone 3: Dashboard route

Expose prototype surface through dashboard dev UI.

### Milestone 4: Live update demo

Two clients should see record updates.

## Interfaces and Dependencies

- Build on the data platform public API.
- Prefer existing dashboard/data explorer primitives over a new product UI system.
- Do not add external CRM-specific services.
- This plan should wait until indexes, saved views, and relationships are ready enough to exercise realistic workflows.

## Plan of Work

Define the fixture app and seed data first, then add list/detail UI and live-update demonstration. Keep the prototype explicitly scoped as a validation surface, not a product commitment.

## Concrete Steps

1. Add `testdata/apps/crm-prototype`.
2. Seed company/person/deal/activity objects.
3. Add list and detail views.
4. Add create/update path.
5. Add live update demonstration.
6. Add tests and docs.

## Validation and Acceptance

```sh
SCENERY_TEST_DATABASE_URL=... go test ./testdata/...
cd ui && bun run typecheck && bun run build
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

```text
- prototype creates dynamic objects
- list/detail UI works
- live updates are visible
- agents can inspect/debug through existing commands
```

## Idempotence and Recovery

Fixture setup should be repeatable against a clean tenant. Seed data should either use deterministic IDs or explicitly remap IDs.

## Artifacts and Notes

Expected artifacts include the fixture app, seed data, tests, and any dashboard prototype route needed to demonstrate the workflow.
