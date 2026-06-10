# CLI Help and Human Session Status

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

Scenery's root help currently behaves like a full grammar dump. That is precise
for agents, but noisy for humans: long command lines wrap, related commands are
hard to scan, and the first impression hides the product model behind flags.
Root help should orient. Detailed grammar belongs in command-specific help,
`scenery help all`, and machine-readable help.

This plan changes the CLI help contract so:

```sh
scenery
scenery help
scenery help <command>
scenery help all
scenery help --json
```

are first-class surfaces with separate human and machine roles. The root output
introduces Scenery in grouped, under-80-column prose. `scenery help all` prints the
complete grouped command reference without expanding every flag in the root
view. `scenery help <command>` owns exact command usage, flags, and subcommands.
`scenery help --json` exposes a stable command manifest for agents, harnesses,
and documentation checks.

This plan also changes `scenery ps` to have a useful human default table. Agents
and automation continue to use `scenery ps --json`, with the existing JSON shape
preserved unless the implementation explicitly updates schemas, docs, and tests.

The observable end state is that a new user can run `scenery` or `scenery help`
and see a calm command overview, while an agent can run `scenery help --json` or
`scenery ps --json` and get stable machine-readable contracts.

## Progress

- [x] 2026-06-09: Created ExecPlan `0069-cli-help-and-human-ps.md` from the requested CLI help and `ps` brief.
- [x] 2026-06-09: Implemented command metadata and root/help routing.
- [x] 2026-06-09: Added `scenery help all`, command-specific help, and `help --json`.
- [x] 2026-06-09: Made `scenery ps` render a human table by default while preserving `scenery ps --json`.
- [x] 2026-06-09: Updated CLI contract docs, README, agent guidance, installable skill, schemas, and harness drift checks.
- [x] 2026-06-09: Validated with focused CLI tests, full Go tests, docs inspection, source-driven help smokes, and self-harness.

## Surprises & Discoveries

- 2026-06-09: `scenery inspect docs --json` reported 42 documents, 0 missing, 1 review-due document, and 0 stale documents. The review-due document is `docs/ui-agent-contract.md`, which is unrelated to this CLI help work.
- 2026-06-09: `cmd/scenery/main.go` currently stores root usage as one large literal string in `usageError()`, including `scenery ps --json` as the advertised form.
- 2026-06-09: `cmd/scenery/agent.go` already allows `scenery ps` without `--json`, but its human output is an unheaded tab-separated list of `session_id`, `status`, and `app_root`. The plan should improve that output rather than inventing a new status backend.
- 2026-06-09: `cmd/scenery/harness_drift.go` currently checks for exact usage needles inside `usageError().Error()`. This must move toward command metadata or updated smoke checks so root help can stay compact without failing the drift harness.
- 2026-06-09: Adding the human `ps` table helpers directly to `cmd/scenery/agent.go` pushed that file above the 1000-line architecture warning threshold. The helpers were moved into `cmd/scenery/agent_status_table.go` to keep the change focused and avoid worsening an existing large-file hotspot.

## Decision Log

- 2026-06-09: Use root help for orientation, command help for flags, and `help all` for the complete command list. Rationale: it preserves discoverability without making every invocation print the entire CLI grammar.
- 2026-06-09: Keep `scenery ps --json` as the machine contract and make bare `scenery ps` human-readable. Rationale: this matches the broader CLI principle that human output is default and JSON is explicit, while preserving agent workflows.
- 2026-06-09: Add `scenery help --json` instead of requiring agents to scrape text help. Rationale: help/manifest drift is best handled as structured data with tests and docs.
- 2026-06-09: Add `docs/schemas/scenery.help.v1.schema.json` and include help JSON in self-harness schema validation. Rationale: `help --json` is a stable machine-readable surface and should be validated like other command JSON contracts.

## Outcomes & Retrospective

Completed on 2026-06-09.

Shipped:

- Compact root help for bare `scenery` and `scenery help`.
- Grouped full command reference through `scenery help all`.
- Command-specific help through `scenery help <command>`, including nested topic resolution such as `db branch status`.
- Stable machine-readable command manifest through `scenery help --json` with schema `scenery.help.v1`.
- Human table output for bare `scenery ps`, with `scenery ps --json` preserving the existing `scenery.agent.status.v1` response shape.
- Updated drift checks, schema validation, CLI contract docs, README, agent guide, installable skill, and tests.

Validation:

- `go test ./cmd/scenery` passed.
- `go test ./...` passed.
- `go run ./cmd/scenery help --json | python3 -m json.tool` passed.
- Source-driven smoke tests for `go run ./cmd/scenery`, `go run ./cmd/scenery help all`, and `go run ./cmd/scenery help logs` passed.
- `go run ./cmd/scenery inspect docs --json` passed with 43 documents, 0 missing, 1 review-due document, and 0 stale documents.
- `go run ./cmd/scenery harness self --summary --write` passed with warnings only. The remaining warnings are the known review-due UI contract doc, existing large-file warnings, and slow-test timing warnings; schema validation, contract drift checks, focused tests, and full Go tests all passed.

## Context and Orientation

Start with these files:

- `cmd/scenery/main.go` owns top-level dispatch and the current `usageError()` grammar dump.
- `cmd/scenery/agent.go` owns `statusCommand`, `parseStatusArgs`, `writeStatus`, and therefore `scenery ps`.
- `cmd/scenery/harness_drift.go` contains self-harness CLI contract checks that currently inspect root usage text.
- `cmd/scenery/*_test.go`, especially `run_json_test.go`, `agent_test.go`, and any main/usage tests added during this plan, should cover parser behavior and output contracts.
- `docs/local-contract.md` is the source of truth for CLI grammar, JSON contracts, stability labels, and command output semantics.
- `docs/agent-guide.md`, `SKILL.md`, and `README.md` contain user- and agent-facing command examples that currently emphasize `scenery ps --json`.
- `docs/knowledge.json` indexes active ExecPlans until deterministic plan indexing exists.

The target root help should be materially equivalent to:

```text
Scenery - build, run, and inspect app services.

Usage:
  scenery <command> [args] [flags]
  scenery help <command>
  scenery help all
  scenery help --json

Local session:
  up         Start a local dev session
  ps         Show local sessions
  logs       Read, follow, or query session logs
  console    Open the source-aware dev console
  down       Stop a session
  prune      Remove old stopped session state

Build and runtime:
  serve      Run the API server once
  worker     Run workers and manage worker deployments
  build      Build the deployable binary
  check      Check the app model
  test       Run Go tests

App resources:
  inspect    Inspect app model and diagnostics as JSON
  generate   Generate clients, SQLC, and configured outputs
  db         Manage database lifecycle, branches, and local Neon
  task       List, inspect, graph, and run app tasks
  validate   Run validation profiles
  harness    Run Scenery harnesses

Workspace:
  worktree   Create, list, and remove app worktrees

Observability:
  traces     List or clear local traces
  metrics    List, query, and inspect local metrics

System:
  doctor     Check host and app readiness
  version    Print version information
  system     Manage agent, edge, trust, and toolchain

Use "scenery help <command>" for flags and subcommands.
```

Use ASCII punctuation in repository files unless an existing file or explicit
product copy requires otherwise. The pasted brief used an em dash in the first
line; this plan intentionally uses a hyphen for repo consistency.

`scenery help all` should print grouped command invocations such as:

```text
Scenery command reference

Usage:
  scenery <command> [args] [flags]
  scenery help <command>

Local session:
  scenery up
  scenery ps
  scenery logs
  scenery logs query
  scenery logs tail
  scenery console
  scenery down
  scenery prune

Database:
  scenery db psql
  scenery db apply
  scenery db seed
  scenery db setup
  scenery db reset
  scenery db drop
  scenery db snapshot create
  scenery db snapshot restore
  scenery db branch status
  scenery db branch list
  scenery db branch checkout
  scenery db branch reset
  scenery db branch delete
  scenery db branch restore
  scenery db branch diff
  scenery db branch expire
  scenery db branch prune
  scenery db neon install
  scenery db neon start
  scenery db neon status
  scenery db neon logs
  scenery db neon stop
  scenery db neon restart
  scenery db neon uninstall

Use "scenery help <command>" for exact flags.
Use "scenery help --json" for the machine-readable command manifest.
```

The implementation should include the rest of the command groups from the
current CLI: workspace, generation, tasks, validation, runtime, build/checks,
harness, inspection, observability, and system.

## Milestones

1. Help Metadata Foundation: replace the single root usage literal with a small
   command-help registry that can render root orientation, grouped full
   reference, command-specific help, and JSON manifest from one source.
2. Help CLI Surface: add dispatch for `scenery help`, `scenery help all`,
   `scenery help <command>`, and `scenery help --json`; keep unknown-command
   errors concise and point users toward `scenery help`.
3. Human `ps`: improve bare `scenery ps` into a headed, aligned human table with
   enough session information to be useful, and keep `scenery ps --json` stable.
4. Contract and Drift Updates: update docs, README, skill text, and
   self-harness CLI drift checks to use structured help metadata instead of
   assuming the root help contains every grammar line.
5. Validation: add tests for root help width/content, `help all`, representative
   command help, help JSON, `ps` human output, and `ps --json` compatibility.

## Plan of Work

First inventory the full command surface from the current dispatchers and docs.
The registry should include command group, command path, one-line summary,
usage forms, common flags, subcommands, JSON support, and stability where the
contract already names it. Do not delete command support while reshaping help.

Then introduce rendering functions in `cmd/scenery/main.go` or a new focused
file such as `cmd/scenery/help.go`. Keep the data close to the CLI package so
parser tests can call it. The root renderer should be intentionally short and
should not include full flag lists. The full renderer should group command
paths. The command renderer should include exact usage and flags for one command
family, starting with high-use commands such as `logs`, `ps`, `db`, `task`,
`validate`, `harness`, `inspect`, `worker`, and `system`.

Next wire `help` into top-level `run(args []string)`. Bare `scenery` and
`scenery help` should print the root help. `scenery help all` should print the
full command reference. `scenery help --json` should emit an indented JSON
manifest with a schema version. `scenery help <command>` should resolve command
families such as `db branch`, `logs`, and `worker deployment` without requiring
the caller to know an internal parser type.

After help is stable, update `writeStatus` in `cmd/scenery/agent.go`. Bare
`scenery ps` should render a table with stable columns. A practical first table
is:

```text
SESSION     STATUS    APP        ROUTE/API                         UPDATED
dev-a       running   /repo/app   https://api.dev-a.onlv.dev/       2m ago
```

If there are no sessions, print a short human message such as `No scenery
sessions found.` and return success. If `--watch` is used in human mode, keep
the separator or clear marker simple and testable. `--json` should continue to
encode `schema_version`, `agent`, `sessions`, and `substrates`.

Finally update documentation and drift checks. `docs/local-contract.md` should
describe the help surfaces and change the grammar line from `scenery ps --json`
to `scenery ps [--json] ...` or equivalent. README and agent-facing docs should
show bare `scenery ps` for humans and `scenery ps --json` for automation.
Self-harness should validate that command metadata can render expected entries
and that JSON help includes stable commands, rather than requiring the root
string to contain every grammar line.

## Concrete Steps

1. Add help metadata and renderers.
   - Add `cmd/scenery/help.go` if that keeps `main.go` readable.
   - Add structs for command groups and command entries.
   - Render root help, full help, command help, and JSON manifest from the same
     registry.
   - Keep output lines under 80 columns for root help.
2. Wire top-level command behavior.
   - Make bare `scenery` and `scenery help` print root help.
   - Add `scenery help all`.
   - Add `scenery help --json`.
   - Add representative `scenery help <command>` entries before expanding the
     command-specific text to the full surface.
3. Improve `scenery ps`.
   - Keep `parseStatusArgs` accepting `--json`, `--watch`, `--app-root`, and
     `--session`.
   - Replace the current unheaded tab-separated human output with a headed
     table.
   - Preserve JSON output shape and filtering behavior.
4. Update drift and tests.
   - Adjust `cmd/scenery/harness_drift.go` to inspect help metadata or command
     help output instead of the old root usage dump.
   - Add focused tests for help rendering and `ps` output.
   - Ensure existing status JSON tests still pass.
5. Update docs.
   - Update `docs/local-contract.md` CLI grammar and help contract.
   - Update `docs/agent-guide.md`, `SKILL.md`, and `README.md` examples.
   - Update `docs/knowledge.json` if plan status or indexed docs change during
     implementation.

## Validation and Acceptance

Run these commands from the repository root:

```sh
go test ./cmd/scenery
go test ./...
scenery inspect docs --json
scenery harness self --summary --write
```

If self-harness is too expensive for an intermediate stop, run it before marking
the plan complete unless a local environment blocker is recorded in
`Surprises & Discoveries`.

Manual source-driven smoke tests should use the worktree source, for example:

```sh
go run ./cmd/scenery
go run ./cmd/scenery help
go run ./cmd/scenery help all
go run ./cmd/scenery help logs
go run ./cmd/scenery help --json
go run ./cmd/scenery ps
go run ./cmd/scenery ps --json
```

Acceptance criteria:

- Root help is grouped, orienting, and avoids full flag grammar.
- Root help lines stay under 80 columns, except where terminal rendering of a
  path or URL makes that impossible.
- `scenery help all` lists the complete command reference grouped by domain.
- `scenery help <command>` provides exact usage and flags for the requested
  command family.
- `scenery help --json` returns a stable schema version and command entries
  suitable for agents and drift checks.
- Bare `scenery ps` returns a readable human table or a clear empty-state
  message.
- `scenery ps --json` keeps the existing `scenery.agent.status.v1` response shape.
- Docs and self-harness drift checks agree with the new help contract.

## Idempotence and Recovery

The help registry is pure data and render logic. If a command-specific help
entry is incomplete, rerun the focused help tests, update the registry entry,
and rerun the same tests. If the JSON manifest changes, update schemas or docs
in the same change only when the JSON shape is intentionally part of the stable
contract.

For `scenery ps`, keep all session reads through the existing local agent client.
Do not add another session store reader. If human table formatting regresses,
fall back to the existing session list and adjust only formatting. Preserve
`--json` as the recovery path for agents.

If `scenery harness self` flags CLI contract drift after root help is shortened,
update the drift check to query structured help metadata or `help all`; do not
restore the root grammar dump to satisfy the old check.

## Artifacts and Notes

The source brief requested these behavior changes:

- root help should not print the full grammar;
- detailed grammar belongs in `scenery help <command>` and `scenery help all`;
- root help should group commands under Local session, Build and runtime, App
  resources, Workspace, Observability, and System;
- `scenery ps` should have a human default table;
- `scenery ps --json` remains the agent-facing machine contract;
- command-specific help should include examples like `scenery help logs` with
  usage, commands, common flags, filter flags, and follow behavior.

No generated artifacts are expected from this plan. Harness artifacts may be
written under `.scenery/harness/` during validation and must not be committed.

## Interfaces and Dependencies

Affected interfaces:

- CLI text output for `scenery`, `scenery help`, `scenery help all`, and
  `scenery help <command>`.
- New CLI JSON output for `scenery help --json`.
- CLI text output for bare `scenery ps`.
- Existing CLI JSON output for `scenery ps --json`, which should remain stable.
- Documentation contract in `docs/local-contract.md`.
- Self-harness CLI drift checks in `cmd/scenery/harness_drift.go`.

Dependencies and constraints:

- Prefer Go standard library formatting, such as `text/tabwriter`, for human
  tables.
- Do not add external dependencies for help rendering.
- Keep command metadata small and explicit.
- Keep machine-readable JSON behind `--json`; do not make agents scrape human
  tables.
- Preserve current command parsers and behavior unless this plan explicitly
  names a change.
