# Harness Self Summary Output

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

`onlava harness self` currently emits a large machine-readable report by
default. That is useful as archived evidence, but it is too noisy for agents and
humans who need a quick answer to "what failed, what matters, and what should I
run next?"

This plan makes the default stdout a compact agent-facing summary while keeping
the full evidence archive available through `--json`, `--write`, and focused
drill-down artifacts.

## Progress

- [x] 2026-06-08: Restored this missing active ExecPlan file so
  `docs/plans/active.md`, `docs/knowledge.json`, and self-harness link checks
  agree.
- [ ] Define the compact summary shape and failure prioritization.
- [ ] Implement default summary output without breaking existing JSON mode.
- [ ] Update tests, schemas, docs, and harness examples.

## Surprises & Discoveries

- 2026-06-08: `onlava inspect docs --json` and `onlava harness self --json
  --write` reported this indexed active plan as missing. The gap predated the
  CLI observability implementation and blocked clean self-harness validation.

## Decision Log

- 2026-06-08: Keep this as an active plan rather than deleting the index entry,
  because both `docs/plans/active.md` and `docs/knowledge.json` already describe
  the intended work.

## Outcomes & Retrospective

Not yet completed.

## Context and Orientation

Start with these files:

- `cmd/onlava/harness_self.go` for self-harness response construction.
- `cmd/onlava/harness.go` for app harness output patterns.
- `cmd/onlava/harness_schema.go` for schema validation artifacts.
- `docs/harness-engineering.md` for the current harness contract.
- `docs/local-contract.md` for CLI grammar and stability labels.
- `docs/schemas/onlava.harness.self.v1.schema.json` for the current JSON
  envelope.

## Milestones

1. Summary contract: decide the default stdout sections, ordering, and exit-code
   behavior.
2. Output implementation: keep `--json` fully compatible while changing
   human/default stdout.
3. Drill-down artifacts: ensure full evidence remains written under `.onlava/`
   when requested.
4. Documentation and validation: update schemas, docs, and tests together.

## Plan of Work

First inspect the current self-harness command paths and tests to identify where
text output and JSON output split. Then add a compact summary renderer that
uses existing response fields instead of adding a parallel data model. Finally,
update docs and tests so agents know when to use default output versus `--json`
and `--write`.

## Concrete Steps

1. Add focused tests for default stdout, JSON stdout, and `--write` artifacts.
2. Implement a summary renderer that shows overall status, failed steps,
   highest-priority diagnostics, artifacts, and next actions.
3. Preserve `onlava harness self --json` as the full `onlava.harness.self.v1`
   payload.
4. Update `docs/local-contract.md`, `docs/harness-engineering.md`, `SKILL.md`,
   and `docs/agent-guide.md`.
5. Update schema or artifact docs only if the JSON shape changes.

## Validation and Acceptance

Acceptance criteria:

- Plain `onlava harness self` emits a compact summary by default.
- `onlava harness self --json` remains machine-readable and compatible.
- `onlava harness self --json --write` still writes the full evidence artifacts.
- Diagnostics and next actions remain available without scraping long logs.

Validation commands:

```sh
go test ./cmd/onlava
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

## Idempotence and Recovery

This work should be read-only except for requested `.onlava/harness/` artifacts.
If implementation stops midway, leave this plan's progress and surprises current
so the next agent can resume.

## Artifacts and Notes

Relevant generated artifacts:

- `.onlava/harness/self-latest.json`
- `.onlava/harness/schema-validation-latest.json`
- `.onlava/harness/agent-context.json`

## Interfaces and Dependencies

No new external dependencies are expected. Keep implementation in Go standard
library code and existing harness helpers unless tests show a clear reason for a
small local abstraction.
