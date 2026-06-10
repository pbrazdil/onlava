# Remove Objectstore Functionality

This ExecPlan is a living document. Update `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` as implementation proceeds.

## Purpose / Big Picture

Remove the beta dynamic data/objectstore feature from scenery without keeping compatibility aliases or hidden runtime code.

After this change, scenery should no longer expose `scenery.sh/data`, `internal/objectstore`, `internal/datainspect`, `scenery inspect data`, dashboard Data Explorer RPC/UI, data-platform schemas, examples, or fixture apps. Historical plan files may remain as project history, but current docs, harness checks, CLI grammar, dashboard routes, and public package lists must not advertise the removed surface.

## Progress

- [x] 2026-05-30: Started plan after deciding not to pursue SQLite as a replacement path.
- [x] 2026-05-30: Removed Go packages and tests for the data/objectstore surface.
- [x] 2026-05-30: Removed CLI, dashboard RPC, harness, and inspect wiring.
- [x] 2026-05-30: Removed dashboard Data Explorer route, page, tests, and registry item.
- [x] 2026-05-30: Removed current docs/schema/knowledge references that advertise the feature.
- [x] 2026-05-30: Ran Go and UI validation, rebuilt `scenery`, and started self-harness cleanup.
- [x] 2026-05-30: Reran `scenery harness self --json --write`; it passed and wrote `.scenery/harness/self-latest.json`.

## Surprises & Discoveries

- 2026-05-30: `scenery harness self --json --write` exposed two cleanup issues after the first removal pass: `docs/knowledge.json` had invalid JSON from removed entries, and this ExecPlan was missing the required structural headings enforced by `PLANS.md`.

## Decision Log

- Decision: Delete the feature surface outright rather than keeping disabled commands, aliases, or compatibility shims.
  Rationale: The repository rule is no legacy compatibility. A removed public surface should fail at compile/CLI parse time instead of staying as a dormant path.
  Date/Author: 2026-05-30 / Codex.

## Outcomes & Retrospective

Completed on 2026-05-30. The objectstore/data surface was removed rather than hidden behind a disabled compatibility path. Current code, docs, harness knowledge, dashboard routes, UI registry, examples, fixtures, and schemas no longer advertise the feature.

Validation passed with `go test -count=1 ./...`, dashboard UI typecheck/test/build, `go install ./cmd/scenery`, and `scenery harness self --json --write`. The current-source reference search for objectstore/data terms outside historical `docs/plans/**` returned no matches.

## Context and Orientation

The feature spans:

- Public Go package: `data/`.
- Internal implementation: `internal/objectstore/`.
- Inspect implementation: `internal/datainspect/`.
- CLI command: `scenery inspect data`.
- Dashboard RPC: `data/inspect`, `data/query-records`, and `data/outbox-events`.
- Dashboard UI: `ui/src/features/data-explorer`, `ui/src/components/layouts/DataExplorerLayout.tsx`, and `@scenery/data-explorer-layout` registry metadata.
- Docs/schemas/examples/fixtures: `docs/data-platform*.md`, `docs/schemas/scenery.data.export.v1.schema.json`, `docs/schemas/scenery.inspect.data.v1.schema.json`, `examples/data-platform`, and `testdata/apps/data-platform`.

## Milestones

Milestone 1 removes the Go surface. Delete the public `data` package, internal implementation packages, data-platform fixture and example apps, and any parser tests that only validate the removed public API.

Milestone 2 removes command and dashboard wiring. Delete `scenery inspect data`, dashboard data RPC handlers, harness browser routes, and generated/freshness references that treated objectstore as a current surface.

Milestone 3 removes UI and documentation. Delete the Data Explorer route, components, registry item, schemas, and current docs or machine-readable knowledge entries that advertised the feature.

Milestone 4 validates the deletion. The repo must compile and test without the packages, the dashboard UI must build without the route, and the self harness must not report current objectstore/data references.

## Plan of Work

First, remove the Go packages and command wiring so the compiler becomes the source of truth for remaining references. Then remove dashboard UI and registry entries. Finally, remove current docs and harness references. Historical completed ExecPlans can keep their old references unless a current harness check treats them as supported docs.

## Concrete Steps

From `/Users/petrbrazdil/Repos/scenery`, remove the directories and files listed in Context and Orientation. Then run `rg` for objectstore/data public-surface terms outside `docs/plans/**` and remove any current references that remain.

Update Go command parsing so `scenery inspect data` is not recognized and there is no disabled compatibility path. Update tests to expect an unknown subject or unknown flag where they previously exercised data inspect options.

Update UI router and registry metadata so the dashboard has no Data Explorer route, layout, or RPC client. Run the UI typecheck, tests, and build after the deletion.

Update docs and harness knowledge surfaces so current docs do not mention the removed data-platform feature. Historical ExecPlans may retain references as project history.

## Validation and Acceptance

Run:

```sh
go test -count=1 ./...
go install ./cmd/scenery
scenery harness self --json --write
```

Acceptance criteria:

- `go list ./...` no longer includes `scenery.sh/data`, `internal/objectstore`, or `internal/datainspect`.
- `scenery inspect data` is not listed in usage and returns an unknown inspect subject/flag path rather than a dormant compatibility path.
- Dashboard Data Explorer route and RPC methods are gone.
- Current docs and harness knowledge do not advertise the removed data/objectstore surface.

## Idempotence and Recovery

This is a deletion refactor. If a later test exposes a missed current reference, remove that reference rather than reintroducing a compatibility facade.

## Artifacts and Notes

Validation artifacts are written by the harness under `.scenery/harness/`. The deletion intentionally leaves historical plan references alone, but current docs, schemas, examples, fixtures, UI registry entries, and command usage must be clean.

The first self-harness run after the removal wrote `.scenery/harness/self-latest.json` with failures for `docs/knowledge.json` and this ExecPlan structure; those are cleanup items in this same plan.

## Interfaces and Dependencies

Removed public interfaces:

- Go import path `scenery.sh/data`.
- CLI subject `scenery inspect data`.
- Dashboard RPC methods `data/inspect`, `data/query-records`, and `data/outbox-events`.
- Dashboard route and registry item for Data Explorer.
- JSON schemas `scenery.data.export.v1` and `scenery.inspect.data.v1`.

Remaining dependencies such as Postgres helpers and `pgxpool` are not objectstore-specific and stay available for auth, dev services, and other runtime features.
