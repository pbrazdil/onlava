# Active Plans

This file tracks active or near-term plans that affect implementation choices.

## Active ExecPlans

- [Split `pulse dev` From Headless `pulse run`](devrun-command-split.md): turn the PRD-4 command split into an executable implementation plan.

## Agent-Friendly Local Runtime

- Status: active
- Owner: Pulse runtime
- Last reviewed: 2026-04-27
- Review after: 2026-05-27
- Quality: B

Current focus:

- Keep expanding stable JSON surfaces instead of requiring agents to scrape terminal output or dashboard DOM.
- Add harness checks only when they enforce a real project invariant.
- Keep dependency growth intentional and documented.

## Dashboard Source Parity

- Status: active
- Owner: Pulse dashboard
- Last reviewed: 2026-04-27
- Review after: 2026-05-27
- Quality: B

Current focus:

- Maintain editable source dashboard behavior under `ui/`.
- Keep supported local-only surfaces first: API Explorer, traces, Pub/Sub, DB explorer, cron, service metadata.
- Avoid reintroducing cloud, Clerk, deploy, or marketing surfaces.

## Runtime Compatibility

- Status: active
- Owner: Pulse runtime
- Last reviewed: 2026-04-27
- Review after: 2026-05-27
- Quality: B

Current focus:

- Preserve common Encore-compatible behavior only where it helps migration.
- Prefer Pulse-native naming and contracts.
- Keep generated artifacts deterministic and inspectable.
