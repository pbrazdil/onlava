# Pulse Documentation Index

This is the human entry point for Pulse's local knowledge base.

For agents, the machine-readable source of truth is [knowledge.json](knowledge.json). Validate it with:

```text
pulse inspect docs --json
pulse harness self --json
```

## Core Contracts

- [Architecture](../ARCHITECTURE.md): high-level repo map, boundaries, and architectural invariants.
- [Local Contract](local-contract.md): CLI grammar, stable JSON schemas, generated artifacts, and local runtime contracts.
- [Harness Engineering](harness-engineering.md): agent validation loop, harness outputs, and self-harness expectations.
- [Execution Plan Standard](../PLANS.md): required structure for long-running agent-executable implementation plans.

## Product Plans

- [Root Plan](../PLAN.md): current agent-first implementation plan inspired by OpenAI's harness engineering article.
- [Active Plans](plans/active.md): planned or in-progress work that agents should consider when editing the repo.
- [Completed Plans](plans/completed.md): shipped milestones and acceptance notes.
- [Tech Debt](tech-debt.md): known cleanup, risk, and follow-up items.

## PRDs

- [Pub/Sub PRD](PRD-1-pubsub.md): embedded NATS-backed Pub/Sub behavior and developer API.
- [Proto/Wire PRD](PRD-2-proto.md): binary wire and generated transport plan.
- [Release Readiness PRD](PRD-3-release.md): audit and recommendations for freezing a smaller production-ready v0.
- [Dev/Run Command Split PRD](PRD-4-devrun.md): product direction for `pulse dev`, headless `pulse run`, and `pulse build`.

## Schemas

JSON schemas live in [schemas/](schemas/). They are part of the local agent contract and must stay in sync with CLI output.

Start with:

- [pulse.app.v1](schemas/pulse.app.v1.schema.json)
- [pulse.check.result.v1](schemas/pulse.check.result.v1.schema.json)
- [pulse.harness.result.v1](schemas/pulse.harness.result.v1.schema.json)
- [pulse.harness.self.v1](schemas/pulse.harness.self.v1.schema.json)
- [pulse.inspect.docs.v1](schemas/pulse.inspect.docs.v1.schema.json)
- [pulse.docs.index.v1](schemas/pulse.docs.index.v1.schema.json)
- [pulse.version.v1](schemas/pulse.version.v1.schema.json)
