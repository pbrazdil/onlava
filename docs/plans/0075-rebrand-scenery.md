# Rebrand Onlava to Scenery

This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`,
`Decision Log`, and `Outcomes & Retrospective` current as work proceeds.

## Purpose / Big Picture

Rename the project from Onlava to Scenery as one deliberate pre-v1 breaking
change. The desired end state is a repo, module, CLI, app model, documentation
set, release pipeline, and local state layout that all agree on the Scenery
identity:

```text
github.com/pbrazdil/onlava -> github.com/scenery-sh/scenery
github.com/pbrazdil/onlava -> scenery.sh
onlava CLI -> scenery CLI
.onlava.json -> .scenery.json
.onlava/ and ~/.onlava/ -> .scenery/ and ~/.scenery/
//onlava: -> //scenery:
ONLAVA_* -> SCENERY_*
onlava.toolchain.json -> scenery.toolchain.json
scenery.sh -> scenery.sh
```

This is broader than a binary rename. The old Onlava identity appeared in Go imports, package
names, command paths, release metadata, generated paths, docs, schemas, app
configuration, environment variables, and agent instructions. The rebrand is
successful only when new users can install and use Scenery from the canonical
`scenery.sh` module path and existing app authors have a clear migration path.

## Progress

- [x] 2026-06-10: Created this ExecPlan as `docs/plans/0075-rebrand-scenery.md`.
- [x] 2026-06-10: Linked this plan from `docs/plans/active.md`.
- [x] 2026-06-10: Indexed this plan in `docs/knowledge.json`.
- [x] 2026-06-12: Confirmed `scenery.sh` serves Go vanity import metadata.
- [x] 2026-06-10: Audited all brand-bearing paths and strings in the current tree; current-contract content has no accidental Onlava references outside this migration plan.
- [x] 2026-06-10: Renamed repo-local code, command paths, config files, local state paths, docs, release config, schemas, and UI registry paths to Scenery.
- [x] 2026-06-10: Validated the renamed codebase with the commands in `Validation and Acceptance`; full self-harness passes with warnings only.
- [x] 2026-06-10: Migrated the local ONLV app to the Scenery surface and validated it with `scenery serve` plus `GET /healthy`.
- [x] 2026-06-10: Renamed the GitHub repository to `scenery-sh/scenery`, updated its homepage to `https://scenery.sh`, and updated local `origin`.
- [x] 2026-06-12: Published the first artifact-bearing Scenery release as `v0.2.1` after `v0.2.0` failed before artifact publication.

## Surprises & Discoveries

- 2026-06-10: `scenery inspect docs --json` reported `review_due_count: 0` and `stale_count: 0`, so creating this plan does not require unrelated doc gardening.
- 2026-06-10: The attached rebrand analysis recommends `scenery.sh` as the canonical Go module path and `scenery-sh/scenery` as the GitHub repository name. The main tradeoff is that `scenery.sh` is a bare-domain module path, while `scenery.sh/scenery` would be more conventional for a root package named `scenery`.
- 2026-06-10: The same analysis identifies vanity import readiness as a release blocker. `scenery.sh` must serve `go-import` and `go-source` metadata before tagging the rebrand, or `go install scenery.sh/cmd/scenery@latest` and `pkg.go.dev` indexing can fail or lag.
- 2026-06-10: External vanity import readiness is not yet satisfied. `curl -fsSL 'https://scenery.sh?go-get=1'` failed with a TLS handshake failure, so release/tag publication remains blocked until the domain serves the required metadata.
- 2026-06-10: The repo-side rename now builds and passes `go test ./...` as module `scenery.sh` with CLI package `cmd/scenery`.
- 2026-06-10: An initial self-harness attempt saw embedded UI steps receive SIGKILL immediately, but standalone UI commands passed and a rerun of `go run ./cmd/scenery harness self --json --write` passed with warnings only. Treat the SIGKILL as transient local pressure unless it recurs.
- 2026-06-10: ONLV needed an app-side migration because the Scenery rename intentionally removed old compatibility aliases. The local ONLV migration updated `.scenery.json`, Scenery imports/directives/env, generated TypeScript client references, standard auth seed paths, Justfile commands, and app-local agent instructions.
- 2026-06-10: `scenery up --detach` for ONLV was blocked by the local edge, not by the app: Scenery refused to publish portless `onlv.dev` URLs because DNS, the privileged listener, and Caddy were not ready. Headless `scenery serve` with the managed validation DB was used for running-app validation.
- 2026-06-10: `gh repo rename -R pbrazdil/onlava scenery --yes` succeeded first, then `gh api -X POST repos/pbrazdil/scenery/transfer -f new_owner=scenery-sh` transferred the repository into the `scenery-sh` organization. `gh repo view scenery-sh/scenery` now reports homepage `https://scenery.sh`, while local `origin` points at `git@github.com:scenery-sh/scenery.git`.
- 2026-06-12: `https://scenery.sh?go-get=1` now serves valid `go-import` and `go-source` metadata for `scenery.sh`.
- 2026-06-12: The first release attempt created public source tag `v0.2.0`, but GoReleaser failed before publishing artifacts because the Windows cross-build exposed a Unix-only `syscall.Stat_t` use in edge target validation.
- 2026-06-12: `v0.2.1` supersedes `v0.2.0` as the artifact-bearing Scenery rebrand release. It includes the Windows release-build repair, published macOS/Linux/Windows amd64 and arm64 archives, and installs through `go install scenery.sh/cmd/scenery@v0.2.1`.

## Decision Log

- Decision: Use `scenery.sh` as the planned canonical module path, not `github.com/scenery-sh/scenery`.
  Rationale: The project is CLI/runtime/product-first, and the purchased domain should be the public identity. The shorter path is worth the small package-path convention awkwardness for the root package.
  Date/Author: 2026-06-10 / pbrazdil + Codex

- Decision: Rename the GitHub repository to `scenery-sh/scenery`, not `scenery-sh/scenery.sh`.
  Rationale: The repository host and the vanity import domain have different jobs. A short repo name keeps release assets, URLs, and automation cleaner while `scenery.sh` remains canonical for imports and docs.
  Date/Author: 2026-06-10 / pbrazdil + Codex

- Decision: Treat the rebrand as a pre-v1 breaking release and target `v0.2.0`.
  Rationale: The current public line is `v0.1.x`; this is the right moment for a single decisive rename without compatibility aliases, while deferring `v1.0.0` until the renamed surface settles.
  Date/Author: 2026-06-10 / pbrazdil + Codex

- Decision: Make vanity import metadata a release precondition.
  Rationale: Go custom-domain module resolution depends on `?go-get=1` metadata. The release should not publish a tag that cannot be installed by the new canonical path.
  Date/Author: 2026-06-10 / pbrazdil + Codex

- Decision: Treat `v0.2.1` as the first artifact-bearing Scenery release.
  Rationale: The public `v0.2.0` tag already existed after the failed release workflow, and moving or deleting it would be more disruptive than publishing a patch release with the release-build repair and complete release notes.
  Date/Author: 2026-06-12 / Codex

## Outcomes & Retrospective

Repo-side rebrand work is implemented and validated: Go tests pass, docs and
schema JSON parse, the Scenery CLI builds and reports Scenery JSON contracts,
UI typecheck/test/build pass, current-contract legacy-name audit is clean
outside this migration plan, and full self-harness passes with warnings only.

ONLV has been migrated locally and validated against the renamed runtime.
`scenery check --app-root /Users/petrbrazdil/Repos/onlv --json`,
`go test ./...`, Pulse `bun run typecheck`, Pulse `bun run lint`,
`just repo-harness`, `git diff --check`, `scenery db setup`, and a headless
`scenery serve` smoke all pass. The running API registered 300 routes and
`GET http://127.0.0.1:4099/healthy` returned `200 OK` with
`{"status":"ok"}`.

Release is complete. `https://scenery.sh?go-get=1` serves vanity import
metadata, main and tag CI passed, release-mode self-harness passed with
`can_proceed:true`, GoReleaser published `v0.2.1`, and
`go install scenery.sh/cmd/scenery@v0.2.1` installed a binary reporting
`version:"v0.2.1"`. The public `v0.2.0` tag remains a source tag without a
GitHub Release because its GoReleaser workflow failed before artifact
publication; `v0.2.1` is the verified artifact release.

## Context and Orientation

Start by reading the current repo rules and public contracts:

- `AGENTS.md`
- `PLANS.md`
- `README.md`
- `SECURITY.md`
- `SKILL.md`
- `docs/local-contract.md`
- `docs/agent-guide.md`
- `docs/app-development-cookbook.md`
- `docs/environment.md`
- `docs/environment.registry.json`
- `docs/ui-agent-contract.md`
- `.github/workflows/`
- `.goreleaser.yaml`
- `go.mod`
- `scenery.go`
- `cmd/scenery/`
- `ui/registry/scenery/`
- `scenery.toolchain.json`

The brand-bearing surfaces to audit include:

- GitHub repository URL and metadata: old `github.com/pbrazdil/onlava`,
  `onlava.com`, and `security@onlava.com`; current `https://github.com/scenery-sh/scenery`,
  `https://scenery.sh`, and `security@scenery.sh`.
- Go module imports and public package docs: old `github.com/pbrazdil/onlava`; current `scenery.sh`,
  `package scenery`, linker variables such as `sceneryVersion`, and examples
  under public package docs.
- CLI command paths and binaries: `cmd/scenery`, `go run ./cmd/scenery`,
  `go install ./cmd/scenery`, and generated release assets named `scenery`.
- App model tokens: `.scenery.json`, `.scenery/`, `~/.scenery/`,
  `//scenery:`, `SCENERY_*`, `scenery.toolchain.json`, and generated
  examples such as `scenery-client.ts`.
- UI and agent surfaces: `ui/registry/scenery`, dashboard copy, docs,
  schema names, harness output, installable skill instructions, and
  app-local guidance.

The preferred vanity import metadata for `scenery.sh?go-get=1` is:

```html
<meta name="go-import" content="scenery.sh git https://github.com/scenery-sh/scenery">
<meta name="go-source" content="scenery.sh https://github.com/scenery-sh/scenery https://github.com/scenery-sh/scenery/tree/main{/dir} https://github.com/scenery-sh/scenery/blob/main{/dir}/{file}#L{line}">
```

If maintainers later decide that public Go package ergonomics matter more than
the shortest product identity, update this plan before implementation and use
`scenery.sh/scenery` consistently instead.

## Milestones

Milestone 1 is external identity readiness. Reserve or rename the GitHub repo to
`scenery-sh/scenery`, point the homepage to `https://scenery.sh`, and make
`scenery.sh` serve the Go vanity metadata above. This milestone is a release
blocker and should be verified with Go tooling before tagging.

Milestone 2 is mechanical repository rename. Rename the command directory,
root package file, root package name, toolchain file, UI registry path, and
obvious tests with `git mv` where possible. Rewrite imports and brand tokens
across tracked source, docs, schemas, examples, release config, and CI.

Milestone 3 is contract reconciliation. Update `docs/local-contract.md`,
`docs/agent-guide.md`, `SKILL.md`, `README.md`, `docs/environment.md`,
`docs/environment.registry.json`, schemas, harness expectations, and UI docs so
they describe Scenery as the current contract rather than a future alias.

Milestone 4 is validation and migration polish. Run Go, CLI, UI, docs, and
self-harness validation. Audit for remaining legacy names and decide whether any
intentional historical references need a short note explaining why they remain.

Milestone 5 is release. Publish a GitHub release, attach GoReleaser artifacts,
verify install by `scenery.sh`, warm or request Go proxy/pkg.go.dev indexing,
then publish `v0.2.1` with explicit migration notes.

## Plan of Work

First, make the vanity import path real. Verify `scenery.sh?go-get=1` returns
the expected metadata and that `go env GOPROXY`-normal tooling can resolve the
new path against a local or pre-release branch. Do not tag `v0.2.0` until this
works.

Second, perform the rename in history-preserving chunks. Use `git mv` for
directory and file moves, then use a deterministic text replacement pass over
tracked text files. Review the resulting diff by subsystem rather than accepting
all replacements blindly, because historical notes, URLs, schema identifiers,
and migration prose may need intentional wording.

Third, repair code and tests after the mechanical pass. Run `gofmt`, `go mod
tidy`, update linker variables, update command help, update fixtures, and make
the generated model, CLI JSON contracts, schemas, and harness outputs agree on
Scenery.

Fourth, update user-facing migration docs. The README, SECURITY policy,
installable skill, agent guide, local contract, environment docs, app cookbook,
GoReleaser config, and CI should all explain the new names directly. Migration
notes may mention the old names, but the current contract should be singular.

Finally, validate, release, and record outcomes in this plan. If any old
compatibility spelling survives, record the reason in `Decision Log` and update
the relevant contract docs.

## Concrete Steps

1. Run a brand audit:

   ```sh
   rg -n --hidden --glob '!.git' --glob '!**/.scenery/**' --glob '!**/.onlava/**' \
     -e '\bonlava\b' \
     -e '\bOnlava\b' \
     -e 'github\.com/pbrazdil/onlava' \
     -e 'onlava\.com' \
     -e 'security@onlava\.com' \
     -e '\.onlava(\.json|/)' \
     -e '//onlava:' \
     -e '\bONLAVA_' \
     -e 'cmd/onlava' \
     -e 'onlava\.toolchain\.json' \
     -e 'onlava-client'
   ```

2. Verify external readiness:

   ```sh
   curl -fsSL 'https://scenery.sh?go-get=1'
   go env GOPROXY
   ```

   The HTML must contain the `go-import` and `go-source` metadata from this
   plan before any public release tag is cut.

3. Rename history-sensitive paths:

   ```sh
   git mv cmd/onlava cmd/scenery
   git mv onlava.go scenery.go
   git mv onlava.toolchain.json scenery.toolchain.json
   git mv ui/registry/onlava ui/registry/scenery
   ```

   Also rename root-level `onlava_*` test files if they exist in the current
   tree.

4. Rewrite tracked text files for the primary name changes. Use the migration
   helper from the attached analysis or an equivalent reviewed script, but keep
   these replacements explicit:

   ```text
   module github.com/pbrazdil/onlava -> module scenery.sh
   github.com/pbrazdil/onlava -> scenery.sh
   https://github.com/pbrazdil/onlava -> https://github.com/scenery-sh/scenery
   onlava.com -> scenery.sh
   security@onlava.com -> security@scenery.sh
   package onlava -> package scenery
   onlavaVersion/onlavaCommit/onlavaBuiltAt -> sceneryVersion/sceneryCommit/sceneryBuiltAt
   .onlava.json -> .scenery.json
   .onlava/ -> .scenery/
   ~/.onlava/ -> ~/.scenery/
   //onlava: -> //scenery:
   ONLAVA_ -> SCENERY_
   onlava.toolchain.json -> scenery.toolchain.json
   onlava-client.ts -> scenery-client.ts
   ```

5. Update release and CI plumbing:

   - `.goreleaser.yaml` should use project and binary name `scenery`, command
     path `./cmd/scenery`, release repo `scenery`, and linker variables under
     `main.scenery*`.
   - `.github/workflows/**` should run `go run ./cmd/scenery`, install
     `./cmd/scenery`, and smoke-test the installed `scenery` binary.
   - Release notes should call this a breaking pre-v1 rename and target
     `v0.2.0`.

6. Update app-facing contracts and docs:

   - `docs/local-contract.md`
   - `docs/agent-guide.md`
   - `SKILL.md`
   - `README.md`
   - `SECURITY.md`
   - `docs/app-development-cookbook.md`
   - `docs/environment.md`
   - `docs/environment.registry.json`
   - `docs/knowledge.json`
   - relevant schemas under `docs/schemas/`
   - any fixture app configs and expected harness artifacts

7. Reformat and reconcile:

   ```sh
   gofmt -w $(git ls-files '*.go')
   go mod tidy
   jq empty docs/knowledge.json
   ```

8. Run the final legacy-name audit. For every remaining result, either remove
   it or leave a clear historical/migration explanation in the owning doc.

## Validation and Acceptance

Acceptance criteria:

- `go.mod` declares `module scenery.sh`.
- Root package docs expose `package scenery`.
- The CLI is built from `cmd/scenery` and the binary is named `scenery`.
- Public docs describe `.scenery.json`, `.scenery/`, `//scenery:`,
  `SCENERY_*`, and `scenery.toolchain.json` as the current contract.
- `scenery.sh?go-get=1` serves valid Go vanity metadata before release.
- GoReleaser and CI no longer publish or smoke-test an `onlava` binary.
- `rg` has no accidental current-contract references to `onlava`, `Onlava`,
  `github.com/pbrazdil/onlava`, `onlava.com`, `security@onlava.com`,
  `.onlava.json`, `//onlava:`, or `ONLAVA_`.
- Remaining `onlava` mentions are historical migration notes, old release notes,
  or compatibility explanations with explicit context.
- The rebrand release is prepared as `v0.2.0`, not `v1.0.0` or `v2.0.0`.

Validation commands:

```sh
jq empty docs/knowledge.json
go test ./...
go run ./cmd/scenery check --app-root testdata/apps/basic --json
go install ./cmd/scenery
"$(go env GOPATH)/bin/scenery" version --json
"$(go env GOPATH)/bin/scenery" harness self --json --write
(
  cd ui
  bun run typecheck
  bun run test
  bun run build
)
git diff --check
```

If the UI dependencies are not installed, run the repo-standard install command
for the current branch before the UI checks. Do not add gated tests to skip
real validation merely because a command is slow.

## Idempotence and Recovery

All mechanical rename steps should be safe to rerun after checking the current
state. Prefer scripts that skip already-renamed paths and print a final legacy
reference audit.

If a text replacement overreaches, inspect the diff for that file and repair the
specific wording. Do not revert unrelated dirty work in the repository. If the
worktree already contains active implementation changes, stage or review only
the Scenery rebrand files for this plan.

If vanity import validation fails, stop before release work. Fix the website or
DNS hosting, then rerun the metadata and `go install`/`go list` checks. Do not
publish a tag that depends on local `replace` directives or an unavailable
custom import path.

If `pkg.go.dev` indexing lags after release, verify `go list -m scenery.sh@v0.2.1`
first, then request indexing from pkg.go.dev. Treat proxy/index lag as a release
follow-up only after module resolution itself works.

## Artifacts and Notes

Expected artifacts:

- Renamed repository metadata at `https://github.com/scenery-sh/scenery`.
- Vanity import metadata served by `https://scenery.sh?go-get=1`.
- `scenery.toolchain.json`.
- `cmd/scenery/`.
- `scenery.go`.
- `ui/registry/scenery/`.
- Updated README migration section.
- A `v0.2.1` GitHub Release with explicit breaking rename notes.
- Harness evidence from `scenery harness self --json --write`.

The attached rebrand analysis includes a longer draft README, migration helper,
release-note outline, and source URL list. Keep this ExecPlan as the executable
source of truth; copy from the attachment only when implementing the named
milestones.

## Interfaces and Dependencies

External dependencies:

- `scenery.sh` DNS and web hosting must serve Go vanity import metadata.
- GitHub repository settings must rename the repo to `scenery-sh/scenery` and set
  homepage metadata to `https://scenery.sh`.
- Go proxy and `pkg.go.dev` must be able to resolve `scenery.sh` after tagging.
- GoReleaser must publish a `scenery` binary and checksums against the renamed
  repository.

Internal interfaces affected:

- Go module path and imports.
- Public root package name.
- CLI grammar, help, JSON contracts, and schemas.
- App root discovery and generated state paths.
- Directive parser prefix.
- Environment registry and env var names.
- Managed toolchain manifest path.
- Dashboard and UI registry import paths.
- Installable skill and app-local agent guidance.

Do not carry old Onlava aliases unless this plan is explicitly amended with a
compatibility decision, affected docs, tests, and a removal path.
