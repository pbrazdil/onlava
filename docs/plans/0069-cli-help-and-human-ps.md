# CLI Help and Human Session Status

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

Onlava's root help currently behaves like a full grammar dump. That is precise
for agents, but noisy for humans: long command lines wrap, related commands are
hard to scan, and the first impression hides the product model behind flags.
Root help should orient. Detailed grammar belongs in command-specific help,
`onlava help all`, and machine-readable help.

This plan changes the CLI help contract so:

```sh
onlava
onlava help
onlava help <command>
onlava help all
onlava help --json
```

are first-class surfaces with separate human and machine roles. The root output
introduces Onlava in grouped, under-80-column prose. `onlava help all` prints the
complete grouped command reference without expanding every flag in the root
view. `onlava help <command>` owns exact command usage, flags, and subcommands.
`onlava help --json` exposes a stable command manifest for agents, harnesses,
and documentation checks.

This plan also changes `onlava ps` to have a useful human default table. Agents
and automation continue to use `onlava ps --json`, with the existing JSON shape
preserved unless the implementation explicitly updates schemas, docs, and tests.

The observable end state is that a new user can run `onlava` or `onlava help`
and see a calm command overview, while an agent can run `onlava help --json` or
`onlava ps --json` and get stable machine-readable contracts.

## Progress

- [x] 2026-06-09: Created ExecPlan `0069-cli-help-and-human-ps.md` from the requested CLI help and `ps` brief.
- [x] 2026-06-09: Implemented command metadata and root/help routing.
- [x] 2026-06-09: Added `onlava help all`, command-specific help, and `help --json`.
- [x] 2026-06-09: Made `onlava ps` render a human table by default while preserving `onlava ps --json`.
- [x] 2026-06-09: Updated CLI contract docs, README, agent guidance, installable skill, schemas, and harness drift checks.
- [x] 2026-06-09: Validated with focused CLI tests, full Go tests, docs inspection, source-driven help smokes, and self-harness.

## Surprises & Discoveries

- 2026-06-09: `onlava inspect docs --json` reported 42 documents, 0 missing, 1 review-due document, and 0 stale documents. The review-due document is `docs/ui-agent-contract.md`, which is unrelated to this CLI help work.
- 2026-06-09: `cmd/onlava/main.go` currently stores root usage as one large literal string in `usageError()`, including `onlava ps --json` as the advertised form.
- 2026-06-09: `cmd/onlava/agent.go` already allows `onlava ps` without `--json`, but its human output is an unheaded tab-separated list of `session_id`, `status`, and `app_root`. The plan should improve that output rather than inventing a new status backend.
- 2026-06-09: `cmd/onlava/harness_drift.go` currently checks for exact usage needles inside `usageError().Error()`. This must move toward command metadata or updated smoke checks so root help can stay compact without failing the drift harness.
- 2026-06-09: Adding the human `ps` table helpers directly to `cmd/onlava/agent.go` pushed that file above the 1000-line architecture warning threshold. The helpers were moved into `cmd/onlava/agent_status_table.go` to keep the change focused and avoid worsening an existing large-file hotspot.

## Decision Log

- 2026-06-09: Use root help for orientation, command help for flags, and `help all` for the complete command list. Rationale: it preserves discoverability without making every invocation print the entire CLI grammar.
- 2026-06-09: Keep `onlava ps --json` as the machine contract and make bare `onlava ps` human-readable. Rationale: this matches the broader CLI principle that human output is default and JSON is explicit, while preserving agent workflows.
- 2026-06-09: Add `onlava help --json` instead of requiring agents to scrape text help. Rationale: help/manifest drift is best handled as structured data with tests and docs.
- 2026-06-09: Add `docs/schemas/onlava.help.v1.schema.json` and include help JSON in self-harness schema validation. Rationale: `help --json` is a stable machine-readable surface and should be validated like other command JSON contracts.

## Outcomes & Retrospective

Completed on 2026-06-09.

Shipped:

- Compact root help for bare `onlava` and `onlava help`.
- Grouped full command reference through `onlava help all`.
- Command-specific help through `onlava help <command>`, including nested topic resolution such as `db branch status`.
- Stable machine-readable command manifest through `onlava help --json` with schema `onlava.help.v1`.
- Human table output for bare `onlava ps`, with `onlava ps --json` preserving the existing `onlava.agent.status.v1` response shape.
- Updated drift checks, schema validation, CLI contract docs, README, agent guide, installable skill, and tests.

Validation:

- `go test ./cmd/onlava` passed.
- `go test ./...` passed.
- `go run ./cmd/onlava help --json | python3 -m json.tool` passed.
- Source-driven smoke tests for `go run ./cmd/onlava`, `go run ./cmd/onlava help all`, and `go run ./cmd/onlava help logs` passed.
- `go run ./cmd/onlava inspect docs --json` passed with 43 documents, 0 missing, 1 review-due document, and 0 stale documents.
- `go run ./cmd/onlava harness self --summary --write` passed with warnings only. The remaining warnings are the known review-due UI contract doc, existing large-file warnings, and slow-test timing warnings; schema validation, contract drift checks, focused tests, and full Go tests all passed.

## Context and Orientation

Start with these files:

- `cmd/onlava/main.go` owns top-level dispatch and the current `usageError()` grammar dump.
- `cmd/onlava/agent.go` owns `statusCommand`, `parseStatusArgs`, `writeStatus`, and therefore `onlava ps`.
- `cmd/onlava/harness_drift.go` contains self-harness CLI contract checks that currently inspect root usage text.
- `cmd/onlava/*_test.go`, especially `run_json_test.go`, `agent_test.go`, and any main/usage tests added during this plan, should cover parser behavior and output contracts.
- `docs/local-contract.md` is the source of truth for CLI grammar, JSON contracts, stability labels, and command output semantics.
- `docs/agent-guide.md`, `SKILL.md`, and `README.md` contain user- and agent-facing command examples that currently emphasize `onlava ps --json`.
- `docs/knowledge.json` indexes active ExecPlans until deterministic plan indexing exists.

The target root help should be materially equivalent to:

```text
Onlava - build, run, and inspect app services.

Usage:
  onlava <command> [args] [flags]
  onlava help <command>
  onlava help all
  onlava help --json

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
  harness    Run Onlava harnesses

Workspace:
  worktree   Create, list, and remove app worktrees

Observability:
  traces     List or clear local traces
  metrics    List, query, and inspect local metrics

System:
  doctor     Check host and app readiness
  version    Print version information
  system     Manage agent, edge, trust, and toolchain

Use "onlava help <command>" for flags and subcommands.
```

Use ASCII punctuation in repository files unless an existing file or explicit
product copy requires otherwise. The pasted brief used an em dash in the first
line; this plan intentionally uses a hyphen for repo consistency.

`onlava help all` should print grouped command invocations such as:

```text
Onlava command reference

Usage:
  onlava <command> [args] [flags]
  onlava help <command>

Local session:
  onlava up
  onlava ps
  onlava logs
  onlava logs query
  onlava logs tail
  onlava console
  onlava down
  onlava prune

Database:
  onlava db psql
  onlava db apply
  onlava db seed
  onlava db setup
  onlava db reset
  onlava db drop
  onlava db snapshot create
  onlava db snapshot restore
  onlava db branch status
  onlava db branch list
  onlava db branch checkout
  onlava db branch reset
  onlava db branch delete
  onlava db branch restore
  onlava db branch diff
  onlava db branch expire
  onlava db branch prune
  onlava db neon install
  onlava db neon start
  onlava db neon status
  onlava db neon logs
  onlava db neon stop
  onlava db neon restart
  onlava db neon uninstall

Use "onlava help <command>" for exact flags.
Use "onlava help --json" for the machine-readable command manifest.
```

The implementation should include the rest of the command groups from the
current CLI: workspace, generation, tasks, validation, runtime, build/checks,
harness, inspection, observability, and system.

## Milestones

1. Help Metadata Foundation: replace the single root usage literal with a small
   command-help registry that can render root orientation, grouped full
   reference, command-specific help, and JSON manifest from one source.
2. Help CLI Surface: add dispatch for `onlava help`, `onlava help all`,
   `onlava help <command>`, and `onlava help --json`; keep unknown-command
   errors concise and point users toward `onlava help`.
3. Human `ps`: improve bare `onlava ps` into a headed, aligned human table with
   enough session information to be useful, and keep `onlava ps --json` stable.
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

Then introduce rendering functions in `cmd/onlava/main.go` or a new focused
file such as `cmd/onlava/help.go`. Keep the data close to the CLI package so
parser tests can call it. The root renderer should be intentionally short and
should not include full flag lists. The full renderer should group command
paths. The command renderer should include exact usage and flags for one command
family, starting with high-use commands such as `logs`, `ps`, `db`, `task`,
`validate`, `harness`, `inspect`, `worker`, and `system`.

Next wire `help` into top-level `run(args []string)`. Bare `onlava` and
`onlava help` should print the root help. `onlava help all` should print the
full command reference. `onlava help --json` should emit an indented JSON
manifest with a schema version. `onlava help <command>` should resolve command
families such as `db branch`, `logs`, and `worker deployment` without requiring
the caller to know an internal parser type.

After help is stable, update `writeStatus` in `cmd/onlava/agent.go`. Bare
`onlava ps` should render a table with stable columns. A practical first table
is:

```text
SESSION     STATUS    APP        ROUTE/API                         UPDATED
dev-a       running   /repo/app   https://api.dev-a.onlv.dev/       2m ago
```

If there are no sessions, print a short human message such as `No onlava
sessions found.` and return success. If `--watch` is used in human mode, keep
the separator or clear marker simple and testable. `--json` should continue to
encode `schema_version`, `agent`, `sessions`, and `substrates`.

Finally update documentation and drift checks. `docs/local-contract.md` should
describe the help surfaces and change the grammar line from `onlava ps --json`
to `onlava ps [--json] ...` or equivalent. README and agent-facing docs should
show bare `onlava ps` for humans and `onlava ps --json` for automation.
Self-harness should validate that command metadata can render expected entries
and that JSON help includes stable commands, rather than requiring the root
string to contain every grammar line.

## Concrete Steps

1. Add help metadata and renderers.
   - Add `cmd/onlava/help.go` if that keeps `main.go` readable.
   - Add structs for command groups and command entries.
   - Render root help, full help, command help, and JSON manifest from the same
     registry.
   - Keep output lines under 80 columns for root help.
2. Wire top-level command behavior.
   - Make bare `onlava` and `onlava help` print root help.
   - Add `onlava help all`.
   - Add `onlava help --json`.
   - Add representative `onlava help <command>` entries before expanding the
     command-specific text to the full surface.
3. Improve `onlava ps`.
   - Keep `parseStatusArgs` accepting `--json`, `--watch`, `--app-root`, and
     `--session`.
   - Replace the current unheaded tab-separated human output with a headed
     table.
   - Preserve JSON output shape and filtering behavior.
4. Update drift and tests.
   - Adjust `cmd/onlava/harness_drift.go` to inspect help metadata or command
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
go test ./cmd/onlava
go test ./...
onlava inspect docs --json
onlava harness self --summary --write
```

If self-harness is too expensive for an intermediate stop, run it before marking
the plan complete unless a local environment blocker is recorded in
`Surprises & Discoveries`.

Manual source-driven smoke tests should use the worktree source, for example:

```sh
go run ./cmd/onlava
go run ./cmd/onlava help
go run ./cmd/onlava help all
go run ./cmd/onlava help logs
go run ./cmd/onlava help --json
go run ./cmd/onlava ps
go run ./cmd/onlava ps --json
```

Acceptance criteria:

- Root help is grouped, orienting, and avoids full flag grammar.
- Root help lines stay under 80 columns, except where terminal rendering of a
  path or URL makes that impossible.
- `onlava help all` lists the complete command reference grouped by domain.
- `onlava help <command>` provides exact usage and flags for the requested
  command family.
- `onlava help --json` returns a stable schema version and command entries
  suitable for agents and drift checks.
- Bare `onlava ps` returns a readable human table or a clear empty-state
  message.
- `onlava ps --json` keeps the existing `onlava.agent.status.v1` response shape.
- Docs and self-harness drift checks agree with the new help contract.

## Idempotence and Recovery

The help registry is pure data and render logic. If a command-specific help
entry is incomplete, rerun the focused help tests, update the registry entry,
and rerun the same tests. If the JSON manifest changes, update schemas or docs
in the same change only when the JSON shape is intentionally part of the stable
contract.

For `onlava ps`, keep all session reads through the existing local agent client.
Do not add another session store reader. If human table formatting regresses,
fall back to the existing session list and adjust only formatting. Preserve
`--json` as the recovery path for agents.

If `onlava harness self` flags CLI contract drift after root help is shortened,
update the drift check to query structured help metadata or `help all`; do not
restore the root grammar dump to satisfy the old check.

## Artifacts and Notes

The source brief requested these behavior changes:

- root help should not print the full grammar;
- detailed grammar belongs in `onlava help <command>` and `onlava help all`;
- root help should group commands under Local session, Build and runtime, App
  resources, Workspace, Observability, and System;
- `onlava ps` should have a human default table;
- `onlava ps --json` remains the agent-facing machine contract;
- command-specific help should include examples like `onlava help logs` with
  usage, commands, common flags, filter flags, and follow behavior.

No generated artifacts are expected from this plan. Harness artifacts may be
written under `.onlava/harness/` during validation and must not be committed.

## Interfaces and Dependencies

Affected interfaces:

- CLI text output for `onlava`, `onlava help`, `onlava help all`, and
  `onlava help <command>`.
- New CLI JSON output for `onlava help --json`.
- CLI text output for bare `onlava ps`.
- Existing CLI JSON output for `onlava ps --json`, which should remain stable.
- Documentation contract in `docs/local-contract.md`.
- Self-harness CLI drift checks in `cmd/onlava/harness_drift.go`.

Dependencies and constraints:

- Prefer Go standard library formatting, such as `text/tabwriter`, for human
  tables.
- Do not add external dependencies for help rendering.
- Keep command metadata small and explicit.
- Keep machine-readable JSON behind `--json`; do not make agents scrape human
  tables.
- Preserve current command parsers and behavior unless this plan explicitly
  names a change.
