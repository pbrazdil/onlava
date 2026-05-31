# Dev Event Backend Cutover and Parity

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

`docs/plans/0055-structured-dev-events-and-console.md` delivered the first structured dev event plane: source-aware `onlava.dev.event.v1` records, SQLite `dev_sources`/`dev_events` storage, `onlava logs`/`onlava attach` filters, an observing-only terminal console, and VictoriaLogs dual-write/read support for side-by-side verification.

The next step is to make the dev event backend boring, unified, and trustworthy. Today the implementation has multiple backend paths:

```text
producer
  -> SQLite dev_events
  -> VictoriaLogs dual-write

onlava logs
  -> --backend auto|victoria|sqlite

onlava console / attach --tui
  -> SQLite store only

legacy fallback
  -> process_output rows
```

That is acceptable for 0055, but it is not the final contract. It risks drift between CLI logs, the TUI, dashboard views, agent consumers, and future retention/prune behavior.

After this work, all dev-event consumers use one backend abstraction with the same query and follow semantics. VictoriaLogs can become the preferred read backend in `auto` mode without breaking local compatibility. SQLite remains a compatibility cache and fallback while the project proves parity and retention behavior.

## Progress

- [x] 2026-05-31: Created this ExecPlan and linked it from `docs/plans/active.md`.
- [x] 2026-06-01: Added a backend-neutral dev-event reader abstraction for CLI logs, attach follow, and the terminal console, with SQLite and VictoriaLogs implementations.
- [x] 2026-06-01: Made dev event IDs store-assigned before SQLite insertion and Victoria export by adding a persistent SQLite sequence and explicit insert IDs.
- [x] 2026-06-01: Made `onlava console` and `onlava attach --tui` honor the same `--backend auto|victoria|sqlite` option path as `onlava logs`.
- [x] 2026-06-01: Added `onlava logs compare` as read-only parity tooling for SQLite vs VictoriaLogs event streams.
- [x] 2026-06-01: Documented backend selection, auto fallback, TUI backend behavior, compare output, and stable producer-owned IDs in `docs/local-contract.md`.
- [x] 2026-06-01: Defined prune behavior and made `onlava prune` delete matching SQLite `dev_events`/`dev_sources` cache rows for pruned sessions while leaving VictoriaLogs storage untouched.
- [x] 2026-06-01: Added changed-area harness guidance that recommends `onlava logs compare --session current --backend-a sqlite --backend-b victoria --limit 500 --json` when dev-event backend files change.
- [x] 2026-06-01: Ran focused tests, full tests, install, diff checks, and self harness. Functional tests and install passed; self harness wrote `.onlava/harness/self-latest.json` but still failed the known Go test timing budget.

## Surprises & Discoveries

Record implementation findings here with commands, test output, or file references.

- 2026-05-31: 0055 is already present in `docs/plans/active.md` and claims completion of the structured event plane, CLI filters, terminal console, and VictoriaLogs backend verification path.
- 2026-05-31: `cmd/onlava/logs.go` already has `--backend auto|victoria|sqlite`, but `cmd/onlava/dev_console.go` refreshes directly from the SQLite `devdash.Store`.
- 2026-05-31: `internal/devdash/dev_events.go` assigns event IDs from SQLite via `LastInsertId`. If VictoriaLogs becomes canonical, IDs need to be assigned before storage.
- 2026-05-31: 0055 reports `onlava harness self --json --write` as functionally green except the existing Go test timing budget. Do not hide that failure; either fix timing separately or annotate it as a known non-functional gate.
- 2026-06-01: Local agent state can make `--backend auto` discover VictoriaLogs even in unit tests. Plain non-following logs still need legacy process-output fallback when no structured rows exist, while fresh follow mode must keep the selected backend even if the first query is empty.
- 2026-06-01: The SQLite version available through `modernc.org/sqlite` accepts `insert or ignore ... values (...)` for sequence initialization, but rejected the original `insert ... select ... on conflict do nothing` form.
- 2026-06-01: `onlava harness self --json --write` still fails outside this implementation scope because the harness Go test timing gate exceeded the 7.000s target. The latest run reported 8.704s total and passed install, changed-area, knowledge, architecture, drift, UI typecheck/build, fixture matrix, and schema validation.

## Decision Log

- Decision: Unify the backend API before adding more console features.
  Rationale: A richer TUI on top of split SQLite/Victoria paths would create duplicated behavior and parity bugs. Backend semantics are the foundation.
  Date/Author: 2026-05-31 / Codex

- Decision: Use producer-owned monotonic event IDs before VictoriaLogs becomes canonical.
  Rationale: Followers and JSONL consumers rely on stable integer `id` ordering. SQLite autoincrement cannot remain the source of truth when SQLite is only a fallback/cache.
  Date/Author: 2026-05-31 / Codex

- Decision: Keep SQLite readable as a compatibility cache for now.
  Rationale: Existing local databases, fallback behavior, and legacy `process_output` rows still matter. The cutover should be reversible until parity is proven by harness checks.
  Date/Author: 2026-05-31 / Codex

- Decision: `--backend auto` must have deterministic empty-stream behavior.
  Rationale: A fresh `--follow` should not permanently choose SQLite only because VictoriaLogs had no rows at startup. Backend selection must use session/backend availability, not only initial result count.
  Date/Author: 2026-05-31 / Codex

- Decision: Keep legacy process-output fallback for non-following plain logs when the selected backend has no structured events.
  Rationale: Existing local databases and tests may only have `process_output` rows. Auto-selecting VictoriaLogs must not make old plain logs appear empty, but follow mode still needs deterministic empty-stream behavior.
  Date/Author: 2026-06-01 / Codex

- Decision: Use a persistent SQLite global dev-event sequence for producer-owned IDs in this pass.
  Rationale: The existing SQLite schema uses `dev_events.id` as a table-wide primary key. A global sequence preserves current ordering and avoids a disruptive row-id/event-id migration while still assigning IDs before backend writes.
  Date/Author: 2026-06-01 / Codex

## Outcomes & Retrospective

Completed on 2026-06-01.

The implementation adds a shared dev-event backend reader used by `onlava logs`, plain `attach` following, `attach --tui`, and `onlava console`. SQLite and VictoriaLogs now sit behind the same list/source/follow interface, and `--backend auto` has deterministic empty-follow behavior while preserving legacy process-output fallback for older non-following plain logs.

Dev event IDs are assigned before SQLite insertion through a persistent store-backed sequence, so SQLite and VictoriaLogs can receive the same public event ID. `onlava logs compare` provides a read-only parity oracle for SQLite and VictoriaLogs streams, and the changed-area harness now recommends that comparison when dev-event backend files change.

`onlava prune` now deletes matching SQLite `dev_events`/`dev_sources` compatibility-cache rows for pruned sessions. VictoriaLogs retention remains explicitly outside `prune` and is documented as storage-owned by the VictoriaLogs sidecar. Validation passed for focused tests, the full Go suite, diff checks, and binary install. The self harness snapshot was written and passed functional steps, but still failed the existing Go test timing budget.

Expected outcome:

- `onlava logs`, `onlava attach`, `onlava attach --tui`, `onlava console`, and dashboard/agent dev-event consumers use one backend abstraction.
- `--backend auto` prefers the configured/current dev-event backend deterministically and falls back only under documented conditions.
- `--backend victoria` and `--backend sqlite` produce equivalent JSONL for the same session/query while dual-write is enabled.
- VictoriaLogs can serve follow, source/kind/level/grep/since filters, and empty-session follow correctly.
- SQLite remains a bounded compatibility cache and legacy local database reader, not the hidden canonical source.
- The harness has a repeatable parity check that catches schema/query drift.

## Context and Orientation

0055 moved the repo from raw process output toward source-aware structured dev events:

```text
internal/devdash/types.go
internal/devdash/dev_events.go
internal/devdash/store.go
cmd/onlava/logs.go
cmd/onlava/dev_console.go
cmd/onlava/dev_supervisor.go
cmd/onlava/dev_typescript.go
cmd/onlava/dev_frontends.go
cmd/onlava/victoria.go
docs/schemas/onlava.dev.event.v1.schema.json
docs/local-contract.md
docs/plans/0055-structured-dev-events-and-console.md
```

Current observed state:

- `onlava.dev.event.v1` exists and requires app, id, time, source, level, message, and parse fields.
- `dev_sources` and `dev_events` are SQLite tables with source, level, message, fields, raw output, parse metadata, and indexes.
- `onlava logs` accepts `--source`, `--kind`, `--level`, `--grep`, `--since`, and `--backend auto|victoria|sqlite`.
- `onlava attach --tui` and `onlava console` exist and provide an observing-only terminal cockpit.
- The TUI currently uses `store.ListDevEvents` / `store.ListDevSources` directly.
- 0055 added VictoriaLogs dual-write and backend reads for plain logs/attach, but the cutover is not yet hardened.

## Milestones

### Milestone 1: Backend abstraction

Introduce an interface in a package that both CLI and dashboard code can use without circular dependencies:

```go
type DevEventBackend interface {
    ListDevEvents(ctx context.Context, query devdash.DevEventQuery) ([]devdash.DevEvent, error)
    ListDevSources(ctx context.Context, appID, sessionID string) ([]devdash.DevSource, error)
    FollowDevEvents(ctx context.Context, query devdash.DevEventQuery, afterID int64) (<-chan []devdash.DevEvent, <-chan error)
    BackendName() string
}
```

The exact shape can differ, but the point is that `logs.go` and `dev_console.go` should not independently decide how to read SQLite or VictoriaLogs.

Acceptance:

- `onlava logs` and `onlava console` consume the same backend selection helper.
- Focused tests cover backend selection for `auto`, `sqlite`, `victoria`, unavailable Victoria, and empty result sets.
- No consumer has to duplicate query/follow loops for SQLite vs Victoria unless wrapped behind the abstraction.

### Milestone 2: Producer-owned event IDs

Stop relying on SQLite `LastInsertId` as the canonical event id.

Options:

1. Session-local monotonic sequence stored in the supervisor and included in both SQLite and Victoria writes.
2. Store-backed sequence row keyed by `(app_id, session_id)` for restart continuity.
3. Hybrid timestamp + sequence internally, while keeping JSON `id` as integer.

Prefer the simplest stable integer sequence that survives supervisor restart within the same session. If exact restart continuity is too expensive, document the boundary and ensure follows cannot drop events.

Acceptance:

- `devdash.DevEvent.ID` is assigned before writing to any backend.
- SQLite insert accepts the provided ID or stores a separate `event_id` that JSONL uses.
- VictoriaLogs receives the same ID.
- Tests prove ordering and follow-after-ID semantics without depending on SQLite autoincrement.

### Milestone 3: Victoria backend parity

Make VictoriaLogs a first-class read backend for all current dev event queries:

```sh
onlava logs --backend victoria --jsonl
onlava logs --backend victoria --source api
onlava logs --backend victoria --kind frontend
onlava logs --backend victoria --level error
onlava logs --backend victoria --grep "activity failed"
onlava logs --backend victoria --since 15m
onlava logs --backend victoria --follow
```

Acceptance:

- Query behavior matches SQLite for source, kind, level, stream, grep, since, limit, and after-id/follow.
- JSONL events are schema-valid and stable across backends.
- Victoria query failures in `--backend victoria` fail loudly.
- Victoria query failures in `--backend auto` fall back only with a clear warning or structured diagnostic.

### Milestone 4: Console/TUI backend support

Teach `onlava console` and `onlava attach --tui` to honor backend selection.

CLI target:

```sh
onlava console --backend auto
onlava console --backend victoria
onlava console --backend sqlite
onlava attach --tui --backend victoria
```

Acceptance:

- Console source summaries, grouped errors, search, errors-only mode, and expanded JSON use the selected backend.
- Non-TTY fallback preserves the same backend option when it calls plain attach/log following.
- A focused rendering test covers SQLite and a fake Victoria backend through the same abstraction.

### Milestone 5: Backend parity oracle

Add a dev/harness-visible comparison command or harness check.

Possible CLI:

```sh
onlava logs compare --session current --backend-a sqlite --backend-b victoria --limit 500 --json
```

Possible harness integration:

```sh
onlava harness self --json --write
```

with a changed-area check that runs a synthetic dev-event parity test when `cmd/onlava/logs.go`, `cmd/onlava/dev_console.go`, `internal/devdash/dev_events.go`, or Victoria code changes.

Acceptance:

- Parity check compares event count, IDs, source IDs, levels, messages, raw text, parse metadata, and selected fields.
- It tolerates known backend-specific metadata such as Victoria ingest time if not part of the public schema.
- It emits machine-readable mismatch diagnostics.

### Milestone 6: Retention, prune, and failure policy

Define what happens to dev events over time.

Questions to answer in the implementation:

- Is SQLite a bounded cache or an indefinite local history?
- Does `onlava prune` delete SQLite dev events, VictoriaLogs data, or both?
- What happens when Victoria is enabled but unavailable after a session starts?
- Does `ONLAVA_LOGS_BACKEND` affect TUI and dashboard reads?
- How does a user force fully local SQLite-only mode?

Acceptance:

- `docs/local-contract.md` describes dev-event backend modes and fallback behavior.
- `onlava prune` behavior is explicit and tested for dev events/source state.
- CLI warnings are quiet in JSON mode and structured where applicable.

## Plan of Work

Start by introducing the backend-neutral read/follow abstraction while preserving the current SQLite and VictoriaLogs behavior behind concrete implementations. Once logs and console consumers share that interface, move event ID assignment to the producer side so both backends can store and return the same stable IDs.

After the shared backend path is in place, make VictoriaLogs pass the same query and follow semantics as SQLite, wire the TUI and attach paths through backend selection, then add a parity oracle that agents and humans can run repeatedly. Finish by documenting retention, prune, fallback, and local-only behavior so the backend cutover has an explicit operational contract.

## Concrete Steps

1. Add `DevEventBackend` and backend selection helpers.
2. Move SQLite query/follow logic behind a SQLite backend implementation.
3. Move Victoria query/follow logic behind a Victoria backend implementation.
4. Replace duplicated `followDevEvents` / `followVictoriaDevEvents` call sites with backend-driven follow.
5. Update `dev_console.go` to use the backend interface instead of direct store calls.
6. Add producer-owned event id assignment and preserve IDs in both backends.
7. Add parity test fixtures with fake backends and, where practical, a real VictoriaLogs integration path guarded by existing env/download controls.
8. Add CLI docs and usage updates.
9. Update this plan’s Progress, Surprises & Discoveries, Decision Log, and Outcomes.

## Validation and Acceptance

Required validation:

```sh
go test ./internal/devdash ./cmd/onlava
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
git diff --check
```

Manual or scripted scenario validation:

```sh
onlava dev --detach --new-session
onlava logs --session current --backend sqlite --jsonl --limit 100 > /tmp/onlava-sqlite.jsonl
onlava logs --session current --backend victoria --jsonl --limit 100 > /tmp/onlava-victoria.jsonl
onlava logs compare --session current --backend-a sqlite --backend-b victoria --limit 100 --json
onlava attach --tui --backend victoria
onlava console --backend sqlite
onlava down --session current
```

Acceptance criteria:

- The same query returns equivalent public JSONL events from SQLite and Victoria while dual-write is enabled.
- A fresh `--follow --backend auto` on a quiet session still follows the preferred backend correctly when new events arrive.
- TUI behavior is independent of backend selection.
- `--backend victoria` is explicit and fails clearly if VictoriaLogs is not available.
- `--backend auto` fallback is deterministic and documented.
- Harness output either passes or reports only explicitly pre-existing, non-functional timing-budget failures.

## Idempotence and Recovery

- Re-running migrations must not duplicate source/event rows.
- Backend parity tooling must be read-only.
- If Victoria is unavailable, SQLite fallback must continue to serve existing local sessions.
- If SQLite is unavailable but Victoria has events, `--backend victoria` should still read events.
- `onlava down` must not depend on the dev-event backend.

## Artifacts and Notes

Expected changed artifacts:

```text
docs/plans/0056-dev-event-backend-cutover-and-parity.md
docs/plans/active.md
cmd/onlava/logs.go
cmd/onlava/dev_console.go
cmd/onlava/*logs*test.go
cmd/onlava/*console*test.go
cmd/onlava/victoria.go
internal/devdash/dev_events.go
internal/devdash/store.go
internal/devdash/*test.go
docs/local-contract.md
docs/schemas/onlava.dev.event.v1.schema.json
```

Potential later follow-up, not in this plan:

- Mutating console controls such as restart selected service, stop worker, clear backend logs, or open URL.
- Full dashboard migration to VictoriaLogs as the only local log browser backend.
- Removing legacy `process_output` writes after at least one compatibility window.

## Interfaces and Dependencies

Expected public CLI surface:

```sh
onlava logs --backend auto|victoria|sqlite
onlava console --backend auto|victoria|sqlite
onlava attach --tui --backend auto|victoria|sqlite
onlava logs compare --session current --backend-a sqlite --backend-b victoria --limit 500 --json
```

Internal interface names may change during implementation, but the shared reader should expose list, source-list, follow, and backend-name behavior for dev events. The implementation should stay in Go and avoid new dependencies unless a concrete backend-parity or TUI payoff justifies them.
