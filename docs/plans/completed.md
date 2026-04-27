# Completed Plans

This file records completed milestones so agents can distinguish shipped behavior from future intent.

## Pulse Go Runner Phase 1

- Status: completed
- Owner: Pulse runtime
- Completed: 2026-04-27
- Quality: B

Shipped:

- `pulse run`, `pulse build`, `pulse test`, `pulse check`, `pulse logs`, `pulse psql`
- Pulse API parser/codegen/runtime for common Go service behavior
- Secrets from `.env`
- local HTTPS proxy support
- cron, middleware, Pub/Sub, tracing, logging, DB query tracing, and dashboard support

## Stable Inspect And Harness Contracts

- Status: completed
- Owner: Pulse runtime
- Completed: 2026-04-27
- Quality: A

Shipped:

- `pulse inspect app|routes|services|endpoints|wire|build|paths --json`
- `pulse inspect traces|metrics|docs --json`
- `.pulse/gen/*` and `.pulse/build/latest.json`
- `pulse harness --json --write`
- `pulse harness self --json --write`

## Queryable Observability

- Status: completed
- Owner: Pulse observability
- Completed: 2026-04-27
- Quality: B

Shipped:

- Trace query filters for service, endpoint, trace ID, status, duration, time window, and sort order.
- Metrics rollups by service and endpoint.
- Log-level counts and trace event counts from the dashboard SQLite store.
