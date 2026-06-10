This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

# Typed Lifecycle Graph Phase 1

## Purpose / Big Picture

scenery should not replace app `Justfile` usage with one generic JSON shell runner. The first useful step is to add typed lifecycle commands for the workflows that are already scenery-shaped, while keeping a small, explicit escape hatch for repository-specific tasks.

This plan implements the phase-1 slice:

- `scenery generate` for file-producing generators.
- `scenery generate client` as a configured wrapper around the existing TypeScript client generator.
- `scenery generate sqlc` as a Scenery-owned SQLC generator that can refresh Atlas-derived schema SQL before running `sqlc generate`.
- `scenery db sync` as an explicit database lifecycle command that runs a configured DB apply provider and then refreshes dependent SQLC artifacts.
- `scenery task list|run|graph` for thin repo-local workflows that are not core scenery concepts.
- `scenery inspect generators --json` so agents can see generator inputs and outputs without scraping config files.

The observable result is a CLI surface that separates pure generation, database mutation, and custom repo tasks. This plan intentionally does not implement native Atlas migration planning, protected-schema enforcement, backups, or generated freshness checks; those remain later phases.

## Progress

- [x] 2026-06-01 01:37 CEST: Read the pasted lifecycle-graph design note, `PLANS.md`, `docs/local-contract.md`, `docs/agent-guide.md`, `docs/plans/active.md`, and `docs/tech-debt.md`.
- [x] 2026-06-01 01:37 CEST: Created this ExecPlan as `docs/plans/0057-typed-lifecycle-graph-phase1.md`.
- [x] 2026-06-01 02:08 CEST: Implemented config types, schema updates, CLI dispatch, and usage text for `generate`, `db sync`, `task`, and `inspect generators`.
- [x] 2026-06-01 02:08 CEST: Implemented configured client/sqlc generation, DB sync provider execution, task execution, task graph JSON, and generator inspection.
- [x] 2026-06-01 02:08 CEST: Added focused tests for generation parsing/planning, SQLC execution wiring, DB sync, and task graph/run behavior.
- [x] 2026-06-01 02:08 CEST: Updated docs and command references in the local contract, agent guide, README, skill, cookbook, and JSON schemas.
- [x] 2026-06-01 02:52 CEST: Ran focused tests, full Go tests, install, generator dry-run, docs inspection, and self-harness. Self-harness feature checks passed after fixes, but the overall harness remains red on the known full-suite timing budget tracked by `docs/plans/0050-test-suite-speed-hardening.md`.
- [x] 2026-06-01 02:53 CEST: Moved this plan from active to completed and reran `scenery harness self --json --write --quick`; quick self-harness passed.

## Surprises & Discoveries

- 2026-06-01 01:37 CEST: The worktree already has unrelated modified and untracked files, including `docs/plans/0056-dev-event-backend-cutover-and-parity.md` and several dev event/logging files. This plan must not revert or overwrite those changes.
- 2026-06-01 01:37 CEST: The repo already has `scripts/gen-auth-sqlc.sh`, `atlas.hcl`, and `sqlc.yaml` for the standard auth schema. The new `generate sqlc` path can cover that workflow without adding a new dependency.
- 2026-06-01 02:08 CEST: `gopkg.in/yaml.v3` was already present in the module graph, so SQLC config parsing did not require adding a new external dependency.
- 2026-06-01 02:42 CEST: `scenery harness self --json --write` initially exposed two feature-related issues: direct YAML dependency allowlisting and config schema nullability for nil env maps. Both were fixed before the final validation run.
- 2026-06-01 02:52 CEST: Final self-harness run passed architecture, drift, UI, fixture matrix, and schema-validation checks, but remained red because its `go test -count=1 ./...` timing was 10.544s against the existing 7.000s budget. The harness itself points to active plan `0050-test-suite-speed-hardening.md`.
- 2026-06-01 02:53 CEST: Quick self-harness passed after moving the ExecPlan to `docs/plans/completed.md`, proving the plan structure/link changes were clean.

## Decision Log

- Decision: Scope this plan to phase 1 rather than the full lifecycle graph.
  Rationale: The pasted design explicitly separates phase 1 from native Atlas/sqlc safety and stale-output checking. A smaller slice gives users useful commands without prematurely baking a full migration engine.
  Date/Author: 2026-06-01, Codex.

- Decision: Keep `scenery gen client` working and add `scenery generate client` as the lifecycle spelling.
  Rationale: `scenery gen client` is part of the current stable v0 surface, so phase 1 should layer the new command without breaking existing users.
  Date/Author: 2026-06-01, Codex.

- Decision: Make DB mutation require explicit `.scenery.json` database provider configuration.
  Rationale: `scenery db sync` can mutate live state, so it should not infer a shell script or auto-apply Atlas changes by convention.
  Date/Author: 2026-06-01, Codex.

- Decision: Keep `scenery task` intentionally thin: named tasks can run a command or ordered steps, and steps can reference other tasks or typed scenery commands.
  Rationale: The task layer is an escape hatch for workflows that are not core scenery lifecycle concepts; it should not become a replacement build system.
  Date/Author: 2026-06-01, Codex.

- Decision: Use `gopkg.in/yaml.v3` for SQLC config parsing and add it to the architecture allowlist.
  Rationale: SQLC configuration is YAML; using the existing module-graph YAML parser avoids brittle shell/text parsing while keeping the direct dependency rationale explicit.
  Date/Author: 2026-06-01, Codex.

## Outcomes & Retrospective

Phase 1 is implemented. scenery now has a typed lifecycle slice for configured generation, explicit DB sync, thin repo tasks, and generator graph inspection without making `scenery generate` a generic script runner. The legacy `scenery gen client` command still works.

Validation completed with focused tests, full Go tests, install, generator dry-run JSON, docs inspection, default self-harness, and quick self-harness. The only remaining red validation is the pre-existing default self-harness full-suite timing budget covered by `docs/plans/0050-test-suite-speed-hardening.md`; no feature-specific diagnostics remain, and quick self-harness passed after plan completion bookkeeping.

## Context and Orientation

The CLI dispatcher lives in `cmd/scenery/main.go`. Existing generation code lives in `cmd/scenery/gen.go`; it parses the app model and writes a generated TypeScript client. Database helper commands live in `cmd/scenery/psql.go`. App config loading lives in `internal/app/root.go` and rejects unknown `.scenery.json` fields, so any new config shape must update both Go structs and `docs/schemas/scenery.config.v1.schema.json`.

Relevant docs that must stay in sync:

- `docs/local-contract.md` for CLI grammar and `.scenery.json` config.
- `docs/agent-guide.md`, `SKILL.md`, and `README.md` for agent and human command guidance.
- `docs/app-development-cookbook.md` for practical app recipes.
- `PLANS.md` for ExecPlan structure.

The current scenery repo itself contains `atlas.hcl`, `sqlc.yaml`, and `scripts/gen-auth-sqlc.sh`. The new SQLC generator should be able to read `sqlc.yaml`, infer `auth/db/gen/schema.sql` from the SQLC schema entry, infer `auth/db/schema.hcl` by convention, write the generated schema SQL, and run `sqlc generate`.

## Milestones

Milestone 1 adds the config and CLI skeleton. It is complete when `scenery generate`, `scenery db sync`, `scenery task`, and `scenery inspect generators --json` parse arguments, reject invalid forms clearly, and appear in usage/docs.

Milestone 2 implements generator planning and execution. It is complete when configured TypeScript clients can be generated through `scenery generate client`, SQLC plans can infer Atlas schema inputs from `sqlc.yaml`, `scenery generate --dry-run --json` reports planned generators, and `scenery inspect generators --json` reports the same graph in stable JSON.

Milestone 3 implements DB sync and thin tasks. It is complete when `scenery db sync` runs an explicit exec provider and then `generate sqlc`, and `scenery task list|run|graph` supports configured commands and steps.

Milestone 4 updates documentation and validates the change. It is complete when focused tests, `go test ./...`, `go install ./cmd/scenery`, and practical harness validation are recorded here.

## Plan of Work

First, extend `internal/app.Config` with small typed config structs:

- `generators.clients[]` for TypeScript client outputs.
- `generators.sqlc` for SQLC config path plus optional Atlas schema mappings.
- `database.apply` for an explicit DB apply provider.
- `tasks` for repo-local task definitions.

Then add CLI files under `cmd/scenery/` for the new command families, sharing helper functions with existing `gen.go` where possible. `scenery gen client` should continue to call the existing behavior, while `scenery generate client` should use either explicit flags or configured client entries.

Next, add generator planning structures used by both `scenery generate --dry-run --json` and `scenery inspect generators --json`. SQLC planning should use `gopkg.in/yaml.v3`, already present in the module graph, to read schema and query paths from `sqlc.yaml`. The command runner should stay simple and testable by keeping external process calls behind package variables.

Finally, update tests and docs together. Focus tests on parser behavior, generator graph output, SQLC convention inference, DB sync command wiring, and task graph/run behavior without requiring Atlas, sqlc, or Postgres to be installed.

## Concrete Steps

1. Edit `internal/app/root.go` and `docs/schemas/scenery.config.v1.schema.json` to accept the new typed config fields.
2. Refactor `cmd/scenery/gen.go` so TypeScript client generation can be called by both `gen client` and `generate client`.
3. Add `cmd/scenery/generate.go` with argument parsing, generator planning, dry-run JSON output, and SQLC execution.
4. Add `scenery inspect generators --json` support in `cmd/scenery/inspect.go`.
5. Extend `cmd/scenery/psql.go` with `db sync` and provider execution.
6. Add `cmd/scenery/task.go` for `task list`, `task run <name>`, and `task graph --json`.
7. Update `cmd/scenery/main.go` usage text and dispatch.
8. Add focused Go tests under `cmd/scenery`.
9. Update docs in `docs/local-contract.md`, `docs/agent-guide.md`, `README.md`, `SKILL.md`, and `docs/app-development-cookbook.md`.
10. Run `gofmt`, focused tests, `go test ./...`, `go install ./cmd/scenery`, and `scenery harness self --json --write` if practical.

## Validation and Acceptance

Acceptance checks:

- `go test ./cmd/scenery -run 'Test(ParseGenerate|BuildSQLC|RunGenerate|RunSQLC|DBSync|TaskGraph|DBCommand)'` passed.
- `go test ./cmd/scenery` passed.
- `go test ./...` passed.
- `go install ./cmd/scenery` passed.
- `go run ./cmd/scenery generate sqlc --dry-run --json` passed and reported the repo SQLC graph.
- `scenery inspect docs --json` passed with zero missing docs.
- `scenery harness self --json --write` ran twice. The final run cleared feature-specific architecture/schema/fixture diagnostics and failed only on the known full-suite timing budget: 10.544s over the 7.000s target.
- `scenery harness self --json --write --quick` passed after moving this plan to completed.

Commands that require Atlas, sqlc, psql, or a managed dev database should be unit-tested through command-runner fakes unless the tools are available locally.

## Idempotence and Recovery

The new `generate --dry-run` and `inspect generators` paths are read-only. `generate client` and `generate sqlc` overwrite generated outputs deterministically. If `generate sqlc` fails after writing schema SQL but before `sqlc generate`, rerun the same command after fixing the external tool error.

`db sync` may mutate a database, but only when `.scenery.json` explicitly configures `database.apply`. If the apply command fails, it stops before dependent SQLC generation and returns the provider error. Rerun after resolving the external script/tool failure.

Task execution is sequential. A failed step stops the task. Rerun the task after fixing the failed command or step; earlier successful steps should be written to be idempotent by the app repository.

## Artifacts and Notes

Expected changed files include:

- `docs/plans/0057-typed-lifecycle-graph-phase1.md`
- `cmd/scenery/main.go`
- `cmd/scenery/gen.go`
- `cmd/scenery/generate.go`
- `cmd/scenery/task.go`
- `cmd/scenery/psql.go`
- `cmd/scenery/inspect.go`
- `internal/app/root.go`
- `docs/schemas/scenery.config.v1.schema.json`
- `docs/local-contract.md`
- `docs/agent-guide.md`
- `README.md`
- `SKILL.md`
- `docs/app-development-cookbook.md`
- focused tests under `cmd/scenery`

This plan does not remove `scripts/gen-auth-sqlc.sh`; it becomes redundant for repos that adopt `scenery generate sqlc`, but removing it is outside this phase.

## Interfaces and Dependencies

New `.scenery.json` config fields:

- `generators.clients[]`: `{ "kind": "typescript-client", "target": "<app-id-or-name>", "output": "<path>" }`
- `generators.sqlc`: `{ "provider": "sqlc", "config": "sqlc.yaml", "schemas": [...] }`
- `database.apply`: `{ "provider": "exec", "command": "./scripts/db-safe-apply.sh" }`
- `tasks.<name>`: `{ "cwd": "<path>", "run": "<command>", "env": {"KEY": "value"} }` or `{ "steps": ["task:repo-harness", "check", "test:go"] }`

New commands:

- `scenery generate [--app-root <path>] [--dry-run] [--json]`
- `scenery generate client [<target>] [--lang typescript] [--output <path>] [--app-root <path>] [--dry-run] [--json]`
- `scenery generate sqlc [--app-root <path>] [--dry-run] [--json]`
- `scenery db sync [--app-root <path>]`
- `scenery task list [--app-root <path>] [--json]`
- `scenery task run <name> [--app-root <path>]`
- `scenery task graph --json [--app-root <path>]`
- `scenery inspect generators --json [--app-root <path>]`

External tools used when executing configured SQLC generation are `atlas` and `sqlc`. Tests should not require those tools to be installed.
