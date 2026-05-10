# ONLV Direct onlava Registry Adoption

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

ONLV app should use onlava-owned UI registry items directly, not local ONLV app-owned copies or broad re-export shims, while preserving the current ONLV app design exactly.

The target source-of-truth flow is:

```text
onlava ui/src/components/primitives + layouts + registry source
        |
        v
onlava ui/registry/onlava @onlava/* items
        |
        v
ONLV apps/app installs or mirrors approved @onlava registry outputs
        |
        v
ONLV app screens import onlava-facing primitives/layouts directly
        |
        v
visual output intentionally unchanged
```

This is not a redesign and not a rewrite of ONLV app UI. The goal is to make the semantics and guardrails different while keeping the pixels the same. Existing app app-specific logic, data fetching, copy, and workflows stay in ONLV. Generic UI building blocks move to the `@onlava/*` registry and are consumed from there.

This plan builds on `0026 ONLV Layout Migration into onlava`, but it narrows the next work to direct registry adoption in `onlv/apps/app`.

## Progress

* [x] 2026-05-10: Create this ExecPlan as `docs/plans/0031-onlv-direct-onlava-registry-adoption.md`.
* [x] 2026-05-10: Link this ExecPlan from `docs/plans/active.md`.
* [x] 2026-05-10: Audit remaining ONLV generic primitives, layouts, and local shadcn wrappers.
* [x] 2026-05-10: Add missing generic ONLV elements to onlava registry source under `ui/src/components/registry/primitives`.
* [x] 2026-05-10: Add missing `@onlava/*` registry items under `ui/registry/onlava`.
* [x] 2026-05-10: Mirror registry outputs into `onlv/apps/app`.
* [x] 2026-05-10: Update ONLV app imports to direct onlava-facing registry outputs.
* [x] 2026-05-10: Remove broad local primitive re-export shims that hide raw `components/ui` usage.
* [x] 2026-05-10: Add static guardrails so ONLV screens cannot drift back to local raw shadcn usage.
* [x] 2026-05-10: Run onlava validation.
* [x] 2026-05-10: Run ONLV validation and visual harness.
* [x] 2026-05-10: Record outcomes and move this plan to completed.

## Surprises & Discoveries

Record discoveries here as work proceeds.

Known starting discoveries:

* ONLV app already has `apps/app/components.json` configured with an `@onlava` registry namespace at `http://127.0.0.1:4873/r/{name}.json`.
* ONLV app already exposes `bun run shadcn:add`, wired to onlava's guarded wrapper with `ONLAVA_SHADCN_REGISTRY_ROOT=../../../onlava/ui/registry/onlava`.
* onlava's current registry includes `button`, `dialog`, `card`, `input`, `dashboard-page`, `data-explorer-layout`, `page-toolbar`, `app-surface`, `filter-pill`, and `sidebar-item`.
* ONLV app still has `apps/app/src/components/primitives/index.ts` re-exporting `../ui`, which means screens can appear to use the primitive surface while still depending on the entire local raw shadcn component set.
* ONLV app has a generic `SidebarSectionHeader` that is not currently present as an onlava registry item.
* ONLV app still has `apps/app/src/components/app/*` compatibility shims for product layout, filter pill, sidebar item, and sidebar section header.
* ONLV app has many local `apps/app/src/components/ui/*` shadcn-derived components. Not all should become onlava primitives immediately; this plan should migrate the generic elements that ONLV app screens actually use as app-facing primitives.
* `0026` recorded unresolved ONLV app visual harness diffs. Do not update visual baselines in this plan without explicit approval.

Implementation discoveries:

* ONLV app-facing code imports 55 generic primitive/component files either through `@/components/primitives` or direct primitive paths. Moving only `SidebarSectionHeader` would not be enough to remove the broad `../ui` barrel.
* The safest visual-preserving move was to add a dedicated onlava registry source tree at `ui/src/components/registry/primitives`, excluded from the dashboard TypeScript project but still validated by the onlava UI static harness.
* `@onlava/primitives --dry-run` reports 10 overwrite candidates even though `diff -qr` shows the mirrored registry source and ONLV primitive output are identical. Individual dry runs for `@onlava/button` and `@onlava/sidebar-section-header` report identical skips.
* The ONLV app visual harness passed all 24 snapshots after the migration.
* Vite's dev module graph can keep requesting the old public `.ts` primitive entrypoints for `filter-pill`, `sidebar-item`, and `sidebar-section-header` after the migration. Keeping tiny registry-owned `.ts` re-export entrypoints avoids dev-time 404s while preserving the `.tsx` implementations.

## Decision Log

* Decision: Use `@onlava/*` registry items as the source of truth for shared primitives and layouts consumed by ONLV app.
  Rationale: The user wants ONLV to use `registry/onlava` directly, with onlava owning the generic UI contract.
  Date/Author: 2026-05-10 / Codex

* Decision: Preserve ONLV app visual output during registry adoption.
  Rationale: This is a source and guardrail migration, not a design change.
  Date/Author: 2026-05-10 / Codex

* Decision: Do not move app-specific ONLV app components into onlava.
  Rationale: onlava should own generic UI contracts; ONLV should keep business-specific behavior, copy, data loading, and product workflows.
  Date/Author: 2026-05-10 / Codex

* Decision: Remove broad `../ui` re-export from `apps/app/src/components/primitives/index.ts` only after equivalent registry-owned exports exist.
  Rationale: The broad re-export hides raw shadcn dependencies. Removing it too early would create noisy breakage and increase visual risk.
  Date/Author: 2026-05-10 / Codex

* Decision: Keep onlava-approved shadcn-derived registry source under `ui/src/components/registry/primitives` instead of importing those files into the onlava dashboard bundle.
  Rationale: These files are a registry/source-generator contract for downstream apps with the required Radix/shadcn dependencies; the onlava dashboard should not gain that dependency surface or bundle weight just to host registry source.
  Date/Author: 2026-05-10 / Codex

* Decision: Add a full `@onlava/primitives` registry item in addition to individual primitive items.
  Rationale: ONLV can mirror the whole approved primitive surface in one guarded operation, while individual `@onlava/<item>` installs still work for narrower consumers.
  Date/Author: 2026-05-10 / Codex

## Outcomes & Retrospective

Completed on 2026-05-10.

Shipped:

* Added 55 onlava-approved onlava primitive registry source files under `ui/src/components/registry/primitives`.
* Added or updated 56 primitive registry items plus `@onlava/primitives` and refreshed `ui/registry/onlava/registry.json`.
* Mirrored the registry output into `/Users/petrbrazdil/Repos/onlv/apps/app/src/components/primitives`.
* Replaced ONLV app-facing imports from `@/components/ui/*` and `@/components/layouts/product-layout` with registry-owned `@/components/primitives/*` and `@/components/layouts/AppSurface`.
* Removed unused ONLV app compatibility shims for product layout, filter pill, sidebar item, and sidebar section header.
* Removed the old local `apps/app/src/components/ui` source tree after consumers were migrated, and pointed `components.json` `aliases.ui` at the vendor layer.
* Added registry-owned `.ts` public entrypoints for `filter-pill`, `sidebar-item`, and `sidebar-section-header` so existing Vite dev import URLs continue to resolve.
* Replaced the broad ONLV primitive `../ui` re-export with an explicit registry-owned barrel.
* Added `apps/app/scripts/check-onlava-ui-registry.mjs` and wired it into `bun run typecheck`.
* Updated ONLV app agent instructions to point agents at `@onlava` registry-owned primitives/layouts instead of `components/ui` or ONLV app shims.
* Adjusted onlava UI static checks so registry source files can contain the low-level Radix/shadcn imports they are explicitly meant to wrap.
* Fixed a pre-existing dashboard type-narrowing error in `ui/src/components/layout.tsx` so `onlava harness self --json --write` can pass.

Validation:

* `go test ./...` in onlava: passed.
* `cd /path/to/onlava/ui && bun run typecheck && bun run test && bun run build`: passed.
* `onlava harness self --json --write`: passed.
* `cd /Users/petrbrazdil/Repos/onlv && onlava check --json`: passed.
* `cd /Users/petrbrazdil/Repos/onlv && go test ./...`: passed.
* `cd /Users/petrbrazdil/Repos/onlv/apps/app && bun run typecheck && bun run build`: passed.
* `cd /Users/petrbrazdil/Repos/onlv/apps/app && bun run ui-harness`: 24/24 snapshots passed.

## Context and Orientation

Relevant onlava files:

```text
docs/plans/0026-onlv-layout-migration.md
docs/ui-agent-contract.md
ui/components.json
ui/scripts/onlava-shadcn.mjs
ui/registry/onlava/registry.json
ui/registry/onlava/*.json
ui/src/components/registry/primitives/*
ui/src/components/primitives/*
ui/src/components/layouts/*
ui/src/components/layouts/index.ts
```

Relevant ONLV files:

```text
/Users/petrbrazdil/Repos/onlv/apps/app/AGENTS.md
/Users/petrbrazdil/Repos/onlv/apps/app/components.json
/Users/petrbrazdil/Repos/onlv/apps/app/package.json
/Users/petrbrazdil/Repos/onlv/apps/app/src/components/primitives/index.ts
/Users/petrbrazdil/Repos/onlv/apps/app/src/components/primitives/*
/Users/petrbrazdil/Repos/onlv/apps/app/src/components/layouts/*
/Users/petrbrazdil/Repos/onlv/apps/app/src/components/app/*
/Users/petrbrazdil/Repos/onlv/apps/app/src/components/ui/*
/Users/petrbrazdil/Repos/onlv/apps/app/src/pages/**/*
```

Known existing registry items:

```text
@onlava/button
@onlava/dialog
@onlava/card
@onlava/input
@onlava/dashboard-page
@onlava/data-explorer-layout
@onlava/page-toolbar
@onlava/app-surface
@onlava/filter-pill
@onlava/sidebar-item
```

Likely missing or incomplete registry candidates to audit:

```text
sidebar-section-header
button-group
empty-state / empty
scroll-area
separator
badge
tabs
tooltip
dropdown-menu
popover
sheet
table
skeleton
spinner
resizable
command
primitive barrel/index
layout barrel/index
```

Do not add all of these blindly. Add the items required to remove broad `../ui` re-exports and move ONLV app screens to a direct onlava-facing surface without visual drift.

## Scope

In onlava:

```text
add missing generic primitive/layout source files under ui/src/components
add registry item JSON under ui/registry/onlava
update ui/registry/onlava/registry.json
add render tests for new primitives/layouts where practical
update docs/ui-agent-contract.md only if the agent contract changes
```

In ONLV app:

```text
install or mirror @onlava registry outputs into apps/app/src/components/primitives and apps/app/src/components/layouts
update imports from local ONLV app shims or raw ui wrappers to onlava-facing primitive/layout files
remove broad primitive barrel re-export of ../ui
keep app-specific feature components in ONLV
preserve current visual output
```

Non-goals:

```text
visual redesign
renaming app product copy
moving ONLV data/state/business logic into onlava
porting every shadcn component in apps/app/src/components/ui at once
deleting local ui wrappers before all consumers are migrated
updating visual baselines without approval
adding new external dependencies
```

## Milestones

### Milestone 1: Inventory remaining ONLV generic UI usage

Build a concrete inventory of ONLV app imports and usages.

Commands:

```sh
cd /Users/petrbrazdil/Repos/onlv
rg '@/components/(ui|primitives|layouts|app)' apps/app/src -g'*.tsx' -g'*.ts'
rg 'from "@/components/primitives"|from "@/components/layouts"|from "@/components/app"|from "@/components/ui"' apps/app/src -g'*.tsx' -g'*.ts'
```

Acceptance:

```text
- inventory lists each generic primitive/layout still sourced locally
- inventory distinguishes generic UI from app-specific ONLV app components
- inventory identifies which current @onlava item should own each generic UI element
- inventory identifies missing registry items
```

### Milestone 2: Complete onlava registry coverage for required generic UI

Add missing onlava-owned primitive/layout source files and registry item JSON only for audited needs.

Acceptance:

```text
- every required ONLV generic UI element has an @onlava registry item
- registry item files target approved aliases under @components/primitives or @components/layouts
- registry dependencies use only @onlava/* where possible
- onlava UI static checks pass
```

### Milestone 3: Install registry outputs into ONLV app

Use the existing guarded wrapper from `apps/app`:

```sh
cd /Users/petrbrazdil/Repos/onlv/apps/app
bun run shadcn:add @onlava/<item>
```

If the wrapper detects files already identical, record that in this plan. If generated output differs, inspect every diff before accepting it.

Acceptance:

```text
- ONLV files under src/components/primitives and src/components/layouts match registry output or have documented ONLV app-only adapters
- no direct raw shadcn install command is used
- no registry URL install is used
```

### Milestone 4: Replace imports with direct onlava-facing surfaces

Update ONLV app screens to import from registry-owned primitive/layout files or barrels, not from `components/app` compatibility shims or raw `components/ui`.

Preferred imports:

```ts
import { Button } from "@/components/primitives/Button";
import { AppMain } from "@/components/layouts/AppSurface";
```

Barrel imports are acceptable only if the barrel itself is registry-owned and no longer re-exports all of `../ui`.

Acceptance:

```text
- no app screen imports generic layout primitives from @/components/app/*
- no app screen imports raw shadcn wrappers from @/components/ui/*
- @/components/primitives/index.ts no longer re-exports ../ui wholesale
- visible ONLV app UI labels remain unchanged
```

### Milestone 5: Add ONLV guardrails

Add lightweight static checks to ONLV app so future agents cannot drift back to raw local shadcn usage.

Possible implementation:

```text
apps/app/scripts/check-onlava-ui-registry.mjs
package.json script ui:registry-check
```

Checks:

```text
disallow raw shadcn add scripts except guarded shadcn:add
disallow route/page imports from @/components/ui/*
disallow product screens importing generic layouts from @/components/app/*
warn or fail on primitives/index.ts re-exporting ../ui
```

Acceptance:

```text
- ONLV guardrail script passes
- violations point to file and import
- script is included in ONLV app validation or documented in AGENTS.md
```

### Milestone 6: Visual validation

Run ONLV app visual harness without intentional design changes.

Commands:

```sh
cd /Users/petrbrazdil/Repos/onlv/apps/app
bun run typecheck
bun run build
bun run ui-harness
```

Acceptance:

```text
- no new visual diffs are introduced by registry adoption
- if existing 0026 diffs remain, separate old diffs from new diffs
- visual baselines are not updated without explicit approval
```

## Plan of Work

Start with inventory, not code movement. The most important risk is accidentally turning "use registry" into a visual redesign or a broad shadcn rewrite. Treat each migrated component as a source-of-truth move:

```text
1. identify current ONLV app source and consumers
2. add/verify equivalent onlava source
3. add/verify registry item
4. install into ONLV app
5. update imports
6. run typecheck/build/visual harness
```

For registry outputs that already exist, prefer installing and verifying identity rather than rewriting ONLV files by hand.

For items not yet in the registry, port the current vetted ONLV app implementation into onlava with onlava-owned naming. Preserve class names and semantic tokens unless a separate visual/design change is approved.

## Concrete Steps

1. In onlava, inspect `ui/registry/onlava` and current primitive/layout files.

2. In ONLV, inventory current imports:

   ```sh
   cd /Users/petrbrazdil/Repos/onlv
   rg '@/components/(ui|primitives|layouts|app)' apps/app/src -g'*.tsx' -g'*.ts'
   ```

3. Fill an inventory table in this plan.

4. For each missing generic primitive, add an onlava registry source file under:

   ```text
   ui/src/components/registry/primitives
   ```

5. For each missing generic layout, add an onlava source file under:

   ```text
   ui/src/components/layouts
   ```

6. Add or update registry items:

   ```text
   ui/registry/onlava/<item>.json
   ui/registry/onlava/registry.json
   ```

7. Add tests for new onlava components where practical.

8. Install or mirror registry items into:

   ```text
   /Users/petrbrazdil/Repos/onlv/apps/app/src/components/primitives
   /Users/petrbrazdil/Repos/onlv/apps/app/src/components/layouts
   ```

9. Replace ONLV imports to direct onlava-facing files/barrels.

10. Remove or narrow compatibility shims only after imports are migrated.

11. Add ONLV static guardrail script.

12. Run onlava validation.

13. Run ONLV validation and visual harness.

14. Update this plan's Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective.

## Validation and Acceptance

onlava validation:

```sh
cd /path/to/onlava
go test ./...
go install ./cmd/onlava
cd ui && bun run typecheck && bun run test && bun run build
cd ..
onlava harness self --json --write
```

ONLV validation:

```sh
cd /Users/petrbrazdil/Repos/onlv
onlava check --json
go test ./...
cd apps/app
bun run typecheck
bun run build
bun run ui-harness
```

Registry validation:

```sh
cd /Users/petrbrazdil/Repos/onlv/apps/app
bun run shadcn:add @onlava/app-surface --dry-run
bun run shadcn:add @onlava/button --dry-run
bun run shadcn:add @onlava/filter-pill --dry-run
```

Acceptance criteria:

```text
- ONLV app uses @onlava registry-owned primitives/layouts directly for migrated generic UI.
- ONLV app visual output is unchanged.
- App-specific ONLV app logic remains in ONLV.
- No broad primitives barrel re-export of ../ui remains.
- No ONLV app screen imports generic UI directly from @/components/ui/*.
- No ONLV app screen imports generic layout primitives from @/components/app/*.
- onlava and ONLV validations pass, with any pre-existing 0026 visual harness diffs clearly separated.
```

## Idempotence and Recovery

Migrate one component family at a time. Keep compatibility shims until direct imports and visual harness results are understood.

If a registry install changes visual output:

```text
stop
compare old and new source
compare class names and CSS tokens
restore the old ONLV app output or port the missing token/class exactly into onlava
rerun visual harness
```

Do not update visual baselines without explicit approval.

If ONLV cannot consume a registry item directly because the item target does not match ONLV aliases, fix the registry target or add an onlava-owned registry item for the needed adapter. Do not hand-edit generated files into a divergent copy without recording the reason.

## Artifacts and Notes

Expected onlava artifacts:

```text
ui/src/components/primitives/<NewPrimitive>.tsx
ui/src/components/registry/primitives/<new-primitive>.tsx
ui/src/components/layouts/<NewLayout>.tsx
ui/registry/onlava/<new-item>.json
ui/registry/onlava/registry.json
ui/src/components/**/*.test.tsx
docs/plans/0031-onlv-direct-onlava-registry-adoption.md
```

Expected ONLV artifacts:

```text
apps/app/src/components/primitives/*
apps/app/src/components/layouts/*
apps/app/src/components/app/* shims removed or narrowed
apps/app/src/pages/**/* imports updated
apps/app/scripts/check-onlava-ui-registry.mjs, if added
apps/app/package.json script updates, if added
apps/app/test-results/ui-harness/diff-report.md, only if diffs occur
```

## Interfaces and Dependencies

No new external dependencies are expected.

Use existing interfaces:

```text
@onlava/* shadcn registry namespace
ui/scripts/onlava-shadcn.mjs guarded wrapper
apps/app/components.json registries.@onlava
apps/app package script shadcn:add
docs/ui-agent-contract.md
apps/app ui-harness
```

## Inventory Table

Fill this during Milestone 1.

| ONLV source | Current element | Generic? | Current registry item | Action | Visual risk |
| --- | --- | ---: | --- | --- | --- |
| `apps/app/src/components/layouts/AppSurface.tsx` | `AppSidebar`, `AppMain`, `AppHeader`, `AppToolbar`, `AppPanel`, `AppMetaBox` | yes | `@onlava/app-surface` | verify installed output and update imports to direct layout file | low |
| `apps/app/src/components/primitives/filter-pill.tsx` | `FilterPill` | yes | `@onlava/filter-pill` | mirrored from registry output | low |
| `apps/app/src/components/primitives/sidebar-item.tsx` | `SidebarItem*` helpers | yes | `@onlava/sidebar-item` | mirrored from registry output | low |
| `apps/app/src/components/primitives/sidebar-section-header.tsx` | `SidebarSectionHeader` | yes | `@onlava/sidebar-section-header` | ported to onlava registry and mirrored | low |
| `apps/app/src/components/primitives/index.ts` | explicit primitive barrel | yes | `@onlava/primitives` | replaced `export * from "../ui"` with explicit registry-owned exports | medium |
| `apps/app/src/components/ui/*` | raw shadcn-derived wrappers | mixed | `@onlava/<item>` plus `@onlava/primitives` | app-facing generic wrappers mirrored into primitives; legacy source kept for now | medium |
