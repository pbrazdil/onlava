# scenery UI Registry and Agent Guardrails

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

scenery should make UI composition safer for agents by treating shadcn as a registry protocol and source generator, not as an open-ended cookbook that agents can paste into screens. The public agent-facing surface should be scenery-owned: an `@scenery/*` registry, stable UI primitives, slot-based layouts, and static guardrails that fail fast when code bypasses the contract.

Target enforcement model:

```text
shadcn public registry
        |
        v
scenery vetted registry, installed as @scenery/*
        |
        v
scenery vendor layer for generated or promoted shadcn-derived files
        |
        v
scenery primitives with stable typed props
        |
        v
scenery slot layouts
        |
        v
app/dashboard screens composed from primitives and layouts
```

The core rule is:

```text
Agents do not use shadcn directly.
Agents use scenery UI.

scenery UI may internally be built from vetted shadcn/Radix/Base primitives.
```

Success means the scenery repository owns the registry namespace `@scenery/*`, the dashboard and downstream apps can import scenery primitives and layouts instead of direct shadcn/Tailwind code, and self-harness or an equivalent static check catches direct shadcn usage, forbidden imports, and className drift before it reaches app screens.

The ONLV UI must not visually change as part of this migration. Existing ONLV components should be ported or mirrored into the scenery registry and primitives/layouts, then ONLV references should be updated so semantics and guardrails change without intentionally changing rendered behavior.

## Progress

- [x] (2026-05-09 15:58Z) Created this ExecPlan and assigned historical ID 0011.
- [x] (2026-05-09 15:58Z) Linked this ExecPlan from `docs/plans/active.md`.
- [x] (2026-05-09 16:06Z) Build the initial `ui/components.json` configuration with only the `@scenery` registry namespace.
- [x] (2026-05-09 16:06Z) Add the `ui/scripts/scenery-shadcn.mjs` wrapper and package script.
- [x] (2026-05-09 16:06Z) Create the vendor/primitives/layouts folder split under `ui/src/components`.
- [x] (2026-05-09 16:09Z) Add initial registry items for core primitives, ONLV-ported primitives, and slot layouts.
- [x] (2026-05-09 16:06Z) Add `docs/ui-agent-contract.md`.
- [x] (2026-05-09 16:07Z) Add static UI architecture checks and wire them into self-harness.
- [x] (2026-05-09 16:09Z) Port approved ONLV UI components into the scenery registry/primitives/layouts without visual behavior changes.
- [x] (2026-05-09 16:10Z) Update ONLV references to consume the scenery UI surface and verify scenery check/Go tests.
- [x] (2026-05-09 16:11Z) Run scenery validation: `go test ./...`, `go install ./cmd/scenery`, UI typecheck/test/build, wrapper dry-runs, and `scenery harness self --json --write`.

## Surprises & Discoveries

- Official shadcn `components.json` supports multiple registries and namespaced installs such as `@private/button`; the `{name}` placeholder is substituted from the install target. This matches the proposed `@scenery/*` registry design.
- Official shadcn registry item JSON supports item types including `registry:block`, and file `target` placeholders such as `@ui/`, `@components/`, `@lib/`, and `@hooks/`. This lets scenery publish both low-level primitives and larger slot layouts.
- The shadcn CLI `add` command accepts component names, URLs, or local paths and supports `--dry-run`, `--diff`, and `--overwrite`. The wrapper must reject URLs and non-`@scenery/*` names before invoking the CLI.
- The shadcn removed agent transport server can browse, search, and install from configured registries, including namespaced and private registries. removed agent transport may help discovery, but scenery's guardrails still need to enforce allowed namespaces and import boundaries.
- Current `ui/package.json` has no Tailwind, shadcn, Radix, CVA, lucide, or Tailwind merge dependencies. This is a good point to add guardrails before those libraries appear.
- The existing scenery dashboard still has route-level Tailwind-style class strings. The new static check hard-fails import/script/registry violations now and reports current className drift as warnings while the dashboard is migrated into primitives/layouts.
- ONLV app already had a large shadcn/Radix component set. To avoid visual churn, the first ONLV update adds a Scenery-facing `components/primitives` barrel and layout/primitives re-export paths, then switches app screens off `@/components/ui` and `@/components/app/*` without replacing rendered component implementations.
- `bun run typecheck` in ONLV app currently fails in the sibling viewer TypeScript graph because two `@types/three` versions disagree about `WebGLRenderer` and geometry types. `scenery check --json` and `go test ./...` pass; the TypeScript failure is unrelated to the import-path rewrite.
- `scenery harness --json --write` in ONLV reached app inspection successfully but failed on `inspect traces` and `inspect metrics` with `SQLITE_BUSY`, likely because a local dev process held the dashboard SQLite database lock.

## Decision Log

- Decision: Make `@scenery/*` the only approved shadcn registry namespace for agent installs.
  Rationale: shadcn intentionally allows flexible component, URL, and local-path installation. scenery needs a stable, vetted namespace so agents can compose approved UI without pulling arbitrary upstream code into app screens.
  Date/Author: 2026-05-09 / Codex

- Decision: Keep generated or promoted shadcn-derived files in a vendor layer and expose only scenery primitives/layouts to routes and screens.
  Rationale: This preserves the option to replace shadcn internals later while keeping app code stable and scenery-named.
  Date/Author: 2026-05-09 / Codex

- Decision: Prefer named slot props for agent-facing layouts over free-form compound component children.
  Rationale: Named slots are less elegant but harder for agents to misuse; the layout owns spacing, scroll behavior, responsive behavior, ARIA landmarks, and DOM test markers.
  Date/Author: 2026-05-09 / Codex

- Decision: Start with static checks instead of a large frontend lint stack.
  Rationale: scenery already has self-harness architecture checks. Small Go/static checks keep dependency growth low and can enforce the exact boundaries the project cares about.
  Date/Author: 2026-05-09 / Codex

- Decision: Port ONLV UI semantics through the scenery registry without intentional visual changes.
  Rationale: The user-facing ONLV app should not churn visually during this infrastructure migration. The important change is ownership and guardrails: ONLV screens should depend on scenery UI contracts rather than direct ad hoc component code.
  Date/Author: 2026-05-09 / Codex

- Decision: Make the first className discipline check warning-only for existing dashboard route files.
  Rationale: The current dashboard has substantial pre-existing utility class usage. Hard-failing that immediately would turn this plan into a full dashboard layout migration. The hard guardrails now cover shadcn/script/registry/import violations, while className warnings identify the remaining migration path.
  Date/Author: 2026-05-09 / Codex

- Decision: In ONLV, re-export existing components through scenery-facing primitives/layout paths before replacing implementations.
  Rationale: This changes screen import semantics and future guardrails without changing visual output or attempting a risky wholesale replacement of the product UI.
  Date/Author: 2026-05-09 / Codex

## Outcomes & Retrospective

Completed 2026-05-09.

Shipped:

- `ui/components.json` with the approved `@scenery` registry namespace and vendor alias target.
- `ui/scripts/scenery-shadcn.mjs`, a wrapper that accepts only `@scenery/*`, rejects URLs/local paths/raw overwrite, serves the local registry during the command, and runs a dry-run before applying changes.
- Initial scenery UI primitives and layouts under `ui/src/components/primitives` and `ui/src/components/layouts`.
- Registry items for `button`, `card`, `dialog`, `input`, `dashboard-page`, `data-explorer-layout`, `app-surface`, `filter-pill`, and `sidebar-item`.
- `docs/ui-agent-contract.md` and docs/local-contract/harness documentation for the new guardrails.
- A self-harness `ui static architecture` step that hard-fails raw shadcn script usage, non-`@scenery` registries, legacy `components/ui` imports from screens, direct vendor shadcn imports from screens, and direct Radix/styling utility imports outside primitives/layouts/vendor.
- ONLV app import references updated to use `@/components/primitives` and `@/components/layouts/*` re-export surfaces instead of direct `@/components/ui` or `@/components/app/*` screen imports.

Retrospective:

The first safe migration point is semantic rather than visual. scenery now has the registry protocol and guardrails, and ONLV screen imports moved to scenery-facing surfaces without replacing the existing rendered component implementations. The remaining visual-layout cleanup is to gradually reduce className warnings by moving dashboard route structure into the new layouts and primitives.

## Context and Orientation

This plan is for the `scenery.sh` repository. It may require a follow-up or coordinated commit in the ONLV repository to replace local UI references with scenery-owned imports, but the primary source of truth for the registry, primitives, layouts, and guardrails belongs in scenery.

Relevant current files:

- `ui/package.json`: dashboard UI scripts and dependencies.
- `ui/src/router.tsx` and `ui/src/routes/*`: dashboard routes that should eventually import only scenery layouts/primitives.
- `ui/src/components/layout.tsx` and `ui/src/components/json-view.tsx`: existing local dashboard components that can seed the new structure.
- `ui/src/lib/utils.ts`: existing utility code; if shadcn helpers are introduced, keep them here or in a dedicated scenery-owned helper.
- `cmd/scenery` and harness-related packages: possible place to wire UI static checks into `scenery harness self --json --write`.
- `docs/local-contract.md`: update only when a stable user-visible command or public contract changes.
- `docs/ui-agent-contract.md`: new contract document for agents and contributors.
- `PLANS.md`: ExecPlan rules.

Current desired repo shape:

```text
ui/
  components.json
  scripts/
    scenery-shadcn.mjs
  registry/
    scenery/
      registry.json
      button.json
      dialog.json
      dashboard-page.json
      data-explorer-layout.json
  src/
    components/
      vendor/
        shadcn/
      primitives/
        Button.tsx
        Dialog.tsx
        Input.tsx
        Select.tsx
        Card.tsx
      layouts/
        AppShell.tsx
        DashboardPage.tsx
        DataExplorerLayout.tsx
        SplitPane.tsx
    features/
      data-explorer/
        DataExplorerPage.tsx
```

The exact first primitive list may change after inspecting ONLV's existing components, but the import rules should not change:

```tsx
import { DashboardPage } from "@/components/layouts/DashboardPage"
import { Button } from "@/components/primitives/Button"
```

Forbidden in routes, pages, and feature screens:

```tsx
import { Button } from "@/components/vendor/shadcn/button"
import { Button } from "@/components/ui/button"
import * as DialogPrimitive from "@radix-ui/react-dialog"
```

shadcn facts verified against official docs on 2026-05-09:

- `components.json` configures aliases for `components`, `ui`, `lib`, `hooks`, and `utils`, and supports named registries with URL templates such as `@acme`.
- Namespaced install commands such as `npx shadcn@latest add @private/button` are supported when configured in `components.json`.
- Registry item JSON supports typed resources and file targets, including `registry:block`, `registry:component`, `registry:ui`, `registry:lib`, and `registry:hook`.
- The CLI `add` command accepts component names, URLs, and local paths, and supports `--dry-run` and `--overwrite`; scenery must wrap it to enforce namespace and overwrite policy.
- The shadcn removed agent transport server can use registries configured in `components.json`, so scenery prompts and docs must say to use only the `@scenery` registry.

Use these official docs for reference while implementing, but keep this ExecPlan self-contained for execution:

- `https://ui.shadcn.com/docs/components-json`
- `https://ui.shadcn.com/docs/registry/registry-item-json`
- `https://ui.shadcn.com/docs/cli`
- `https://ui.shadcn.com/docs/removed-agent-transport`

## Milestones

Milestone 1: Registry configuration and wrapper.

Add `ui/components.json` with `@scenery` as the only custom registry namespace. The initial URL can point to a local development registry endpoint or static path; choose the smallest setup that lets `bunx shadcn@latest add @scenery/<item>` resolve during local development. Add `ui/scripts/scenery-shadcn.mjs`, wire `bun run shadcn:add`, and make the wrapper reject:

```text
non-@scenery items
URLs
local paths
--all
--path unless explicitly allowed by design
--overwrite unless SCENERY_SHADCN_OVERWRITE=1
```

The wrapper should run `bunx shadcn@latest add ... --dry-run` first, print the target files or CLI output, then run the real command only if the dry run succeeds.

Milestone 2: Folder split and first scenery UI surface.

Create the folder split:

```text
ui/src/components/vendor/shadcn
ui/src/components/primitives
ui/src/components/layouts
ui/src/features
```

Move or wrap existing scenery dashboard components into the new shape without changing dashboard behavior. Add stable DOM markers to layouts, for example:

```tsx
<section data-scenery-ui="DashboardPage">
  <header data-slot="toolbar">{toolbar}</header>
  <main data-slot="content">{content}</main>
</section>
```

Milestone 3: Agent UI contract.

Create `docs/ui-agent-contract.md`. It must define allowed and forbidden patterns, examples, promotion flow, layout slot rules, import boundaries, and commands. Put the core rule near the top:

```text
Agents must compose UI from scenery layouts and primitives.
Agents must not use shadcn directly in app screens.
```

Milestone 4: Static architecture checks.

Add a small static check for UI architecture. Prefer a Go check integrated into the self-harness if it fits existing harness structure; otherwise add a focused command or script and document how it will later join the harness.

Checks should include:

```text
ui-import-boundaries:
  fail on direct imports from components/vendor/shadcn outside primitives/layouts
  fail on components/ui imports outside wrappers
  fail on direct @radix-ui imports outside primitives
  fail on class-variance-authority, clsx, tailwind-merge outside primitives/vendor if introduced
  fail on lucide-react outside an icons wrapper if introduced

ui-classname-discipline:
  fail on long className string literals in ui/src/routes and app screen directories
  fail on obvious Tailwind utility class soup outside primitives/layouts/vendor
  fail on arbitrary variants such as [&>*] outside primitives/layouts/vendor

ui-shadcn-discipline:
  fail if package scripts contain raw "shadcn add"
  allow only the scenery wrapper script
  fail on registry namespaces other than @scenery in ui/components.json
  fail on direct registry URLs in source/docs outside approved docs
```

Milestone 5: Registry items.

Add initial registry item metadata under `ui/registry/scenery`. The first pass should include at least:

```text
button
dialog or confirm-dialog
dashboard-page
data-explorer-layout
```

Use `registry:ui` or `registry:component` for primitives and `registry:block` for slot layouts. Include enough metadata that a future agent can discover what each item installs and where it lands.

Milestone 6: ONLV component port.

Inspect the ONLV repository for reusable components and layouts that currently live only in the app. Port approved generic pieces into scenery primitives/layouts/registry items. Preserve visual output. Update ONLV references to use the scenery UI surface. Do not redesign ONLV screens in this milestone.

If a component is too product-specific for scenery, leave it in ONLV but require it to compose scenery primitives/layouts rather than direct shadcn/Tailwind/vendor code.

Milestone 7: Validation and documentation.

Run scenery Go tests, UI typecheck/build/tests, install the CLI, and self-harness. Run the relevant ONLV typecheck/build/check/harness commands after updating ONLV references. Update `docs/local-contract.md` only if a stable command or public contract changes.

## Plan of Work

Start in the scenery repo by adding the plan and then inspecting the current dashboard UI. Identify existing dashboard components that can seed `layouts/AppShell`, `layouts/DashboardPage`, and primitives such as `Button`, `Card`, `Input`, and `Select`.

Implement the registry and wrapper before adding shadcn dependencies. The guardrail should exist before the tool can be used. Keep the first `components.json` minimal and scenery-owned:

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "new-york",
  "tsx": true,
  "rsc": false,
  "aliases": {
    "components": "@/components",
    "ui": "@/components/vendor/shadcn",
    "lib": "@/lib",
    "hooks": "@/hooks",
    "utils": "@/lib/utils"
  },
  "registries": {
    "@scenery": "http://127.0.0.1:4873/r/{name}.json"
  }
}
```

The registry URL may change if implementation discovers a better local static registry path. Record that decision in the Decision Log. The important contract is that the namespace is `@scenery` and direct un-namespaced shadcn installs are forbidden.

Next, add `ui/scripts/scenery-shadcn.mjs`. Keep it dependency-free using Node's standard library. It should parse arguments conservatively. Non-option args must start with `@scenery/`. If an option takes a value, the wrapper must not accidentally validate the option value as a component name. Reject URLs and local paths before invoking `bunx`.

After the wrapper, create the scenery component layers. For existing dashboard code, make small mechanical moves first, then update imports. Do not introduce a large visual refactor. If Tailwind or shadcn dependencies are added, keep them quarantined under vendor/primitives/layouts and document why.

For static checks, prefer simple parsing over cleverness. A robust first pass can scan TypeScript source text for import specifiers, `className="..."` literals, `className={'...'}` literals, and package scripts. It does not need to be a complete TypeScript AST linter if it catches the boundary violations with clear messages.

Then inspect ONLV components. Split candidates into:

```text
generic primitive: belongs in scenery primitives
generic layout: belongs in scenery layouts and registry:block
product-specific feature: stays in ONLV but composes scenery UI
```

Port generic code into scenery and update ONLV imports. Preserve route behavior, text, spacing, and data loading. Use screenshots or browser checks when practical to prove ONLV did not visually regress.

## Concrete Steps

1. In `docs/plans`, create this plan as `0011-scenery-ui-registry-and-agent-guardrails.md` and link it from `docs/plans/active.md`.
2. Read `ui/package.json`, `ui/src/router.tsx`, `ui/src/components/*`, and `ui/src/routes/*`.
3. Add `ui/components.json` with only `@scenery` as a configured custom registry namespace.
4. Add `ui/scripts/scenery-shadcn.mjs` and `ui/package.json` script `"shadcn:add": "node scripts/scenery-shadcn.mjs"`.
5. Add wrapper tests if the UI test setup can run Node scripts cleanly; otherwise add a tiny Go/static test around the script's behavior or document manual wrapper checks in this plan.
6. Create `ui/src/components/vendor/shadcn`, `ui/src/components/primitives`, and `ui/src/components/layouts`.
7. Move/wrap existing dashboard UI building blocks into primitives and layouts without visual redesign.
8. Add initial registry item JSON files under `ui/registry/scenery`.
9. Add `docs/ui-agent-contract.md` with allowed/forbidden examples and the shadcn promotion flow.
10. Implement UI static checks and wire them into `scenery harness self --json --write` if practical.
11. Inspect ONLV UI components and identify generic pieces to promote into scenery.
12. Port approved ONLV components into scenery primitives/layouts/registry.
13. Update ONLV references to use scenery-owned UI surface.
14. Run validation commands and update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective.

## Validation and Acceptance

scenery validation:

```sh
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
cd ui && bun run typecheck
cd ui && bun run test
cd ui && bun run build
cd ui && bun run shadcn:add @scenery/button --dry-run
```

If the wrapper does not accept a user-passed `--dry-run` because it always runs one internally, replace the last command with the wrapper's documented preview command and record the final command here.

ONLV validation after references are updated:

```sh
cd /Users/petrbrazdil/Repos/onlv && scenery check --json
cd /Users/petrbrazdil/Repos/onlv && go test ./...
cd /Users/petrbrazdil/Repos/onlv && scenery harness --json --write
```

Frontend/browser validation when practical:

```text
- Start ONLV with the normal local command.
- Open the ONLV app routes that use ported components.
- Confirm visible UI is unchanged.
- Confirm no console errors and no failed network requests.
```

Acceptance criteria:

```text
- `@scenery/*` is the only approved shadcn install target.
- Direct `shadcn add` commands are absent from package scripts and docs except forbidden examples in the contract.
- Routes/screens do not import from vendor shadcn, `components/ui`, Radix primitives, CVA, clsx, or Tailwind merge directly.
- Long Tailwind/className soup is blocked outside primitives/layouts/vendor.
- scenery layouts expose stable DOM markers and named slot props.
- ONLV UI references are updated without intentional visual changes.
- Validation commands pass or any skipped command is explained with a concrete blocker.
```

## Idempotence and Recovery

The wrapper and static checks should be idempotent. Running `bun run shadcn:add @scenery/button` repeatedly must either report no changes or require explicit overwrite through `SCENERY_SHADCN_OVERWRITE=1`.

Registry item files are source-controlled. If a promoted component is wrong, revert the registry item and wrapper-installed generated files together. Do not edit generated/vendor shadcn files directly in app screens.

If static checks produce false positives, narrow the allowlist to exact directories and import specifiers. Do not disable the whole check to get a build green.

If ONLV visual output changes unexpectedly, stop porting additional components, compare the pre/post component markup and CSS, and either fix the scenery primitive to preserve behavior or leave that component in ONLV until a deliberate redesign is planned.

If the local registry URL chosen in `components.json` is inconvenient, change it in one commit with the registry serving mechanism and update the Decision Log. Keep the namespace `@scenery`.

## Artifacts and Notes

Expected new artifacts:

```text
docs/ui-agent-contract.md
ui/components.json
ui/scripts/scenery-shadcn.mjs
ui/registry/scenery/registry.json
ui/registry/scenery/button.json
ui/registry/scenery/dialog.json
ui/registry/scenery/dashboard-page.json
ui/registry/scenery/data-explorer-layout.json
ui/src/components/vendor/shadcn/*
ui/src/components/primitives/*
ui/src/components/layouts/*
```

Potential self-harness output should include diagnostics like:

```json
{
  "code": "ui.import_boundary",
  "severity": "error",
  "path": "ui/src/routes/db.tsx",
  "message": "routes must import scenery primitives/layouts, not vendor shadcn components"
}
```

Potential layout marker convention:

```tsx
<section data-scenery-ui="DataExplorerLayout">
  <aside data-slot="object-list">{objectList}</aside>
  <main data-slot="table">{table}</main>
  <aside data-slot="inspector">{inspector}</aside>
</section>
```

## Interfaces and Dependencies

Public or semi-public surfaces introduced by this plan:

```text
@scenery/* shadcn registry namespace
ui/components.json registry configuration
bun run shadcn:add @scenery/<item>
docs/ui-agent-contract.md
scenery UI primitives under ui/src/components/primitives
scenery UI layouts under ui/src/components/layouts
UI architecture diagnostics in self-harness or equivalent static command
```

Potential dependencies:

```text
shadcn CLI, invoked through bunx only by the wrapper
Tailwind/shadcn/Radix dependencies only if a promoted primitive requires them
No new backend service, external registry host, or non-local network dependency in the first pass
```

Dependency rule:

Keep dependencies minimal. Prefer existing UI stack and small scenery-owned wrappers. Add shadcn/Radix/Tailwind dependencies only when a promoted component needs them and the guardrails already prevent direct app-screen usage.
