# Harness Self Summary Output

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

`onlava harness self --json --write` currently behaves like a full evidence archive
printed to stdout. That is the wrong default for agents: a green run can still
paste drift inventories, environment-variable references, large-file debt,
timing diagnostics, stdout tails, absolute paths, and artifact evidence into the
primary response. Agents need a compact routing packet first, then explicit
artifact drill-downs for the sections that matter.

This plan changes the self-harness output model to three levels:

1. Summary: a 4-10 KB agent-facing decision packet printed by default.
2. Focused detail: 10-50 KB topic-specific inspector output.
3. Full archive: unbounded evidence written to artifacts and printed only when
   explicitly requested.

The observable behavior is that a successful self-harness run answers four
questions without embedding archive data: can the agent proceed, what needs
attention, what changed, and where is full evidence.

## Progress

- [x] 2026-06-08: Created ExecPlan `0066-harness-self-summary-output.md` from the requested harness-output cleanup brief.
- [x] 2026-06-08: Implemented compact self-harness summary response and `onlava.harness.self.summary.v1` schema.
- [x] 2026-06-08: Added focused harness artifact, diagnostics, and timing drill-down commands.
- [x] 2026-06-08: Updated agent-facing docs to prefer summary mode and avoid pasting full archives.
- [x] 2026-06-08: Validated summary size, full artifact availability, changed-area noise filtering, and version parsing.

## Surprises & Discoveries

- 2026-06-08: `onlava inspect docs --json` reports one review-due document, `docs/ui-agent-contract.md`; this plan should preserve that signal as compact attention, not bury it in full diagnostics.
- 2026-06-08: `docs/local-contract.md` currently documents `onlava harness self --json` as a single JSON document conforming to `onlava.harness.self.v1`, with `--write` persisting the same result to `.onlava/harness/self-latest.json`. The implementation must update this public contract when changing stdout behavior.

- 2026-06-08: Local `onlava-harness-self-*.json` captures were being scanned as runtime env references; changed-area and drift scans now classify them as ignored local artifacts instead of source.
- 2026-06-08: A first full self-harness rerun exposed a transient `TestOnlavaDevDashboardNotificationsAndRoutes` failure; the targeted test passed immediately after and a subsequent full self-harness run passed with only warnings.
## Decision Log

- 2026-06-08: Use a clean summary-default cutover for self-harness stdout. `onlava harness self --json --write`, `onlava harness self --summary --write`, and `onlava harness self --json=summary --write` should print the compact summary; `onlava harness self --json=full --write` prints the existing full archive.
- 2026-06-08: Keep full evidence available as checked local artifacts, not as stdout. The self-harness should still write `.onlava/harness/self-latest.json` and the topic artifacts needed for audit, debugging, and release review.
- 2026-06-08: Add a new summary schema, `onlava.harness.self.summary.v1`, rather than shrinking `onlava.harness.self.v1` in place. The full archive schema remains the contract for `--json=full` and for the full artifact.
- 2026-06-08: Treat unchanged historical architecture debt as debt summary unless it intersects `changed_area` or is new relative to the previous baseline. Agent attention should be about current risk, not every known warning.

## Outcomes & Retrospective

Completed 2026-06-08.

Shipped summary-first self-harness stdout: `--summary`, `--json`, and
`--json=summary` now print `onlava.harness.self.summary.v1`, while
`--json=full` preserves the full `onlava.harness.self.v1` archive stdout. Full
evidence remains in `.onlava/harness/self-latest.json`; the compact summary is
also written to `.onlava/harness/self-summary-latest.json`.

Added bounded drill-downs under `onlava inspect harness`: artifact payloads,
diagnostics by severity, and top-N timing. Added local harness/report artifact
ignore rules, repo-relative architecture diagnostics, and JSON-aware installed
`onlava version --json` parsing.

Validation passed:

- `go test ./cmd/onlava`
- `go test ./...`
- `go build -o .onlava/harness/bin/onlava ./cmd/onlava`
- `onlava harness self --summary --write`
- `onlava harness self --json=summary --write`
- `onlava harness self --json=full --write`
- `onlava inspect harness --json`
- `onlava inspect harness artifact test-timing --json`
- `onlava inspect harness diagnostics --severity warning --json`
- `onlava inspect harness timing --top 10 --json`

## Context and Orientation

Start with these files and surfaces:

- `cmd/onlava` command parsing for `onlava harness self` and `onlava inspect harness`.
- The self-harness implementation that builds the current `onlava.harness.self.v1` response and writes `.onlava/harness/self-latest.json`.
- The changed-area oracle that reports changed files, ignored files, affected packages, risk flags, and recommended commands.
- The drift report writer that produces environment-variable and contract-drift evidence.
- The Go test timing report writer that produces `.onlava/harness/test-timing-latest.json` and Go test JSONL artifacts.
- `docs/schemas/onlava.harness.self.v1.schema.json` for the full archive schema.
- `docs/schemas/onlava.inspect.harness.v1.schema.json` and `docs/schemas/onlava.harness.artifact.v1.schema.json` for inspector and evidence shapes.
- `docs/local-contract.md` for CLI grammar, JSON modes, artifact paths, and stability labels.
- `docs/harness-engineering.md`, `AGENTS.md`, and `SKILL.md` for the agent validation loop.

The current full archive is still valuable. The bug is that archive data is the
primary response. Summary mode must never embed drift variable inventories,
successful stdout/stderr tails, full timing package lists, full large-file lists,
or repeated absolute paths.

## Milestones

1. Summary Contract: add `onlava.harness.self.summary.v1`, a builder from the
   existing full result, status classification, attention buckets, compact steps,
   artifact references, drill-down commands, and size caps.
2. CLI Cutover: teach `onlava harness self` to print summary for `--json`,
   `--summary`, and `--json=summary`, and to print the full archive only for
   `--json=full`.
3. Focused Inspection: add topic drill-downs for harness artifacts,
   diagnostics, and timing so agents can fetch detail without opening the full
   archive.
4. Noise Fixes: ignore local harness/report artifacts in changed-area analysis,
   make architecture warnings changed-area-sensitive, omit successful output
   tails in summary mode, normalize paths, and fix `onlava version --json`
   parsing.
5. Documentation and Validation: update contracts, schemas, agent docs, and
   tests so compact stdout is enforced as a harness contract.

## Plan of Work

First build summary mode as a pure projection of the existing full self-harness
result. This keeps the full archive and all current evidence writers intact while
making stdout compact. The summary builder should derive `ok`, `status`,
`can_proceed`, `diagnostic_summary`, `attention`, compact `steps`, compact report
summaries, `artifacts`, and `drilldowns` from the full result.

Next adjust CLI parsing and output selection. Summary mode is the default JSON
surface for agents. Full mode is explicit. `--write` continues to write the full
archive and topic artifacts; it may also write a summary snapshot if useful, but
stdout must stay compact.

Then add focused inspector commands. `onlava inspect harness --json` can remain
the manifest view, but agents also need `artifact <name>`, diagnostics filtering,
and timing top-N queries. These commands should read existing `.onlava/harness/`
artifacts and return bounded JSON.

Finally update docs and tests. The size limits and forbidden embedded fields are
part of the contract, not preferences.

## Concrete Steps

1. Locate the current self-harness result type, writer, and command handler.
2. Add a `harnessSelfSummaryResponse` type and builder in a focused file such as
   `harness_self_summary.go` near the existing self-harness implementation.
3. Define status classification:
   - `pass` when all steps pass and there are no warnings or debt attention.
   - `pass_with_warnings` when `ok` is true and warnings need current attention.
   - `pass_with_debt` when `ok` is true and only unchanged baseline debt exists.
   - `fail` when any required step fails.
   - `skipped` only for explicitly skipped modes or steps.
4. Build `attention` as capped buckets, not a flat diagnostic dump. Include at
   most 10 items, each with severity, category, message, next action, top entries
   where useful, omitted count when capped, and an artifact or drill-down command.
5. Build `diagnostic_summary` with totals and category counts, for example
   `docs.review_due`, `architecture.large_files`, and `test_timing.budget`.
6. Build compact `steps` with id/name, status, duration, error/warning counts,
   concise summary, and artifact refs. Do not include successful stdout or stderr
   tails. Failed steps may include at most a 2000-byte output tail plus artifact
   references.
7. Build compact report summaries:
   - Drift: env var count, diagnostics count, CLI command count, embed count,
     artifact path. Do not include `drift.env.variables`.
   - Test timing: total seconds, budget seconds, package count, slow test count,
     warning count, top 5 slow tests, top 5 slow packages, artifact path.
   - Knowledge: document counts, review-due/stale counts, top review-due docs,
     artifact or inspector command.
   - Architecture: blocking count, warning count, changed-area-intersecting
     warning count, top changed/new warnings, debt summary count, artifact path.
   - Schema validation and fixture matrix: pass/fail counts and artifact paths.
8. Normalize paths in summary mode. Use `$REPO` for repo root and repo-relative
   paths for files and cwd fields; keep full absolute paths only in full mode.
9. Add hard caps in summary mode:
   - `attention`: 10.
   - Diagnostics per step: 3.
   - Slow tests: 5.
   - Slow packages: 5.
   - Large files shown: 5.
   - Changed files shown: 20.
   - Artifacts: references only, no embedded body.
   - `stdout_tail` and `stderr_tail`: never for passing steps.
10. Add changed-area ignore/classification rules for local diagnostics and cache
    artifacts:
    - `*.harness*.json`
    - `onlava-harness-self-*.json`
    - `.onlava/**`
    - `coverage/**`
    - `test-results/**`
    Ignored-only changes must not recommend `go test ./...` or self-harness as
    changed-area-driven commands.
11. Promote large-file warnings to attention only when their file is in
    `changed_area` or when the warning is new compared with the previous
    baseline. Otherwise report them under debt summary.
12. Fix `onlava version --json` parsing in the toolchain report. Parse JSON and
    expose concise version/build fields; never report the version as the first
    byte of JSON such as `"{"`.
13. Add CLI support:
    - `onlava harness self --summary --write`
    - `onlava harness self --json=summary --write`
    - `onlava harness self --json=full --write`
    - `onlava harness self --json --write` as summary mode after the clean cutover.
14. Add inspect support:
    - `onlava inspect harness --json`
    - `onlava inspect harness artifact <name> --json`
    - `onlava inspect harness diagnostics --severity error|warning --json`
    - `onlava inspect harness timing --top 10 --json`
15. Add `docs/schemas/onlava.harness.self.summary.v1.schema.json` and update any
    schema registry or docs knowledge metadata needed by self-harness validation.
16. Update `docs/local-contract.md` with the new CLI grammar, summary/full mode
    semantics, artifact paths, stability labels, and size budgets.
17. Update `docs/harness-engineering.md` with the summary-first harness model,
    focused drill-down commands, and the rule that full archives are evidence,
    not prompt context.
18. Update `AGENTS.md` and `SKILL.md` so agents run
    `onlava harness self --summary --write` and avoid pasting full
    `.onlava/harness/self-latest.json` into chat.
19. Update `docs/knowledge.json` for the new summary schema and any changed docs.
20. Add tests for summary shape, size, forbidden embedded fields, changed-area
    ignored artifacts, architecture debt promotion, version parsing, and CLI mode
    selection.

## Validation and Acceptance

Acceptance criteria:

- `onlava harness self --summary --write` prints `onlava.harness.self.summary.v1`
  and writes full evidence artifacts.
- `onlava harness self --json=summary --write` prints the same summary shape.
- `onlava harness self --json --write` prints compact summary after the cutover.
- `onlava harness self --json=full --write` prints the full
  `onlava.harness.self.v1` archive.
- A green self-harness summary is no larger than 12 KB.
- A failed self-harness summary is no larger than 32 KB while preserving the first
  actionable failure and artifact references.
- Summary JSON does not contain `drift.env.variables`, successful `stdout_tail`,
  successful `stderr_tail`, full timing package lists, or full large-file lists.
- Summary diagnostics are bucketed and capped, with omitted counts and drill-downs
  when data is suppressed.
- Untracked files matching local harness/report artifact rules are ignored by
  changed-area recommendations and reported separately as ignored local artifacts.
- Large-file warnings outside the changed area and not new relative to baseline
  stay in debt summary, not attention.
- The toolchain report parses installed `onlava version --json` into meaningful
  version/build fields and never reports `"{"` as the version.
- `onlava inspect harness artifact <name> --json`,
  `onlava inspect harness diagnostics --severity warning --json`, and
  `onlava inspect harness timing --top 10 --json` return bounded focused detail.
- `docs/local-contract.md`, `docs/harness-engineering.md`, `AGENTS.md`,
  `SKILL.md`, schemas, and docs knowledge metadata agree on summary-first usage.

Validation commands:

```sh
go test ./...
go build -o .onlava/harness/bin/onlava ./cmd/onlava
onlava harness self --summary --write
onlava harness self --json=summary --write
onlava harness self --json=full --write
onlava inspect harness --json
onlava inspect harness artifact test-timing --json
onlava inspect harness diagnostics --severity warning --json
onlava inspect harness timing --top 10 --json
onlava inspect docs --json
```

Also add unit tests equivalent to:

```go
func TestHarnessSelfSummaryStaysSmall(t *testing.T) {
    resp := fakeGreenHarnessSelfResponseWithLargeArtifacts()
    summary := buildHarnessSelfSummary(resp)

    data, err := json.MarshalIndent(summary, "", "  ")
    if err != nil {
        t.Fatal(err)
    }
    if len(data) > 12_000 {
        t.Fatalf("summary too large: got %d bytes, want <= 12000", len(data))
    }
    if bytes.Contains(data, []byte(`"variables"`)) {
        t.Fatalf("summary must not embed drift.env.variables")
    }
    if bytes.Contains(data, []byte(`"stdout_tail"`)) {
        t.Fatalf("summary must not embed successful stdout tails")
    }
}
```

## Idempotence and Recovery

Summary generation should be a pure projection over the full self-harness result.
If summary tests fail, rerun the builder against the same fixture result and
adjust caps or field selection without changing the evidence writers.

The CLI cutover should be recoverable by keeping `--json=full` wired to the
existing full response. If a downstream tool still needs archive stdout, it can
switch to `--json=full` while agents and default docs move to summary mode.

Changed-area ignore rules should only classify local diagnostic artifacts. If the
oracle accidentally ignores a source, docs, schema, or test file, remove or
narrow the pattern and add a regression test with that path.

No `.onlava/` cache output, coverage output, test-results output, or generated
local harness artifacts should be committed.

## Artifacts and Notes

Expected summary artifacts and drill-downs:

```text
.onlava/harness/self-latest.json
.onlava/harness/agent-context.json
.onlava/harness/drift-latest.json
.onlava/harness/test-timing-latest.json
.onlava/harness/fixture-matrix-latest.json
.onlava/harness/schema-validation-latest.json
.onlava/harness/artifacts/<run-id>/go-test.jsonl
```

Expected agent-facing drill-down commands:

```text
onlava inspect harness --json
onlava inspect harness artifact test-timing --json
onlava inspect harness artifact drift --json
onlava inspect harness diagnostics --severity warning --json
onlava inspect harness timing --top 10 --json
```

A representative summary should include:

- `schema_version: "onlava.harness.self.summary.v1"`
- `ok`
- `status`
- `generated_at`
- `mode`
- `repo`
- `changed_area`
- `can_proceed`
- `attention`
- compact `steps`
- artifact references
- drill-down commands

The uploaded green run would be summarized as passable with warnings: one docs
review-due warning, Go tests above timing budget with top slow tests, unchanged
architecture debt as a count, full drift inventory behind `drift-latest.json`,
and ignored local harness JSON outside changed-area recommendations.

## Interfaces and Dependencies

This plan changes public CLI and JSON contract surfaces for the onlava repo
self-harness:

- `onlava harness self --json` changes from full archive stdout to summary stdout.
- `onlava harness self --summary` is added as an explicit summary spelling.
- `onlava harness self --json=summary` is added as an explicit summary JSON mode.
- `onlava harness self --json=full` is added as the explicit full archive stdout
  mode.
- `onlava.inspect.harness` gains focused artifact, diagnostics, and timing
  drill-down commands.
- `onlava.harness.self.summary.v1` is added as a new compact schema.
- `onlava.harness.self.v1` remains the full archive schema for explicit full mode
  and full evidence artifacts.

The implementation depends on the existing self-harness full result, evidence
artifact model, docs knowledge inspection, changed-area oracle, drift report,
test-timing report, and schema validation surfaces. It should prefer the Go
standard library and avoid new dependencies.
