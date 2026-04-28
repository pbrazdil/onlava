# Pulse Harness Engineering

Pulse treats agent support as a runtime feature, not as prompt folklore.

The harness contract gives Codex and other agents a short feedback loop:

1. discover the app and its stable generated metadata
2. compile the generated runtime exactly like `pulse run` would
3. report diagnostics as structured JSON
4. expose inspect outputs and artifact paths without scraping terminal text
5. persist the latest harness result when requested

## Command

```text
pulse harness [--app-root <path>] [--json] [--write]
pulse harness self [--repo-root <path>] [--json] [--write]
```

Use this before large edits and after fixes when an agent needs a single machine-readable status snapshot.

Recommended agent loop:

```text
pulse harness --json --write
pulse harness self --json --write
```

The command runs:

- `pulse check --json`
- `pulse inspect app --json`
- `pulse inspect routes --json`
- `pulse inspect services --json`
- `pulse inspect endpoints --json`
- `pulse inspect wire --json`
- `pulse inspect build --json`
- `pulse inspect paths --json`
- `pulse inspect traces --json`
- `pulse inspect metrics --json`
- `pulse inspect docs --json`

`pulse inspect traces --json` and `pulse inspect metrics --json` are included
as beta diagnostic inputs for agents. Their schema versions are useful for
automation, but their rollup and backend-selection semantics are not stable v0
API yet; see [local-contract.md](local-contract.md).

## Output

JSON output conforms to:

- [pulse.harness.result.v1.schema.json](schemas/pulse.harness.result.v1.schema.json)

When `--write` is present, Pulse writes:

```text
<app-root>/.pulse/harness/latest.json
```

That file is intentionally stable. Agents should use it as the latest local validation snapshot instead of guessing from cache directories or parsing human logs.

For the Pulse repo itself, `pulse harness self --json --write` writes:

```text
<repo-root>/.pulse/harness/self-latest.json
```

The self harness validates the local Pulse development loop:

- `go test ./cmd/pulse ./internal/devdash ./runtime`
- docs knowledge base integrity through `docs/knowledge.json`
- local markdown links and schema JSON syntax
- `pulse inspect docs --json`
- architecture checks for dependency policy, package boundaries, generated-file hygiene, and oversized source files
- dashboard UI typecheck and build
- DB Studio UI typecheck and build
- dashboard and DB Studio build freshness
- `go install ./cmd/pulse`
- installed `pulse` binary freshness against repo sources

## Design Rules

- Keep `AGENTS.md` short. It should point to source-of-truth docs instead of becoming an encyclopedia.
- Prefer stable JSON commands over terminal scraping.
- Prefer repo-local generated artifacts over hidden cache discovery.
- Put remediation text in diagnostics so agents know what to do next.
- Promote repeated review feedback into docs, schemas, or mechanical checks.

## Architecture Checks

`pulse harness self` includes a fast `architecture checks` step.

Hard failures:

- direct Go dependencies must be listed in the self-harness allowlist with a concrete rationale
- forbidden CLI/router/color framework imports are rejected in source
- packages outside `cmd/pulse` may not import `pulse.dev/cmd/pulse`
- required generated/vendored ignore markers must exist in `.gitignore` and `.gitattributes`
- non-generated source files over 2500 lines are rejected

Warnings:

- non-generated source files over 1000 lines
- cgo imports, because they require native build handling
- `.DS_Store` files found in the working tree
The dependency allowlist is intentionally small and lives in code next to the check. New direct dependencies should be rare and must include the reason they justify the added maintenance surface.

## Non-Goals

- The harness is not a CI replacement.
- It does not run external services by itself.
- It does not invent architecture rules. Add new checks only when the repo has a concrete invariant worth enforcing.
