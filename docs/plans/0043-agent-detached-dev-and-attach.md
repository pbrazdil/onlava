# Agent Detached Dev and Attach

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

the agent-native local-dev ExecPlan series makes `scenery dev` an agent client and defines an attached/detached lifecycle:

```text
scenery dev              # attached; Ctrl-C stops session
scenery dev --detach     # start session and return
scenery attach           # attach to logs
scenery down             # stop current session
```

The current agent-native local-dev implementation has agent sessions, routed URLs, `scenery status`, `scenery down`, and session-scoped logs, but `scenery dev` is still only an attached foreground process and there is no `scenery attach` command.

After this work, `scenery dev --detach` starts a normal dev supervisor as a background agent-owned session, returns after that session is registered, and prints enough information for tools and humans to inspect or attach. `scenery attach` follows the current session's logs by default and can target a specific session. `scenery down` remains the stopping surface by signalling the detached session owner PID recorded in the agent registry.

## Progress

* [x] 2026-05-27: Create this ExecPlan and link it from `docs/plans/active.md`.
* [x] 2026-05-27: Implement `scenery dev --detach` argument parsing, detached child spawning, startup polling, and output.
* [x] 2026-05-27: Make detached child processes survive the launcher returning without disabling normal parent-death cleanup for attached dev sessions.
* [x] 2026-05-27: Add `scenery attach` as the current-contract log attachment command.
* [x] 2026-05-27: Update command usage, local contract docs, README command list, and tests.
* [x] 2026-05-27: Run focused tests and full repository tests.
* [x] 2026-05-27: Run binary install, self harness, and diff checks.

## Surprises & Discoveries

Record implementation findings here with commands, test output, or file references.

* 2026-05-27: The detached child must not receive the original relative `--app-root` after the parent changes the child working directory. The launcher now rewrites child args to use the discovered absolute app root.

* 2026-05-27: The existing parent monitor is still correct for attached `scenery dev`, but detached children need a narrowly-scoped environment marker so they survive the launcher returning.

## Decision Log

* Decision: Make detached dev require the local agent.
  Rationale: The agent lifecycle is agent-owned. A detached process without an agent session has no stable route, owner PID record, or reliable `down`/`attach` target.
  Date/Author: 2026-05-27 / Codex

* Decision: Implement `scenery attach` on top of the existing session-scoped log store.
  Rationale: The log store already carries app id, session id, pid, stream, output, and JSONL formatting. Reusing it gives a stable machine-readable attachment surface without inventing terminal multiplexing in this milestone.
  Date/Author: 2026-05-27 / Codex

## Outcomes & Retrospective

Completed on 2026-05-27.

Shipped outcome:

* Added `scenery dev --detach`, which requires the local agent, starts a background dev child, redirects supervisor stdout/stderr to an agent log file, waits for that child PID to register as the app root's agent session owner, and returns human or JSON startup information.
* Detached children run the same `scenery dev` supervisor path but skip parent-process monitoring through a private environment marker, so the launcher can exit without stopping the session.
* Added `scenery attach`, which follows the current session's logs by default and supports the same app-root, session, limit, stream, and JSONL options as `scenery logs`.
* Updated CLI usage, README, local contract docs, and tests.

Validation:

```sh
go test ./cmd/scenery ./internal/agent ./internal/devdash
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
git diff --check
```

All validation commands passed. The self harness wrote `.scenery/harness/self-latest.json` and reported existing review-due and large-file warnings, but no errors.

## Context and Orientation

Relevant files:

```text
docs/plans/0037-scenery-agent-mvp.md
cmd/scenery/main.go
cmd/scenery/watch.go
cmd/scenery/logs.go
cmd/scenery/agent.go
cmd/scenery/process_*.go
internal/agent/*
internal/devdash/*
docs/local-contract.md
README.md
```

Current state:

* `scenery dev` starts an attached foreground watcher/supervisor.
* `runWithWatch` registers an agent session before building and records the `OwnerPID` used by `scenery down`.
* `startParentMonitor` cancels attached dev sessions when their invoking parent process exits.
* `scenery logs --session current --follow` can already follow the current session's process output.

## Milestones

Milestone 1 adds the CLI surface and detached process launcher while preserving the existing attached path.

Milestone 2 teaches the detached child path to skip parent-process monitoring while keeping attached cleanup unchanged.

Milestone 3 adds `scenery attach` as a focused alias over session log following and documents the lifecycle.

Milestone 4 validates process/session behavior with focused tests plus the normal repository gates.

## Plan of Work

Start with the smallest agent-native lifecycle that satisfies agent-native local-dev: detached mode spawns the same `scenery dev` implementation in a background child, with an environment marker that only affects parent monitoring. The parent waits for the child PID to appear as the owner of an agent session for the app root, then returns a concise human or JSON result. Keep logs in the existing dashboard store so `scenery attach` and `scenery logs` share output semantics.

## Concrete Steps

1. Extend `devOptions` and `parseDevArgs` with `--detach`.
2. Add a detached dev launcher that filters `--detach` out of the child args, sets a private child marker env var, redirects child stdio to an agent log file, starts the child with detached process attributes, and waits for an agent session owned by the child PID.
3. Make `runWithWatch` skip `startParentMonitor` only when the detached child marker is present.
4. Add `scenery attach` to the command switch and usage text, defaulting to `--session current --follow`.
5. Add unit tests for parsing, command dispatch, child arg filtering, parent monitor behavior, attach argument translation, and detached startup polling.
6. Update docs and mark the plan complete after validation.

## Validation and Acceptance

Expected validation:

```sh
go test ./cmd/scenery ./internal/agent ./internal/devdash
go test ./...
go install ./cmd/scenery
scenery harness self --json --write
git diff --check
```

Observable behavior:

* `scenery dev --detach` starts a background session and returns after the agent registry has that child PID as owner.
* `scenery dev` without `--detach` remains attached and Ctrl-C still stops the session.
* A detached child survives the launcher returning.
* `scenery attach` follows the current session's logs by default.
* `scenery down` stops a detached session through the existing agent owner-PID signal path.

## Idempotence and Recovery

If startup polling times out or the child exits before session registration, the launcher should try to interrupt the child and return the detached child log path. A subsequent `scenery dev --detach` can retry normally. `scenery attach` should not mutate session state; it only reads from the log store.

## Artifacts and Notes

Expected changed artifacts:

```text
cmd/scenery/main.go
cmd/scenery/watch.go
cmd/scenery/logs.go
cmd/scenery/dev_detach.go
cmd/scenery/*test.go
docs/local-contract.md
README.md
docs/plans/0043-agent-detached-dev-and-attach.md
```

## Interfaces and Dependencies

No new external dependencies expected.

New or clarified CLI:

```text
scenery dev --detach [--app-root <path>] [--json] [other dev flags]
scenery attach [--app-root <path>] [--session current|<id>] [--limit <n>] [--stream all|stdout|stderr] [--jsonl|--json]
```
